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
	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
)

// PackType is the struct that represents the installation of a
// single pack
type PackType struct {
	xml.PdscTag

	//IsLocallySourced tells whether the pack's source is local or an HTTP URL
	IsLocallySourced bool

	// IsPublic tells whether the pack exists in the public index or not
	IsPublic bool

	// isInstalled tells whether the pack is already installed
	isInstalled bool

	// isDownloaded tells whether the file needed to be downloaded from a server
	isDownloaded bool

	// isPackID tells whether the path is in packID format: Vendor.PackName[.x.y.z]
	isPackID bool

	// toBeRemoved indicates
	toBeRemoved bool

	// exactVersion tells wether this pack identifier is specifying an exact version
	// or is requesting a newer one, e.g. >=x.y.z
	versionModifier int

	// targetVersion is the most recent version of a pack in case exactVersion==true
	targetVersion string

	// path points to a file in the local system, whether or not it's local
	path string

	// Subfolder stores the subfolder this pack is in the compressed file.
	Subfolder string

	// pdsc holds a pointer to the PDSC file already parsed as XML
	Pdsc *xml.PdscXML

	// zipReader holds a pointer to the uncompress pack file
	zipReader *zip.ReadCloser
}

// preparePack does some sanity validation regarding pack name
// and check if it's public and if it's installed or not
func preparePack(packPath string, toBeRemoved bool, timeout int) (*PackType, error) {
	pack := &PackType{
		path:        packPath,
		toBeRemoved: toBeRemoved,
	}

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
	}

	info, err := utils.ExtractPackInfo(packPath)
	if err != nil {
		return pack, err
	}

	pack.URL = info.Location
	pack.Name = info.Pack
	pack.Vendor = info.Vendor
	pack.Version = info.Version
	pack.versionModifier = info.VersionModifier
	pack.isPackID = info.IsPackID

	if !strings.HasPrefix(pack.URL, "http://") && !strings.HasPrefix(pack.URL, "https://") && strings.HasPrefix(pack.URL, "file://") {
		pack.IsLocallySourced = true
	}

	if pack.IsPublic, err = Installation.packIsPublic(pack, timeout); err != nil {
		return pack, err
	}

	pack.isInstalled = Installation.PackIsInstalled(pack)

	return pack, nil
}

// fetch will download the pack file if it's on the Internet, or
// will use the one in .Download/ if previously downloaded.
// If the path is not a URL, it will make sure the file exists in the local file system
func (p *PackType) fetch(timeout int) error {
	log.Debugf("Fetching pack file \"%s\" (or just making sure it exists locally)", p.path)
	var err error
	if strings.HasPrefix(p.path, "http") {
		p.path, err = utils.DownloadFile(p.path, timeout)
		if err == errs.ErrTerminatedByUser {
			log.Infof("Aborting pack download. Removing \"%s\"", p.path)
		}

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
	pdscFileName := p.PdscFileName()
	for _, file := range p.zipReader.File {
		if filepath.Base(file.Name) == pdscFileName {

			// Check if pack was compressed in a subfolder
			subfoldersCount := strings.Count(file.Name, "/") + strings.Count(file.Name, "\\")
			if subfoldersCount > 1 {
				return errs.ErrPdscFileTooDeepInPack
			} else if subfoldersCount == 1 {
				p.Subfolder = filepath.Dir(file.Name)
			}

			// Read pack's pdsc
			tmpPdscFileName := utils.RandStringBytes(10)
			defer os.RemoveAll(tmpPdscFileName)

			if err := utils.SecureInflateFile(file, tmpPdscFileName, ""); err != nil {
				return err
			}

			p.Pdsc = xml.NewPdscXML(filepath.Join(tmpPdscFileName, file.Name)) // #nosec
			if err := p.Pdsc.Read(); err != nil {
				return err
			}

			// Sanity check: make sure the version being installed actually exists in the PDSC file
			version := p.GetVersion()
			latestVersion := p.Pdsc.LatestVersion()

			log.Debugf("Making sure %s is the latest release in %s", p.targetVersion, pdscFileName)

			if latestVersion != version {
				releaseTag := p.Pdsc.FindReleaseTagByVersion(version)
				if releaseTag == nil {
					log.Errorf("The pack's pdsc (%s) has no release tag matching version \"%s\"", pdscFileName, version)
					return errs.ErrPackVersionNotFoundInPdsc
				}

				log.Errorf("The latest release (%s) in pack's pdsc (%s) does not match pack version \"%s\"", latestVersion, pdscFileName, version)
				return errs.ErrPackVersionNotLatestReleasePdsc
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
//   - Save an unversioned copy of the pdsc file in "CMSIS_PACK_ROOT/.Local/"
func (p *PackType) install(installation *PacksInstallationType, checkEula bool) error {
	log.Debugf("Installing \"%s\"", p.path)

	var err error
	p.zipReader, err = zip.OpenReader(p.path)
	if err != nil {
		log.Errorf("Can't decompress \"%s\": %s", p.path, err)
		return errs.ErrFailedDecompressingFile
	}

	if err = p.validate(); err != nil {
		return err
	}

	packHomeDir := filepath.Join(Installation.PackRoot, p.Vendor, p.Name, p.GetVersion())
	packBackupPath := filepath.Join(Installation.DownloadDir, p.PackFileName())

	if len(p.Pdsc.License) > 0 {
		if checkEula {
			ok, err := p.checkEula()
			if err != nil {
				if err == errs.ErrExtractEula {
					return p.extractEula(packBackupPath)
				}
				return err
			}

			if !ok {
				log.Info("User does not agree with the pack's license, not installing it")
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

	var progress *progressbar.ProgressBar
	if utils.IsTerminalInteractive() {
		progress = progressbar.Default(int64(len(p.zipReader.File)), "I: Extracting files to "+packHomeDir)
	} else {
		log.Infof("Extracting files to %s...", packHomeDir)
	}

	for _, file := range p.zipReader.File {
		if utils.IsTerminalInteractive() {
			_ = progress.Add(1)
		}
		err = utils.SecureInflateFile(file, packHomeDir, p.Subfolder)
		if err != nil {
			defer p.zipReader.Close()

			if err == errs.ErrTerminatedByUser {
				log.Infof("Aborting pack extraction. Removing \"%s\"", packHomeDir)
				if newErr := p.uninstall(installation); newErr != nil {
					log.Error(err)
				}
			}
			return err
		}
	}

	// Close zip file so Windows can't complain if we rename it
	p.zipReader.Close()

	pdscFileName := p.PdscFileName()
	pdscFilePath := filepath.Join(packHomeDir, pdscFileName)
	newPdscFileName := p.PdscFileNameWithVersion()

	if !p.IsPublic {
		_ = utils.CopyFile(pdscFilePath, filepath.Join(Installation.LocalDir, pdscFileName))
	}

	_ = utils.CopyFile(pdscFilePath, filepath.Join(Installation.DownloadDir, newPdscFileName))

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
//   - Remove "p.Vendor.p.Name.pdsc" from "CMSIS_PACK_ROOT/.Local/"
func (p *PackType) uninstall(installation *PacksInstallationType) error {
	log.Debugf("Uninstalling \"%v\"", p.path)

	// Remove Vendor/Pack/x.y.z
	packPath := filepath.Join(Installation.PackRoot, p.Vendor, p.Name, p.GetVersion())
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
		if !p.IsPublic {
			localPdscFileName := p.PdscFileName()
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
func (p *PackType) extractEula(packPath string) error {
	log.Debug("Extracting EULA")

	eulaContents, err := p.readEula()
	if err != nil {
		return err
	}

	eulaFileName := packPath + "." + path.Base(p.Pdsc.License)

	log.Infof("Extracting embedded license to %v", eulaFileName)

	return ioutil.WriteFile(eulaFileName, eulaContents, utils.FileModeRO)
}

// resolveVersionModifier takes into account eventual versionModifiers (@, @~ and >=) to determine
// which version of a pack should be targeted for installation
func (p *PackType) resolveVersionModifier(pdscXML *xml.PdscXML) {
	log.Debugf("Resolving version modifier for \"%s\" using PDSC \"%s\"", p.path, pdscXML.FileName)

	if p.versionModifier == utils.ExactVersion {
		p.targetVersion = p.Version
		log.Debugf("- resolved(@) as %s", p.targetVersion)
		return
	}

	if p.versionModifier == utils.LatestVersion ||
		p.versionModifier == utils.AnyVersion {
		p.targetVersion = pdscXML.LatestVersion()
		log.Debugf("- resolved(@latest) as %s", p.targetVersion)
		return
	}

	if p.versionModifier == utils.GreaterVersion {
		// No minimum version exists to satisfy target version
		if p.targetVersion == "" && utils.SemverCompare(p.Version, pdscXML.LatestVersion()) > 0 {
			log.Errorf("Tried to install at least version %s, highest available version is %s", p.Version, pdscXML.LatestVersion())
		} else {
			p.targetVersion = pdscXML.LatestVersion()
			log.Debugf("- resolved(>=) as %s", p.targetVersion)
		}
		return
	}

	// The trickiest one is @~, because it needs to be the latest
	// release matching the major number.
	// The releases in the PDSC file are sorted from latest to oldest
	if p.versionModifier == utils.GreatestCompatibleVersion {
		for _, version := range pdscXML.AllReleases() {
			sameMajor := utils.SemverMajor(version) == utils.SemverMajor(p.Version)
			if sameMajor && utils.SemverCompare(version, p.Version) >= 0 {
				p.targetVersion = version
				log.Debugf("- resolved (@~) as %s", p.targetVersion)
				return
			}
		}
		// Check if at least same Major version exists
		if utils.SemverCompare(p.targetVersion, p.Version) > 0 {
			p.targetVersion = pdscXML.LatestVersion()
			log.Debugf("- resolved (@~) as %s", p.targetVersion)
		} else {
			log.Errorf("Tried to install major version %s.x.x, highest available major version is %s.x.x", p.Version[:1], pdscXML.LatestVersion()[:1])
		}
		return
	}

	log.Warn("Could not resolve version modifier")
}

// PackID retuns the most generic name of a pack: Vendor.PackName
func (p *PackType) PackID() string {
	return p.Vendor + "." + p.Name
}

// PackIDWithVersion returns the packID with version: Vendor.PackName.x.y.z
func (p *PackType) PackIDWithVersion() string {
	return p.PackID() + "." + p.GetVersion()
}

// PackFileName returns a string with how the pack file name would be: Vendor.PackName.x.y.z.pack
func (p *PackType) PackFileName() string {
	return p.PackIDWithVersion() + ".pack"
}

// PdscFileName returns a string with how the pack's pdsc file name would be: Vendor.PackName.pdsc
func (p *PackType) PdscFileName() string {
	return p.PackID() + ".pdsc"
}

// PdscFileNameWithVersion returns a string with how the pack's pdsc file name would be: Vendor.PackName.x.y.z.pdsc
func (p *PackType) PdscFileNameWithVersion() string {
	return p.PackIDWithVersion() + ".pdsc"
}

// GetVersion makes sure to get the latest version for the pack
// after parsing possible version modifiers (@~, >=)
func (p *PackType) GetVersion() string {
	if p.versionModifier != utils.ExactVersion {
		return p.targetVersion
	}
	return p.Version
}

// toggleReadOnly will be used by Lock() and Unlock() to set or unset Read-Only flag on all pack files
func (p *PackType) toggleReadOnly(setReadOnly bool) {
	// Vendor/Pack/x.y.z/
	packHomeDir := filepath.Join(Installation.PackRoot, p.Vendor, p.Name, p.GetVersion())

	// .Download/Vendor.Pack.z.y.z.pack
	packBackupPath := filepath.Join(Installation.DownloadDir, p.PackFileName())

	// .Download/Vendor.Pack.x.y.z.pdsc
	packVersionedPdscPath := filepath.Join(Installation.DownloadDir, p.PdscFileNameWithVersion())

	// .Web/Vendor.Pack.pdsc or .Local/Vendor.Pack.pdsc
	packPdscPath := filepath.Join(Installation.WebDir, p.PdscFileName())
	if !p.IsPublic {
		packPdscPath = filepath.Join(Installation.LocalDir, p.PdscFileName())
	}

	if setReadOnly {
		utils.SetReadOnlyR(packHomeDir)
		utils.SetReadOnly(packBackupPath)
		utils.SetReadOnly(packVersionedPdscPath)
		utils.SetReadOnly(packPdscPath)
	} else {
		utils.UnsetReadOnlyR(packHomeDir)
		utils.UnsetReadOnly(packBackupPath)
		utils.UnsetReadOnly(packVersionedPdscPath)
		utils.UnsetReadOnly(packPdscPath)
	}
}

// Lock sets all files and directories for this pack to Read-Only
func (p *PackType) Lock() {
	p.toggleReadOnly(true)
}

// Unlock sets all files and directories for this pack to Read/Write mode
func (p *PackType) Unlock() {
	p.toggleReadOnly(false)
}
