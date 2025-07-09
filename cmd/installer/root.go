/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"bufio"
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
	"sync"
	"syscall"
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

const PublicIndexName = "index.pidx"
const PdscExtension = ".pdsc"
const PackExtension = ".pack"
const KeilDefaultPackRoot = "https://www.keil.com/pack/"
const ConnectionTryURL = "https://www.keil.com/pack/keil.vidx"

// DefaultPublicIndex is the public index to use in "default mode"
const DefaultPublicIndex = KeilDefaultPackRoot + PublicIndexName

// would be reset to the public index URL when reading the public index
var ActualPublicIndex = DefaultPublicIndex

type lockedSlice struct {
	lock  sync.Mutex
	slice []xml.PdscTag
}

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

// AddPack installs a pack from the given packPath, handling dependencies, EULA agreements,
// and reinstallation logic.
//
// Parameters:
//   - packPath: The path or identifier of the pack to be installed.
//   - checkEula: If true, checks for EULA acceptance before installation.
//   - extractEula: If true, extracts the EULA for the pack without installing it.
//   - forceReinstall: If true, forces reinstallation of the pack even if it is already installed.
//   - noRequirements: If true, skips checking and installing dependencies for the pack.
//   - testing: If true, skips certain operations for testing purposes.
//   - timeout: The timeout duration (in seconds) for network operations.
//
// Returns:
//   - error: An error if the installation fails, or nil if successful.
//
// Behavior:
//   - Handles global pack updates by checking and updating the public index if necessary.
//   - Supports reinstallation by temporarily backing up and restoring existing installations.
//   - Fetches the pack from its source and installs it, ensuring dependencies are satisfied unless skipped.
//   - Provides detailed logging for each step of the installation process.
//   - Ensures proper cleanup and rollback in case of errors during installation.
func AddPack(packPath string, checkEula, extractEula, forceReinstall, noRequirements, testing bool, timeout int) error {

	isDep := false
	// tag dependency packs with $ for correct logging output
	if strings.TrimPrefix(packPath, "$") != packPath {
		isDep = true
		packPath = packPath[1:]
	}

	if !isDep {
		if !testing {
			global, err := isGlobal(packPath)
			if err != nil {
				return err
			}
			if global {
				if err := UpdatePublicIndexIfOnline(); err != nil {
					return err
				}
			}
		}
		log.Infof("Adding pack %q", packPath)
	}

	pack, err := preparePack(packPath, false, false, false, true)
	if err != nil {
		return err
	}
	if pack.isPackID {
		if pack.path, err = FindPackURL(pack, testing); err != nil {
			return err
		}
	}

	dropPreInstalled := false
	fullPackPath := ""
	backupPackPath := ""
	if !extractEula && pack.isInstalled {
		if forceReinstall {

			log.Debugf("Making temporary backup of pack %q", packPath)

			// Get target pack's full path and move it to a temporary "_tmp" directory
			fullPackPath = filepath.Join(Installation.PackRoot, pack.Vendor, pack.Name, pack.GetVersionNoMeta())
			backupPackPath = fullPackPath + "_tmp"

			if err := utils.MoveFile(fullPackPath, backupPackPath); err != nil {
				return err
			}

			log.Debugf("Moved pack to temporary path %q", backupPackPath)
			dropPreInstalled = true
		} else {
			if pack.versionModifier == utils.AnyVersion {
				if len(pack.installedVersions) > 1 {
					log.Infof("Pack %q is already installed here: %q, versions: %s",
						packPath, filepath.Join(Installation.PackRoot, pack.Vendor, pack.Name), utils.VersionList(pack.installedVersions))
				} else {
					log.Infof("Pack %q is already installed here: %q",
						packPath, filepath.Join(Installation.PackRoot, pack.Vendor, pack.Name, pack.installedVersions[0]))
				}
			} else {
				log.Errorf("Pack \"%s@%s\" is already installed here: %q, use the --force-reinstall (-F) flag to force installation",
					packPath, pack.targetVersion, filepath.Join(Installation.PackRoot, pack.Vendor, pack.Name, pack.GetVersionNoMeta()))
			}
			return nil
		}
	}

	if err = pack.fetch(timeout); err != nil {
		return err
	}

	// Since we only get the target version here, can only
	// print the message now for dependencies
	if isDep {
		log.Infof("Adding pack %s", pack.VName()+"."+pack.targetVersion)
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
		log.Debugf("Successfully deleted temporary pack %q", backupPackPath)
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
				pack, err := preparePack(path, false, false, false, true)
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
func RemovePack(packPath string, purge, testing bool) (bool, error) {
	log.Debugf("Removing pack \"%v\"", packPath)

	if err := ReadIndexFiles(); err != nil {
		return false, err
	}

	// TODO: by default, remove latest version first
	// if no version is given

	pack, err := preparePack(packPath, true, false, false, true)
	if err != nil {
		return false, err
	}

	if pack.isInstalled {
		// TODO: If removing-all is enabled, get rid of the version
		// pack.Version = ""
		pack.Unlock()
		if err = pack.uninstall(Installation); err != nil {
			return false, err
		}

		if purge {
			ok := false
			if ok, err = pack.purge(); err != nil {
				return ok, err
			}
			if ok { // if there was no error, but no files to remove
				return ok, nil
			}
		}

		return false, Installation.touchPackIdx()
	} else if purge {
		pack.Unlock()
		ok, err := pack.purge()
		return ok, err
	}

	log.Errorf("Pack \"%v\" is not installed", packPath)
	return false, errs.ErrPackNotInstalled
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
func massDownloadPdscFiles(pdscTag xml.PdscTag, skipInstalledPdscFiles bool, timeout int, errTags *lockedSlice) {
	if err := Installation.downloadPdscFile(pdscTag, skipInstalledPdscFiles, true, timeout); err != nil {
		errTags.lock.Lock()
		errTags.slice = append(errTags.slice, pdscTag)
		errTags.lock.Unlock()
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
func UpdatePack(packPath string, checkEula, noRequirements, subCall, testing bool, timeout int) error {

	if !subCall {
		if packPath == "" {
			if !testing {
				if err := UpdatePublicIndexIfOnline(); err != nil {
					return err
				}
			}
		} else {
			global, err := isGlobal(packPath)
			if err != nil {
				return err
			}
			if global && !testing {
				if err := UpdatePublicIndexIfOnline(); err != nil {
					return err
				}
			}
		}
	}
	if packPath == "" {
		installedPacks, err := findInstalledPacks(false, true)
		if err != nil {
			return err
		}
		for _, installedPack := range installedPacks {
			err = UpdatePack(installedPack.VName(), checkEula, noRequirements, true, testing, timeout)
			if err != nil {
				log.Error(err)
			}
		}
		return nil
	}
	pack, err := preparePack(packPath, false, true, true, true)
	if err != nil {
		return err
	}

	if pack.isInstalled {
		log.Infof("Pack \"%s@%s\" is the latest pack version and is already installed", packPath, pack.targetVersion)
		return nil
	}

	if !pack.IsPublic {
		log.Infof("Pack %q is not installed", packPath)
		return nil
	}

	log.Infof("Updating pack %q", packPath)

	if pack.isPackID {
		if pack.path, err = FindPackURL(pack, testing); err != nil {
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
				pack, err := preparePack(path, false, false, false, true)
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
	// if err := Installation.PublicIndexXML.Read(); err != nil {
	// 	return err
	// }

	pdscTags := Installation.PublicIndexXML.ListPdscTags()
	numPdsc := len(pdscTags)
	if numPdsc == 0 {
		log.Info("(no packs in public index)")
		return nil
	}

	if utils.GetEncodedProgress() {
		log.Infof("[J%d:F%q]", numPdsc, Installation.PublicIndex)
	}

	ctx := context.TODO()
	concurrency = CheckConcurrency(concurrency)
	sem := semaphore.NewWeighted(int64(concurrency))

	var errTags lockedSlice

	for _, pdscTag := range pdscTags {
		if concurrency == 0 {
			massDownloadPdscFiles(pdscTag, skipInstalledPdscFiles, timeout, &errTags)
		} else {
			if err := sem.Acquire(ctx, 1); err != nil {
				log.Errorf("Failed to acquire semaphore: %v", err)
				break
			}

			go func(pdscTag xml.PdscTag, errTags *lockedSlice) {
				defer sem.Release(1)
				massDownloadPdscFiles(pdscTag, skipInstalledPdscFiles, timeout, errTags)
			}(pdscTag, &errTags)
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
func UpdateInstalledPDSCFiles(pidxXML, oldPidxXML *xml.PidxXML, concurrency int, timeout int) error {
	log.Info("Updating PDSC files of public packs")

	ctx := context.TODO()
	concurrency = CheckConcurrency(concurrency)
	sem := semaphore.NewWeighted(int64(concurrency))

	var errTags lockedSlice

	if oldPidxXML != nil && !oldPidxXML.Empty() {
		for _, pdscTag := range pidxXML.ListPdscTags() {
			if oldPidxXML.HasPdsc(pdscTag) != xml.PdscIndexNotFound {
				continue // found in old pidx and same URL
			}
			oldTags := oldPidxXML.FindPdscNameTags(pdscTag)
			if len(oldTags) != 0 {
				log.Infof("%s::%s has a new version %q, previous was %q", pdscTag.Vendor, pdscTag.Name, pdscTag.Version, oldTags[0].Version)

				if concurrency == 0 {
					massDownloadPdscFiles(pdscTag, false, timeout, &errTags)
				} else {
					if err := sem.Acquire(ctx, 1); err != nil {
						log.Errorf("Failed to acquire semaphore: %v", err)
						break
					}

					go func(pdscTag xml.PdscTag, errTags *lockedSlice) {
						defer sem.Release(1)
						massDownloadPdscFiles(pdscTag, false, timeout, errTags)
					}(pdscTag, &errTags)
				}
			}
		}
		if len(errTags.slice) > 0 {
			for _, tag := range errTags.slice {
				tags := oldPidxXML.FindPdscNameTags(tag)
				if len(tags) != 0 {
					_ = pidxXML.ReplacePdscVersion(tags[0])
				}
			}
		}
	}

	if concurrency > 1 {
		if err := sem.Acquire(ctx, int64(concurrency)); err != nil {
			log.Errorf("Failed to acquire semaphore: %v", err)
		}
	}

	if len(errTags.slice) > 0 {
		if err := pidxXML.Write(); err != nil {
			return err
		}
		utils.UnsetReadOnly(Installation.PublicIndex)
		if err := utils.MoveFile(pidxXML.GetFileName(), Installation.PublicIndex); err != nil {
			return err
		}
		utils.SetReadOnly(Installation.PublicIndex)
	}

	pdscFiles, err := utils.ListDir(Installation.LocalDir, ".pdsc$")
	if err != nil {
		return err
	}

	numPdsc := len(pdscFiles)
	if utils.GetEncodedProgress() {
		log.Infof("[J%d:F%q]", numPdsc, Installation.LocalDir)
	}

	for _, pdscFile := range pdscFiles {
		log.Debugf("Checking if %q needs updating", pdscFile)
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

		if err := Installation.loadPdscFile(xml.PdscTag{
			URL:    pdscXML.URL,
			Vendor: pdscXML.Vendor,
			Name:   pdscXML.Name,
		}, timeout); err != nil {
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
			log.Infof("%s::%s can be upgraded from %q to %q", pdscXML.Vendor, pdscXML.Name, originalLatestVersion, latestVersion)
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
/* func GetIndexPath(indexPath string) (string, error) {
	if indexPath == "" {
		indexPath = strings.TrimSuffix(Installation.PublicIndexXML.URL, "/")
	}

	if !utils.GetEncodedProgress() {
		log.Infof("Using path: \"%v\"", indexPath)
	}

	var err error

	if strings.HasPrefix(indexPath, "http://") || strings.HasPrefix(indexPath, "https://") {
		if !strings.HasPrefix(indexPath, "https://") {
			log.Warnf("Non-HTTPS url: %q", indexPath)
		}
	}

	return indexPath, err
}
*/

// UpdatePublicIndexIfOnline checks if the public index file exists and updates it if necessary.
// If the public index file exists, it first checks for an active internet connection and the
// timestamp of the file. If the system is online and the file is outdated, it downloads the
// latest version of the public index. If the system is offline, it skips the update process.
//
// If the public index file does not exist, it downloads the public index without performing
// any checks and creates an update configuration file.
//
// Returns an error if any step in the update process fails.
func UpdatePublicIndexIfOnline() error {
	// If public index already exists then first check if online, then its timestamp
	// if we are online and it is too old then download a current version
	if utils.FileExists(Installation.PublicIndex) {
		err := utils.CheckConnection(ConnectionTryURL, 0)
		if err != nil && errors.Unwrap(err) != errs.ErrOffline {
			return err
		}
		if errors.Unwrap(err) != errs.ErrOffline {
			var updateConf updateCfg
			err = Installation.checkUpdateCfg(&updateConf)
			if err != nil {
				UnlockPackRoot()
				err1 := UpdatePublicIndex(ActualPublicIndex, true, false, false, false, 0, 0)
				if err1 != nil {
					return err1
				}
				_ = Installation.updateUpdateCfg(&updateConf)
			}
		} else {
			log.Debug("Offline mode: Skipping public index update")
		}
	}
	// if public index does not or not yet exist then download without check
	if !utils.FileExists(Installation.PublicIndex) {
		UnlockPackRoot()
		err1 := UpdatePublicIndex(ActualPublicIndex, true, false, false, false, 0, 0)
		if err1 != nil {
			return err1
		}
		var updateConf updateCfg
		updateConf.Auto = true
		_ = Installation.updateUpdateCfg(&updateConf) // create the update config file
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
//   - downloadRemainingPdscFiles: A boolean flag to indicate whether to download all remaining PDSC files.
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
		indexPath = strings.TrimSuffix(Installation.PublicIndexXML.URL, "/") + "/" + PublicIndexName
	}

	var err error

	if strings.HasPrefix(indexPath, "http://") || strings.HasPrefix(indexPath, "https://") {
		if !strings.HasPrefix(indexPath, "https://127.0.0.1") {
			err = utils.CheckConnection(ConnectionTryURL, 0)
			if err != nil && errors.Unwrap(err) == errs.ErrOffline {
				return err
			}
		}
	}

	if log.IsLevelEnabled(log.DebugLevel) {
		log.Debugf("Updating public index with \"%v\"", indexPath)
	} else {
		log.Infof("Updating public index")
	}

	if strings.HasPrefix(indexPath, "http://") || strings.HasPrefix(indexPath, "https://") {
		if !strings.HasPrefix(indexPath, "https://") {
			log.Warnf("Non-HTTPS url: %q", indexPath)
		}

		indexPath, err = utils.DownloadFile(indexPath, false, false, timeout)
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

	Installation.PublicIndexXML.SetFileName(indexPath) // The downloaded index.pidx
	if err := Installation.PublicIndexXML.Read(); err != nil {
		return err
	}

	var oldPidxXML *xml.PidxXML
	if !sparse {
		oldPidxXML = xml.NewPidxXML(Installation.PublicIndex)
		if err := oldPidxXML.Read(); err != nil { // old public index XML
			return err
		}
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
		err = UpdateInstalledPDSCFiles(Installation.PublicIndexXML, oldPidxXML, concurrency, timeout)
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
		pdscPath := strings.ReplaceAll(match, Installation.PackRoot, "")
		packName, _ := filepath.Split(pdscPath)
		packName = strings.ReplaceAll(packName, "/", " ")
		packName = strings.ReplaceAll(packName, "\\", " ")
		packName = strings.Trim(packName, " ")
		packName = strings.ReplaceAll(packName, " ", ".")

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
				pack.pdscPath = pdsc.URL + pack.Vendor + "/" + pack.Name + PdscExtension

				parsedURL, err := url.ParseRequestURI(pdsc.URL)
				pack.err = err
				if err != nil {
					installedPacks = append(installedPacks, pack)
					continue
				}

				pack.pdscPath = filepath.Join(utils.CleanPath(parsedURL.Path), pack.VName()+PdscExtension)
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
					//nolint:staticcheck // intentional logic for clarity
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
			log.Infof("Listing packs from the public index, filtering by %q", listFilter)
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
			packFilePath := filepath.Join(Installation.DownloadDir, pdscTag.Key()) + PackExtension

			if ok, _ := Installation.PackIsInstalled(&PackType{PdscTag: pdscTag}, false); ok {
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
			log.Infof("Listing cached packs, filtering by %q", listFilter)
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
			packFilePath = strings.ReplaceAll(packFilePath, PackExtension, "")
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
			if ok, _ := Installation.PackIsInstalled(&PackType{PdscTag: pdscTag}, false); ok {
				logMessage += " (installed)"
			}

			if listFilter == "" || utils.FilterPackID(logMessage, listFilter) != "" {
				log.Info(logMessage)
			}
		}
	} else {
		if listUpdates {
			if listFilter != "" {
				log.Infof("Listing installed packs with available update, filtering by %q", listFilter)
			} else {
				log.Infof("Listing installed packs with available update")
			}
		} else {
			if listRequirements {
				log.Info("Listing installed packs with dependencies")
			} else {
				if listFilter != "" {
					log.Infof("Listing installed packs, filtering by %q", listFilter)
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
			p, err := preparePack(pack.Key(), false, listUpdates, listUpdates, false)
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
func FindPackURL(pack *PackType, testing bool) (string, error) {
	log.Debugf("Finding URL for \"%v\"", pack.path)

	if pack.IsPublic {
		packPdscFileName := filepath.Join(Installation.WebDir, pack.PdscFileName())
		if pack.versionModifier == utils.ExactVersion {
			pack.targetVersion = pack.Version
			log.Debugf("- resolved(@) as %s", pack.targetVersion)
			tags := Installation.PublicIndexXML.FindPdscTags(xml.PdscTag{
				Vendor: pack.Vendor,
				Name:   pack.Name,
			})
			if len(tags) != 0 {
				if err := Installation.downloadPdscFile(tags[0], true, false, 0); err != nil {
					return "", err
				}
			} else {
				return "", errs.ErrPdscEntryNotFound
			}
		} else {
			if err := Installation.downloadPdscFile(xml.PdscTag{
				URL:    pack.URL,
				Vendor: pack.Vendor,
				Name:   pack.Name,
			}, true, false, 0); err != nil {
				return "", err
			}
		}
		packPdscXML := xml.NewPdscXML(packPdscFileName)
		if err := packPdscXML.Read(); err != nil {
			if errors.Unwrap(err) == syscall.ENOENT {
				err = fmt.Errorf("%q: %w", packPdscFileName, errs.ErrPackPdscCannotBeFound)
			}
			return "", err
		}

		// check if the latest version of the pack exists in the public index
		// if not, download the pack PDSC file from the URL
		xmlTag := xml.PdscTag{
			Vendor: pack.Vendor,
			Name:   pack.Name,
		}
		if !testing && packPdscXML.LatestVersion() != "" {
			pidxVersions := Installation.PublicIndexXML.FindPdscTags(xmlTag)
			if len(pidxVersions) > 0 && pidxVersions[0].Version != packPdscXML.LatestVersion() {
				logVersion := pidxVersions[0].Version
				infoInsteadWarn := false
				if err := Installation.downloadPdscFile(xml.PdscTag{
					URL:    pack.URL,
					Vendor: pack.Vendor,
					Name:   pack.Name,
				}, false, false, 0); err != nil {
					log.Warnf("Latest pdsc %q does not exist in public index", xmlTag.Key())
					return "", err
				}
				packPdscXML = xml.NewPdscXML(packPdscFileName)
				if err := packPdscXML.Read(); err != nil {
					log.Warnf("Latest pdsc %q does not exist in public index", xmlTag.Key())
					if errors.Unwrap(err) == syscall.ENOENT {
						err = fmt.Errorf("%q: %w", packPdscFileName, errs.ErrPackPdscCannotBeFound)
					}
					return "", err
				}
				// Re-check the latest version after downloading the PDSC file
				xmlTag.Version = packPdscXML.LatestVersion()
				if xmlTag.Version != "" {
					pidxVersions = Installation.PublicIndexXML.FindPdscTags(xmlTag)
					if len(pidxVersions) == 0 {
						filenameSave := Installation.PublicIndexXML.GetFileName()
						defer Installation.PublicIndexXML.SetFileName(filenameSave)                                       // restore the original file name
						Installation.PublicIndexXML.SetFileName(filepath.Join(Installation.DownloadDir, PublicIndexName)) // use cache as temporary store
						_ = Installation.PublicIndexXML.ReplacePdscVersion(xmlTag)
						if err := Installation.PublicIndexXML.Write(); err != nil {
							log.Warnf("Latest version of pdsc %q does not exist in public index", xmlTag.Key())
							return "", err
						}
						utils.UnsetReadOnly(Installation.PublicIndex)
						if err := utils.MoveFile(Installation.PublicIndexXML.GetFileName(), Installation.PublicIndex); err != nil {
							log.Warnf("Latest version of pdsc %q does not exist in public index", xmlTag.Key())
							return "", err
						}
						utils.SetReadOnly(Installation.PublicIndex)
						infoInsteadWarn = true // if we downloaded the PDSC file and it has the same version as in the public index, we can log it as info
					}
				}
				if infoInsteadWarn {
					log.Infof("Latest pack version in index.pidx (%s) differs from latest version in current %s.pdsc(%s).", logVersion, xmlTag.VName(), xmlTag.Version)
				} else {
					log.Warnf("Latest version of pdsc %q does not exist in public index", xmlTag.Key())
				}
			}
		}
		// Figures out which pack release to fetch and assign that to pack.targetVersion
		pack.resolveVersionModifier(packPdscXML)

		releaseTag := packPdscXML.FindReleaseTagByVersion(pack.targetVersion)
		if releaseTag == nil {
			return "", fmt.Errorf("%s.%s: %w", pack.PackID(), pack.targetVersion, errs.ErrPackVersionNotFoundInPdsc)
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
	Installation.PublicIndex = filepath.Join(Installation.WebDir, PublicIndexName)
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
		log.Errorf("Directory(ies) %q are missing! Was %s initialized correctly?", strings.Join(missingDirs[:], ", "), packRoot)
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
		Installation.PublicIndexXML.URL = "" // set URL to empty to avoid using it
		return err
	}
	if Installation.PublicIndexXML.URL != "" {
		ActualPublicIndex = Installation.PublicIndexXML.URL + PublicIndexName
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
	//	packs map[string]bool

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
	// Default struct {
	Date string
	Auto bool
	// }
}

// checkUpdateCfg reads and parses the "update.cfg" file located in the WebDir directory
// to populate the provided updateCfg structure with configuration values. It checks
// the "Date" and "Auto" fields in the file and validates whether the "Date" field
// indicates a timestamp older than 24 hours.
//
// Parameters:
//   - conf (*updateCfg): A pointer to the updateCfg structure that will be populated
//     with the parsed configuration values.
//
// Returns:
//   - error: An error is returned if the "update.cfg" file cannot be opened, if the
//     "Date" field cannot be parsed, or if the timestamp in the "Date" field is older
//     than 24 hours. If no errors occur, nil is returned.
func (p *PacksInstallationType) checkUpdateCfg(conf *updateCfg) error {
	f, err := os.Open(filepath.Join(p.WebDir, "update.cfg"))
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f) // Read the file line by line
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Date=") {
			conf.Date = strings.TrimPrefix(line, "Date=")
		} else if strings.HasPrefix(line, "Auto=") {
			conf.Auto = strings.TrimPrefix(line, "Auto=") == "true"
		}
	}
	if t, err := time.Parse("2-1-2006", conf.Date); err != nil {
		return err
	} else {
		if time.Since(t).Hours() > 24 { // index.pidx older than 1 day
			return errs.ErrIndexTooOld
		}
	}
	return nil
}

// updateUpdateCfg updates the "update.cfg" configuration file with the provided settings.
// It writes the current date and the auto-update flag to the file.
//
// Parameters:
//   - conf: A pointer to an updateCfg struct containing the configuration to be written.
//
// Returns:
//   - An error if there is an issue opening, writing to, or syncing the file; otherwise, nil.
func (p *PacksInstallationType) updateUpdateCfg(conf *updateCfg) error {
	conf.Date = time.Now().Local().Format("2-1-2006")
	flags := os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	f, err := os.OpenFile(filepath.Join(p.WebDir, "update.cfg"), flags, os.FileMode(0o644))
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString("Date=" + conf.Date + "\n"); err != nil {
		return err
	}
	if _, err := f.WriteString("Auto="); err != nil {
		return err
	}
	if conf.Auto {
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
func (p *PacksInstallationType) PackIsInstalled(pack *PackType, noLocal bool) (found bool, installedVersions []string) {
	log.Debugf("Checking if %s is installed", pack.PackIDWithVersion())

	found = false
	installedVersions = []string{}

	// First make sure there's at least one version of the pack installed
	installationDir := filepath.Join(p.PackRoot, pack.Vendor, pack.Name)
	if !utils.DirExists(installationDir) {
		return
	}

	// Exact version is easy, just find a matching installation folder
	if pack.versionModifier == utils.ExactVersion {
		version := pack.GetVersionNoMeta()
		packDir := filepath.Join(installationDir, version)
		log.Debugf("Checking if %q exists", packDir)
		found = utils.DirExists(packDir)
		if found {
			installedVersions = append(installedVersions, version)
		}
		return
	}

	if noLocal {
		if pack.isPackID || !pack.IsLocallySourced {
			if err := UpdatePublicIndexIfOnline(); err != nil {
				return
			}
		}
		if err := ReadIndexFiles(); err != nil {
			return
		}
	} else {
		// Gather all versions in local_repository.idx for local .psdc installed packs
		if err := p.LocalPidx.Read(); err != nil {
			log.Warn("Could not read local index")
			return
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
		log.Warnf("Could not list installed packs in %q: %v", installationDir, err)
		return
	}

	for _, path := range installedDirs {
		base := filepath.Base(path)
		installedVersions = append(installedVersions, base)
	}

	// Empty version also means any version
	if pack.Version == "" || pack.versionModifier == utils.AnyVersion {
		found = true
		return
	}

	// Check if greater version is specified
	if pack.versionModifier == utils.GreaterVersion {
		log.Debugf("Checking for installed packs >=%s", pack.Version)
		for _, version := range installedVersions {
			log.Debugf("- checking if %s >= %s", version, pack.Version)
			if utils.SemverCompare(version, pack.Version) >= 0 {
				log.Debugf("- found newer version %s", version)
				pack.targetVersion = version
				found = true
				return
			}
		}
		log.Debugf("- no version matched")
		return
	}

	// Check if there is a greater version with same Major number
	if pack.versionModifier == utils.GreatestCompatibleVersion {
		log.Debugf("Checking for installed packs @^%s", pack.Version)
		for _, version := range installedVersions {
			log.Debugf("- checking against: %s", version)
			sameMajor := semver.Major("v"+version) == semver.Major("v"+pack.Version)
			if sameMajor && utils.SemverCompare(version, pack.Version) >= 0 {
				pack.targetVersion = version
				found = true
				return
			}
		}
		return
	}

	// Check if there is a greater version with same Major and Minor number
	if pack.versionModifier == utils.PatchVersion {
		log.Debugf("Checking for installed packs @~%s", pack.Version)
		for _, version := range installedVersions {
			log.Debugf("- checking against: %s", version)
			sameMajorMinor := semver.MajorMinor("v"+version) == semver.MajorMinor("v"+pack.Version)
			if sameMajorMinor && utils.SemverCompare(version, pack.Version) >= 0 {
				pack.targetVersion = version
				found = true
				return
			}
		}
		return
	}

	if pack.versionModifier == utils.RangeVersion {
		log.Debugf("Checking for installed packs %s", utils.FormatVersions(pack.Version))
		for _, version := range installedVersions {
			log.Debugf("- checking against: %s", version)
			if utils.SemverCompareRange(version, pack.Version) == 0 {
				found = true
				return
			}
		}
		return
	}

	if pack.versionModifier == utils.LatestVersion {
		// so cpackget needs to know first the latest available
		// version for that pack to then check if it's installed
		var latestVersion string
		if pack.IsPublic {
			tags := Installation.PublicIndexXML.FindPdscTags(xml.PdscTag{
				Vendor: pack.Vendor,
				Name:   pack.Name,
			})
			if len(tags) > 0 {
				latestVersion = tags[0].Version
			}
		} else {
			pdscFilePath := filepath.Join(Installation.LocalDir, pack.PdscFileName())
			pdscXML := xml.NewPdscXML(pdscFilePath)
			if err := pdscXML.Read(); err != nil {
				log.Debugf("Could not retrieve pack's PDSC file from %q", pdscFilePath)
				return
			}
			latestVersion = pdscXML.LatestVersion()
		}
		if latestVersion == "" {
			log.Debugf("Could not find latest version for %q", pack.PackIDWithVersion())
		} else {
			packDir := filepath.Join(installationDir, latestVersion)
			found = utils.DirExists(packDir)
			pack.targetVersion = latestVersion
		}
	}

	return
}

// packIsPublic checks whether the pack is public or not.
// Being public means a PDSC file is present in ".Web/" folder
// It first checks the local cache of PDSC files in the ".Web/" directory.
// If the pack is not found locally, it searches for the pack's PDSC file in the public index.
// If the pack is marked for removal or is locally sourced, it is considered public without further checks.
//
// Parameters:
//   - pack: The pack to check for public availability.
//
// Returns:
//   - bool: True if the pack is public, false otherwise.
func (p *PacksInstallationType) packIsPublic(pack *PackType, pdscTag *xml.PdscTag) (bool, error) {
	if p.PublicIndexXML.Empty() {
		if err := ReadIndexFiles(); err != nil {
			return false, err
		}
	}

	// Try to retrieve the packs's PDSC file out of the index.pidx
	pdscTags := p.PublicIndexXML.FindPdscTags(xml.PdscTag{
		Vendor: pack.Vendor,
		Name:   pack.Name,
	})
	if len(pdscTags) == 0 {
		log.Debugf("Not found %q tag in %q", pack.PdscFileName(), p.PublicIndex)
		return false, nil
	}

	// Sometimes a pidx file might have multiple pdsc tags for same key
	// which is not the case here, so we'll take only the first one
	*pdscTag = pdscTags[0]

	// If the pack is being removed, there's no need to get its PDSC file under .Web
	// Same applies to locally sourced packs
	if pack.toBeRemoved || pack.IsLocallySourced {
		return true, nil
	}

	return true, nil
}

// downloadPdscFile downloads a PDSC (Pack Description) file from a specified URL and saves it to the local file system.
// It handles scenarios such as skipping already installed files, switching to a cache URL, and managing file permissions.
//
// Parameters:
//   - pdscTag: An xml.PdscTag object containing metadata about the PDSC file to be downloaded.
//   - skipInstalledPdscFiles: A boolean flag indicating whether to skip downloading if the PDSC file already exists locally.
//   - noProgressBar: A boolean flag indicating whether to suppress the progress bar during the download.
//   - timeout: An integer specifying the timeout duration (in seconds) for the download operation.
//
// Returns:
//   - error: An error object if any issues occur during the download or file operations, or nil if the operation succeeds.
//
// Behavior:
//   - If skipInstalledPdscFiles is true and the file already exists locally, the function returns without downloading.
//   - If the PDSC file's URL is not the default Keil pack root and the public index XML URL matches the default root,
//     the function switches to the cache URL for downloading.
//   - The function downloads the PDSC file, temporarily saves it, and then moves it to the target location,
//     ensuring proper file permissions are set before and after the operation.
func (p *PacksInstallationType) downloadPdscFile(pdscTag xml.PdscTag, skipInstalledPdscFiles, noProgressBar bool, timeout int) error {
	basePdscFile := fmt.Sprintf("%s.pdsc", pdscTag.VName())
	pdscFilePath := filepath.Join(p.WebDir, basePdscFile)

	if skipInstalledPdscFiles {
		if utils.FileExists(pdscFilePath) {
			log.Debugf("File already exists: %q", pdscFilePath)
			return nil
		}
		log.Debugf("File does not exist and will be copied: %q", pdscFilePath)
	}

	pdscURL := pdscTag.URL

	// switch  to keil.com cache for PDSC file
	if pdscURL != KeilDefaultPackRoot && Installation.PublicIndexXML.URL == KeilDefaultPackRoot {
		log.Debugf("Switching to cache: %q", KeilDefaultPackRoot)
		pdscURL = KeilDefaultPackRoot
	}

	log.Debugf("Downloading %s from %q", basePdscFile, pdscURL)

	pdscFileURL, err := url.Parse(pdscURL)
	if err != nil {
		log.Errorf("Could not parse pdsc url %q: %s", pdscURL, err)
		return errs.ErrAlreadyLogged
	}

	pdscFileURL.Path = path.Join(pdscFileURL.Path, basePdscFile)

	localFileName, err := utils.DownloadFile(pdscFileURL.String(), true, noProgressBar, timeout)
	defer os.Remove(localFileName)

	if err != nil {
		//		log.Errorf("Could not download %q: %s", pdscFileURL, err)
		//		return fmt.Errorf("%q: %w", pdscFileURL, errs.ErrPackPdscCannotBeFound)
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
	basePdscFile := fmt.Sprintf("%s.pdsc", pdscTag.VName())
	pdscFilePath := filepath.Join(p.LocalDir, basePdscFile)

	pdscURL := pdscTag.URL

	log.Debugf("Loading %s from %q", basePdscFile, pdscURL)

	pdscFileURL, err := url.Parse(pdscURL)
	if err != nil {
		log.Errorf("Could not parse pdsc url %q: %s", pdscURL, err)
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
			log.Errorf("Could not copy pdsc %q: %s", sourceFilePath, err)
			return errs.ErrAlreadyLogged
		}
		return nil
	}

	localFileName, err := utils.DownloadFile(pdscFileURL.String(), true, false, timeout)
	defer os.Remove(localFileName)

	if err != nil {
		//		log.Errorf("Could not download %q: %s", pdscFileURL, err)
		//		return fmt.Errorf("%q: %w", pdscFileURL, errs.ErrPackPdscCannotBeFound)
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
