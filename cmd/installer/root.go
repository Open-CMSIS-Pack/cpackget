/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/ui"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
)

// GetDefaultCmsisPackRoot provides a default location
// for the pack root if not provided. This is to enable
// a "default mode", where the public index will be
// automatically initiated if not ready yet.
func GetDefaultCmsisPackRoot() string {
	var root string
	// Workaround to fake default mode in tests,
	// by avoiding writing in any of the default locations,
	// and using the generated testing pack dirs.
	if root = os.Getenv("CPACKGET_DEFAULT_MODE_PATH"); root != "" {
		return filepath.Clean(root)
	}
	if runtime.GOOS == "windows" {
		root = os.Getenv("LOCALAPPDATA")
		if root == "" {
			root = os.Getenv("USERPROFILE")
			if root != "" {
				root = root + "\\AppData\\Local"
			}
		}
		if root != "" {
			root = root + "\\Arm\\Packs"
		}
	} else {
		root = os.Getenv("XDG_CACHE_HOME")
		if root == "" {
			root = os.Getenv("HOME")
			if root != "" {
				root = root + "/.cache"
			}
		}
		if root != "" {
			root = root + "/arm/packs"
		}
	}
	return filepath.Clean(root)
}

// AddPack adds a pack to the pack installation directory structure
func AddPack(packPath string, checkEula, extractEula bool, forceReinstall bool, timeout int) error {

	pack, err := preparePack(packPath, false, timeout)
	if err != nil {
		return err
	}

	log.Infof("Adding pack \"%s\"", packPath)

	dropPreInstalled := false
	fullPackPath := ""
	backupPackPath := ""
	if !extractEula && pack.isInstalled {
		if forceReinstall {

			log.Debugf("Making temporary backup of pack \"%s\"", packPath)

			// Get target pack's full path and move it to a temporary "_tmp" directory
			fullPackPath = filepath.Join(Installation.PackRoot, pack.Vendor, pack.Name, pack.Version)
			backupPackPath = fullPackPath + "_tmp"

			if err := utils.MoveFile(fullPackPath, backupPackPath); err != nil {
				return err
			}

			log.Debugf("Moved pack to temporary path \"%s\"", backupPackPath)
			dropPreInstalled = true
		} else {
			log.Errorf("Pack \"%s\" is already installed here: \"%s\", use the --force-reinstall (-F) flag to force installation", packPath, filepath.Join(Installation.PackRoot, pack.Vendor, pack.Name, pack.GetVersion()))
			return nil
		}
	}

	if pack.isPackID {
		pack.path, err = FindPackURL(pack)
		if err != nil {
			return err
		}
	}

	if err = pack.fetch(timeout); err != nil {
		return err
	}

	// Tells the UI to return right away with the [E]xtract option selected
	ui.Extract = extractEula

	// Unlock the pack (to enable reinstalling) and lock it afterwards
	pack.Unlock()
	defer pack.Lock()

	if err = pack.install(Installation, checkEula || extractEula); err != nil {
		// Just for internal purposes, is not presented as an error to the user
		if err == errs.ErrEula {
			return nil
		}
		if dropPreInstalled {
			log.Error("Error installing pack, reverting temporary pack to original state")
			// Make sure the original directory doesn't exist to avoid moving errors
			if err := os.RemoveAll(fullPackPath); err != nil {
				return err
			}
			if err := utils.MoveFile(backupPackPath, fullPackPath); err != nil {
				return err
			}
		}
		return err
	}

	// Remove the original "temporary" pack
	// Manual removal via os.RemoveAll as "_tmp" is an invalid packPath for RemovePack
	if dropPreInstalled {
		utils.UnsetReadOnlyR(backupPackPath)
		if err := os.RemoveAll(backupPackPath); err != nil {
			return err
		}
		log.Debugf("Succesfully deleted temporary pack \"%s\"", backupPackPath)
	}

	return Installation.touchPackIdx()
}

// RemovePack removes a pack given a pack path
func RemovePack(packPath string, purge bool, timeout int) error {
	log.Debugf("Removing pack \"%v\"", packPath)

	// TODO: by default, remove latest version first
	// if no version is given

	pack, err := preparePack(packPath, true, timeout)
	if err != nil {
		return err
	}

	if pack.isInstalled {
		// TODO: If removing-all is enabled, get rid of the version
		// pack.Version = ""
		pack.Unlock()
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
		pack.Unlock()
		return pack.purge()
	}

	log.Errorf("Pack \"%v\" is not installed", packPath)
	return errs.ErrPackNotInstalled
}

// AddPdsc adds a pack via PDSC file
func AddPdsc(pdscPath string) error {
	log.Infof("Adding pdsc \"%v\"", pdscPath)

	pdsc, err := preparePdsc(pdscPath)
	if err != nil {
		return err
	}

	if err := pdsc.install(Installation); err != nil {
		if err == errs.ErrPdscEntryExists {
			log.Info(err)
			return nil
		}
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

	pdsc, err := preparePdsc(pdscPath)
	if err != nil {
		return err
	}

	// preparePdsc will fill in the full path for this PDSC file path
	// in the URL field, that is not needed for removing
	pdsc.URL = ""

	if err = pdsc.uninstall(Installation); err != nil {
		return err
	}

	if err := Installation.LocalPidx.Write(); err != nil {
		return err
	}

	return Installation.touchPackIdx()
}

// UpdatePublicIndex receives a index path and place it under .Web/index.pidx.
func UpdatePublicIndex(indexPath string, overwrite bool, sparse bool, downloadPdsc bool, concurrency int, timeout int) error {
	// TODO: Remove overwrite when cpackget v1 gets released
	if !overwrite {
		return errs.ErrCannotOverwritePublicIndex
	}

	// For backwards compatibility, allow indexPath to be a file, but ideally it should be empty
	if indexPath == "" {
		indexPath = fmt.Sprintf("%s/index.pidx", strings.TrimSuffix(Installation.PublicIndexXML.URL, "/"))
	}

	log.Debugf("Updating public index with \"%v\"", indexPath)

	var err error

	if strings.HasPrefix(indexPath, "http://") || strings.HasPrefix(indexPath, "https://") {
		if !strings.HasPrefix(indexPath, "https://") {
			log.Warnf("Non-HTTPS url: \"%s\"", indexPath)
		}

		indexPath, err = utils.DownloadFile(indexPath, timeout)
		if err != nil {
			return err
		}
		defer os.Remove(indexPath)
	} else {
		if indexPath != "" {
			if !utils.FileExists(indexPath) && !utils.DirExists(indexPath) {
				return errs.ErrFileNotFound
			}
			fileInfo, err := os.Stat(indexPath)
			if err != nil {
				return err
			}
			if fileInfo.IsDir() {
				return errs.ErrInvalidPublicIndexReference
			}
		}
	}

	pidxXML := xml.NewPidxXML(indexPath)
	if err := pidxXML.Read(); err != nil {
		return err
	}

	utils.UnsetReadOnly(Installation.PublicIndex)
	if err := utils.CopyFile(indexPath, Installation.PublicIndex); err != nil {
		return err
	}
	utils.SetReadOnly(Installation.PublicIndex)

	// Workaround wrapper function to still log errors
	// and not make the linter angry
	massDownloadPdscFiles := func(pdscTag xml.PdscTag, wg *sync.WaitGroup) {
		if err := Installation.downloadPdscFile(pdscTag, wg, timeout); err != nil {
			log.Error(err)
		}
	}
	if downloadPdsc {
		var wg sync.WaitGroup
		log.Info("Downloading all PDSC files available on the public index")
		if err := Installation.PublicIndexXML.Read(); err != nil {
			return err
		}

		pdscTags := Installation.PublicIndexXML.ListPdscTags()
		if len(pdscTags) == 0 {
			log.Info("(no packs in public index)")
			return nil
		}

		queue := concurrency
		for _, pdscTag := range pdscTags {
			if concurrency == 0 {
				if err := Installation.downloadPdscFile(pdscTag, nil, timeout); err != nil {
					log.Error(err)
				}
			} else {
				// Don't queue more downloads than specified
				if queue == 0 {
					if err := Installation.downloadPdscFile(pdscTag, nil, timeout); err != nil {
						log.Error(err)
					}
					wg.Add(concurrency)
					queue = concurrency
				} else {
					wg.Add(1)
					go massDownloadPdscFiles(pdscTag, &wg)
					queue--
				}
			}
		}
	}
	if !sparse {
		var wg sync.WaitGroup
		log.Info("Updating PDSC files of installed packs referenced in index.pidx")
		pdscFiles, err := utils.ListDir(Installation.WebDir, ".pdsc$")
		if err != nil {
			return err
		}

		queue := concurrency
		for _, pdscFile := range pdscFiles {
			log.Debugf("Checking if \"%s\" needs updating", pdscFile)
			pdscXML := xml.NewPdscXML(pdscFile)
			err := pdscXML.Read()
			if err != nil {
				log.Errorf("%s: %v", pdscFile, err)
				continue
			}

			searchTag := xml.PdscTag{
				Vendor: pdscXML.Vendor,
				Name:   pdscXML.Name,
			}

			// Warn the user if the pack is no longer present in index.pidx
			tags := pidxXML.FindPdscTags(searchTag)
			if len(tags) == 0 {
				log.Warnf("The pack %s::%s is no longer present in the updated index.pidx", pdscXML.Vendor, pdscXML.Name)
				continue
			}

			versionInIndex := tags[0].Version
			latestVersion := pdscXML.LatestVersion()
			if versionInIndex != latestVersion {
				log.Infof("%s::%s can be upgraded from \"%s\" to \"%s\"", pdscXML.Vendor, pdscXML.Name, latestVersion, versionInIndex)
				if concurrency == 0 {
					if err := Installation.downloadPdscFile(tags[0], nil, timeout); err != nil {
						log.Error(err)
					}
				} else {
					if queue == 0 {
						if err := Installation.downloadPdscFile(tags[0], nil, timeout); err != nil {
							log.Error(err)
						}
						wg.Add(concurrency)
						queue = concurrency
					} else {
						wg.Add(1)
						go massDownloadPdscFiles(tags[0], &wg)
						queue--
					}
				}
			}
		}
	}

	return nil
}

// ListInstalledPacks generates a list of all packs present in the pack root folder
func ListInstalledPacks(listCached, listPublic bool, listFilter string) error {
	log.Debugf("Listing packs")
	if listPublic {
		if listFilter != "" {
			log.Infof("Listing packs from the public index, filtering by \"%s\"", listFilter)
		} else {
			log.Infof("Listing packs from the public index")
		}

		pdscTags := Installation.PublicIndexXML.ListPdscTags()

		if len(pdscTags) == 0 {
			log.Info("(no packs in public index)")
			return nil
		}

		sort.Slice(pdscTags, func(i, j int) bool {
			return strings.ToLower(pdscTags[i].Key()) < strings.ToLower(pdscTags[j].Key())
		})
		// List all available packs from the index
		for _, pdscTag := range pdscTags {
			logMessage := pdscTag.YamlPackID()
			packFilePath := filepath.Join(Installation.DownloadDir, pdscTag.Key()) + ".pack"

			if Installation.PackIsInstalled(&PackType{PdscTag: pdscTag}) {
				logMessage += " (installed)"
			} else if utils.FileExists(packFilePath) {
				logMessage += " (cached)"
			}

			// To avoid showing empty log lines ("I: ")
			if listFilter == "" || utils.FilterPackID(logMessage, listFilter) != "" {
				log.Info(logMessage)
			}
		}
	} else if listCached {
		if listFilter != "" {
			log.Infof("Listing cached packs, filtering by \"%s\"", listFilter)
		} else {
			log.Infof("Listing cached packs")
		}
		pattern := filepath.Join(Installation.DownloadDir, "*.pack")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}

		if len(matches) == 0 {
			log.Info("(no packs cached)")
			return nil
		}

		sort.Slice(matches, func(i, j int) bool {
			return strings.ToLower(matches[i]) < strings.ToLower(matches[j])
		})
		for _, packFilePath := range matches {
			packFilePath = strings.ReplaceAll(packFilePath, ".pack", "")
			packInfo, err := utils.ExtractPackInfo(packFilePath)
			if err != nil {
				log.Errorf("A pack in the cache folder has malformed pack name: %s", packFilePath)
				return errs.ErrUnknownBehavior
			}

			pdscTag := xml.PdscTag{
				Vendor:  packInfo.Vendor,
				Name:    packInfo.Pack,
				Version: packInfo.Version,
			}

			logMessage := pdscTag.YamlPackID()
			if Installation.PackIsInstalled(&PackType{PdscTag: pdscTag}) {
				logMessage += " (installed)"
			}

			if listFilter == "" || utils.FilterPackID(logMessage, listFilter) != "" {
				log.Info(logMessage)
			}
		}
	} else {
		if listFilter != "" {
			log.Infof("Listing installed packs, filtering by \"%s\"", listFilter)
		} else {
			log.Infof("Listing installed packs")
		}

		type installedPack struct {
			xml.PdscTag
			pdscPath        string
			isPdscInstalled bool
			err             error
		}
		installedPacks := []installedPack{}

		// First, get installed packs from *.pack files
		pattern := filepath.Join(Installation.PackRoot, "*", "*", "*", "*.pdsc")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		for _, match := range matches {
			pdscPath := strings.Replace(match, Installation.PackRoot, "", -1)
			packName, _ := filepath.Split(pdscPath)
			packName = strings.Replace(packName, "/", " ", -1)
			packName = strings.Replace(packName, "\\", " ", -1)
			packName = strings.Trim(packName, " ")
			packName = strings.Replace(packName, " ", ".", -1)

			packNameBits := strings.SplitN(packName, ".", 3)

			pack := installedPack{pdscPath: match}
			pack.Vendor = packNameBits[0]
			pack.Name = packNameBits[1]
			pack.Version = packNameBits[2]
			installedPacks = append(installedPacks, pack)
		}

		// Add packs listed in .Local/local_repository.pidx to the list
		if err := Installation.LocalPidx.Read(); err != nil {
			log.Error(err)
		} else {
			installedPdscs := Installation.LocalPidx.ListPdscTags()
			for _, pdsc := range installedPdscs {
				pack := installedPack{PdscTag: pdsc, isPdscInstalled: true}
				pack.pdscPath = pdsc.URL + pack.Vendor + "/" + pack.Name + ".pdsc"

				parsedURL, err := url.ParseRequestURI(pdsc.URL)
				pack.err = err
				if pack.err != nil {
					installedPacks = append(installedPacks, pack)
					continue
				}

				pack.pdscPath = filepath.Join(utils.CleanPath(parsedURL.Path), pack.Vendor+"."+pack.Name+".pdsc")
				pdscXML := xml.NewPdscXML(pack.pdscPath)
				pack.err = pdscXML.Read()
				if pack.err == nil {
					pack.Version = pdscXML.LatestVersion()
				}
				installedPacks = append(installedPacks, pack)
			}
		}

		if len(installedPacks) == 0 {
			log.Info("(no packs installed)")
			return nil
		}

		numErrors := 0
		printWarning := true
		sort.Slice(installedPacks, func(i, j int) bool {
			return strings.ToLower(installedPacks[i].Key()) < strings.ToLower(installedPacks[j].Key())
		})
		for _, pack := range installedPacks {
			errors := []string{}

			// Validate names
			if !utils.IsPackVendorNameValid(pack.Vendor) {
				errors = append(errors, "vendor")
			}

			if !utils.IsPackNameValid(pack.Name) {
				errors = append(errors, "pack name")
			}

			if !utils.IsPackVersionValid(pack.Version) {
				errors = append(errors, "pack version")
			}

			logMessage := pack.YamlPackID()

			// Print the PDSC path on packs installed via PDSC file
			if pack.isPdscInstalled {
				logMessage += fmt.Sprintf(" (installed via %s)", pack.pdscPath)
			}

			// Append errors to the message, if any
			if len(errors) > 0 {
				numErrors += 1
				logMessage += " - error: " + strings.Join(errors[:], ", ") + " incorrect format"
				if pack.err != nil {
					logMessage += fmt.Sprintf(", %v", pack.err)
				}
				if listFilter != "" && utils.FilterPackID(logMessage, listFilter) != "" {
					printWarning = false
				}
				log.Error(logMessage)
			} else if pack.err != nil {
				numErrors += 1
				logMessage += fmt.Sprintf(" - error: %v", pack.err)
				if listFilter != "" && utils.FilterPackID(logMessage, listFilter) != "" {
					printWarning = false
				}
				log.Error(logMessage)
			} else {
				if listFilter == "" || utils.FilterPackID(logMessage, listFilter) != "" {
					log.Info(logMessage)
				}
			}
		}

		if numErrors > 0 && printWarning {
			log.Warnf("%d error(s) detected", numErrors)
		}
	}

	return nil
}

// FindPackURL uses pack.path as packID and try to find the pack URL
// Finding step are as follows:
// 1. Find pack.Vendor, pack.Name, pack.Version in Installation.PublicIndex
// 1.1. if pack.IsPublic == true
// 1.1.1. read .Web/PDSC file into pdscXML
// 1.1.2. releastTag = pdscXML.FindReleaseTagByVersion(pack.Version)
// 1.1.3. if releaseTag.URL != "", return releaseTag.URL
// 1.1.4. return pdscTag.URL + pack.Vendor + "." + pack.Name + "." + pack.Version + ".pack"
// 1.2. if pack.IsPublic == false
// 1.2.1. if pack's pdsc file not found in Installation.LocalDir then raise errs.ErrPackURLCannotBeFound
// 1.2.2. read .Local/PDSC file into pdscXML
// 1.2.3. releastTag = pdscXML.FindReleaseTagByVersion(pack.Version)
// 1.2.3. if releaseTag == nil then raise ErrPackVersionNotFoundInPdsc
// 1.2.4. if releaseTag.URL != "", return releaseTag.URL
// 1.2.5. return pdscTag.URL + pack.Vendor + "." + pack.Name + "." + pack.Version + ".pack"
func FindPackURL(pack *PackType) (string, error) {
	log.Debugf("Finding URL for \"%v\"", pack.path)

	if pack.IsPublic {
		packPdscFileName := filepath.Join(Installation.WebDir, pack.PdscFileName())
		packPdscXML := xml.NewPdscXML(packPdscFileName)
		if err := packPdscXML.Read(); err != nil {
			return "", err
		}

		// Figures out which pack release to fetch and assign that to pack.targetVersion
		pack.resolveVersionModifier(packPdscXML)

		releaseTag := packPdscXML.FindReleaseTagByVersion(pack.targetVersion)

		// Can't satisfy minimum target version
		if pack.versionModifier == utils.GreaterVersion || pack.versionModifier == utils.GreatestCompatibleVersion {
			if semver.Compare("v"+releaseTag.Version, "v"+pack.Version) < 0 {
				return "", errs.ErrPackVersionNotAvailable
			}
		}
		if pack.versionModifier == utils.GreatestCompatibleVersion {
			if semver.Major("v"+releaseTag.Version) != semver.Major("v"+pack.Version) {
				return "", errs.ErrPackVersionNotAvailable
			}
		}

		if releaseTag == nil {
			return "", errs.ErrPackVersionNotFoundInPdsc
		}
		if releaseTag.URL != "" {
			return releaseTag.URL, nil
		}

		return packPdscXML.PackURL(pack.targetVersion), nil
	}

	// if pack.IsPublic == false, it doesn't mean yet it's an actual non-Public pack, need to check in .Local
	packPdscFileName := filepath.Join(Installation.LocalDir, pack.PdscFileName())
	if !utils.FileExists(packPdscFileName) {
		return "", errs.ErrPackURLCannotBeFound
	}

	packPdscXML := xml.NewPdscXML(packPdscFileName)
	if err := packPdscXML.Read(); err != nil {
		return "", err
	}

	// Figures out which pack release to fetch and assign that to pack.targetVersion
	pack.resolveVersionModifier(packPdscXML)

	releaseTag := packPdscXML.FindReleaseTagByVersion(pack.targetVersion)

	if pack.versionModifier == utils.GreaterVersion {
		if semver.Compare("v"+releaseTag.Version, "v"+pack.Version) < 0 {
			return "", errs.ErrPackVersionNotAvailable
		}
	}
	if pack.versionModifier == utils.GreatestCompatibleVersion {
		if semver.Major("v"+releaseTag.Version) != semver.Major("v"+pack.Version) {
			return "", errs.ErrPackVersionNotAvailable
		}
	}

	if releaseTag == nil {
		return "", errs.ErrPackVersionNotFoundInPdsc
	}

	if releaseTag.URL != "" {
		return releaseTag.URL, nil
	}

	return packPdscXML.PackURL(pack.targetVersion), nil
}

// Installation is a singleton variable that keeps the only reference
// to PacksInstallationType
var Installation *PacksInstallationType

// SetPackRoot sets the working directory of the packs installation
// if create == true, cpackget will try to create needed resources
func SetPackRoot(packRoot string, create bool) error {
	if len(packRoot) == 0 {
		return errs.ErrPackRootNotFound
	}

	packRoot = filepath.Clean(packRoot)
	if !utils.DirExists(packRoot) && !create {
		return errs.ErrPackRootDoesNotExist
	}
	if packRoot == GetDefaultCmsisPackRoot() {
		log.Infof("Using pack root: \"%v\" (default mode - no specific CMSIS_PACK_ROOT chosen)", packRoot)
	} else {
		log.Infof("Using pack root: \"%v\"", packRoot)
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
	Installation.PublicIndexXML = xml.NewPidxXML(Installation.PublicIndex)

	missingDirs := []string{}
	for _, dir := range []string{packRoot, Installation.DownloadDir, Installation.LocalDir, Installation.WebDir} {
		log.Debugf("Making sure \"%v\" exists", dir)
		exists := utils.DirExists(dir)
		if !exists {
			if !create {
				missingDirs = append(missingDirs, dir)
			} else {
				if err := utils.EnsureDir(dir); err != nil {
					return err
				}
			}
		}
	}

	if len(missingDirs) > 0 {
		log.Errorf("Directory(ies) \"%s\" are missing! Was %s initialized correctly?", strings.Join(missingDirs[:], ", "), packRoot)
		return errs.ErrAlreadyLogged
	}

	// Make sure utils.DownloadFile always downloads files to .Download/
	utils.CacheDir = Installation.DownloadDir

	err := Installation.PublicIndexXML.Read()
	if err != nil {
		return err
	}

	err = Installation.LocalPidx.Read()
	if err != nil {
		return err
	}

	LockPackRoot()

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
	utils.UnsetReadOnly(p.PackIdx)
	err := utils.TouchFile(p.PackIdx)
	utils.SetReadOnly(p.PackIdx)
	return err
}

// PackIsInstalled checks whether a given pack is already installed or not
func (p *PacksInstallationType) PackIsInstalled(pack *PackType) bool {
	log.Debugf("Checking if %s is installed", pack.PackIDWithVersion())

	// First make sure there's at least one version of the pack installed
	installationDir := filepath.Join(p.PackRoot, pack.Vendor, pack.Name)
	if !utils.DirExists(installationDir) {
		return false
	}

	// Empty version means any version (pack.VersionModifier == utils.AnyVersion)
	if pack.Version == "" {
		return true
	}

	// Exact version is easy, just find a matching installation folder
	if pack.versionModifier == utils.ExactVersion {
		packDir := filepath.Join(installationDir, pack.Version)
		log.Debugf("Checking if \"%s\" exists", packDir)
		return utils.DirExists(packDir)
	}

	installedVersions := []string{}
	// Gather all versions in local_repository.idx for local .psdc installed packs
	if err := p.LocalPidx.Read(); err != nil {
		log.Warn("Could not read local index")
		return false
	}
	for _, pdsc := range p.LocalPidx.ListPdscTags() {
		if pack.Vendor == pdsc.Vendor && pack.Name == pdsc.Name {
			installedVersions = append(installedVersions, pdsc.Version)
		}
	}

	// Get all remaining versions installed and check if it satisfies the versionModifier condition
	installedDirs, err := utils.ListDir(installationDir, "")
	if err != nil {
		log.Warnf("Could not list installed packs in \"%s\": %v", installationDir, err)
		return false
	}

	for _, path := range installedDirs {
		base := filepath.Base(path)
		installedVersions = append(installedVersions, base)
	}

	// Check if greater version is specified
	if pack.versionModifier == utils.GreaterVersion {
		log.Debugf("Checking for installed packs >=%s", pack.Version)
		for _, version := range installedVersions {
			log.Debugf("- checking if %s >= %s", version, pack.Version)
			if semver.Compare("v"+version, "v"+pack.Version) >= 0 {
				log.Debugf("- found newer version %s", version)
				pack.targetVersion = version
				return true
			}
		}

		log.Debugf("- no version matched")
		return false
	}

	// Check if there is a greater version with same Major number
	if pack.versionModifier == utils.GreatestCompatibleVersion {
		log.Debugf("Checking for installed packs @~%s", pack.Version)
		for _, version := range installedVersions {
			log.Debugf("- checking against: %s", version)
			sameMajor := semver.Major("v"+version) == semver.Major("v"+pack.Version)
			if sameMajor && semver.Compare("v"+version, "v"+pack.Version) >= 0 {
				pack.targetVersion = version
				return true
			}
		}

		return false
	}

	log.Debug("Checking if the latest version is installed")

	// Specified versionModifier == LatestVersion
	// so cpackget needs to know first the latest available
	// version for that pack to then check if it's installed
	var pdscFilePath string
	var pdscLookupDir string
	if pack.IsPublic {
		pdscLookupDir = Installation.WebDir
	} else {
		pdscLookupDir = Installation.LocalDir
	}

	pdscFilePath = filepath.Join(pdscLookupDir, pack.PdscFileName())
	pdscXML := xml.NewPdscXML(pdscFilePath)
	if err := pdscXML.Read(); err != nil {
		log.Debugf("Could not retrieve pack's PDSC file from \"%s\"", pdscFilePath)
		return false
	}

	latestVersion := pdscXML.LatestVersion()
	packDir := filepath.Join(installationDir, latestVersion)
	found := utils.DirExists(packDir)
	if found {
		pack.targetVersion = latestVersion
	}
	return found
}

// packIsPublic checks whether the pack is public or not.
// Being public means a PDSC file is present in ".Web/" folder
func (p *PacksInstallationType) packIsPublic(pack *PackType, timeout int) (bool, error) {
	// lazyly lists all pdsc files in the ".Web/" folder only once
	if p.packs == nil {
		p.packs = make(map[string]bool)
		files, _ := utils.ListDir(p.WebDir, `^.*\.pdsc$`)
		for _, file := range files {
			_, baseFileName := filepath.Split(file)
			p.packs[baseFileName] = true
		}
	}

	_, ok := p.packs[pack.PdscFileName()]
	if ok {
		log.Debugf("Found \"%s\" in \"%s\"", pack.PdscFileName(), p.WebDir)
		return true, nil
	}

	log.Debugf("Not found \"%s\" in \"%s\"", pack.PdscFileName(), p.WebDir)

	// Try to retrieve the packs's PDSC file out of the index.pidx
	searchPdscTag := xml.PdscTag{Vendor: pack.Vendor, Name: pack.Name}
	pdscTags := p.PublicIndexXML.FindPdscTags(searchPdscTag)
	if len(pdscTags) == 0 {
		log.Debugf("Not found \"%s\" tag in \"%s\"", pack.PdscFileName(), p.PublicIndex)
		return false, nil
	}

	// If the pack is being removed, there's no need to get its PDSC file under .Web
	// Same applies to locally sourced packs
	if pack.toBeRemoved || pack.IsLocallySourced {
		return true, nil
	}

	// Sometimes a pidx file might have multiple pdsc tags for same key
	// which is not the case here, so we'll take only the first one
	pdscTag := pdscTags[0]
	return true, p.downloadPdscFile(pdscTag, nil, timeout)
}

// downloadPdscFile takes in a xml.PdscTag containing URL, Vendor and Name of the pack
// so it can be downloaded into .Web/
func (p *PacksInstallationType) downloadPdscFile(pdscTag xml.PdscTag, wg *sync.WaitGroup, timeout int) error {
	// Only change use if it's not a concurrent download
	if wg != nil {
		defer wg.Done()
	}

	basePdscFile := fmt.Sprintf("%s.%s.pdsc", pdscTag.Vendor, pdscTag.Name)
	pdscFilePath := filepath.Join(p.WebDir, basePdscFile)

	log.Debugf("Downloading %s from \"%s\"", basePdscFile, pdscTag.URL)

	pdscFileURL, err := url.Parse(pdscTag.URL)
	if err != nil {
		log.Errorf("Could not parse pdsc url \"%s\": %s", pdscTag.URL, err)
		return errs.ErrAlreadyLogged
	}

	pdscFileURL.Path = path.Join(pdscFileURL.Path, basePdscFile)
	localFileName, err := utils.DownloadFile(pdscFileURL.String(), timeout)
	defer os.Remove(localFileName)

	if err != nil {
		log.Errorf("Could not download \"%s\": %s", pdscFileURL, err)
		return errs.ErrPackPdscCannotBeFound
	}

	utils.UnsetReadOnly(pdscFilePath)
	err = utils.MoveFile(localFileName, pdscFilePath)
	utils.SetReadOnly(pdscFilePath)
	return err
}

// LockPackRoot enable the read-only flag for the pack-root directory
func LockPackRoot() {
	utils.SetReadOnly(Installation.WebDir)
	utils.SetReadOnly(Installation.LocalDir)
	utils.SetReadOnly(Installation.DownloadDir)
	utils.SetReadOnly(Installation.PackRoot)
}

// UnlockPackRoot disable the read-only flag for the pack-root directory
func UnlockPackRoot() {
	utils.UnsetReadOnly(Installation.PackRoot)
	utils.UnsetReadOnly(Installation.WebDir)
	utils.UnsetReadOnly(Installation.LocalDir)
	utils.UnsetReadOnly(Installation.DownloadDir)
}
