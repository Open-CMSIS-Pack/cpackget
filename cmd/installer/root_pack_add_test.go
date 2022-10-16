/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/ui"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	"github.com/stretchr/testify/assert"
)

// Tests should cover all possible scenarios for adding packs. Here are all possible ones:
// cpackget pack add Vendor.PackName                               # packID without version
// cpackget pack add Vendor.PackName.x.y.z                         # packID with version
// cpackget pack add Vendor::PackName                              # packID using legacy syntax
// cpackget pack add Vendor::PackName@x.y.z                        # packID using legacy syntax specifying an exact version
// cpackget pack add Vendor::PackName@~x.y.z                       # packID using legacy syntax specifying a minumum compatible version
// cpackget pack add Vendor::PackName>=x.y.z                       # packID using legacy syntax specifying a minumum version
// cpackget pack add Vendor.PackName.x.y.z.pack                    # pack file name
// cpackget pack add https://vendor.com/Vendor.PackName.x.y.z.pack # pack URL
//
// So it doesn't really matter how the pack is specified, cpackget should
// handle is as normal.
func TestAddPack(t *testing.T) {

	assert := assert.New(t)

	// Sanity tests
	t.Run("test installing a pack with bad name", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-bad-name"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := malformedPackName

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrBadPackName)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack previously installed", func(t *testing.T) {
		localTestingDir := "test-add-pack-already-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

		packPath := publicLocalPack123
		addPack(t, packPath, ConfigType{
			IsPublic: true,
		})

		packIdxModTime := getPackIdxModTime(t, Start)

		// Attempt installing it again, this time it should noop
		packPath = publicLocalPack123
		assert.Nil(installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout))

		// Make sure pack.idx did NOT get touched
		assert.Equal(packIdxModTime, getPackIdxModTime(t, End))
	})

	t.Run("test force-reinstalling a pack not yet installed", func(t *testing.T) {
		localTestingDir := "test-add-pack-force-reinstall-not-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := publicLocalPack123
		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, ForceReinstall, Timeout)
		assert.Nil(err)

		packInfo, err := utils.ExtractPackInfo(packPath)
		assert.Nil(err)
		checkPackIsInstalled(t, packInfoToType(packInfo))
	})

	t.Run("test force-reinstalling an installed pack", func(t *testing.T) {
		localTestingDir := "test-add-pack-force-reinstall-already-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packToReinstall
		addPack(t, packPath, ConfigType{})

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, ForceReinstall, Timeout)
		assert.Nil(err)

		packToReinstall, err := utils.ExtractPackInfo(packPath)
		assert.Nil(err)
		checkPackIsInstalled(t, packInfoToType(packToReinstall))
	})

	t.Run("test force-reinstalling a pack with a user interruption", func(t *testing.T) {
		localTestingDir := "test-add-pack-force-reinstall-user-interruption"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packToReinstall
		addPack(t, packPath, ConfigType{})

		// Simulate a ctrl+c (as done in security_test.go)
		utils.ShouldAbortFunction = func() bool {
			return true
		}

		defer func() {
			utils.ShouldAbortFunction = nil
		}()

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, ForceReinstall, Timeout)
		// Should not install anything, and revert the temporary pack to its original directory
		originalPackPath := filepath.Join(installer.Installation.PackRoot, "TheVendor", "PackToReinstall", "1.2.3")
		assert.True(errs.Is(err, errs.ErrTerminatedByUser))
		assert.DirExists(originalPackPath)
		assert.NoDirExists(originalPackPath + "_tmp")
	})

	t.Run("test installing local pack that does not exist", func(t *testing.T) {
		localTestingDir := "test-add-local-pack-that-does-not-exist"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packThatDoesNotExist

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrFileNotFound)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing remote pack that does not exist", func(t *testing.T) {
		localTestingDir := "test-add-remote-pack-that-does-not-exist"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		notFoundServer := NewServer()

		packPath := notFoundServer.URL() + packThatDoesNotExist

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrBadRequest)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack with corrupt zip file", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-corrupt-zip"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packWithCorruptZip

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrFailedDecompressingFile)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack with bad URL format", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-malformed-url"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packWithMalformedURL

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrBadPackURL)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack with no PDSC file inside", func(t *testing.T) {
		localTestingDir := "test-add-pack-without-pdsc-file"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packWithoutPdscFileInside

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrPdscFileNotFound)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	// FIXME: This test does currently pass on arm64, but for some
	// reason it fails on the github actions pipeline.
	// Might be related to running in a dockerized environment.
	if runtime.GOARCH != "arm64" {
		t.Run("test installing a pack that has problems with its directory", func(t *testing.T) {
			localTestingDir := "test-add-pack-with-unaccessible-directory"
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			installer.UnlockPackRoot()
			installer.Installation.WebDir = filepath.Join(testDir, "public_index")
			defer removePackRoot(localTestingDir)

			packPath := publicLocalPack123

			// Force a bad file path
			installer.Installation.PackRoot = filepath.Join(string(os.PathSeparator), "CON")
			err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)

			// Sanity check
			assert.NotNil(err)
			assert.Equal(err, errs.ErrFailedCreatingDirectory)

			// Make sure pack.idx never got touched
			assert.False(utils.FileExists(installer.Installation.PackIdx))
		})
	}

	t.Run("test installing a pack with tainted compressed files", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-tainted-compressed-files"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

		packPath := packWithTaintedCompressedFiles

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(errs.ErrInsecureZipFileName, err)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack with version not present in the pdsc file", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-version-not-present-in-the-pdsc-file"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := pack123MissingVersion

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrPackVersionNotFoundInPdsc)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack with version not the latest in the pdsc file", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-version-not-the-latest-in-the-pdsc-file"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := pack123VersionNotLatest

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrPackVersionNotLatestReleasePdsc)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	// Test installing a combination of public/non-public local/remote packs
	t.Run("test installing public pack via local file", func(t *testing.T) {
		localTestingDir := "test-add-public-local-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

		packPath := publicLocalPack123
		addPack(t, packPath, ConfigType{
			IsPublic: true,
		})
	})

	t.Run("test installing public pack via remote file", func(t *testing.T) {
		localTestingDir := "test-add-public-remote-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

		zipContent, err := ioutil.ReadFile(publicRemotePack123)
		assert.Nil(err)
		packServer := NewServer()
		packServer.AddRoute("*", zipContent)

		_, packBasePath := filepath.Split(publicRemotePack123)

		packPath := packServer.URL() + packBasePath

		addPack(t, packPath, ConfigType{
			IsPublic: true,
		})
	})

	t.Run("test installing non-public pack via local file", func(t *testing.T) {
		localTestingDir := "test-add-non-public-local-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

		packPath := nonPublicLocalPack123
		addPack(t, packPath, ConfigType{
			IsPublic: false,
		})
	})

	t.Run("test installing non-public pack via remote file", func(t *testing.T) {
		localTestingDir := "test-add-non-public-remote-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

		packPath := nonPublicRemotePack123
		addPack(t, packPath, ConfigType{
			IsPublic: false,
		})
	})

	// Test that cpackget will attempt to retrieve the PDSC file of public packs and place it under .Web/
	t.Run("test installing public pack retrieving pdsc file", func(t *testing.T) {
		localTestingDir := "test-add-public-pack-retrieving-pdsc-file"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := publicLocalPack123
		packInfo, err := utils.ExtractPackInfo(packPath)
		assert.Nil(err)
		pack := packInfoToType(packInfo)

		packPdscFilePath := filepath.Join(filepath.Dir(packPath), pack.PdscFileName())
		packPdscContent, err := ioutil.ReadFile(packPdscFilePath)
		assert.Nil(err)

		pdscServer := NewServer()
		pdscServer.AddRoute(pack.PdscFileName(), packPdscContent)

		packPdscTag := xml.PdscTag{
			URL:     pdscServer.URL(),
			Vendor:  pack.Vendor,
			Name:    pack.Name,
			Version: pack.Version,
		}

		assert.Nil(installer.Installation.PublicIndexXML.AddPdsc(packPdscTag))
		assert.Nil(installer.Installation.PublicIndexXML.Write())

		addPack(t, packPath, ConfigType{
			IsPublic: true,
		})
	})

	// Test licenses
	t.Run("test installing pack without license", func(t *testing.T) {
		localTestingDir := "test-add-pack-without-license"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

		packPath := nonPublicLocalPack123
		addPack(t, packPath, ConfigType{
			CheckEula: true,
		})
	})

	t.Run("test installing pack with license disagreed", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-license-disagreed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packWithLicense

		info, err := utils.ExtractPackInfo(packPath)
		assert.Nil(err)

		// Should NOT be installed if license is not agreed
		ui.LicenseAgreed = &ui.Disagreed
		err = installer.AddPack(packPath, CheckEula, !ExtractEula, !ForceReinstall, Timeout)

		// Sanity check
		assert.Nil(err)
		assert.False(utils.FileExists(installer.Installation.PackIdx))

		// Check in installer internals
		pack := packInfoToType(info)
		assert.False(installer.Installation.PackIsInstalled(pack))
	})

	t.Run("test installing pack with license agreed", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-license-agreed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packWithLicense
		ui.LicenseAgreed = &ui.Agreed
		addPack(t, packPath, ConfigType{
			CheckEula: true,
		})
	})

	t.Run("test installing pack with rtf license agreed", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-rtf-license-agreed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packWithRTFLicense
		ui.LicenseAgreed = &ui.Agreed
		addPack(t, packPath, ConfigType{
			CheckEula: true,
		})
	})

	t.Run("test installing pack with license agreement skipped", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-license-skipped"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packWithLicense
		addPack(t, packPath, ConfigType{
			CheckEula: false,
		})
	})

	t.Run("test installing pack with license extracted", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-license-extracted"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packWithLicense

		extractedLicensePath := filepath.Join(installer.Installation.DownloadDir, filepath.Base(packPath)+".LICENSE.txt")

		ui.LicenseAgreed = nil
		addPack(t, packPath, ConfigType{
			CheckEula:   true,
			ExtractEula: true,
		})

		assert.True(utils.FileExists(extractedLicensePath))
		os.Remove(extractedLicensePath)
	})

	t.Run("test installing pack with missing license", func(t *testing.T) {
		// Missing license means it is specified in the PDSC file, but the actual license
		// file is not there
		localTestingDir := "test-add-pack-with-missing-license"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packWithMissingLicense

		info, err := utils.ExtractPackInfo(packPath)
		assert.Nil(err)

		// Should NOT be installed if license is missing
		err = installer.AddPack(packPath, CheckEula, !ExtractEula, !ForceReinstall, Timeout)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(errs.ErrLicenseNotFound, err)
		assert.False(utils.FileExists(installer.Installation.PackIdx))

		// Check in installer internals
		pack := packInfoToType(info)
		assert.False(installer.Installation.PackIsInstalled(pack))
	})

	t.Run("test installing pack with missing license extracted", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-license-extracted"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packWithMissingLicense

		extractedLicensePath := packPath + ".LICENSE.txt"

		ui.Extract = true
		ui.LicenseAgreed = nil
		err := installer.AddPack(packPath, CheckEula, ExtractEula, !ForceReinstall, Timeout)
		assert.NotNil(err)
		assert.Equal(errs.ErrLicenseNotFound, err)
		assert.False(utils.FileExists(extractedLicensePath))
		os.Remove(extractedLicensePath)
	})

	// Pack with the entire pack structure within another folder
	t.Run("test installing pack within subfolder", func(t *testing.T) {
		localTestingDir := "test-add-pack-within-subfolder"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packWithSubFolder
		addPack(t, packPath, ConfigType{})
	})

	t.Run("test installing pack within too many subfolders", func(t *testing.T) {
		localTestingDir := "test-add-pack-within-too-many-subfolder"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		packPath := packWithSubSubFolder
		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.NotNil(err)
		assert.Equal(err, errs.ErrPdscFileTooDeepInPack)
	})

	// Install packs with pack id: Vendor.PackName[.x.y.z]
	for _, packPath := range []string{publicRemotePack123PackID, publicRemotePackPackID, publicRemotePackLegacyPackID, publicRemotePack123LegacyPackID} {

		safePackPath := strings.ReplaceAll(packPath, "::", "..")
		safePackPath = strings.ReplaceAll(safePackPath, "@", "-at-")

		t.Run("test installing pack with pack id pdsc file not found "+packPath, func(t *testing.T) {
			localTestingDir := "test-add-pack-with-pack-id-pdsc-file-not-found-" + safePackPath
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			installer.UnlockPackRoot()
			defer removePackRoot(localTestingDir)

			// Fake public index server
			publicIndexServer := NewServer()

			// Tweak the URL for the pack's pdsc
			packInfo, err := utils.ExtractPackInfo(packPath)
			assert.Nil(err)
			packPdscTag := xml.PdscTag{Vendor: packInfo.Vendor, Name: packInfo.Pack, Version: packInfo.Version}
			packPdscTag.URL = publicIndexServer.URL()
			err = installer.Installation.PublicIndexXML.AddPdsc(packPdscTag)
			assert.Nil(err)

			err = installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)

			assert.NotNil(err)
			assert.Equal(err, errs.ErrPackPdscCannotBeFound)

			// Make sure pack.idx never got touched
			assert.False(utils.FileExists(installer.Installation.PackIdx))
		})

		// This also tests the case where the URL in the pdsc tag serves the correct
		// pdsc file, but DOES NOT serve a pack file
		t.Run("test installing pack with pack id version not found "+packPath, func(t *testing.T) {
			localTestingDir := "test-add-pack-with-pack-id-version-not-found-" + safePackPath
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			installer.UnlockPackRoot()
			defer removePackRoot(localTestingDir)

			packInfo, err := utils.ExtractPackInfo(packPath)
			assert.Nil(err)
			pack := packInfoToType(packInfo)

			// Place the bogus pdsc file in .Web/
			assert.Nil(utils.CopyFile(pdscPack123MissingVersion, filepath.Join(installer.Installation.WebDir, pack.PdscFileName())))

			err = installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)

			assert.NotNil(err)
			assert.Equal(errs.ErrPackVersionNotFoundInPdsc, err)

			// Make sure pack.idx never got touched
			assert.False(utils.FileExists(installer.Installation.PackIdx))
		})

		t.Run("test installing pack with pack id using release url"+packPath, func(t *testing.T) {
			localTestingDir := "test-add-pack-with-pack-id-using-release-url" + safePackPath
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			installer.UnlockPackRoot()
			defer removePackRoot(localTestingDir)

			// Prep pack info
			packInfo, err := utils.ExtractPackInfo(packPath)
			assert.Nil(err)
			pack := packInfoToType(packInfo)

			// Prep the pdsc file to go in .Web/
			packPdscFilePath := filepath.Join(installer.Installation.WebDir, pack.PdscFileName())
			assert.Nil(utils.CopyFile(pdscPack123MissingVersion, packPdscFilePath))

			packContent, err := ioutil.ReadFile(publicRemotePack123)
			assert.Nil(err)

			// Fake server
			// should serve pack.zip with the pack content
			server := NewServer()
			server.AddRoute("pack.zip", packContent)

			// Prep the release tag
			releaseTag := xml.ReleaseTag{URL: server.URL() + "pack.zip", Version: "1.2.3"}
			if pack.Version != "" {
				releaseTag.Version = pack.Version
			}

			// Inject the tag into the pdsc file
			pdscXML := xml.NewPdscXML(packPdscFilePath)
			assert.Nil(pdscXML.Read())
			pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, releaseTag)
			assert.Nil(utils.WriteXML(pdscXML.FileName, pdscXML))

			err = installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
			assert.Nil(err)

			pack.Version = "1.2.3"
			pack.IsPublic = true
			checkPackIsInstalled(t, pack)
		})

		t.Run("test installing pack with pack id using pdsc url "+packPath, func(t *testing.T) {
			localTestingDir := "test-add-pack-with-pack-id-using-pdsc-url-" + safePackPath
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			installer.UnlockPackRoot()
			defer removePackRoot(localTestingDir)

			// Prep pack info
			packInfo, err := utils.ExtractPackInfo(packPath)
			assert.Nil(err)
			pack := packInfoToType(packInfo)
			pack.Version = "1.2.3"

			// Prep the pdsc file to go in .Web/
			packPdscFilePath := filepath.Join(installer.Installation.WebDir, pack.PdscFileName())
			assert.Nil(utils.CopyFile(pdscPack123MissingVersion, packPdscFilePath))

			packContent, err := ioutil.ReadFile(publicRemotePack123)
			assert.Nil(err)

			// Fake server
			// should serve pack.zip with the pack content
			server := NewServer()
			server.AddRoute(pack.PackFileName(), packContent)

			// Prep the release tag
			releaseTag := xml.ReleaseTag{Version: "1.2.3"}

			// Inject the tag into the pdsc file
			pdscXML := xml.NewPdscXML(packPdscFilePath)
			assert.Nil(pdscXML.Read())
			pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, releaseTag)
			pdscXML.URL = server.URL()
			assert.Nil(utils.WriteXML(pdscXML.FileName, pdscXML))

			err = installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
			assert.Nil(err)

			pack.IsPublic = true
			checkPackIsInstalled(t, pack)
		})
	}

	t.Run("test installing pack with pack id using pdsc url when version is not the latest in index.pidx", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-pack-id-using-pdsc-url-version-not-the-latest"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Prep pack info
		packPath := publicLocalPack123
		packInfo, err := utils.ExtractPackInfo(packPath)
		assert.Nil(err)
		pack := packInfoToType(packInfo)

		// Fake server
		// should serve *.pack and *.pdsc
		server := NewServer()

		packContent, err := ioutil.ReadFile(packPath)
		assert.Nil(err)

		packPdscXML := xml.NewPdscXML(filepath.Join(filepath.Dir(packPath), pack.PdscFileName()))
		assert.Nil(packPdscXML.Read())
		packPdscXML.URL = server.URL()
		assert.Nil(utils.WriteXML(pack.PdscFileName(), packPdscXML))
		defer os.Remove(pack.PdscFileName())
		packPdscContent, err := ioutil.ReadFile(pack.PdscFileName())
		assert.Nil(err)

		server.AddRoute(pack.PackFileName(), packContent)
		server.AddRoute(pack.PdscFileName(), packPdscContent)

		// Inject the pdsc tag into .Web/index.pix pointing to a version above of
		// the version being installed
		packPdscTag := xml.PdscTag{
			URL:     server.URL(),
			Vendor:  pack.Vendor,
			Name:    pack.Name,
			Version: "1.2.4",
		}

		assert.Nil(installer.Installation.PublicIndexXML.AddPdsc(packPdscTag))
		assert.Nil(installer.Installation.PublicIndexXML.Write())

		addPack(t, pack.PackIDWithVersion(), ConfigType{
			IsPublic: true,
		})
	})

	t.Run("test installing non-public pack via packID", func(t *testing.T) {
		localTestingDir := "test-add-non-public-local-packid"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

		pack123Path := nonPublicLocalPack123
		pack124Path := nonPublicLocalPack124
		pack123ID := nonPublicLocalPack123PackID
		//pack124ID := nonPublicLocalPack124PackID

		pack124Info, err := utils.ExtractPackInfo(pack124Path)
		assert.Nil(err)
		pack124 := packInfoToType(pack124Info)

		pack123Info, err := utils.ExtractPackInfo(pack123Path)
		assert.Nil(err)
		pack123 := packInfoToType(pack123Info)

		pack123Content, err := ioutil.ReadFile(pack123Path)
		assert.Nil(err)

		server := NewServer()
		server.AddRoute(pack123.PackFileName(), pack123Content)

		// Attempt to install with PackID only first time, with no success (no pdsc in .Local)
		err = installer.AddPack(pack123ID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Equal(err, errs.ErrPackURLCannotBeFound)

		// Add the pack via file, then remove it just to leave the pdsc in .Local
		addPack(t, pack124Path, ConfigType{
			IsPublic: false,
		})

		// The 1.2.4 pack's PDSC does NOT contain the 1.2.3 release tag on purpose
		// so an attemp to install it should raise an error
		err = installer.AddPack(pack123ID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Equal(err, errs.ErrPackVersionNotFoundInPdsc)

		// Tweak the URL to retrieve version 1.2.3 and inject the 1.2.3 tag
		pdscXML := xml.NewPdscXML(filepath.Join(installer.Installation.LocalDir, pack124.PdscFileName()))
		utils.UnsetReadOnly(pdscXML.FileName)
		assert.Nil(pdscXML.Read())
		pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, xml.ReleaseTag{Version: "1.2.3"})
		pdscXML.URL = server.URL()
		assert.Nil(utils.WriteXML(pdscXML.FileName, pdscXML))

		err = installer.AddPack(pack123ID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Nil(err)
		checkPackIsInstalled(t, pack123)
	})

	t.Run("test installing a pack that got cancelled during download", func(t *testing.T) {
		localTestingDir := "test-add-pack-cancelled-during-download"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

		// Fake a user termination request
		utils.ShouldAbortFunction = func() bool {
			return true
		}

		// Reset it at the end
		defer func() {
			utils.ShouldAbortFunction = nil
		}()

		zipContent, err := ioutil.ReadFile(publicRemotePack123)
		assert.Nil(err)
		packServer := NewServer()
		packServer.AddRoute("*", zipContent)

		_, packBasePath := filepath.Split(publicRemotePack123)

		packPath := packServer.URL() + packBasePath
		err = installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.NotNil(err)
		assert.Equal(errs.ErrTerminatedByUser, err)

		// Make sure there's no pack file in the .Download
		assert.False(utils.FileExists(filepath.Join(installer.Installation.DownloadDir, packBasePath)))

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack that got cancelled during extraction", func(t *testing.T) {
		localTestingDir := "test-add-cancelled-during-extraction"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

		// Fake a user termination request
		skipAbortingOnPdscCopy := -1
		utils.ShouldAbortFunction = func() bool {
			skipAbortingOnPdscCopy += 1
			return skipAbortingOnPdscCopy > 1
		}

		// Reset it at the end
		defer func() {
			utils.ShouldAbortFunction = nil
		}()

		packPath := publicLocalPack123

		packInfo, err := utils.ExtractPackInfo(packPath)
		assert.Nil(err)
		pack := packInfoToType(packInfo)

		err = installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.NotNil(err)
		assert.Equal(errs.ErrTerminatedByUser, err)

		// Make sure there's no pack folder in Vendor/PackName/x.y.z/, Vendor/PackName/ and Vendor/
		assert.False(utils.DirExists(filepath.Join(installer.Installation.PackRoot, pack.Vendor, pack.Name, pack.Version)))
		assert.False(utils.DirExists(filepath.Join(installer.Installation.PackRoot, pack.Vendor, pack.Name)))
		assert.False(utils.DirExists(filepath.Join(installer.Installation.PackRoot, pack.Vendor)))

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	//
	// Tests below cover the following syntax for both public and local packs:
	// - TheVendor::PackName
	// - TheVendor::PackName@latest
	// - TheVendor::PackName@1.2.3
	// - TheVendor::PackName@~1.2.3
	// - TheVendor::PackName>=1.2.3

	t.Run("test installing a pack with a minimum version specified and newer version pre-installed", func(t *testing.T) {
		// This test case checks the following use case:
		// 1. There's already a pack 1.2.4 installed
		// 2. An attempt to install a pack with >=1.2.3
		// 3. No installation should proceed because the pre-installed 1.2.4 pack already satisfies the >=1.2.3 condition

		localTestingDir := "test-installing-pack-with-minimum-version-new-pre-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Inject pdsc into .Web folder
		packPdscFilePath := filepath.Join(installer.Installation.WebDir, filepath.Base(pdscPublicLocalPack))
		assert.Nil(utils.CopyFile(pdscPublicLocalPack, packPdscFilePath))

		// Install 1.2.4
		addPack(t, publicLocalPack124, ConfigType{
			IsPublic: true,
		})

		// Install >=1.2.3 and make sure nothing gets installed
		packIdx, err := os.Stat(installer.Installation.PackIdx)
		assert.Nil(err)
		packIdxModTime := packIdx.ModTime()

		err = installer.AddPack(publicLocalPack123WithMinimumVersionLegacyPackID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Nil(err)

		// Make sure pack.idx did NOT get touched
		packIdx, err = os.Stat(installer.Installation.PackIdx)
		assert.Nil(err)
		assert.Equal(packIdxModTime, packIdx.ModTime())
	})

	t.Run("test installing a pack with a minimum version specified without any pre-installed version", func(t *testing.T) {
		// This test case checks the following use case:
		// 1. There are no packs installed
		// 2. An attempt to install a pack with >=1.2.3
		// 3. Then pack 1.2.4 should be installed because that's the latest available

		localTestingDir := "test-installing-pack-with-minimum-version-none-pre-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Inject pdsc into .Web folder
		packPdscFilePath := filepath.Join(installer.Installation.WebDir, filepath.Base(pdscPublicLocalPack))
		assert.Nil(utils.CopyFile(pdscPublicLocalPack, packPdscFilePath))

		// Prepare URLs for downloading the pack
		pack124 := installer.PackType{}
		pack124.Vendor = "TheVendor"
		pack124.Name = "PublicLocalPack"
		pack124.Version = "1.2.4"
		pack124.IsPublic = true

		// Prep server
		pack124Content, err := ioutil.ReadFile(publicLocalPack124)
		assert.Nil(err)
		server := NewServer()
		server.AddRoute(pack124.PackFileName(), pack124Content)

		// Inject URL into pdsc
		pdscXML := xml.NewPdscXML(packPdscFilePath)
		assert.Nil(pdscXML.Read())
		pdscXML.URL = server.URL()
		assert.Nil(utils.WriteXML(packPdscFilePath, pdscXML))

		// Install >=1.2.3
		err = installer.AddPack(publicLocalPack123WithMinimumVersionLegacyPackID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Nil(err)

		// Check that 1.2.4 is installed
		checkPackIsInstalled(t, &pack124)
	})

	t.Run("test installing a pack with a minimum version specified and older version pre-installed", func(t *testing.T) {
		// This test case checks the following use case:
		// 1. Version 1.2.2 is installed
		// 2. An attempt to install a pack with >=1.2.3
		// 3. Then pack 1.2.4 should be installed because the pre-installed 1.2.2 does not satisfy >=1.2.3

		localTestingDir := "test-installing-pack-with-minimum-version-older-pre-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Inject pdsc into .Web folder
		packPdscFilePath := filepath.Join(installer.Installation.WebDir, filepath.Base(pdscPublicLocalPack))
		assert.Nil(utils.CopyFile(pdscPublicLocalPack, packPdscFilePath))

		// Pre-install 1.2.2
		addPack(t, publicLocalPack122, ConfigType{
			IsPublic: true,
		})

		// Prepare URLs for downloading pack 1.2.4
		pack124 := installer.PackType{}
		pack124.Vendor = "TheVendor"
		pack124.Name = "PublicLocalPack"
		pack124.Version = "1.2.4"
		pack124.IsPublic = true

		// Prep server
		pack124Content, err := ioutil.ReadFile(publicLocalPack124)
		assert.Nil(err)
		server := NewServer()
		server.AddRoute(pack124.PackFileName(), pack124Content)

		// Inject URL into pdsc
		pdscXML := xml.NewPdscXML(packPdscFilePath)
		utils.UnsetReadOnly(packPdscFilePath)
		assert.Nil(pdscXML.Read())
		pdscXML.URL = server.URL()
		assert.Nil(utils.WriteXML(packPdscFilePath, pdscXML))

		// Install >=1.2.3
		err = installer.AddPack(publicLocalPack123WithMinimumVersionLegacyPackID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Nil(err)

		// Check that 1.2.4 is installed
		checkPackIsInstalled(t, &pack124)
	})

	t.Run("test installing a pack with a minimum version specified higher than the latest available", func(t *testing.T) {
		// This test case checks the following use case:
		// 1. There are no packs installed
		// 2. Versions 1.2.2, 1.2.3, 1.2.4 are available to install
		// 3. Attempt to install >=1.2.5
		// 4. Should fail as the minimum version is not available to install

		localTestingDir := "test-installing-pack-with-minimum-version-higher-latest"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Inject pdsc into .Web folder
		packPdscFilePath := filepath.Join(installer.Installation.WebDir, filepath.Base(pdscPublicLocalPack))
		assert.Nil(utils.CopyFile(pdscPublicLocalPack, packPdscFilePath))

		err := installer.AddPack(publicLocalPack125WithMinimumVersionLegacyPackID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Equal(err, errs.ErrPackVersionNotAvailable)
	})

	t.Run("test installing a pack with a minimum compatible version specified and newer major version pre-installed", func(t *testing.T) {
		// This test case checks the following use case:
		// 1. There's already a pack 1.2.4 installed
		// 2. An attempt to install a pack with @~0.1.0
		// 3. Should install 0.1.1 because it's the latest compatible version with 0.1.0

		localTestingDir := "test-installing-pack-with-minimum-compatible-version-new-major-pre-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Inject pdsc into .Web folder
		packPdscFilePath := filepath.Join(installer.Installation.WebDir, filepath.Base(pdscPublicLocalPack))
		assert.Nil(utils.CopyFile(pdscPublicLocalPack, packPdscFilePath))

		// Install 1.2.4
		addPack(t, publicLocalPack124, ConfigType{
			IsPublic: true,
		})

		// Prepare URLs for downloading pack 0.1.1
		pack011 := installer.PackType{}
		pack011.Vendor = "TheVendor"
		pack011.Name = "PublicLocalPack"
		pack011.Version = "0.1.1"
		pack011.IsPublic = true

		// Prep server
		pack011Content, err := ioutil.ReadFile(publicLocalPack011)
		assert.Nil(err)
		server := NewServer()
		server.AddRoute(pack011.PackFileName(), pack011Content)

		// Inject URL into pdsc
		pdscXML := xml.NewPdscXML(packPdscFilePath)
		utils.UnsetReadOnly(packPdscFilePath)
		assert.Nil(pdscXML.Read())
		pdscXML.URL = server.URL()
		assert.Nil(utils.WriteXML(packPdscFilePath, pdscXML))

		// Install @~0.1.0
		err = installer.AddPack(publicLocalPack010WithMinimumCompatibleVersionLegacyPackID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Nil(err)

		// Check that 0.1.1 is installed
		checkPackIsInstalled(t, &pack011)
	})

	t.Run("test installing a pack with a minimum compatible version specified without any pre-installed version", func(t *testing.T) {
		// This test case checks the following use case:
		// 1. There are no packs installed
		// 2. An attempt to install a pack with @~0.1.0
		// 3. Should install 0.1.1 because it's the latest compatible version with 0.1.0

		localTestingDir := "test-installing-pack-with-minimum-compatible-none-pre-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Inject pdsc into .Web folder
		packPdscFilePath := filepath.Join(installer.Installation.WebDir, filepath.Base(pdscPublicLocalPack))
		assert.Nil(utils.CopyFile(pdscPublicLocalPack, packPdscFilePath))

		// Prepare URLs for downloading pack 0.1.1
		pack011 := installer.PackType{}
		pack011.Vendor = "TheVendor"
		pack011.Name = "PublicLocalPack"
		pack011.Version = "0.1.1"
		pack011.IsPublic = true

		// Prep server
		pack011Content, err := ioutil.ReadFile(publicLocalPack011)
		assert.Nil(err)
		server := NewServer()
		server.AddRoute(pack011.PackFileName(), pack011Content)

		// Inject URL into pdsc
		pdscXML := xml.NewPdscXML(packPdscFilePath)
		assert.Nil(pdscXML.Read())
		pdscXML.URL = server.URL()
		assert.Nil(utils.WriteXML(packPdscFilePath, pdscXML))

		// Install @~0.1.0
		err = installer.AddPack(publicLocalPack010WithMinimumCompatibleVersionLegacyPackID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Nil(err)

		// Check that 0.1.1 is installed
		checkPackIsInstalled(t, &pack011)
	})

	t.Run("test installing a pack with a minimum compatible version specified and older version pre-installed", func(t *testing.T) {
		// This test case checks the following use case:
		// 1. There's already a pack 0.1.0 installed
		// 2. An attempt to install a pack with @~0.1.1
		// 3. Should install 0.1.1 because it's the more recent compatible version if compared to with 0.1.0

		localTestingDir := "test-installing-pack-with-minimum-compatible-version-older-pre-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Inject pdsc into .Web folder
		packPdscFilePath := filepath.Join(installer.Installation.WebDir, filepath.Base(pdscPublicLocalPack))
		assert.Nil(utils.CopyFile(pdscPublicLocalPack, packPdscFilePath))

		// Install 0.1.0
		addPack(t, publicLocalPack010, ConfigType{
			IsPublic: true,
		})

		// Prepare URLs for downloading pack 0.1.1
		pack011 := installer.PackType{}
		pack011.Vendor = "TheVendor"
		pack011.Name = "PublicLocalPack"
		pack011.Version = "0.1.1"
		pack011.IsPublic = true

		// Prep server
		pack011Content, err := ioutil.ReadFile(publicLocalPack011)
		assert.Nil(err)
		server := NewServer()
		server.AddRoute(pack011.PackFileName(), pack011Content)

		// Inject URL into pdsc
		pdscXML := xml.NewPdscXML(packPdscFilePath)
		utils.UnsetReadOnly(packPdscFilePath)
		assert.Nil(pdscXML.Read())
		pdscXML.URL = server.URL()
		assert.Nil(utils.WriteXML(packPdscFilePath, pdscXML))

		// Install @~0.1.0
		err = installer.AddPack(publicLocalPack011WithMinimumCompatibleVersionLegacyPackID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Nil(err)

		// Check that 0.1.1 is installed
		checkPackIsInstalled(t, &pack011)
	})

	t.Run("test installing a pack with a minimum compatible version specified and exact version pre-installed", func(t *testing.T) {
		// This test case checks the following use case:
		// 1. There's already a pack 0.1.1 installed
		// 2. An attempt to install a pack with @~0.1.1
		// 3. Should not do anything

		localTestingDir := "test-installing-pack-with-minimum-compatible-version-same-pre-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Inject pdsc into .Web folder
		packPdscFilePath := filepath.Join(installer.Installation.WebDir, filepath.Base(pdscPublicLocalPack))
		assert.Nil(utils.CopyFile(pdscPublicLocalPack, packPdscFilePath))

		// Install 0.1.1
		addPack(t, publicLocalPack011, ConfigType{
			IsPublic: true,
		})

		// Install @~0.1.1 and make sure nothing gets installed
		packIdx, err := os.Stat(installer.Installation.PackIdx)
		assert.Nil(err)
		packIdxModTime := packIdx.ModTime()

		err = installer.AddPack(publicLocalPack011WithMinimumCompatibleVersionLegacyPackID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Nil(err)

		// Make sure pack.idx did NOT get touched
		packIdx, err = os.Stat(installer.Installation.PackIdx)
		assert.Nil(err)
		assert.Equal(packIdxModTime, packIdx.ModTime())
	})

	t.Run("test installing a pack with a minimum compatible version higher than the latest available", func(t *testing.T) {
		// This test case checks the following use case:
		// 1. There are no packs installed
		// 2. Versions 1.2.2, 1.2.3, 1.2.4 are available to install
		// 3. Attempt to install @~2.1.1
		// 4. Should fail as the minimum comaptible version is not available to install

		localTestingDir := "test-installing-pack-with-minimum-compatible-version-higher-latest"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Inject pdsc into .Web folder
		packPdscFilePath := filepath.Join(installer.Installation.WebDir, filepath.Base(pdscPublicLocalPack))
		assert.Nil(utils.CopyFile(pdscPublicLocalPack, packPdscFilePath))

		err := installer.AddPack(publicLocalPack211WithMinimumCompatibleVersionLegacyPackID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Equal(err, errs.ErrPackVersionNotAvailable)
	})

	t.Run("test installing a pack with @latest version specified without any pre-installed version", func(t *testing.T) {
		// This test case checks the following use case:
		// 1. There are no packs installed
		// 2. An attempt to install a pack with @latest
		// 3. Then pack 1.2.4 should be installed because it's the latest one

		localTestingDir := "test-installing-pack-with-at-latest-version-none-pre-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Inject pdsc into .Web folder
		packPdscFilePath := filepath.Join(installer.Installation.WebDir, filepath.Base(pdscPublicLocalPack))
		assert.Nil(utils.CopyFile(pdscPublicLocalPack, packPdscFilePath))

		// Prepare URLs for downloading pack 1.2.4
		pack124 := installer.PackType{}
		pack124.Vendor = "TheVendor"
		pack124.Name = "PublicLocalPack"
		pack124.Version = "1.2.4"
		pack124.IsPublic = true

		// Prep server
		pack124Content, err := ioutil.ReadFile(publicLocalPack124)
		assert.Nil(err)
		server := NewServer()
		server.AddRoute(pack124.PackFileName(), pack124Content)

		// Inject URL into pdsc
		pdscXML := xml.NewPdscXML(packPdscFilePath)
		assert.Nil(pdscXML.Read())
		pdscXML.URL = server.URL()
		assert.Nil(utils.WriteXML(packPdscFilePath, pdscXML))

		// Install @latest
		err = installer.AddPack(publicLocalPackLatestVersionLegacyPackID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Nil(err)

		// Check that 1.2.4 is installed
		checkPackIsInstalled(t, &pack124)
	})

	t.Run("test installing a pack with @latest version specified and any pre-installed version", func(t *testing.T) {
		// This test case checks the following use case:
		// 1. Version 1.2.3 is installed
		// 2. An attempt to install a pack with @latest
		// 3. Then pack 1.2.4 should be installed because it's more up-to-date than 1.2.3

		localTestingDir := "test-installing-pack-with-at-latest-version-none-pre-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Inject pdsc into .Web folder
		packPdscFilePath := filepath.Join(installer.Installation.WebDir, filepath.Base(pdscPublicLocalPack))
		assert.Nil(utils.CopyFile(pdscPublicLocalPack, packPdscFilePath))

		// Pre-install 1.2.3
		addPack(t, publicLocalPack123, ConfigType{
			IsPublic: true,
		})

		// Prepare URLs for downloading pack 1.2.4
		pack124 := installer.PackType{}
		pack124.Vendor = "TheVendor"
		pack124.Name = "PublicLocalPack"
		pack124.Version = "1.2.4"
		pack124.IsPublic = true

		// Prep server
		pack124Content, err := ioutil.ReadFile(publicLocalPack124)
		assert.Nil(err)
		server := NewServer()
		server.AddRoute(pack124.PackFileName(), pack124Content)

		// Inject URL into pdsc
		pdscXML := xml.NewPdscXML(packPdscFilePath)
		utils.UnsetReadOnly(packPdscFilePath)
		assert.Nil(pdscXML.Read())
		pdscXML.URL = server.URL()
		assert.Nil(utils.WriteXML(packPdscFilePath, pdscXML))

		// Install @latest
		err = installer.AddPack(publicLocalPackLatestVersionLegacyPackID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Nil(err)

		// Check that 1.2.4 is installed
		checkPackIsInstalled(t, &pack124)
	})

	t.Run("test installing a pack with @latest version specified and most updated pre-installed version", func(t *testing.T) {
		// This test case checks the following use case:
		// 1. There's already a pack 1.2.4 installed
		// 2. An attempt to install a pack with @latest
		// 3. No installation should proceed because the pre-installed 1.2.4 pack already satisfies the @latest condition

		localTestingDir := "test-installing-pack-with-at-latest-version-latest-pre-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Inject pdsc into .Web folder
		packPdscFilePath := filepath.Join(installer.Installation.WebDir, filepath.Base(pdscPublicLocalPack))
		assert.Nil(utils.CopyFile(pdscPublicLocalPack, packPdscFilePath))

		// Install 1.2.4
		addPack(t, publicLocalPack124, ConfigType{
			IsPublic: true,
		})

		// Install @latest and make sure nothing gets installed
		packIdx, err := os.Stat(installer.Installation.PackIdx)
		assert.Nil(err)
		packIdxModTime := packIdx.ModTime()

		err = installer.AddPack(publicLocalPackLatestVersionLegacyPackID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Nil(err)

		// Make sure pack.idx did NOT get touched
		packIdx, err = os.Stat(installer.Installation.PackIdx)
		assert.Nil(err)
		assert.Equal(packIdxModTime, packIdx.ModTime())
	})

	// Now test for local packs, just to increase coverage

	t.Run("test installing a local pack with a minimum version specified and newer version pre-installed", func(t *testing.T) {
		// This test case checks the following use case:
		// 1. There's already a pack 1.2.2 installed
		// 2. An attempt to install a pack with >=1.2.3
		// 3. Then pack 1.2.4 should be installed because that's the latest available

		localTestingDir := "test-installing-local-pack-with-minimum-version-new-pre-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Install 1.2.2
		addPack(t, publicLocalPack122, ConfigType{})

		// Inject the URL for downloading 1.2.4 in .Local/
		packPdscFilePath := filepath.Join(installer.Installation.LocalDir, filepath.Base(pdscPublicLocalPack))

		// Prepare URLs for downloading the pack
		pack124 := installer.PackType{}
		pack124.Vendor = "TheVendor"
		pack124.Name = "PublicLocalPack"
		pack124.Version = "1.2.4"

		// Prep server
		pack124Content, err := ioutil.ReadFile(publicLocalPack124)
		assert.Nil(err)
		server := NewServer()
		server.AddRoute(pack124.PackFileName(), pack124Content)

		// Inject URL and 1.2.4. release tag into pdsc
		pdscXML := xml.NewPdscXML(packPdscFilePath)
		utils.UnsetReadOnly(packPdscFilePath)
		assert.Nil(pdscXML.Read())
		pdscXML.URL = server.URL()
		releaseTag := xml.ReleaseTag{Version: "1.2.4"}
		pdscXML.ReleasesTag.Releases = append([]xml.ReleaseTag{releaseTag}, pdscXML.ReleasesTag.Releases...)
		assert.Nil(utils.WriteXML(packPdscFilePath, pdscXML))

		// Install >=1.2.3
		err = installer.AddPack(publicLocalPack123WithMinimumVersionLegacyPackID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Nil(err)

		// Check that 1.2.4 is installed
		checkPackIsInstalled(t, &pack124)
	})

	t.Run("test installing a local pack with @latest version specified and matching version pre-installed", func(t *testing.T) {
		// This test case checks the following use case:
		// 1. There's already a pack 1.2.4 installed
		// 2. An attempt to install a pack with @latest
		// 3. Should do nothing

		localTestingDir := "test-installing-local-pack-with-at-latest-version-matching-pre-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Install 1.2.4
		addPack(t, publicLocalPack124, ConfigType{})

		// Install @latest and make sure nothing gets installed
		packIdx, err := os.Stat(installer.Installation.PackIdx)
		assert.Nil(err)
		packIdxModTime := packIdx.ModTime()

		err = installer.AddPack(publicLocalPackLatestVersionLegacyPackID, !CheckEula, !ExtractEula, !ForceReinstall, Timeout)
		assert.Nil(err)

		// Make sure pack.idx did NOT get touched
		packIdx, err = os.Stat(installer.Installation.PackIdx)
		assert.Nil(err)
		assert.Equal(packIdxModTime, packIdx.ModTime())
	})
}
