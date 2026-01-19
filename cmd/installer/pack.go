/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/url"
	"os"
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

	// IsLocallySourced tells whether the pack's source is local or an HTTP URL
	IsLocallySourced bool

	// IsPublic tells whether the pack exists in the public index or not
	IsPublic bool

	// isDownloaded tells whether the file needed to be downloaded from a server
	isDownloaded bool

	// isPackID tells whether the path is in packID format: Vendor.PackName[.x.y.z]
	isPackID bool

	// toBeRemoved indicates
	toBeRemoved bool

	// exactVersion tells whether this pack identifier is specifying an exact version
	// or is requesting a newer one, e.g. @>=x.y.z
	versionModifier int

	// targetVersion is the most recent version of a pack in case exactVersion==true
	targetVersion string

	// isInstalled tells whether the pack is already installed
	isInstalled bool

	// list of all the packs that are installed
	installedVersions []string

	// path points to a file in the local system, whether or not it's local
	path string

	// Subfolder stores the subfolder this pack is in the compressed file.
	Subfolder string

	// Pdsc holds a pointer to the PDSC file already parsed as XML
	Pdsc *xml.PdscXML

	// zipReader holds a pointer to the uncompressed pack file
	zipReader *zip.ReadCloser

	// Requirements represents a packs' dependencies
	Requirements struct {
		packages []struct {
			info      []string // [Name, Vendor, Version]
			installed bool
		}
		satisfied bool // true if all dependencies are installed
	}
}

func isGlobal(packPath string) (bool, error) {
	info, err := utils.ExtractPackInfo(packPath)
	if err != nil {
		return false, err
	}
	if info.IsPackID {
		return true, nil
	}
	if strings.HasPrefix(info.Location, "http://") || strings.HasPrefix(info.Location, "https://") {
		return true, nil
	}
	return false, nil
}

// preparePack prepares a PackType object based on the provided parameters,
// it does some sanity validation regarding pack name
// and check if it's public and if it's installed or not
//
// Parameters:
//   - packPath: The path or URL to the pack.
//   - toBeRemoved: A boolean indicating if the pack is to be removed.
//   - forceLatest: A boolean indicating if the latest version of the pack should be used.
//   - noLocal: A boolean indicating if local installations should be ignored.
//
// Returns:
//   - *PackType: A pointer to the prepared PackType object.
//   - error: An error if any issues occur during preparation.
func preparePack(packPath string, toBeRemoved, forceLatest, noLocal, nometa bool) (*PackType, error) {
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

	if forceLatest {
		info.Version = "latest"
		info.VersionModifier = utils.LatestVersion
	}
	pack.URL = info.Location
	pack.Vendor = info.Vendor
	pack.Name = info.Pack
	pack.Version = info.Version
	pack.versionModifier = info.VersionModifier
	pack.isPackID = info.IsPackID

	// TODO: shouldn't this check only compare "file://" and not http(s)?
	if !strings.HasPrefix(pack.URL, "http://") && !strings.HasPrefix(pack.URL, "https://") && strings.HasPrefix(pack.URL, "file://") {
		pack.IsLocallySourced = true
	}

	var pdscTag xml.PdscTag
	if pack.IsPublic, err = Installation.packIsPublic(pack, &pdscTag); err != nil {
		return pack, err
	}

	if pack.isPackID && nometa {
		if meta, found := utils.SemverHasMeta(pack.Version); found {
			return pack, fmt.Errorf("%w: %q. Expected vendor%s.version", errs.ErrBadPackVersion, meta, utils.PackExtension)
		}
	}

	if pdscTag.URL != "" {
		pack.URL = pdscTag.URL
	}

	pack.isInstalled, pack.installedVersions = Installation.PackIsInstalled(pack, noLocal)

	if pdscTag.Vendor != "" {
		pack.Vendor = pdscTag.Vendor
	}
	if pdscTag.Name != "" {
		pack.Name = pdscTag.Name
	}

	return pack, nil
}

// If the path is not a URL, it will make sure the file exists in the local file system
// fetch downloads the pack file if the path is a URL or verifies its existence locally.
// If the path starts with "http", it attempts to download the file using the provided timeout.
// If the download is aborted by the user, it logs the event and removes the partially downloaded file.
// If the path is not a URL, it checks if the file exists locally.
// Returns an error if the file does not exist or if there is an issue during download.
//
// Parameters:
//   - insecureSkipVerify: a boolean indicating whether to skip TLS certificate verification.
//   - timeout: an integer specifying the timeout duration for the download operation.
//
// Returns:
//   - error: An error object if the file does not exist or if there is an issue during download.
func (p *PackType) fetch(insecureSkipVerify bool, timeout int) error {
	log.Debugf("Fetching pack file %q (or just making sure it exists locally)", p.path)
	var err error
	if strings.HasPrefix(p.path, "http") {
		p.path, err = utils.DownloadFile(p.path, true, true, true, insecureSkipVerify, timeout)
		if err == errs.ErrTerminatedByUser {
			log.Infof("Aborting pack download. Removing %q", p.path)
		}

		p.isDownloaded = true
		return err
	}

	if !utils.FileExists(p.path) {
		log.Errorf("File %q doesn't exist", p.path)
		return errs.ErrFileNotFound
	}

	return nil
}

// validate ensures the pack is legit and it has all minimal requirements to be installed.
func (p *PackType) validate() error {
	log.Debug("Validating pack")
	var err error
	myPdscFileName := p.PdscFileName()
	var validPdscFiles []*zip.File

	// Single pass: validate all files and collect valid PDSC files
	for _, file := range p.zipReader.File {
		ext := strings.ToLower(filepath.Ext(file.Name))

		// Ensure all file paths do not contain ".."
		if strings.Contains(file.Name, "..") {
			if ext == utils.PdscExtension {
				log.Errorf("File %q invalid file path", file.Name)
				return errs.ErrInvalidFilePath
			} else {
				return errs.ErrInsecureZipFileName
			}
		}

		if ext == utils.PdscExtension {
			// Check if pack was compressed in a subfolder
			subfoldersCount := strings.Count(file.Name, "/") + strings.Count(file.Name, "\\")
			if subfoldersCount > 1 {
				err = errs.ErrPdscFileTooDeepInPack // save for later decision if error or warning
			} else {
				tmpFileName := filepath.Base(file.Name) // normalize file name
				if !strings.EqualFold(tmpFileName, myPdscFileName) {
					if err != nil {
						log.Warnf("Pack %q contains an additional %s file in a deeper subfolder, this may cause issues", p.path, utils.PdscExtension)
					}
					return fmt.Errorf("%q: %w", file.Name, errs.ErrPdscWrongName)
				}
				// Collect valid PDSC files for processing
				validPdscFiles = append(validPdscFiles, file)

				if subfoldersCount == 1 {
					p.Subfolder = filepath.Dir(file.Name)
				}
			}
		}
	}

	if len(validPdscFiles) > 1 {
		return errs.ErrMultiplePdscFilesInPack
	}

	if err != nil {
		if len(validPdscFiles) > 0 {
			log.Warnf("Pack %q contains an additional %s file in a deeper subfolder, this may cause issues", p.path, utils.PdscExtension)
		} else {
			return err
		}
	}

	if len(validPdscFiles) == 0 {
		log.Errorf("%q not found in %q", myPdscFileName, p.path)
		return errs.ErrPdscFileNotFound
	}

	// Process the first valid PDSC file found
	file := validPdscFiles[0]

	// Read pack's pdsc
	tmpPdscFileName := filepath.Join(os.TempDir(), utils.RandStringBytes(10))
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

	log.Debugf("Making sure %s is the latest release in %s", version, myPdscFileName)

	if utils.SemverCompare(version, latestVersion) != 0 {
		releaseTag := p.Pdsc.FindReleaseTagByVersion(version)
		if releaseTag == nil {
			log.Errorf("The pack's pdsc (%s) has no release tag matching version %q", myPdscFileName, version)
			return errs.ErrPackVersionNotFoundInPdsc
		}

		log.Errorf("The latest release (%s) in pack's pdsc (%s) does not match pack version %q", latestVersion, myPdscFileName, version)
		return errs.ErrPackVersionNotLatestReleasePdsc
	}

	p.Pdsc.FileName = file.Name

	pdscFileName := p.PdscFileName()                          // destination file name in .Download directory
	pdscFilePath := filepath.Join(tmpPdscFileName, file.Name) // #nosec
	newPdscFileName := p.PdscFileNameWithVersion()

	if !p.IsPublic {
		_ = utils.CopyFile(pdscFilePath, filepath.Join(Installation.LocalDir, pdscFileName))
	}

	_ = utils.CopyFile(pdscFilePath, filepath.Join(Installation.DownloadDir, newPdscFileName))

	return nil
}

// purge removes all cached files matching the pattern derived from the PackType's
// Vendor, Name, and Version fields from the download directory. The pattern
// matches files with extensions `.pack`, `.zip`, or `.pdsc`.
//
// It logs the purging process, including the files to be removed. If no files
// match the pattern, it logs that the pack version is already removed and
// returns true with no error.
//
// Returns:
//   - A boolean indicating whether the pack version was already removed (true)
//     or not (false).
//   - An error if any issue occurs during the file listing or removal process.
func (p *PackType) purge() (bool, error) {
	log.Debugf("Purging \"%v\"", p.path)

	fileNamePattern := p.Vendor + "\\." + p.Name
	if len(p.Version) > 0 {
		fileNamePattern += "\\." + p.GetVersionNoMeta() + ".*"
	} else {
		fileNamePattern += "\\..*?"
	}
	fileNamePattern += "\\.(?:pack|zip|pdsc)"

	files, err := utils.ListDir(Installation.DownloadDir, fileNamePattern)
	if err != nil {
		return false, err
	}

	cTag := xml.PdscTag{Vendor: p.Vendor, Name: p.Name, Version: p.Version}
	if err = Installation.PublicCacheIndexXML.RemovePdsc(cTag); err == nil {
		webFile := Installation.WebDir + "/" + p.VName() + utils.PdscExtension
		files = append(files, webFile) // also add pdsc file in .Web
	}
	_ = Installation.PublicCacheIndexXML.Write()

	log.Debugf("Files to be purged \"%v\"", files)
	if len(files) == 0 {
		log.Infof("pack %s.%s already removed from %s", p.path, p.Version, Installation.DownloadDir)
		return true, nil
	}

	for _, file := range files {
		if err := os.Remove(file); err != nil {
			return false, err
		}
	}

	return false, nil
}

// install installs pack files to installation's directories
// It:
//   - Extracts all files to "CMSIS_PACK_ROOT/p.Vendor/p.Name/p.Version/"
//   - Saves a copy of the pack in "CMSIS_PACK_ROOT/.Download/"
//   - Saves a versioned pdsc file in "CMSIS_PACK_ROOT/.Download/"
//   - If "CMSIS_PACK_ROOT/.Web/p.Vendor.p.Name.pdsc" does not exist then
//   - Save an unversioned copy of the pdsc file in "CMSIS_PACK_ROOT/.Local/"
func (p *PackType) install(installation *PacksInstallationType, checkEula bool) error {

	// normalize pack path
	p.path = filepath.FromSlash(p.path)
	p.path = filepath.Clean(p.path)

	log.Debugf("Installing %q", p.path)

	var err error
	p.zipReader, err = zip.OpenReader(p.path)
	if err != nil {
		log.Errorf("Can't decompress %q: %s", p.path, err)
		return errs.ErrFailedDecompressingFile
	}

	if err = p.validate(); err != nil {
		return err
	}

	packHomeDir := filepath.Join(Installation.PackRoot, p.Vendor, p.Name, p.GetVersionNoMeta())
	packBackupPath := filepath.Join(Installation.DownloadDir, p.PackFileName())
	packBackupPath = filepath.FromSlash(packBackupPath)
	packBackupPath = filepath.Clean(packBackupPath)
	if utils.SameFile(packBackupPath, p.path) {
		p.isDownloaded = true
	}

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
		if utils.GetEncodedProgress() {
			return nil
		}
		return errs.ErrLicenseNotFound
	}

	// Inflate all files
	err = utils.EnsureDir(packHomeDir)
	if err != nil {
		log.Errorf("Can't access pack directory %q: %s", packHomeDir, err)
		return err
	}

	if log.IsLevelEnabled(log.DebugLevel) {
		log.Debugf("Extracting files from %q to %q", p.path, packHomeDir)
	} else {
		log.Infof("Extracting files to %s...", packHomeDir)
	}
	// Avoid repeated calls to IsTerminalInteractive
	// as it cleans the stdout buffer
	interactiveTerminal := utils.IsTerminalInteractive()
	var progress *progressbar.ProgressBar
	var encodedProgress *utils.EncodedProgress

	if utils.GetEncodedProgress() {
		encodedProgress = utils.NewEncodedProgress(int64(len(p.zipReader.File)), 0, p.path)
	} else if interactiveTerminal && log.GetLevel() != log.ErrorLevel {
		progress = progressbar.Default(int64(len(p.zipReader.File)), "I:")
	}

	for _, file := range p.zipReader.File {
		if utils.GetEncodedProgress() {
			_ = encodedProgress.Add(1)
		} else if interactiveTerminal && log.GetLevel() != log.ErrorLevel {
			_ = progress.Add64(1)
		}
		err = utils.SecureInflateFile(file, packHomeDir, p.Subfolder)
		if err != nil {
			defer p.zipReader.Close()

			if err == errs.ErrTerminatedByUser {
				log.Infof("Aborting pack extraction. Removing %q", packHomeDir)
				if newErr := p.uninstall(installation); newErr != nil {
					log.Error(err)
				}
			}
			return err
		}
	}

	// Close zip file so Windows can't complain if we rename it
	p.zipReader.Close()

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
	packPath := filepath.Join(installation.PackRoot, p.Vendor, p.Name, p.GetVersionNoMeta())
	if err := os.RemoveAll(packPath); err != nil {
		return err
	}

	// Remove Vendor/Pack/ if empty
	packPath = filepath.Join(installation.PackRoot, p.Vendor, p.Name)
	if utils.IsEmpty(packPath) {
		if err := os.Remove(packPath); err != nil {
			return err
		}

		// Remove local pdsc file if pack is not public and if there are no more versions of this pack installed
		if !p.IsPublic {
			localPdscFileName := p.PdscFileName()
			filePath := filepath.Join(installation.LocalDir, localPdscFileName)
			if err := os.Remove(filePath); err != nil {
				return err
			}
		}
	}

	// Remove Vendor/ if empty
	vendorPath := filepath.Join(installation.PackRoot, p.Vendor)
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

	licenseFileName := strings.ReplaceAll(p.Pdsc.License, "\\", "/")

	// License contains the license path inside the pack file
	for _, file := range p.zipReader.File {
		possibleLicense := strings.ReplaceAll(file.Name, "\\", "/")
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

	eulaFileName := packPath + "." + filepath.Base(p.Pdsc.License)

	if utils.GetEncodedProgress() {
		log.Infof("[L:F%q]", eulaFileName)
	} else {
		log.Infof("Extracting embedded license to %v", eulaFileName)
	}

	if utils.FileExists(eulaFileName) {
		utils.UnsetReadOnly(eulaFileName)
		os.Remove(eulaFileName)
	}
	if utils.FileExists(eulaFileName) {
		log.Errorf("Cannot remove previous copy of license file: %q", eulaFileName)
		return errs.ErrFailedCreatingFile
	}

	return os.WriteFile(eulaFileName, eulaContents, utils.FileModeRO)
}

// resolveVersionModifier takes into account eventual versionModifiers (@, @^, @~ and @>=) to determine
// which version of a pack should be targeted for installation
func (p *PackType) resolveVersionModifier(pdscXML *xml.PdscXML) {
	log.Debugf("Resolving version modifier for %q using PDSC %q", p.path, pdscXML.FileName)

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
			log.Debugf("- resolved(@>=) as %s", p.targetVersion)
		}
		return
	}

	// The trickiest one is @^, because it needs to be the latest
	// release matching the major number.
	// The releases in the PDSC file are sorted from latest to oldest
	if p.versionModifier == utils.GreatestCompatibleVersion {
		for _, version := range pdscXML.AllReleases() {
			sameMajor := utils.SemverMajor(version) == utils.SemverMajor(p.Version)
			if sameMajor && utils.SemverCompare(version, p.Version) >= 0 {
				p.targetVersion = version
				log.Debugf("- resolved (@^) as %s", p.targetVersion)
				return
			}
		}
		// Check if at least same Major version exists
		if utils.SemverCompare(p.targetVersion, p.Version) > 0 {
			p.targetVersion = pdscXML.LatestVersion()
			log.Debugf("- resolved (@^) as %s", p.targetVersion)
		} else {
			log.Errorf("No compatible minor version available for pack version >= %s, highest available major version is %s.x.x", p.Version, utils.SemverMajor(pdscXML.LatestVersion()))
		}
		return
	}

	// The next tricky one is @~, because it needs to be the latest
	// release matching the major and minor number.
	// The releases in the PDSC file are sorted from latest to oldest
	if p.versionModifier == utils.PatchVersion {
		for _, version := range pdscXML.AllReleases() {
			sameMajorMinor := utils.SemverMajorMinor(version) == utils.SemverMajorMinor(p.Version)
			if sameMajorMinor && utils.SemverCompare(version, p.Version) >= 0 {
				p.targetVersion = version
				log.Debugf("- resolved (@~) as %s", p.targetVersion)
				return
			}
		}
		// Check if at least same Major.Minor version exists
		if utils.SemverCompare(p.targetVersion, p.Version) > 0 {
			p.targetVersion = pdscXML.LatestVersion()
			log.Debugf("- resolved (@~) as %s", p.targetVersion)
		} else {
			log.Errorf("No compatible patch version available for pack version >= %s, highest available major.minor version is %s.x", p.Version, utils.SemverMajorMinor(pdscXML.LatestVersion()))
		}
		return
	}

	// Try to install the highest available version in the
	// specified min:max range.
	if p.versionModifier == utils.RangeVersion {
		for _, version := range pdscXML.AllReleases() {
			if utils.SemverCompareRange(version, p.Version) == 0 {
				// the first matching is the best (latest)
				p.targetVersion = version
				log.Debugf("- resolved range %s as %s", p.Version, p.targetVersion)
				return
			}
		}
		log.Errorf("No compatible version available for pack version %s", utils.FormatVersions(p.Version))
		return
	}

	log.Warn("Could not resolve version modifier")
}

// loadDependencies verifies and registers a pack's required packages
func (p *PackType) loadDependencies(nometa bool) error {
	deps := p.Pdsc.Dependencies()
	installed := 0
	if deps == nil {
		return nil
	}
	for i := 0; i < len(deps); i++ {
		version := deps[i][2]
		var pack *PackType
		var err error
		if version == "" {
			pack, err = preparePack(deps[i][1]+"."+deps[i][0], false, false, false, nometa)
			if err != nil {
				return err
			}
		} else {
			pack, err = preparePack(deps[i][1]+"."+deps[i][0]+"."+deps[i][2], false, false, false, nometa)
			if err != nil {
				return err
			}
		}

		// Need to convert the spec <package/requirements/packages> "version" to the one internally used
		if version != "latest" {
			if len(strings.Split(version, ":")) >= 1 {
				pack.versionModifier = utils.RangeVersion
			} else {
				pack.versionModifier = utils.ExactVersion
			}
		} else {
			pack.versionModifier = utils.LatestVersion
		}
		if ok, _ := Installation.PackIsInstalled(pack, false); ok {
			p.Requirements.packages = append(p.Requirements.packages, struct {
				info      []string
				installed bool
			}{deps[i], true})
			installed++
		} else {
			p.Requirements.packages = append(p.Requirements.packages, struct {
				info      []string
				installed bool
			}{deps[i], false})
		}
	}
	if installed == len(deps) {
		p.Requirements.satisfied = true
	}
	return nil
}

func (p *PackType) RequirementsSatisfied() bool {
	return p.Requirements.satisfied
}

// PackID returns the most generic name of a pack: Vendor.PackName
func (p *PackType) PackID() string {
	return p.Vendor + "." + p.Name
}

// PackIDWithVersion returns the packID with version: Vendor.PackName.x.y.z
func (p *PackType) PackIDWithVersion() string {
	return p.PackID() + "." + p.GetVersionNoMeta()
}

// PackFileName returns a string with how the pack file name would be: Vendor.PackName.x.y.z.pack
func (p *PackType) PackFileName() string {
	return p.PackIDWithVersion() + utils.PackExtension
}

// PdscFileName returns a string with how the pack's pdsc file name would be: Vendor.PackName.pdsc
func (p *PackType) PdscFileName() string {
	return p.PackID() + utils.PdscExtension
}

// PdscFileNameWithVersion returns a string with how the pack's pdsc file name would be: Vendor.PackName.x.y.z.pdsc
func (p *PackType) PdscFileNameWithVersion() string {
	return p.PackIDWithVersion() + utils.PdscExtension
}

// GetVersion makes sure to get the latest version for the pack
// after parsing possible version modifiers (@^, @~, @>=)
func (p *PackType) GetVersion() string {
	if p.versionModifier != utils.ExactVersion {
		return p.targetVersion
	}
	return p.Version
}

// GetVersionNoMeta return the version without meta information
func (p *PackType) GetVersionNoMeta() string {
	return utils.SemverStripMeta(p.GetVersion())
}

// toggleReadOnly will be used by Lock() and Unlock() to set or unset Read-Only flag on all pack files
func (p *PackType) toggleReadOnly(setReadOnly bool) {
	// Vendor/Pack/x.y.z/
	packHomeDir := filepath.Join(Installation.PackRoot, p.Vendor, p.Name, p.GetVersionNoMeta())

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
