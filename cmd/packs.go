/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"archive/zip"
	"fmt"
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

	// IsLocal tells if the file comes from a local file system or the Internet
	IsLocal bool

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
		if err != nil {
			return err
		}
	} else if !FileExists(p.Path) {
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

// Install installs itself using PackManager's info
// It should:
//   * extract all files from the pack into CMSIS_PACK_ROOT/Vendor/Name/Version
//   * if pack comes from file system
//     * add this pack's pdscTag to CMSIS_PACK_ROOT/.Local/local_repository.pidx
//   * if pack comes from the Internet
//     * add this pack's pdscTag to CMSIS_PACK_ROOT/.Web/index.pidx
//     * place the original pack file under CMSIS_PACK_ROOT/.Download
func (p *PackInstallationType) Install() error {
	packHomeDir := path.Join(p.Manager.PackRoot, p.Vendor, p.Name, p.Version)
	err := EnsureDir(packHomeDir)
	if err != nil {
		log.Errorf("Can't access pack directory \"%s\": %s", packHomeDir, err)
		return err
	}

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

	log.Debugf("Installing \"%s\" into \"%s\"", p.Path, packHomeDir)

	// Inflate all first
	for _, file := range p.ZipReader.File {
		err = InflateFile(file, packHomeDir)
		if err != nil {
			return err
		}
	}

	pdscFileName := fmt.Sprintf("%s.%s.pdsc", p.Vendor, p.Name)
	pdscFilePath := path.Join(packHomeDir, pdscFileName)
	newPdscFileName := fmt.Sprintf("%s.%s.%s.pdsc", p.Vendor, p.Name, p.Version)

	if p.IsLocal {
		// Keep a copy of a versioned pdsc file under .Local/
		err = CopyFile(pdscFilePath, path.Join(p.Manager.LocalDir, newPdscFileName))
		if err != nil {
			return err
		}
	} else {
		// Keep a copy of a versioned pdsc file under .Download/
		err = CopyFile(pdscFilePath, path.Join(p.Manager.DownloadDir, newPdscFileName))
		if err != nil {
			return err
		}

		// Keep a copy of the original pack under .Download/ as well
		packFileName := path.Base(p.Path)
		err = MoveFile(p.Path, path.Join(p.Manager.DownloadDir, packFileName))
		if err != nil {
			return err
		}
	}

	return nil
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
	Pidx      *PidxXML
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
	manager.Pidx = NewPidx(path.Join(manager.WebDir, "index.pidx"))
	manager.LocalPidx = NewPidx(path.Join(manager.LocalDir, "local_repository.pidx"))

	var err error
	for _, dir := range []string{manager.DownloadDir, manager.LocalDir, manager.WebDir} {
		if err = EnsureDir(dir); err != nil {
			return manager, err
		}
	}

	for _, pidx := range []*PidxXML{manager.Pidx, manager.LocalPidx} {
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
		IsLocal: !strings.HasPrefix(packPath, "http"),
	}

	return packInstallation, nil
}

// Save saves proper modifications to pidx files to disk
func (manager *PacksManagerType) Save() error {
	err := manager.Pidx.Write()
	if err != nil {
		return err
	}

	err = manager.LocalPidx.Write()
	if err != nil {
		return err
	}

	return nil
}
