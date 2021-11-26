/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"os"
	"path/filepath"
	"strings"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/ui"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	log "github.com/sirupsen/logrus"
)

// AddPack adds a pack to the pack installation directory structure
func AddPack(packPath string, checkEula, extractEula bool) error {
	log.Debugf("Adding pack \"%v\"", packPath)

	pack, err := preparePack(packPath)
	if err != nil {
		return err
	}

	if !extractEula && pack.isInstalled {
		return errs.ErrPackAlreadyInstalled
	}

	if pack.isPackID {
		pack.path, err = FindPackURL(pack)
		if err != nil {
			return err
		}
	}

	if err = pack.fetch(); err != nil {
		return err
	}

	// Tells the UI to return right away with the [E]xtract option selected
	ui.Extract = extractEula

	if err = pack.install(Installation, checkEula || extractEula); err != nil {
		return err
	}

	return Installation.touchPackIdx()
}

// RemovePack removes a pack given a pack path
func RemovePack(packPath string, purge bool) error {
	log.Debugf("Removing pack \"%v\"", packPath)

	pack, err := preparePack(packPath)
	if err != nil {
		return err
	}

	if pack.isInstalled {
		if err = pack.uninstall(Installation); err != nil {
			return err
		}

		if purge {
			if err = pack.purge(); err != nil {
				return err
			}
		}

		return Installation.touchPackIdx()
	} else if purge {
		return pack.purge()
	}

	log.Errorf("Pack \"%v\" is not installed", packPath)
	return errs.ErrPackNotInstalled
}

// AddPdsc adds a pack via PDSC file
func AddPdsc(pdscPath string) error {
	log.Debugf("Adding pdsc \"%v\"", pdscPath)

	pdsc, err := preparePdsc(pdscPath, false)
	if err != nil {
		return err
	}

	if err := pdsc.install(Installation); err != nil {
		return err
	}

	if err := Installation.LocalPidx.Write(); err != nil {
		return err
	}

	return Installation.touchPackIdx()
}

// RemovePdsc removes a pack given a pdsc path
func RemovePdsc(pdscPath string) error {
	log.Debugf("Removing pdsc \"%v\"", pdscPath)

	pdsc, err := preparePdsc(pdscPath, true)
	if err != nil {
		return err
	}

	if err = pdsc.uninstall(Installation); err != nil {
		return err
	}

	if err := Installation.LocalPidx.Write(); err != nil {
		return err
	}

	return Installation.touchPackIdx()
}

// UpdatePublicIndex receives a index path and place it under .Web/index.pidx.
func UpdatePublicIndex(indexPath string, overwrite bool) error {
	log.Debugf("Updating public index with \"%v\"", indexPath)

	if utils.FileExists(Installation.PublicIndex) {
		if !overwrite {
			return errs.ErrCannotOverwritePublicIndex
		}
		log.Infof("Overwriting public index file %v", Installation.PublicIndex)
	}

	var err error
	if !strings.HasPrefix(indexPath, "https://") {
		return errs.ErrIndexPathNotSafe
	}

	indexPath, err = utils.DownloadFile(indexPath)
	if err != nil {
		return err
	}

	pidx := xml.NewPidxXML(indexPath)
	if err := pidx.Read(); err != nil {
		_ = os.Remove(indexPath)
		return err
	}

	return utils.MoveFile(indexPath, filepath.Join(Installation.WebDir, "index.pidx"))
}

// PublicIndexUpdated is used by ensurePublicIndexIsUpdated to determine if the
// public index already got updated in this run of cpackget
var PublicIndexUpdated bool

// PublicIndexURL points to keil today, may be read via a config file in the future
var PublicIndexURL = "https://www.keil.com/pack/index.pidx"

// ensurePublicIndexIsUpdated makes sure that the .Web/index.pidx
func EnsurePublicIndexIsUpdated(forceUpdate bool) error {
	log.Debugf("Ensuring public index exists and it's up to date (force update? %v)", forceUpdate)

	if PublicIndexUpdated {
		return nil
	}

	if forceUpdate || !utils.FileExists(Installation.PublicIndex) {
		log.Infof("\"%s\" is missing, retrieving a fresh one from %s", Installation.PublicIndex, PublicIndexURL)
		if err := UpdatePublicIndex(PublicIndexURL, true /* overwrite */); err != nil {
			return err
		}

		PublicIndexUpdated = true
	}

	Installation.PublicIndexXML = xml.NewPidxXML(Installation.PublicIndex)
	return Installation.PublicIndexXML.Read()
}

// FindPackURL uses pack.path as packID and try to find the pack URL
// Finding step are as follows:
// 1. Find pack.Vendor, pack.Name, pack.Version(optional) in Installation.PublicIndex
//    1.1. if pack.Version == "", move to step 2
//    1.2. if Installation.PublicIndex does not exist, call UpdatePublicIndex("https://www.keil.com/pack/index.pidx", true /* overwrite */) and repeat step 1
//    1.3. if not found, raise errs.ErrPackNotFoundInPublicIndex
//    1.4. read the PDSC tag into pdscTag
//    1.5. packURL = pdscTag.URL + pack.Vendor + "." + pack.Name + "." + pack.Version + ".pack"
//    1.6. if HTTP HEAD for packURL is not 200, move to step 2
//    1.7. return packURL
// 2. Find pack.Vendor, pack.Name in Installation.PublicIndex
//    2.1. same as 1.2
//    2.2. same as 1.3
//    2.3. same as 1.4
//    2.4. pdscURL = pdscTag.URL + pack.Vendor + "." + pack.Name + ".pdsc"
//    2.5. if HTTP HEAD for pdscURL is not 200, raise errs.ErrPackPdscURLCannotBeFound
//    2.6. read the PDSC file into pdscXML
//    2.7. releastTag = pdscXML.FindReleaseTagByVersion(pack.Version) // if pack.Version == "", it'll return the most recent one
//    2.8. if releaseTag == nil, raise errs.ErrPackVersionNotFoundInPdsc
//    2.8. if releaseTag.URL == "", raise errs.ErrPackURLCannotBeFound
//    2.9. return releaseTag.URL
func FindPackURL(pack *PackType) (string, error) {
	log.Debugf("Finding URL for \"%v\"", pack.path)

	findPdscTag := func(pack *PackType) (*xml.PdscTag, error) {
		pdscTag := Installation.PublicIndexXML.FindPdscTag(pack.PdscTag) // maybe _, ok := err.(*fs.PathError)
		if pdscTag == nil {
			// Public index may be outdated, force updating it
			if err := EnsurePublicIndexIsUpdated(true); err != nil {
				return nil, err
			}

			pdscTag := Installation.PublicIndexXML.FindPdscTag(pack.PdscTag)
			if pdscTag == nil {
				return nil, errs.ErrPackNotFoundInPublicIndex
			}
		}

		return pdscTag, nil
	}

	if err := EnsurePublicIndexIsUpdated(false /* don't force */); err != nil {
		return "", err
	}

	// First attempt to retrieve packURL straight out of .Web/index.pidx
	pdscTag, err := findPdscTag(pack)
	if err != nil {
		return "", err
	}

	packURL := pdscTag.PackURL()
	if utils.URLExists(packURL) {
		pack.Version = pdscTag.Version
		return packURL, nil
	}

	// Failed to find URL the easy way. Now do the hard way, through releases tag

	// Get pack's pdsc file
	packPdscFileName := filepath.Join(Installation.WebDir, pack.Vendor+"."+pack.Name+".pdsc")
	if !utils.FileExists(packPdscFileName) {
		packPdscURL := pdscTag.URL + filepath.Base(packPdscFileName)
		log.Infof("\"%s\" not found, fetching it from \"%s\"", packPdscFileName, packPdscURL)

		packPdscDownloadFilePath, err := utils.DownloadFile(packPdscURL)
		if err != nil {
			log.Error(err)
			return "", errs.ErrPackPdscCannotBeFound
		}

		if err := utils.MoveFile(packPdscDownloadFilePath, packPdscFileName); err != nil {
			return "", err
		}
	}

	packPdscXML := xml.NewPdscXML(packPdscFileName)
	if err := packPdscXML.Read(); err != nil {
		return "", err
	}

	releaseTag := packPdscXML.FindReleaseTagByVersion(pack.Version)
	if releaseTag == nil {
		log.Errorf("Pack version \"%s\" was not found in \"%s\"", pack.Version, packPdscFileName)
		return "", errs.ErrPackVersionNotFoundInPdsc
	}

	// Nowhere else to look for pack URL
	if releaseTag.URL == "" {
		return "", errs.ErrPackURLCannotBeFound
	}

	// Set the pack version, if empty
	if pack.Version == "" {
		pack.Version = releaseTag.Version
	}

	return releaseTag.URL, nil
}

// Installation is a singleton variable that keeps the only reference
// to PacksInstallationType
var Installation *PacksInstallationType

// SetPackRoot sets the working directory of the packs installation
// if create == true, cpackget will try to create needed resources
func SetPackRoot(packRoot string, create bool) error {
	log.Infof("Using pack root: \"%v\"", packRoot)

	if len(packRoot) == 0 || (!utils.DirExists(packRoot) && !create) {
		return errs.ErrPackRootNotFound
	}

	Installation = &PacksInstallationType{
		PackRoot:    packRoot,
		DownloadDir: filepath.Join(packRoot, ".Download"),
		LocalDir:    filepath.Join(packRoot, ".Local"),
		WebDir:      filepath.Join(packRoot, ".Web"),
	}
	Installation.LocalPidx = xml.NewPidxXML(filepath.Join(Installation.LocalDir, "local_repository.pidx"))
	Installation.PackIdx = filepath.Join(packRoot, "pack.idx")
	Installation.PublicIndex = filepath.Join(Installation.WebDir, "index.pidx")

	for _, dir := range []string{packRoot, Installation.DownloadDir, Installation.LocalDir, Installation.WebDir} {
		log.Debugf("Making sure \"%v\" exists", dir)
		exists := utils.DirExists(dir)
		if !exists {
			if !create {
				return errs.ErrDirectoryNotFound
			} else {
				if err := utils.EnsureDir(dir); err != nil {
					return err
				}
			}
		}
	}

	// Make sure utils.DownloadFile always downloads files to .Download/
	utils.CacheDir = Installation.DownloadDir

	return nil
}

// PacksInstallationType is the scruct tha manages Open-CMSIS-Pack installation/deletion.
type PacksInstallationType struct {
	// PackRoot is the working directory if the packs installation
	PackRoot string

	// packs installed
	packs map[string]bool

	// DownloadDir stores copies of all packs that were installed via pack files
	// from external servers.
	DownloadDir string

	// LocalDir stores "local_repository.pidx" containing a list of all packs
	// installed via PDSC files.
	LocalDir string

	// WebDir stores "index.pidx" containing a list of PDSC tags with all
	// publicly available packs.
	WebDir string

	// PublicIndex stores the path PackRoot/WebDir/index.pidx
	PublicIndex string

	// PublicIndexXML stores a xml.PidxXML reference for PackRoot/WebDir/index.pidx
	PublicIndexXML *xml.PidxXML

	// LocalPidx is a reference to "local_repository.pidx" that contains a flat
	// list of PDSC tags representing all packs installed via PDSC files.
	LocalPidx *xml.PidxXML

	// localIsLoaded is a flag that tells whether the local_repository.pidx has been loaded or not
	localIsLoaded bool

	// PackIdx is the "pack.idx" file used by other tools to be notified that
	// the pack installation had changed.
	PackIdx string
}

// touchPackIdx changes the timestamp of pack.idx.
func (p *PacksInstallationType) touchPackIdx() error {
	return utils.TouchFile(p.PackIdx)
}

// PackIsInstalled checks whether a given pack is already installed or not
func (p *PacksInstallationType) PackIsInstalled(pack *PackType) bool {
	installationDir := filepath.Join(p.PackRoot, pack.Vendor, pack.Name, pack.Version)
	if _, err := os.Stat(installationDir); !os.IsNotExist(err) {
		return true
	}
	return false
}

// packIsPublic checks whether the pack is public or not.
// Being public means a PDSC file is present in ".Web/" folder
func (p *PacksInstallationType) packIsPublic(pack *PackType) bool {
	// lazyly lists all pdsc files in the ".Web/" folder only once
	if p.packs == nil {
		p.packs = make(map[string]bool)
		files, _ := utils.ListDir(p.WebDir, `^.*\.pdsc$`)
		for _, file := range files {
			_, baseFileName := filepath.Split(file)
			p.packs[baseFileName] = true
		}
	}

	key := pack.Vendor + "." + pack.Name + ".pdsc"
	return p.packs[key]
}
