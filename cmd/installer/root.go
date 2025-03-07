/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/ui"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/semaphore"
)

const PublicIndex = "index.pidx"
const KeilDefaultPackRoot = "https://www.keil.com/pack/"

// DefaultPublicIndex is the public index to use in "default mode"
const DefaultPublicIndex = KeilDefaultPackRoot + PublicIndex

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
// AddPack installs a pack from the given packPath. It handles various scenarios such as
// checking and extracting EULA, force reinstalling, and handling dependencies.
//
// Parameters:
// - packPath: The path to the pack to be installed.
// - checkEula: If true, the EULA will be checked before installation.
// - extractEula: If true, the EULA will be extracted.
// - forceReinstall: If true, the pack will be reinstalled even if it is already installed.
// - noRequirements: If true, the requirements check and installation will be skipped.
// - timeout: The timeout duration for fetching the pack.
//
// Returns:
// - error: An error if the installation fails, otherwise nil.
func AddPack(packPath string, checkEula, extractEula, forceReinstall, noRequirements, testing bool, timeout int) error {

	isDep := false
	// tag dependency packs with $ for correct logging output
	if strings.TrimPrefix(packPath, "$") != packPath {
		isDep = true
		packPath = packPath[1:]
	}
	pack, err := preparePack(packPath, false, false, false, true, timeout)
	if err != nil {
		return err
	}

	if !isDep {
		if !testing {
			if pack.isPackID || !pack.IsLocallySourced {
				if err := UpdatePublicIndexIfOnline(); err != nil {
					return err
				}
			}

			if err := ReadIndexFiles(); err != nil {
				return err
			}
			// prepare again after update public files
			pack, err = preparePack(packPath, false, false, false, true, timeout)
			if err != nil {
				return err
			}
		}
		log.Infof("Adding pack \"%s\"", packPath)
	}

	dropPreInstalled := false
	fullPackPath := ""
	backupPackPath := ""
	if !extractEula && pack.isInstalled {
		if forceReinstall {

			log.Debugf("Making temporary backup of pack \"%s\"", packPath)

			// Get target pack's full path and move it to a temporary "_tmp" directory
			fullPackPath = filepath.Join(Installation.PackRoot, pack.Vendor, pack.Name, pack.GetVersionNoMeta())
			backupPackPath = fullPackPath + "_tmp"

			if err := utils.MoveFile(fullPackPath, backupPackPath); err != nil {
				return err
			}

			log.Debugf("Moved pack to temporary path \"%s\"", backupPackPath)
			dropPreInstalled = true
		} else {
			log.Errorf("Pack \"%s\" is already installed here: \"%s\", use the --force-reinstall (-F) flag to force installation", packPath, filepath.Join(Installation.PackRoot, pack.Vendor, pack.Name, pack.GetVersionNoMeta()))
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

	// Since we only get the target version here, can only
	// print the message now for dependencies
	if isDep {
		log.Infof("Adding pack %s", pack.Vendor+"."+pack.Name+"."+pack.targetVersion)
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
		log.Debugf("Successfully deleted temporary pack \"%s\"", backupPackPath)
	}

	if !noRequirements {
		log.Debug("installing package requirements")
		err := pack.loadDependencies(true)
		if err != nil {
			return err
		}
		if !pack.RequirementsSatisfied() {
			// Print all dependencies info on one message
			msg := ""
			for _, p := range pack.Requirements.packages {
				if !p.installed {
					msg += utils.FormatPackVersion(p.info) + " "
				}
			}
			if msg != "" {
				log.Infof("Package requirements not satisfied - installing %s", msg)
			}
			for _, req := range pack.Requirements.packages {
				// Recursively install dependencies
				path := req.info[1] + "." + req.info[0] + "." + req.info[2]
				pack, err := preparePack(path, false, false, false, true, timeout)
				if err != nil {
					return err
				}
				if !pack.isInstalled {
					log.Debug("pack has dependencies, installing")
					err := AddPack("$"+path, checkEula, extractEula, forceReinstall, false, false, timeout)
					if err != nil {
						return err
					}
				} else {
					log.Debugf("required pack %s already installed - skipping", path)
				}
			}
		} else {
			log.Debugf("pack has all required dependencies installed (%d packs)", len(pack.Requirements.packages))
		}
	} else {
		log.Debug("skipping requirements checking and installation")
	}

	return Installation.touchPackIdx()
}

// RemovePack removes a specified pack from the installation.
// If the pack is installed, it will be uninstalled. If the purge option is enabled,
// the pack will be completely removed from the system.
//
// Parameters:
//   - packPath: The path to the pack to be removed.
//   - purge: A boolean indicating whether to completely remove the pack.
//   - timeout: An integer specifying the timeout duration for the operation.
//
// Returns:
//   - error: An error if the removal process fails, or nil if successful.
func RemovePack(packPath string, purge, testing bool, timeout int) error {
	log.Debugf("Removing pack \"%v\"", packPath)

	if !testing {
		if err := ReadIndexFiles(); err != nil {
			return err
		}
	}

	// TODO: by default, remove latest version first
	// if no version is given

	pack, err := preparePack(packPath, true, false, false, true, timeout)
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

// AddPdsc adds a PDSC (Pack Description) file to the installation.
// It prepares the PDSC file and installs it. If the PDSC entry already exists,
// it logs the information and returns nil. After installation, it writes the
// local PIDX (Pack Index) and updates the pack index.
//
// Parameters:
//   - pdscPath: The file path to the PDSC file.
//
// Returns:
//   - error: An error if any step fails, otherwise nil.
func AddPdsc(pdscPath string) error {
	log.Infof("Adding pdsc \"%v\"", pdscPath)

	if err := ReadIndexFiles(); err != nil {
		return err
	}

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

// RemovePdsc removes a PDSC (Pack Description) file from the installation.
//
// Parameters:
//   - pdscPath: The file path to the PDSC file to be removed.
//
// Returns:
//   - error: An error if the removal process fails, otherwise nil.
//
// The function performs the following steps:
//  1. Logs the removal action.
//  2. Prepares the PDSC file for removal by calling preparePdsc.
//  3. Clears the URL field of the PDSC file as it is not needed for removal.
//  4. Uninstalls the PDSC file from the installation.
//  5. Writes the updated local Pidx (Pack Index) to disk.
//  6. Touches the Pack Index to update its timestamp.
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

// massDownloadPdscFiles downloads PDSC files based on the provided PDSC tag.
// It calls downloadPdscFile for each PDSC file in the tag and logs errors if they occur.
// If skipInstalledPdscFiles is true, already installed PDSC files will be skipped.
// The timeout parameter specifies the maximum duration (in seconds) for the download operation.
//
// Parameters:
//   - pdscTag: The PDSC tag containing information about the files to be downloaded.
//   - skipInstalledPdscFiles: A boolean flag indicating whether to skip already installed PDSC files.
//   - timeout: An integer specifying the timeout duration in seconds.
//
// Example:
//
//	massDownloadPdscFiles(pdscTag, true, 30)
func massDownloadPdscFiles(pdscTag xml.PdscTag, skipInstalledPdscFiles bool, timeout int) {
	if err := Installation.downloadPdscFile(pdscTag, skipInstalledPdscFiles, timeout); err != nil {
		log.Error(err)
	}
}

// UpdatePack updates the specified pack or all installed packs if packPath is empty.
// It checks for EULA acceptance and installs any required dependencies unless noRequirements is true.
//
// Parameters:
//   - packPath: The path or identifier of the pack to update. If empty, all installed packs are updated.
//   - checkEula: A boolean indicating whether to check for EULA acceptance.
//   - noRequirements: A boolean indicating whether to skip checking and installing dependencies.
//   - timeout: An integer specifying the timeout duration for operations.
//
// Returns:
//   - error: An error if the update process fails, otherwise nil.
func UpdatePack(packPath string, checkEula, noRequirements bool, timeout int) error {

	if packPath == "" {
		installedPacks, err := findInstalledPacks(false, true)
		if err != nil {
			return err
		}
		for _, installedPack := range installedPacks {
			err = UpdatePack(installedPack.Vendor+"."+installedPack.Name, checkEula, noRequirements, timeout)
			if err != nil {
				log.Error(err)
			}
		}
		return nil
	}
	pack, err := preparePack(packPath, false, true, true, true, timeout)
	if err != nil {
		return err
	}

	if !pack.IsPublic || pack.isInstalled {
		if !pack.isInstalled {
			log.Infof("Pack \"%s\" is not installed", packPath)
		}
		return nil
	}

	log.Infof("Updating pack \"%s\"", packPath)

	if pack.isPackID {
		pack.path, err = FindPackURL(pack)
		if err != nil {
			return err
		}
	}

	if err = pack.fetch(timeout); err != nil {
		return err
	}

	// Unlock the pack (to enable reinstalling) and lock it afterwards
	pack.Unlock()
	defer pack.Lock()

	if err = pack.install(Installation, checkEula); err != nil {
		// Just for internal purposes, is not presented as an error to the user
		if err == errs.ErrEula {
			return nil
		}
		return err
	}

	if !noRequirements {
		log.Debug("installing package requirements")
		err := pack.loadDependencies(true)
		if err != nil {
			return err
		}
		if !pack.RequirementsSatisfied() {
			// Print all dependencies info on one message
			msg := ""
			for _, p := range pack.Requirements.packages {
				if !p.installed {
					msg += utils.FormatPackVersion(p.info) + " "
				}
			}
			if msg != "" {
				log.Infof("Package requirements not satisfied - installing %s", msg)
			}
			for _, req := range pack.Requirements.packages {
				// Recursively install dependencies
				path := req.info[1] + "." + req.info[0] + "." + req.info[2]
				pack, err := preparePack(path, false, false, false, true, timeout)
				if err != nil {
					return err
				}
				if !pack.isInstalled {
					log.Debug("pack has dependencies, installing")
					err := AddPack("$"+path, checkEula, false, false, false, false, timeout)
					if err != nil {
						return err
					}
				} else {
					log.Debugf("required pack %s already installed - skipping", path)
				}
			}
		} else {
			log.Debugf("pack has all required dependencies installed (%d packs)", len(pack.Requirements.packages))
		}
	} else {
		log.Debug("skipping requirements checking and installation")
	}

	return Installation.touchPackIdx()
}

// CheckConcurrency adjusts the given concurrency level based on the maximum
// number of CPU cores available. If the provided concurrency is greater than
// 1, it ensures that it does not exceed the maximum number of CPU cores. If
// the provided concurrency is less than or equal to 1, it sets the concurrency
// to 0.
//
// Parameters:
//
//	concurrency - The desired level of concurrency.
//
// Returns:
//
//	The adjusted level of concurrency, which will be between 0 and the maximum
//	number of CPU cores.
func CheckConcurrency(concurrency int) int {
	maxWorkers := runtime.GOMAXPROCS(0)

	if concurrency > 1 {
		if concurrency > maxWorkers {
			concurrency = maxWorkers
		}
	} else {
		concurrency = 0
	}

	return concurrency
}

// DownloadPDSCFiles downloads all PDSC files available on the public index.
// It reads the public index XML and lists all PDSC tags. If there are no packs
// in the public index, it logs this information and returns nil.
//
// If progress encoding is enabled, it logs the number of PDSC files and the
// public index.
//
// The function supports concurrent downloads, controlled by the `concurrency`
// parameter. If `concurrency` is 0, downloads are performed sequentially.
// Otherwise, a semaphore is used to limit the number of concurrent downloads.
//
// Parameters:
// - skipInstalledPdscFiles: If true, skips downloading PDSC files that are already installed.
// - concurrency: The number of concurrent downloads to allow. If 0, downloads are sequential.
// - timeout: The timeout for each download operation.
//
// Returns:
// - An error if there is an issue reading the public index XML or acquiring the semaphore.
func DownloadPDSCFiles(skipInstalledPdscFiles bool, concurrency int, timeout int) error {
	log.Info("Downloading all PDSC files available on the public index")
	if err := Installation.PublicIndexXML.Read(); err != nil {
		return err
	}

	pdscTags := Installation.PublicIndexXML.ListPdscTags()
	numPdsc := len(pdscTags)
	if numPdsc == 0 {
		log.Info("(no packs in public index)")
		return nil
	}

	if utils.GetEncodedProgress() {
		log.Infof("[J%d:F\"%s\"]", numPdsc, Installation.PublicIndex)
	}

	ctx := context.TODO()
	concurrency = CheckConcurrency(concurrency)
	sem := semaphore.NewWeighted(int64(concurrency))

	for _, pdscTag := range pdscTags {
		if concurrency == 0 {
			massDownloadPdscFiles(pdscTag, skipInstalledPdscFiles, timeout)
		} else {
			if err := sem.Acquire(ctx, 1); err != nil {
				log.Errorf("Failed to acquire semaphore: %v", err)
				break
			}

			go func(pdscTag xml.PdscTag) {
				defer sem.Release(1)
				massDownloadPdscFiles(pdscTag, skipInstalledPdscFiles, timeout)
			}(pdscTag)
		}
	}
	if concurrency > 1 {
		if err := sem.Acquire(ctx, int64(concurrency)); err != nil {
			log.Errorf("Failed to acquire semaphore: %v", err)
		}
	}

	return nil
}

// UpdateInstalledPDSCFiles updates the PDSC files of installed packs referenced in the public index.
// It checks if the PDSC files need updating and downloads the latest versions if necessary.
//
// Parameters:
// - pidxXML: A pointer to the PidxXML structure containing the public index data.
// - concurrency: The number of concurrent downloads allowed. If set to 0, downloads are performed sequentially.
// - timeout: The timeout duration for downloading PDSC files.
//
// Returns:
// - error: An error if any issue occurs during the update process.
//
// The function performs the following steps:
// 1. Lists all PDSC files in the installation web directory.
// 2. For each PDSC file, it reads the file and checks if the pack is still present in the public index.
// 3. If the pack is no longer present, it deletes the PDSC file.
// 4. If the pack is present but has a newer version in the public index, it downloads the latest version.
// 5. Lists all PDSC files in the local directory and repeats the update process for these files.
func UpdateInstalledPDSCFiles(pidxXML *xml.PidxXML, concurrency int, timeout int) error {
	log.Info("Updating PDSC files of installed packs referenced in " + PublicIndex)
	pdscFiles, err := utils.ListDir(Installation.WebDir, ".pdsc$")
	if err != nil {
		return err
	}

	numPdsc := len(pdscFiles)
	if utils.GetEncodedProgress() {
		log.Infof("[J%d:F\"%s\"]", numPdsc, Installation.PublicIndex)
	}

	ctx := context.TODO()
	concurrency = CheckConcurrency(concurrency)
	sem := semaphore.NewWeighted(int64(concurrency))

	for _, pdscFile := range pdscFiles {
		log.Debugf("Checking if \"%s\" needs updating", pdscFile)
		pdscXML := xml.NewPdscXML(pdscFile)
		err := pdscXML.Read()
		if err != nil {
			log.Errorf("%s: %v", pdscFile, err)
			utils.UnsetReadOnly(pdscFile)
			os.Remove(pdscFile)
			continue
		}

		searchTag := xml.PdscTag{
			Vendor: pdscXML.Vendor,
			Name:   pdscXML.Name,
		}

		// Warn the user if the pack is no longer present in index.pidx
		tags := pidxXML.FindPdscTags(searchTag)
		if len(tags) == 0 {
			log.Warnf("The pack %s::%s is no longer present in the updated \"%s\", deleting PDSC file \"%v\"", pdscXML.Vendor, pdscXML.Name, PublicIndex, pdscFile)
			utils.UnsetReadOnly(pdscFile)
			os.Remove(pdscFile)
			continue
		}

		versionInIndex := tags[0].Version
		latestVersion := pdscXML.LatestVersion()
		if versionInIndex != latestVersion {
			log.Infof("%s::%s can be upgraded from \"%s\" to \"%s\"", pdscXML.Vendor, pdscXML.Name, latestVersion, versionInIndex)

			if concurrency == 0 {
				massDownloadPdscFiles(tags[0], false, timeout)
			} else {
				if err := sem.Acquire(ctx, 1); err != nil {
					log.Errorf("Failed to acquire semaphore: %v", err)
					break
				}

				pdscTag := tags[0]
				go func(pdscTag xml.PdscTag) {
					defer sem.Release(1)
					massDownloadPdscFiles(pdscTag, false, timeout)
				}(pdscTag)
			}
		}
	}

	if concurrency > 1 {
		if err := sem.Acquire(ctx, int64(concurrency)); err != nil {
			log.Errorf("Failed to acquire semaphore: %v", err)
		}
	}

	pdscFiles, err = utils.ListDir(Installation.LocalDir, ".pdsc$")
	if err != nil {
		return err
	}

	numPdsc = len(pdscFiles)
	if utils.GetEncodedProgress() {
		log.Infof("[J%d:F\"%s\"]", numPdsc, Installation.LocalDir)
	}

	for _, pdscFile := range pdscFiles {
		log.Debugf("Checking if \"%s\" needs updating", pdscFile)
		pdscXML := xml.NewPdscXML(pdscFile)
		err := pdscXML.Read()
		if err != nil {
			log.Errorf("%s: %v", pdscFile, err)
			utils.UnsetReadOnly(pdscFile)
			os.Remove(pdscFile)
			continue
		}
		if pdscXML.URL == "" {
			continue
		}
		originalLatestVersion := pdscXML.LatestVersion()

		var pdscTag xml.PdscTag
		pdscTag.Name = pdscXML.Name
		pdscTag.Vendor = pdscXML.Vendor
		pdscTag.URL = pdscXML.URL
		if err := Installation.loadPdscFile(pdscTag, timeout); err != nil {
			log.Error(err)
		}

		pdscXML = xml.NewPdscXML(pdscFile)
		err = pdscXML.Read()
		if err != nil {
			log.Errorf("%s: %v", pdscFile, err)
			utils.UnsetReadOnly(pdscFile)
			os.Remove(pdscFile)
			continue
		}
		latestVersion := pdscXML.LatestVersion()
		if originalLatestVersion != latestVersion {
			log.Infof("%s::%s can be upgraded from \"%s\" to \"%s\"", pdscXML.Vendor, pdscXML.Name, originalLatestVersion, latestVersion)
		}
	}

	return nil
}

// GetIndexPath returns the index path based on the provided indexPath argument.
// If indexPath is empty, it defaults to the public index XML URL from the Installation configuration.
// Logs the index path if encoded progress is not enabled.
// Warns if the index path uses a non-HTTPS URL.
//
// Parameters:
//   - indexPath: The initial index path to be processed.
//
// Returns:
//   - string: The final index path.
//   - error: An error if any occurred during processing.
func GetIndexPath(indexPath string) (string, error) {
	if indexPath == "" {
		indexPath = strings.TrimSuffix(Installation.PublicIndexXML.URL, "/")
	}

	if !utils.GetEncodedProgress() {
		log.Infof("Using path: \"%v\"", indexPath)
	}

	var err error

	if strings.HasPrefix(indexPath, "http://") || strings.HasPrefix(indexPath, "https://") {
		if !strings.HasPrefix(indexPath, "https://") {
			log.Warnf("Non-HTTPS url: \"%s\"", indexPath)
		}
	}

	return indexPath, err
}

func UpdatePublicIndexIfOnline() error {
	// If public index already exists then first check if online, then its timestamp
	// if we are online and it is too old then download a current version
	if utils.FileExists(Installation.PublicIndex) {
		err := utils.CheckConnection(DefaultPublicIndex, 0)
		if err != nil && errors.Unwrap(err) != errs.ErrOffline {
			return err
		}
		if errors.Unwrap(err) != errs.ErrOffline {
			v := viper.New()
			var updateConf updateCfg
			err = Installation.checkUpdateCfg(v, &updateConf)
			if err != nil {
				UnlockPackRoot()
				err1 := UpdatePublicIndex(DefaultPublicIndex, true, false, false, false, 0, 0)
				if err1 != nil {
					return err1
				}
				_ = Installation.updateUpdateCfg(v, &updateConf)
			}
		} else {
			log.Debug("Offline mode: Skipping public index update")
		}
	}
	// if public index does not or not yet exist then download without check
	if !utils.FileExists(Installation.PublicIndex) {
		UnlockPackRoot()
		err1 := UpdatePublicIndex(DefaultPublicIndex, true, false, false, false, 0, 0)
		if err1 != nil {
			return err1
		}
		v := viper.New()
		var updateConf updateCfg
		updateConf.Default.Auto = true
		_ = Installation.updateUpdateCfg(v, &updateConf) // create the update config file
	}
	return nil
}

// UpdatePublicIndex updates the public index file from a given path or URL.
//
// Parameters:
//   - indexPath: The path or URL to the public index file. If empty, the default public index URL is used.
//   - overwrite: A boolean flag to indicate whether to overwrite the existing public index. This will be removed in future versions.
//   - sparse: A boolean flag to indicate whether to perform a sparse update.
//   - downloadPdsc: A boolean flag to indicate whether to download PDSC files.
//   - downloadRemainingPdscFiles: A boolean flag to indicate whether to download remaining PDSC files.
//   - concurrency: The number of concurrent operations allowed.
//   - timeout: The timeout duration for network operations.
//
// Returns:
//   - error: An error if the update fails, otherwise nil.
func UpdatePublicIndex(indexPath string, overwrite bool, sparse bool, downloadPdsc bool, downloadRemainingPdscFiles bool, concurrency int, timeout int) error {
	// TODO: Remove overwrite when cpackget v1 gets released
	if !overwrite {
		return errs.ErrCannotOverwritePublicIndex
	}

	// For backwards compatibility, allow indexPath to be a file, but ideally it should be empty
	if indexPath == "" {
		indexPath = strings.TrimSuffix(Installation.PublicIndexXML.URL, "/") + "/" + PublicIndex
	}

	var err error

	if strings.HasPrefix(indexPath, "http://") || strings.HasPrefix(indexPath, "https://") {
		if !strings.HasPrefix(indexPath, "https://127.0.0.1") {
			err = utils.CheckConnection(indexPath, 0)
			if err != nil && errors.Unwrap(err) == errs.ErrOffline {
				return err
			}
		}
	}

	log.Infof("Updating public index")
	log.Debugf("Updating public index with \"%v\"", indexPath)

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
				return errs.ErrFileNotFoundUseInit
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

	if downloadPdsc {
		err = DownloadPDSCFiles(false, concurrency, timeout)
		if err != nil {
			return err
		}
	}

	if !sparse {
		err = UpdateInstalledPDSCFiles(pidxXML, concurrency, timeout)
		if err != nil {
			return err
		}
	}

	if downloadRemainingPdscFiles {
		err = DownloadPDSCFiles(true, concurrency, timeout)
		if err != nil {
			return err
		}
	}

	return Installation.touchPackIdx()
}

// installedPack represents a package that has been installed.
// It embeds xml.PdscTag and includes additional fields to track
// the PDSC file path, installation status, and any errors encountered.
type installedPack struct {
	xml.PdscTag
	pdscPath        string
	isPdscInstalled bool
	err             error
}

// findInstalledPacks retrieves a list of installed packs based on the provided options.
// It searches for installed packs in the specified directory and optionally includes
// local packs and removes duplicates.
//
// Parameters:
//   - addLocalPacks: If true, includes packs listed in the local repository index.
//   - removeDuplicates: If true, removes duplicate packs from the list.
//
// Returns:
//   - A slice of installedPack containing the details of the installed packs.
//   - An error if any issues occur during the retrieval process.
func findInstalledPacks(addLocalPacks, removeDuplicates bool) ([]installedPack, error) {
	installedPacks := []installedPack{}

	// First, get installed packs from *.pack files
	pattern := filepath.Join(Installation.PackRoot, "*", "*", "*", "*.pdsc")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
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

	if addLocalPacks {
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
	}
	if removeDuplicates {
		sort.Slice(installedPacks, func(i, j int) bool {
			vi := strings.ToLower(installedPacks[i].Vendor)
			vj := strings.ToLower(installedPacks[j].Vendor)
			if vi == vj {
				ai := strings.ToLower(installedPacks[i].Name)
				aj := strings.ToLower(installedPacks[j].Name)
				if ai == aj {
					return installedPacks[i].Version > installedPacks[j].Version
				}
				return ai < aj
			}
			return vi < vj
		})
		noDupInstalledPacks := []installedPack{}
		if len(installedPacks) > 0 {
			last := 0
			noDupInstalledPacks = append(noDupInstalledPacks, installedPacks[0])
			for i, installedPack := range installedPacks {
				if i > 0 {
					if !(installedPacks[last].Vendor == installedPack.Vendor && installedPacks[last].Name == installedPack.Name) {
						noDupInstalledPacks = append(noDupInstalledPacks, installedPack)
						last = i
					}
				}
			}
		}
		return noDupInstalledPacks, nil
	}
	return installedPacks, nil
}

// ListInstalledPacks lists the installed packs based on the provided filters.
// It can list packs from the public index, cached packs, installed packs with updates,
// and installed packs with dependencies.
//
// Parameters:
//   - listCached: If true, lists the cached packs.
//   - listPublic: If true, lists the packs from the public index.
//   - listUpdates: If true, lists the installed packs with available updates.
//   - listRequirements: If true, lists the installed packs with dependencies.
//   - listFilter: A string to filter the packs by.
//
// Returns:
//   - error: An error if any occurs during the listing process.
func ListInstalledPacks(listCached, listPublic, listUpdates, listRequirements, testing bool, listFilter string) error {
	log.Debugf("Listing packs")

	if !testing {
		if err := ReadIndexFiles(); err != nil {
			return err
		}
	}
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

			if Installation.PackIsInstalled(&PackType{PdscTag: pdscTag}, false) {
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
			if Installation.PackIsInstalled(&PackType{PdscTag: pdscTag}, false) {
				logMessage += " (installed)"
			}

			if listFilter == "" || utils.FilterPackID(logMessage, listFilter) != "" {
				log.Info(logMessage)
			}
		}
	} else {
		if listUpdates {
			if listFilter != "" {
				log.Infof("Listing installed packs with available update, filtering by \"%s\"", listFilter)
			} else {
				log.Infof("Listing installed packs with available update")
			}
		} else {
			if listRequirements {
				log.Info("Listing installed packs with dependencies")
			} else {
				if listFilter != "" {
					log.Infof("Listing installed packs, filtering by \"%s\"", listFilter)
				} else {
					log.Infof("Listing installed packs")
				}
			}
		}

		installedPacks, err := findInstalledPacks(!listUpdates, listUpdates)
		if err != nil {
			return err
		}

		if len(installedPacks) == 0 {
			log.Info("(no packs installed)")
			return nil
		}

		numErrors := 0
		printWarning := true
		if !listUpdates {
			sort.Slice(installedPacks, func(i, j int) bool {
				return strings.ToLower(installedPacks[i].Key()) < strings.ToLower(installedPacks[j].Key())
			})
		}
		for _, pack := range installedPacks {
			logMessage := pack.YamlPackID()
			// List installed packs and their dependencies
			p, err := preparePack(pack.Key(), false, listUpdates, listUpdates, false, 0)
			if err == nil {
				if listUpdates && !p.IsPublic {
					continue // ignore local packs
				}
				if listUpdates && p.isInstalled {
					continue // ignore already installed packs of newest version
				}
			}
			if listUpdates {
				logMessage = strings.Replace(logMessage, "@", " can be updated from \"", 1)
				logMessage += "\" to \"" + p.targetVersion + "\""
			}
			if listRequirements {
				p.Pdsc = xml.NewPdscXML(pack.pdscPath)
				if err := p.Pdsc.Read(); err != nil {
					return err
				}
				if err := p.loadDependencies(false); err != nil {
					return err
				}
				if len(p.Requirements.packages) > 0 {
					logMessage += " - "
					for _, req := range p.Requirements.packages {
						logMessage += utils.FormatPackVersion(req.info)
						if req.installed {
							logMessage += " (installed) "
						} else {
							logMessage += " (missing) "
						}
					}
				} else {
					// Not interested in packs with no dependencies
					continue
				}
			}
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

			// Print the PDSC path on packs installed via PDSC file
			if pack.isPdscInstalled && !listRequirements {
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
//
// The function resolves the version modifier to determine the correct version of the pack to fetch.
// It then checks the release tag for the specified version and returns the URL if found.
// If the URL is not found or the version is not available, it returns an appropriate error.
//
// Parameters:
//   - pack: A pointer to the PackType struct representing the pack to find the URL for.
//
// Returns:
//   - string: The URL of the pack if found.
//   - error: An error if the URL cannot be found or if there are issues with the pack's version.
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
		if releaseTag == nil {
			return "", errs.ErrPackVersionNotFoundInPdsc
		}

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
		if pack.versionModifier == utils.PatchVersion {
			if semver.MajorMinor("v"+releaseTag.Version) != semver.MajorMinor("v"+pack.Version) {
				return "", errs.ErrPackVersionNotAvailable
			}
		}
		if pack.versionModifier == utils.RangeVersion {
			found := false
			for _, version := range packPdscXML.AllReleases() {
				if utils.SemverCompareRange(version, pack.Version) == 0 {
					found = true
					break
				}
			}
			if !found {
				return "", errs.ErrPackVersionNotAvailable
			}
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
	if pack.versionModifier == utils.PatchVersion {
		if semver.MajorMinor("v"+releaseTag.Version) != semver.MajorMinor("v"+pack.Version) {
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

// SetPackRoot sets the root directory for pack installation, ensuring necessary directories exist
//
// Parameters:
//   - packRoot: The root directory for pack installation.
//   - create: If true, missing directories will be created.
//
// Returns:
//   - error: An error if the pack root is invalid, directories cannot be created.
func SetPackRoot(packRoot string, create bool) error {
	if len(packRoot) == 0 {
		return errs.ErrPackRootNotFound
	}

	packRoot = filepath.Clean(packRoot)
	if !utils.DirExists(packRoot) && !create {
		return errs.ErrPackRootDoesNotExist
	}

	checkConnection := viper.GetBool("check-connection") // TODO: never set
	if checkConnection && !utils.GetEncodedProgress() {
		if packRoot == GetDefaultCmsisPackRoot() {
			log.Infof("Using pack root: \"%v\" (default mode - no specific CMSIS_PACK_ROOT chosen)", packRoot)
		} else {
			log.Infof("Using pack root: \"%v\"", packRoot)
		}
	}

	Installation = &PacksInstallationType{
		PackRoot:    packRoot,
		DownloadDir: filepath.Join(packRoot, ".Download"),
		LocalDir:    filepath.Join(packRoot, ".Local"),
		WebDir:      filepath.Join(packRoot, ".Web"),
	}
	Installation.LocalPidx = xml.NewPidxXML(filepath.Join(Installation.LocalDir, "local_repository.pidx"))
	Installation.PackIdx = filepath.Join(packRoot, "pack.idx")
	Installation.PublicIndex = filepath.Join(Installation.WebDir, PublicIndex)
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

	LockPackRoot()
	return nil
}

// ReadIndexFiles reads the public and local index files.
// If the installation is in read-only mode, it temporarily unlocks the pack root
// to allow reading the files, and then locks it again.
// It returns an error if reading either of the index files fails.
func ReadIndexFiles() error {
	if Installation.ReadOnly {
		UnlockPackRoot()
		defer LockPackRoot()
	}
	err := Installation.PublicIndexXML.Read()
	if err != nil {
		return err
	}

	err = Installation.LocalPidx.Read()
	if err != nil {
		return err
	}

	return nil
}

// PacksInstallationType is the struct that manages Open-CMSIS-Pack installation/deletion.
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

	ReadOnly bool
}

// updateCfg represents the content of "update.cfg" file.
// - Date: a string representing the date of the last update.
// - Auto: a boolean indicating whether automatic updates are enabled.
type updateCfg struct {
	Default struct {
		Date string
		Auto bool
	}
}

// checkUpdateCfg reads the update configuration file, and checks if the index.pidx file is older than one day.
//
// Parameters:
//   - v: A pointer to a viper.Viper instance used to read the configuration file.
//   - conf: A pointer to an updateCfg struct where the configuration will be unmarshaled.
//
// Returns:
//   - error: An error if there is an issue reading the configuration file, unmarshaling it, or if the index.pidx file is older than one day.
func (p *PacksInstallationType) checkUpdateCfg(v *viper.Viper, conf *updateCfg) error {
	v.SetConfigFile(filepath.Join(p.WebDir, "update.cfg"))
	v.SetConfigType("ini")
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	if err := v.Unmarshal(conf); err != nil {
		return err
	}
	if t, err := time.Parse("2-1-2006", conf.Default.Date); err != nil {
		return err
	} else {
		if time.Since(t).Hours() > 24 { // index.pidx older than 1 day
			return errs.ErrIndexTooOld
		}
	}
	return nil
}

// updateUpdateCfg updates the update configuration file with the current date and auto-update settings.
// It uses the provided viper instance and updateCfg struct to set the configuration values.
// The function writes the configuration directly to the file, bypassing viper's WriteConfig method
// due to issues with viper's handling of the configuration file type.
//
// Parameters:
//   - v: A pointer to a viper.Viper instance for configuration management (unused in this function).
//   - conf: A pointer to an updateCfg struct containing the configuration values to be written.
//
// Returns:
//   - error: An error if any occurs during the file operations, otherwise nil.
func (p *PacksInstallationType) updateUpdateCfg(v *viper.Viper, conf *updateCfg) error {
	_ = v
	// v.SetConfigFile(filepath.Join(p.WebDir, "update.cfg"))
	// v.SetConfigType("ini")
	// if err := v.ReadInConfig(); err != nil {
	// 	return err
	// }
	// if err := v.Unmarshal(&updateConf); err != nil {
	// 	return err
	// }
	conf.Default.Date = time.Now().Local().Format("2-1-2006")
	//	v.SetConfigFile(filepath.Join(p.WebDir, "update.cfg")) // have to force type with extension, this does not work
	// if err := v.WriteConfig(); err != nil { // does not use changed conf
	// 	return err
	// }
	// So, we do it by ourselves. Viper does not work as expected
	flags := os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	f, err := os.OpenFile(filepath.Join(p.WebDir, "update.cfg"), flags, os.FileMode(0o644))
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString("Date=" + conf.Default.Date + "\n"); err != nil {
		return err
	}
	if _, err := f.WriteString("Auto="); err != nil {
		return err
	}
	if conf.Default.Auto {
		if _, err := f.WriteString("true\n"); err != nil {
			return err
		}
	} else {
		if _, err := f.WriteString("false\n"); err != nil {
			return err
		}
	}

	return f.Sync()
}

// touchPackIdx updates the timestamp of the PackIdx file to the current time.
// If the skip touch flag is set, the function returns immediately without making any changes.
// The function temporarily removes the read-only attribute from the PackIdx file,
// updates its timestamp, and then restores the read-only attribute.
//
// Returns an error if there is an issue updating the timestamp of the PackIdx file.
func (p *PacksInstallationType) touchPackIdx() error {
	if utils.GetSkipTouch() {
		return nil
	}

	utils.UnsetReadOnly(p.PackIdx)
	err := utils.TouchFile(p.PackIdx)
	utils.SetReadOnly(p.PackIdx)
	return err
}

// PackIsInstalled checks if a specific pack is installed based on the provided
// pack information and version constraints.
//
// Parameters:
// - pack: A pointer to the PackType struct containing information about the pack to check.
// - noLocal: A boolean flag indicating whether to skip checking local repository index.
//
// Returns:
// - bool: True if the pack is installed and meets the version constraints, otherwise false.
//
// The function performs the following checks:
// 1. Verifies if there's at least one version of the pack installed in the installation directory.
// 2. If the pack version is empty, it returns true indicating any version is acceptable.
// 3. If the version modifier is ExactVersion, it checks for an exact version match.
// 4. If noLocal is false, it gathers all versions from the local repository index.
// 5. Lists all installed versions in the installation directory.
// 6. Depending on the version modifier, it checks for:
//   - GreaterVersion: Any installed version greater than or equal to the specified version.
//   - GreatestCompatibleVersion: Any installed version with the same major number and greater than or equal to the specified version.
//   - PatchVersion: Any installed version with the same major and minor numbers and greater than or equal to the specified version.
//   - RangeVersion: Any installed version within the specified version range.
//
// 7. If the version modifier is LatestVersion, it retrieves the latest available version from the PDSC file and checks if it's installed.
func (p *PacksInstallationType) PackIsInstalled(pack *PackType, noLocal bool) bool {
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
		packDir := filepath.Join(installationDir, pack.GetVersionNoMeta())
		log.Debugf("Checking if \"%s\" exists", packDir)
		return utils.DirExists(packDir)
	}
	installedVersions := []string{}
	if noLocal {
		if pack.isPackID || !pack.IsLocallySourced {
			if err := UpdatePublicIndexIfOnline(); err != nil {
				return false
			}
		}
		if err := ReadIndexFiles(); err != nil {
			return false
		}
	} else {
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
			if utils.SemverCompare(version, pack.Version) >= 0 {
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
		log.Debugf("Checking for installed packs @^%s", pack.Version)
		for _, version := range installedVersions {
			log.Debugf("- checking against: %s", version)
			sameMajor := semver.Major("v"+version) == semver.Major("v"+pack.Version)
			if sameMajor && utils.SemverCompare(version, pack.Version) >= 0 {
				pack.targetVersion = version
				return true
			}
		}
		return false
	}

	// Check if there is a greater version with same Major and Minor number
	if pack.versionModifier == utils.PatchVersion {
		log.Debugf("Checking for installed packs @~%s", pack.Version)
		for _, version := range installedVersions {
			log.Debugf("- checking against: %s", version)
			sameMajorMinor := semver.MajorMinor("v"+version) == semver.MajorMinor("v"+pack.Version)
			if sameMajorMinor && utils.SemverCompare(version, pack.Version) >= 0 {
				pack.targetVersion = version
				return true
			}
		}
		return false
	}

	if pack.versionModifier == utils.RangeVersion {
		log.Debugf("Checking for installed packs %s", utils.FormatVersions(pack.Version))
		for _, version := range installedVersions {
			log.Debugf("- checking against: %s", version)
			if utils.SemverCompareRange(version, pack.Version) == 0 {
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
	pack.targetVersion = latestVersion
	return found
}

// packIsPublic checks whether the pack is public or not.
// Being public means a PDSC file is present in ".Web/" folder
// It first checks the local cache of PDSC files in the ".Web/" directory.
// If the pack is not found locally, it searches for the pack's PDSC file in the public index.
// If the pack is marked for removal or is locally sourced, it is considered public without further checks.
//
// Parameters:
//   - pack: The pack to check for public availability.
//   - timeout: The timeout duration for downloading the PDSC file if needed.
//
// Returns:
//   - bool: True if the pack is public, false otherwise.
//   - error: An error if there was an issue downloading the PDSC file.
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
	return true, p.downloadPdscFile(pdscTag, false, timeout)
}

// downloadPdscFile downloads a PDSC file based on the provided pdscTag and saves it to the specified location.
// If skipInstalledPdscFiles is true and the file already exists, the function will skip the download.
// The function also handles switching to a cache URL if necessary and ensures the file is moved to the correct location.
//
// Parameters:
//   - pdscTag: An xml.PdscTag containing the vendor, name, and URL of the PDSC file.
//   - skipInstalledPdscFiles: A boolean indicating whether to skip downloading if the file already exists.
//   - timeout: An integer specifying the timeout duration for the download.
//
// Returns:
//   - error: An error if any issues occur during the download or file operations.
func (p *PacksInstallationType) downloadPdscFile(pdscTag xml.PdscTag, skipInstalledPdscFiles bool, timeout int) error {
	basePdscFile := fmt.Sprintf("%s.%s.pdsc", pdscTag.Vendor, pdscTag.Name)
	pdscFilePath := filepath.Join(p.WebDir, basePdscFile)

	if skipInstalledPdscFiles {
		if utils.FileExists(pdscFilePath) {
			log.Debugf("File already exists: \"%s\"", pdscFilePath)
			return nil
		}
		log.Debugf("File does not exist and will be copied: \"%s\"", pdscFilePath)
	}

	pdscURL := pdscTag.URL

	// switch  to keil.com cache for PDSC file
	if pdscURL != KeilDefaultPackRoot && Installation.PublicIndexXML.URL == KeilDefaultPackRoot {
		log.Debugf("Switching to cache: \"%s\"", KeilDefaultPackRoot)
		pdscURL = KeilDefaultPackRoot
	}

	log.Debugf("Downloading %s from \"%s\"", basePdscFile, pdscURL)

	pdscFileURL, err := url.Parse(pdscURL)
	if err != nil {
		log.Errorf("Could not parse pdsc url \"%s\": %s", pdscURL, err)
		return errs.ErrAlreadyLogged
	}

	pdscFileURL.Path = path.Join(pdscFileURL.Path, basePdscFile)

	localFileName, err := utils.DownloadFile(pdscFileURL.String(), timeout)
	defer os.Remove(localFileName)

	if err != nil {
		//		log.Errorf("Could not download \"%s\": %s", pdscFileURL, err)
		//		return fmt.Errorf("\"%s\": %w", pdscFileURL, errs.ErrPackPdscCannotBeFound)
		return err
	}

	utils.UnsetReadOnly(pdscFilePath)
	os.Remove(pdscFilePath)
	err = utils.MoveFile(localFileName, pdscFilePath)
	utils.SetReadOnly(pdscFilePath)

	return err
}

// loadPdscFile loads a PDSC (Pack Description) file from a specified URL or local file path.
// It handles both remote and local file sources, copying or downloading the file as needed.
//
// Parameters:
//   - pdscTag: An xml.PdscTag struct containing the vendor, name, and URL of the PDSC file.
//   - timeout: An integer specifying the timeout duration for downloading the file.
//
// Returns:
//   - error: An error if the file could not be loaded, parsed, copied, or downloaded successfully.
//
// The function performs the following steps:
//  1. Constructs the base PDSC file name and its local file path.
//  2. Parses the provided URL to determine if it is a local file or a remote URL.
//  3. If the URL scheme is "file", it copies the file from the local source to the destination.
//  4. If the URL scheme is not "file", it downloads the file from the remote URL.
//  5. Sets the file to read-only after copying or downloading it.
func (p *PacksInstallationType) loadPdscFile(pdscTag xml.PdscTag, timeout int) error {
	basePdscFile := fmt.Sprintf("%s.%s.pdsc", pdscTag.Vendor, pdscTag.Name)
	pdscFilePath := filepath.Join(p.LocalDir, basePdscFile)

	pdscURL := pdscTag.URL

	log.Debugf("Loading %s from \"%s\"", basePdscFile, pdscURL)

	pdscFileURL, err := url.Parse(pdscURL)
	if err != nil {
		log.Errorf("Could not parse pdsc url \"%s\": %s", pdscURL, err)
		return errs.ErrAlreadyLogged
	}

	pdscFileURL.Path = path.Join(pdscFileURL.Path, basePdscFile)
	if pdscFileURL.Scheme == "file" {
		sourceFilePath := pdscFileURL.Path
		if runtime.GOOS == "windows" && strings.HasPrefix(sourceFilePath, "/") {
			sourceFilePath = sourceFilePath[1:]
		}
		utils.UnsetReadOnly(pdscFilePath)
		defer utils.SetReadOnly(pdscFilePath)
		if err = utils.CopyFile(sourceFilePath, pdscFilePath); err != nil {
			log.Errorf("Could not copy pdsc \"%s\": %s", sourceFilePath, err)
			return errs.ErrAlreadyLogged
		}
		return nil
	}

	localFileName, err := utils.DownloadFile(pdscFileURL.String(), timeout)
	defer os.Remove(localFileName)

	if err != nil {
		//		log.Errorf("Could not download \"%s\": %s", pdscFileURL, err)
		//		return fmt.Errorf("\"%s\": %w", pdscFileURL, errs.ErrPackPdscCannotBeFound)
		return err
	}

	utils.UnsetReadOnly(pdscFilePath)
	os.Remove(pdscFilePath)
	err = utils.MoveFile(localFileName, pdscFilePath)
	utils.SetReadOnly(pdscFilePath)

	return err
}

// LockPackRoot sets the directories related to the installation as read-only,
// except for the "pack.idx" file which is set to be writable.
func LockPackRoot() {
	if Installation.ReadOnly {
		utils.SetReadOnly(Installation.WebDir)
		utils.SetReadOnly(Installation.LocalDir)
		utils.SetReadOnly(Installation.DownloadDir)
		utils.SetReadOnly(Installation.PackRoot)
	}
	Installation.ReadOnly = true
	// "pack.idx" does not need to be read only
	utils.UnsetReadOnly(Installation.PackIdx)
}

// UnlockPackRoot removes the read-only attribute from several directories
// related to the installation process. It ensures that the PackRoot, WebDir,
// LocalDir, and DownloadDir directories are writable.
func UnlockPackRoot() {
	if Installation.ReadOnly {
		utils.UnsetReadOnly(Installation.PackRoot)
		utils.UnsetReadOnly(Installation.WebDir)
		utils.UnsetReadOnly(Installation.LocalDir)
		utils.UnsetReadOnly(Installation.DownloadDir)
	}
	Installation.ReadOnly = false
}
