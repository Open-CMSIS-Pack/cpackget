/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lu4p/cat"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/ui"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	log "github.com/sirupsen/logrus"
)

// PackType is the struct that represents the installation of a
// single pack
type PackType struct {
	xml.PdscTag

	// isPublic tells whether the pack exists in the public index or not
	isPublic bool

	// isInstalled tells whether the pack is already installed
	isInstalled bool

	// isDownloaded tells whether the file needed to be downloaded from a server
	isDownloaded bool

	// isPackID tells whether the path is in packID format: Vendor.PackName[.x.y.z]
	isPackID bool

	// path points to a file in the local system, whether or not it's local
	path string

	// pdsc holds a pointer to the PDSC file already parsed as XML
	Pdsc *xml.PdscXML

	// zipReader holds a pointer to the uncompress pack file
	zipReader *zip.ReadCloser
}

// preparePack does some sanity validation regarding pack name
// and check if it's public and if it's installed or not
func preparePack(packPath string) (*PackType, error) {
	pack := &PackType{
		path: packPath,
	}

	var shortPath bool

	// Clean out any possible query or user auth in the URL
	// to help finding the correct path info
	if strings.HasPrefix(packPath, "http") {
		url, err := url.Parse(packPath)
		if err != nil {
			log.Error(err)
			return pack, errs.ErrBadPackURL
		}

		url.User = nil
		url.Fragment = ""
		url.RawQuery = ""

		packPath = url.String()
		shortPath = false
	} else if !strings.HasSuffix(packPath, ".pack") && !strings.HasSuffix(packPath, ".zip") {
		pack.isPackID = true
		shortPath = true
	}

	info, err := utils.ExtractPackInfo(packPath, shortPath)
	if err != nil {
		return pack, err
	}

	pack.URL = info.Location
	pack.Name = info.Pack
	pack.Vendor = info.Vendor
	pack.Version = info.Version
	pack.isPublic = Installation.packIsPublic(pack)
	pack.isInstalled = Installation.PackIsInstalled(pack)

	return pack, nil
}

// fetch will download the pack file if it's on the Internet, or
// will use the one in .Download/ if previously downloaded.
// If the path is not a URL, it will make sure the file exists in the local file system
func (p *PackType) fetch() error {
	log.Debugf("Fetching pack file \"%s\" (or just making sure it exists locally)", p.path)
	var err error
	if strings.HasPrefix(p.path, "http") {
		p.path, err = utils.DownloadFile(p.path)
		p.isDownloaded = true
		return err
	}

	if !utils.FileExists(p.path) {
		log.Errorf("File \"%s\" does't exist", p.path)
		return errs.ErrFileNotFound
	}

	return nil
}

// validate ensures the pack is legit and it has all minimal requrements
// to be installed.
func (p *PackType) validate() error {
	log.Debug("Validating pack")
	pdscFileName := fmt.Sprintf("%s.%s.pdsc", p.Vendor, p.Name)
	for _, file := range p.zipReader.File {
		if filepath.Base(file.Name) == pdscFileName {

			// Read pack's pdsc
			tmpPdscFileName := utils.RandStringBytes(10)
			defer os.RemoveAll(tmpPdscFileName)

			if err := utils.SecureInflateFile(file, tmpPdscFileName); err != nil {
				return err
			}

			p.Pdsc = xml.NewPdscXML(filepath.Join(tmpPdscFileName, file.Name)) // #nosec
			if err := p.Pdsc.Read(); err != nil {
				return err
			}

			p.Pdsc.FileName = file.Name
			return nil
		}
	}

	log.Errorf("\"%s\" not found in \"%s\"", pdscFileName, p.path)
	return errs.ErrPdscFileNotFound
}

// purge Removes cached files when
// - It
//   - Removes "CMSIS_PACK_ROOT/.Download/p.Vendor.p.Name.p.Version.pdsc"
//   - Removes "CMSIS_PACK_ROOT/.Download/p.Vendor.p.Name.p.Version.pack" (or zip)
func (p *PackType) purge() error {
	log.Debugf("Purging \"%v\"", p.path)

	fileNamePattern := p.Vendor + "\\." + p.Name
	if len(p.Version) > 0 {
		fileNamePattern += "\\." + p.Version
	} else {
		fileNamePattern += "\\..*?"
	}
	fileNamePattern += "\\.(?:pack|zip|pdsc)"

	files, err := utils.ListDir(Installation.DownloadDir, fileNamePattern)
	if err != nil {
		return err
	}

	log.Debugf("Files to be purged \"%v\"", files)
	if len(files) == 0 {
		return errs.ErrPackNotPurgeable
	}

	for _, file := range files {
		if err := os.Remove(file); err != nil {
			return err
		}
	}

	return nil
}

// install installs pack files to installation's directories
// It:
//   - Extracts all files to "CMSIS_PACK_ROOT/p.Vendor/p.Name/p.Version/"
//   - Saves a copy of the pack in "CMSIS_PACK_ROOT/.Download/"
//   - Saves a versioned pdsc file in "CMSIS_PACK_ROOT/.Download/"
//   - If "CMSIS_PACK_ROOT/.Web/p.Vendor.p.Name.pdsc" does not exist then
//     - Save an unversioned copy of the pdsc file in "CMSIS_PACK_ROOT/.Local/"
func (p *PackType) install(installation *PacksInstallationType, checkEula bool) error {
	log.Debugf("Installing \"%s\"", p.path)

	var err error
	p.zipReader, err = zip.OpenReader(p.path)
	if err != nil {
		log.Errorf("Can't decompress \"%s\": %s", p.path, err)
		return errs.ErrFailedDecompressingFile
	}
	defer p.zipReader.Close()

	if err = p.validate(); err != nil {
		return err
	}

	packHomeDir := filepath.Join(Installation.PackRoot, p.Vendor, p.Name, p.Version)

	if len(p.Pdsc.License) > 0 {
		if checkEula {
			ok, err := p.checkEula()
			if err != nil {
				if err == errs.ErrExtractEula {
					return p.extractEula()
				}
				return err
			}

			if !ok {
				return errs.ErrEula
			}
		} else {
			// Explicitly inform the user that license has been agreed
			fmt.Printf("Agreed to embedded license: %v", filepath.Join(packHomeDir, p.Pdsc.License))
			fmt.Println()
		}
	} else if ui.Extract {
		return errs.ErrLicenseNotFound
	}

	// Inflate all files
	err = utils.EnsureDir(packHomeDir)
	if err != nil {
		log.Errorf("Can't access pack directory \"%s\": %s", packHomeDir, err)
		return err
	}

	log.Debugf("Extracting files from \"%s\" to \"%s\"", p.path, packHomeDir)
	for _, file := range p.zipReader.File {
		err = utils.SecureInflateFile(file, packHomeDir)
		if err != nil {
			return err
		}
	}

	pdscFileName := fmt.Sprintf("%s.%s.pdsc", p.Vendor, p.Name)
	pdscFilePath := filepath.Join(packHomeDir, p.Pdsc.FileName)
	newPdscFileName := fmt.Sprintf("%s.%s.%s.pdsc", p.Vendor, p.Name, p.Version)

	if !p.isPublic {
		_ = utils.CopyFile(pdscFilePath, filepath.Join(Installation.LocalDir, pdscFileName))
	}

	_ = utils.CopyFile(pdscFilePath, filepath.Join(Installation.DownloadDir, newPdscFileName))

	packBackupPath := filepath.Join(Installation.DownloadDir, fmt.Sprintf("%s.%s.%s.pack", p.Vendor, p.Name, p.Version))
	if !p.isDownloaded {
		return utils.CopyFile(p.path, packBackupPath)
	}

	if filepath.Base(p.path) != filepath.Base(packBackupPath) {
		err := utils.MoveFile(p.path, packBackupPath)
		if err != nil {
			return err
		}
		p.path = packBackupPath
	}

	return nil
}

// uninstall removes the pack from the installation directory.
// It:
//   - Removes all pack files from "CMSIS_PACK_ROOT/p.Vendor/p.Name/[p.Version]", where p.Version might be ommited
//   - Removes "CMSIS_PACK_ROOT/p.Vendor/p.Name/" if empty
//   - Removes "CMSIS_PACK_ROOT/p.Vendor/" if empty
//   - If "CMSIS_PACK_ROOT/.Web/p.Vendor.p.Name.pdsc" does not exist then
//     - Remove "p.Vendor.p.Name.pdsc" from "CMSIS_PACK_ROOT/.Local/"
func (p *PackType) uninstall(installation *PacksInstallationType) error {
	log.Debugf("Uninstalling \"%v\"", p.path)

	// Remove Vendor/Pack/x.y.z
	packPath := filepath.Join(Installation.PackRoot, p.Vendor, p.Name, p.Version)
	if err := os.RemoveAll(packPath); err != nil {
		return err
	}

	// Remove Vendor/Pack/ if empty
	packPath = filepath.Join(Installation.PackRoot, p.Vendor, p.Name)
	if utils.IsEmpty(packPath) {
		if err := os.Remove(packPath); err != nil {
			return err
		}

		// Remove local pdsc file if pack is not public and if there are no more versions of this pack installed
		if !p.isPublic {
			localPdscFileName := p.Vendor + "." + p.Name + ".pdsc"
			filePath := filepath.Join(Installation.LocalDir, localPdscFileName)
			if err := os.Remove(filePath); err != nil {
				return err
			}
		}
	}

	// Remove Vendor/ if empty
	vendorPath := filepath.Join(Installation.PackRoot, p.Vendor)
	if utils.IsEmpty(vendorPath) {
		if err := os.Remove(vendorPath); err != nil {
			return err
		}
	}

	return nil
}

// readEula reads in the pack's license into a string
func (p *PackType) readEula() ([]byte, error) {
	log.Debug("Reading EULA")

	licenseFileName := strings.Replace(p.Pdsc.License, "\\", "/", -1)

	// License contains the license path inside the pack file
	for _, file := range p.zipReader.File {
		possibleLicense := strings.Replace(file.Name, "\\", "/", -1)
		if possibleLicense == licenseFileName {

			reader, _ := file.Open()
			defer reader.Close()

			buffer := new(bytes.Buffer)
			_, err := utils.SecureCopy(buffer, reader)
			if err != nil {
				log.Error(err)
				return []byte{}, err
			}

			return buffer.Bytes(), nil
		}
	}

	return []byte{}, errs.ErrLicenseNotFound
}

// checkEula prints out the pack's license (if any) to the user and asks for
// confirmation. Returns false if user has not agreed with the license's terms.
// Returns true if pack has no license specified.
func (p *PackType) checkEula() (bool, error) {
	log.Debug("Checking EULA")

	bytes, err := p.readEula()
	if err != nil {
		return false, err
	}

	eulaContents, err := cat.FromBytes(bytes)
	if err != nil {
		log.Error(err)
		return false, err
	}

	return ui.DisplayAndWaitForEULA(p.Pdsc.License, eulaContents)
}

// extractEula extracts the pack's License to a file next to the pack's location
func (p *PackType) extractEula() error {
	log.Debug("Extracting EULA")

	eulaContents, err := p.readEula()
	if err != nil {
		return err
	}

	eulaFileName := p.path + "." + path.Base(p.Pdsc.License)

	log.Infof("Extracting embedded license to %v", eulaFileName)

	return ioutil.WriteFile(eulaFileName, eulaContents, 0600)
}
