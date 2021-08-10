/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"archive/zip"
	"fmt"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
)

// PackType is the struct that represents the installation of a
// single pack
type PackType struct {
	xml.PdscTag

	// Manager is a reference to PackManagerType
	installation *PacksInstallationType

	// IsPublic tells whether the pack exists in the public index or not
	isPublic bool

	// IsLocal tells whether the pack comes from a local file system (for dev)
	// Local packs are installed via adding pdsc files
	// Ex: cpackget install path/to/Vendor.Pack.pdsc
	isLocal bool

	// IsDownloaded tells whether the file needed to be downloaded from a server
	isDownloaded bool

	// Path points to a file in the local system, whether or not it's local
	path string

	// ZipReader holds a pointer to the uncompress pack file
	zipReader *zip.ReadCloser
}

// ToPdscTag generates a pdscTag out of this pack file name details
func (p *PackType) toPdscTag() xml.PdscTag {
	return xml.PdscTag{
		Vendor:  p.Vendor,
		URL:     p.URL,
		Name:    p.Name,
		Version: p.Version,
	}
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
		return errs.FileNotFound
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
		return errs.PdscNotFound
	}

	return nil
}


// install installs pack files to installation's directories
// It:
//   - Extracts all files to "CMSIS_PACK_ROOT/p.vendor/p.name/p.version/"
//   - Saves a copy of the pack in "CMSIS_PACK_ROOT/.Download/"
//   - Saves a versioned pdsc file in "CMSIS_PACK_ROOT/.Download/"
//   - If "CMSIS_PACK_ROOT/.Web/p.vendor.p.name.pdsc" does not exist then
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
		return errs.FailedDecompressingFile
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

/*
func (manager *PacksManagerType) Uninstall(packName string) error {
	log.Infof("Uninstalling %s", packName)

	pdsc, err := PackPathToPdscTag(packName)
	if err != nil {
		return err
	}
	var pidx *PidxXML
	if manager.Pidx.HasPdsc(pdsc) {
		pidx = manager.Pidx
	} else if manager.LocalPidx.HasPdsc(pdsc) {
		pidx = manager.LocalPidx
	}

	if pidx == nil {
		return ErrPdscNotFound
	}

	packPath := path.Join(manager.PackRoot, pdsc.Vendor, pdsc.Name, pdsc.Version)
	if err := os.RemoveAll(packPath); err != nil {
		return err
	}

	// TODO: If there are left over empty directories

	/*err = pidx.RemovePdsc(pdsc)
	if err != nil {
		log.Errorf("Can't deregister pack %s: %s", pdsc.Key(), err)
		return ErrUnknownBehavior
	}
	return nil
}
*/
