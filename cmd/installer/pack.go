/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"archive/zip"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
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

	// path points to a file in the local system, whether or not it's local
	path string

	// zipReader holds a pointer to the uncompress pack file
	zipReader *zip.ReadCloser
}

// preparePack does some sanity validation regarding pack name
// and check if it's public and if it's installed or not
func preparePack(packPath string, short bool) (*PackType, error) {
	pack := &PackType{
		path: packPath,
	}

	// Clean out any possible query or user auth in the URL
	// to help finding the correct path info
	if strings.HasPrefix(packPath, "https") {
		url, err := url.Parse(packPath)
		if err != nil {
			log.Error(err)
			return pack, errs.ErrBadPackURL
		}

		url.User = nil
		url.Fragment = ""
		url.RawQuery = ""

		packPath = url.String()
	}

	info, err := utils.ExtractPackInfo(packPath, short)
	if err != nil {
		return pack, err
	}

	pack.URL = info.Location
	pack.Name = info.Pack
	pack.Vendor = info.Vendor
	pack.Version = info.Version
	pack.isPublic = installation.packIsPublic(pack)
	pack.isInstalled = installation.packIsInstalled(pack)

	return pack, nil
}

// fetch will download the pack file if it's on the Internet, or
// will make sure the file exists in the local file system
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
	log.Debugf("Validating pack")
	pdscFileName := fmt.Sprintf("%s.%s.pdsc", p.Vendor, p.Name)
	isPdscPresent := false
	for _, file := range p.zipReader.File {
		if file.Name == pdscFileName {
			isPdscPresent = true
			break
		}
	}

	if !isPdscPresent {
		log.Errorf("\"%s\" not found in \"%s\"", pdscFileName, p.path)
		return errs.ErrPdscFileNotFound
	}

	return nil
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

	files, err := utils.ListDir(installation.downloadDir, fileNamePattern)
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
func (p *PackType) install(installation *PacksInstallationType) error {
	log.Debugf("Installing \"%s\"", p.path)

	packHomeDir := path.Join(installation.packRoot, p.Vendor, p.Name, p.Version)
	err := utils.EnsureDir(packHomeDir)
	if err != nil {
		log.Errorf("Can't access pack directory \"%s\": %s", packHomeDir, err)
		return err
	}

	log.Debugf("Extracting files from \"%s\" to \"%s\"", p.path, packHomeDir)

	p.zipReader, err = zip.OpenReader(p.path)
	if err != nil {
		log.Errorf("Can't decompress \"%s\": %s", p.path, err)
		return errs.ErrFailedDecompressingFile
	}
	defer p.zipReader.Close()

	err = p.validate()
	if err != nil {
		return err
	}

	// Inflate all files
	for _, file := range p.zipReader.File {
		err = utils.InflateFile(file, packHomeDir)
		if err != nil {
			return err
		}
	}

	pdscFileName := fmt.Sprintf("%s.%s.pdsc", p.Vendor, p.Name)
	pdscFilePath := path.Join(packHomeDir, pdscFileName)
	newPdscFileName := fmt.Sprintf("%s.%s.%s.pdsc", p.Vendor, p.Name, p.Version)

	if !p.isPublic {
		err = utils.CopyFile(pdscFilePath, path.Join(installation.localDir, pdscFileName))
		if err != nil {
			return err
		}
	}

	err = utils.CopyFile(pdscFilePath, path.Join(installation.downloadDir, newPdscFileName))
	if err != nil {
		return err
	}

	packBackupPath := path.Join(installation.downloadDir, path.Base(p.path))
	if p.isDownloaded {
		return utils.MoveFile(p.path, packBackupPath)
	} else {
		return utils.CopyFile(p.path, packBackupPath)
	}
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
	packPath := path.Join(installation.packRoot, p.Vendor, p.Name, p.Version)
	if err := os.RemoveAll(packPath); err != nil {
		return err
	}

	// Remove Vendor/Pack/ if empty
	packPath = path.Join(installation.packRoot, p.Vendor, p.Name)
	if utils.IsEmpty(packPath) {
		if err := os.Remove(packPath); err != nil {
			return err
		}
	}

	// Remove Vendor/ if empty
	vendorPath := path.Join(installation.packRoot, p.Vendor)
	if utils.IsEmpty(vendorPath) {
		if err := os.Remove(vendorPath); err != nil {
			return err
		}
	}

	// Removes local pdsc file if pack is not public
	if !p.isPublic {
		localPdscFileName := p.Vendor + "." + p.Name + ".pdsc"
		filePath := path.Join(installation.localDir, localPdscFileName)
		if err := os.Remove(filePath); err != nil {
			return err
		}
	}

	return nil
}
