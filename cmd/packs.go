/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
)

// PackInstallationType is the struct that represents the installation of a
// single pack
type PackInstallationType struct {
	PdscTag

	// Manager is a reference to PackManagerType
	Manager *PacksManagerType

	// IsPublic tells whether the pack exists in the public index or not
	IsPublic bool

	// IsLocal tells whether the pack comes from a local file system (for dev)
	// Local packs are installed via adding pdsc files
	// Ex: cpackget install path/to/Vendor.Pack.pdsc
	IsLocal bool

	// IsDownloaded tells whether the file needed to be downloaded from a server
	IsDownloaded bool

	// Path points to a file in the local system, whether or not it's local
	Path string

	// ZipReader holds a pointer to the uncompress pack file
	ZipReader *zip.ReadCloser
}

// ToPdscTag generates a pdscTag out of this pack file name details
func (p *PackInstallationType) ToPdscTag() PdscTag {
	return PdscTag{
		Vendor:  p.Vendor,
		URL:     p.URL,
		Name:    p.Name,
		Version: p.Version,
	}
}

// Fetch will download the pack file if it's on the Internet, or
// will make sure the file exists in the local file system
func (p *PackInstallationType) Fetch() error {
	log.Debugf("Fetching pack file \"%s\" (or just making sure it exists locally)", p.Path)
	var err error
	if strings.HasPrefix(p.Path, "http") {
		p.Path, err = DownloadFile(p.Path)
		p.IsDownloaded = true
		return err
	}

	if !FileExists(p.Path) {
		log.Errorf("File \"%s\" does't exist", p.Path)
		return ErrFileNotFound
	}

	return nil
}

// Validate() should ensure the pack is legit and it has all minimal requrements
// to be installed
func (p *PackInstallationType) Validate() error {
	log.Debugf("Validating pack")
	pdscFileName := fmt.Sprintf("%s.%s.pdsc", p.Vendor, p.Name)
	isPdscPresent := false
	for _, file := range p.ZipReader.File {
		if file.Name == pdscFileName {
			isPdscPresent = true
			break
		}
	}

	if !isPdscPresent {
		log.Errorf("\"%s\" not found in \"%s\"", pdscFileName, p.Path)
		return ErrPdscNotFound
	}

	return nil
}

// IsInstalled returns true if pack is already installed or false otherwise
// First it checks if there's a folder named p.Vendor / p.Name / p.Version
// Then it checks if it's listed in .Local/local_repository.pidx
func (p *PackInstallationType) IsInstalled() bool {
	installationDir := path.Join(p.Manager.PackRoot, p.Vendor, p.Name, p.Vendor)
	if _, err := os.Stat(installationDir); !os.IsNotExist(err) {
		return true
	}

	return p.Manager.LocalPidx.HasPdsc(p.ToPdscTag())
}

// Install installs itself using PackManager's info
// It should:
//   - If pack.IsLocal (pdsc file) is true then
//     - It means the tool is adding a development version of the pack
//       and the only step is to add it to the "CMSIS_PACK_ROOT/.Local/local_repository.pidx"
//       and the URL should be the directory of the pdsc file
//   - If it's no local then
//     - Extract all files to "CMSIS_PACK_ROOT/p.Vendor/p.Name/p.Version/"
//     - Save a copy of the pack in "CMSIS_PACK_ROOT/.Download/"
//     - Save a versioned pdsc file in "CMSIS_PACK_ROOT/.Download/"
//     - If it's not public (pack not in CMSIS_PACK_ROOT/.Web/index.pidx) then
//       - Save an unversioned copy of the pdsc file in "CMSIS_PACK_ROOT/.Local/"
//
func (p *PackInstallationType) Install() error {
	log.Debugf("Installing \"%s\"", p.Path)
	if p.IsLocal {
		pdsc := NewPdsc(p.Path)
		pdsc.Read()
		return p.Manager.LocalPidx.AddPdsc(pdsc.Tag())
	}

	packHomeDir := path.Join(p.Manager.PackRoot, p.Vendor, p.Name, p.Version)
	err := EnsureDir(packHomeDir)
	if err != nil {
		log.Errorf("Can't access pack directory \"%s\": %s", packHomeDir, err)
		return err
	}

	log.Debugf("Extracting files from \"%s\" to \"%s\"", p.Path, packHomeDir)

	p.ZipReader, err = zip.OpenReader(p.Path)
	if err != nil {
		log.Errorf("Can't decompress \"%s\": %s", p.Path, err)
		return ErrFailedDecompressingFile
	}
	defer p.ZipReader.Close()

	err = p.Validate()
	if err != nil {
		return err
	}

	// Inflate all files
	for _, file := range p.ZipReader.File {
		err = InflateFile(file, packHomeDir)
		if err != nil {
			return err
		}
	}

	pdscFileName := fmt.Sprintf("%s.%s.pdsc", p.Vendor, p.Name)
	pdscFilePath := path.Join(packHomeDir, pdscFileName)
	newPdscFileName := fmt.Sprintf("%s.%s.%s.pdsc", p.Vendor, p.Name, p.Version)

	if !p.IsPublic {
		err = CopyFile(pdscFilePath, path.Join(p.Manager.LocalDir, pdscFileName))
		if err != nil {
			return err
		}
	}

	err = CopyFile(pdscFilePath, path.Join(p.Manager.DownloadDir, newPdscFileName))
	if err != nil {
		return err
	}

	packBackupPath := path.Join(p.Manager.DownloadDir, path.Base(p.Path))
	if p.IsDownloaded {
		return MoveFile(p.Path, packBackupPath)
	} else {
		return CopyFile(p.Path, packBackupPath)
	}
}

// PacksManagerType is the scruct tha manages Open-CMSIS-Pack installation/deletion
type PacksManagerType struct {
	DownloadDir string
	LocalDir    string
	WebDir      string

	// Stores CMSIS_PACK_ROOT
	PackRoot string

	// Packs installed
	Packs map[string]*PackInstallationType

	// Package index
	WebPidx   *PidxXML
	LocalPidx *PidxXML
}

// NewConfig will load the local configuration installation of Open-CMSIS-Pack
// It takes in the basePath/CMSIS_PACK_ROOT
func NewPacksManager(packRoot string) (*PacksManagerType, error) {
	log.Debugf("Initializing PacksManager in \"%s\"", packRoot)
	manager := &PacksManagerType{
		PackRoot:    packRoot,
		DownloadDir: path.Join(packRoot, ".Download"),
		LocalDir:    path.Join(packRoot, ".Local"),
		WebDir:      path.Join(packRoot, ".Web"),
	}
	manager.WebPidx = NewPidx(path.Join(manager.WebDir, "index.pidx"))
	manager.LocalPidx = NewPidx(path.Join(manager.LocalDir, "local_repository.pidx"))

	var err error
	for _, dir := range []string{manager.DownloadDir, manager.LocalDir, manager.WebDir} {
		if err = EnsureDir(dir); err != nil {
			return manager, err
		}
	}

	for _, pidx := range []*PidxXML{manager.WebPidx, manager.LocalPidx} {
		if err = pidx.Read(); err != nil {
			return manager, err
		}
	}

	return manager, nil
}

// NewPackInstallatio receives a <package-path> argument and
// starts preparing the package for installation by doing some sanity checks
func (manager *PacksManagerType) NewPackInstallation(packPath string) (*PackInstallationType, error) {
	pdscTag, err := PackPathToPdscTag(packPath)
	if err != nil {
		return nil, err
	}

	packInstallation := &PackInstallationType{
		PdscTag: pdscTag,
		Manager: manager,
		Path:    packPath,
	}

	return packInstallation, nil
}

// Save saves proper modifications to pidx files to disk
func (manager *PacksManagerType) Save() error {
	return manager.LocalPidx.Write()
}
