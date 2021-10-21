/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/ui"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	"github.com/stretchr/testify/assert"
)

func packInfoToType(info utils.PackInfo) *installer.PackType {
	pack := &installer.PackType{}
	pack.Vendor = info.Vendor
	pack.Name = info.Pack
	pack.Version = info.Version
	return pack
}

func packPathToPdsc(packPath string, withVersion bool) string {
	if withVersion {
		return packPath[:len(packPath)-len(".pack")] + ".pdsc"
	}
	return packPath[:len(packPath)-len(".x.y.z.pack")] + ".pdsc"
}

func shortenPackPath(packPath string, withVersion bool) string {
	// Remove extension
	_, packName := filepath.Split(packPath)
	ext := filepath.Ext(packName)

	stripLength := len(packName) - len(ext)

	if !withVersion {
		stripLength -= len(".x.y.z")
	}

	return packName[:stripLength]
}

func getPackIdxModTime() time.Time {
	packIdx, _ := os.Stat(installer.Installation.PackIdx)
	return packIdx.ModTime()
}

func checkPackIsInstalled(t *testing.T, packPath string, isPublic bool) {
	assert := assert.New(t)

	info, err := utils.ExtractPackInfo(packPath, false)
	assert.Nil(err)

	// Check in installer internals
	pack := packInfoToType(info)
	assert.True(installer.Installation.PackIsInstalled(pack))

	// Get only basename of the pack
	_, packPath = filepath.Split(packPath)

	// Make sure there's a copy of the pack file in .Download/
	assert.True(utils.FileExists(filepath.Join(installer.Installation.DownloadDir, packPath)))

	// Make sure there's a versioned copy of the PDSC file in .Download/
	assert.True(utils.FileExists(filepath.Join(installer.Installation.DownloadDir, packPathToPdsc(packPath, true))))

	if isPublic {
		// Make sure no PDSC file got copied to .Local/
		assert.False(utils.FileExists(filepath.Join(installer.Installation.LocalDir, packPathToPdsc(packPath, false))))
	} else {
		// Make sure there's an unversioned copy of the PDSC file in .Local/, in case pack is not public
		assert.True(utils.FileExists(filepath.Join(installer.Installation.LocalDir, packPathToPdsc(packPath, false))))
	}

	// Make sure the pack.idx file gets created
	assert.True(utils.FileExists(installer.Installation.PackIdx))
}

type ConfigType struct {
	IsPublic    bool
	CheckEula   bool
	ExtractEula bool
}

func addPack(t *testing.T, packPath string, config ConfigType) {
	assert := assert.New(t)

	err := installer.AddPack(packPath, config.CheckEula, config.ExtractEula)
	assert.Nil(err)

	if config.ExtractEula {
		return
	}

	checkPackIsInstalled(t, packPath, config.IsPublic)
}

func removePack(t *testing.T, packPath string, withVersion, isPublic, purge bool) {
	assert := assert.New(t)

	// Get pack.idx before removing pack
	packIdxModTime := getPackIdxModTime()

	// [http://vendor.com|path/to]/TheVendor.PackName.x.y.z -> TheVendor.PackName[.x.y.z]
	shortPackPath := shortenPackPath(packPath, withVersion)

	info, err := utils.ExtractPackInfo(shortPackPath, true /*short=true*/)
	assert.Nil(err)

	// Check in installer internals
	pack := packInfoToType(info)
	isInstalled := installer.Installation.PackIsInstalled(pack)

	purgeOnly := !isInstalled && purge

	err = installer.RemovePack(shortPackPath, purge)
	assert.Nil(err)

	if isInstalled {
		assert.False(installer.Installation.PackIsInstalled(pack))
	}

	if withVersion {
		// Make sure files are there (purge=false) or if they no longer exist (purge=true) in .Download/
		assert.Equal(!purge, utils.FileExists(filepath.Join(installer.Installation.DownloadDir, shortPackPath+".pack")))
		assert.Equal(!purge, utils.FileExists(filepath.Join(installer.Installation.DownloadDir, shortPackPath+".pdsc")))
	} else {
		// If withVersion=false, it means shortPackPath=TheVendor.PackName only
		// so we need to add '.*' to make utils.ListDir() list all available files
		files, err := utils.ListDir(installer.Installation.DownloadDir, shortPackPath+".*")
		assert.Nil(err)
		assert.Equal(!purge, len(files) > 0)
	}

	if !isPublic {
		// Make sure that the unversioned copy of the PDSC file in .Local/ was removed, in case pack is not public
		assert.False(utils.FileExists(filepath.Join(installer.Installation.LocalDir, packPathToPdsc(packPath, false))))
	}

	// No touch on purging only
	if !purgeOnly {
		if runtime.GOOS == "windows" {
			// Apparently Windows systems update of file modified times
			// happens 64 times per second, and in some cases that is not
			// enough for the time delta below to show a difference
			// Ref: https://www.lochan.org/2005/keith-cl/useful/win32time.html#timingwin
			// So let's sleep a bit before checking for file mod times
			time.Sleep(1 * time.Second)
		}

		// Make sure the pack.idx file gets trouched
		assert.True(packIdxModTime.Before(getPackIdxModTime()))
	}
}

var (
	// Constant telling pack privacy
	IsPublic    = true
	NotPublic   = false
	CheckEula   = true
	ExtractEula = true

	// Available testing packs
	testDir = filepath.Join("..", "..", "testdata", "integration")

	malformedPackName              = "pack-with-bad-name"
	packThatDoesNotExist           = "ThisPack.DoesNotExist.0.0.1.pack"
	packWithCorruptZip             = filepath.Join(testDir, "FakeZip.PackName.1.2.3.pack")
	packWithMalformedURL           = "http://:malformed-url*/TheVendor.PackName.1.2.3.pack"
	packWithoutPdscFileInside      = filepath.Join(testDir, "PackWithout.PdscFileInside.1.2.3.pack")
	packWithTaintedCompressedFiles = filepath.Join(testDir, "PackWith.TaintedFiles.1.2.3.pack")

	// Public packs
	publicLocalPack123  = filepath.Join(testDir, "1.2.3", "TheVendor.PublicLocalPack.1.2.3.pack")
	publicLocalPack124  = filepath.Join(testDir, "1.2.4", "TheVendor.PublicLocalPack.1.2.4.pack")
	publicRemotePack123 = filepath.Join(testDir, "1.2.3", "TheVendor.PublicRemotePack.1.2.3.pack")

	// Private packs
	nonPublicLocalPack123  = filepath.Join(testDir, "1.2.3", "TheVendor.NonPublicLocalPack.1.2.3.pack")
	nonPublicRemotePack123 = filepath.Join(testDir, "1.2.3", "TheVendor.NonPublicRemotePack.1.2.3.pack")

	// Packs with license
	packWithLicense        = filepath.Join(testDir, "TheVendor.PackWithLicense.1.2.3.pack")
	packWithRTFLicense     = filepath.Join(testDir, "TheVendor.PackWithRTFLicense.1.2.3.pack")
	packWithMissingLicense = filepath.Join(testDir, "TheVendor.PackWithMissingLicense.1.2.3.pack")

	// PDSC packs
	pdscPack123 = filepath.Join(testDir, "1.2.3", "TheVendor.PackName.pdsc")
	pdscPack124 = filepath.Join(testDir, "1.2.4", "TheVendor.PackName.pdsc")

	// Bad local_repository.pidx
	badLocalRepositoryPidx = filepath.Join(testDir, "bad_local_repository.pidx")

	// Sample public index.pidx
	samplePublicIndex = filepath.Join(testDir, "SamplePublicIndex.pidx")

	// Malformed index.pidx
	malformedPublicIndex = filepath.Join("..", "..", "testdata", "MalformedPack.pidx")
)

// Tests should cover all possible scenarios for adding packs
// +----------------+--------+------------+
// | origin/privacy | public | non-public |
// +----------------+--------+------------+
// | local          |        |            |
// +----------------+--------+------------+
// | remote         |        |            |
// +----------------+--------+------------+
func TestAddPack(t *testing.T) {

	assert := assert.New(t)

	// Sanity tests
	t.Run("test installing a pack with bad name", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-bad-name"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := malformedPackName

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula)

		// Sanity check
		assert.NotNil(err)
		assert.True(errs.Is(err, errs.ErrBadPackNameInvalidExtension))

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack previously installed", func(t *testing.T) {
		localTestingDir := "test-add-pack-previously-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := publicLocalPack123
		addPack(t, packPath, ConfigType{
			IsPublic: true,
		})

		packIdx, err := os.Stat(installer.Installation.PackIdx)
		assert.Nil(err)
		packIdxModTime := packIdx.ModTime()

		// Attempt installing it again, this time we should get an error
		packPath = publicLocalPack123
		err = installer.AddPack(packPath, !CheckEula, !ExtractEula)
		assert.NotNil(err)
		assert.Equal(err, errs.ErrPackAlreadyInstalled)

		// Make sure pack.idx did NOT get touched
		packIdx, err = os.Stat(installer.Installation.PackIdx)
		assert.Nil(err)
		assert.Equal(packIdxModTime, packIdx.ModTime())
	})

	t.Run("test installing local pack that does not exist", func(t *testing.T) {
		localTestingDir := "test-add-local-pack-that-does-not-exist"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packThatDoesNotExist

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrFileNotFound)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing remote pack that does not exist", func(t *testing.T) {
		localTestingDir := "test-add-remote-pack-that-does-not-exist"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		notFoundServer := httptest.NewServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				},
			),
		)

		packPath := notFoundServer.URL + "/" + packThatDoesNotExist

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrBadRequest)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack with corrupt zip file", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-corrupt-zip"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithCorruptZip

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrFailedDecompressingFile)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack with bad URL format", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-malformed-url"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithMalformedURL

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrBadPackURL)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack with no PDSC file inside", func(t *testing.T) {
		localTestingDir := "test-add-pack-without-pdsc-file"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithoutPdscFileInside

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrPdscFileNotFound)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack that has problems with its directory", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-unaccessible-directory"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := publicLocalPack123

		// Force a bad file path
		installer.Installation.PackRoot = filepath.Join(string(os.PathSeparator), "CON")
		err := installer.AddPack(packPath, !CheckEula, !ExtractEula)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrFailedCreatingDirectory)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack with tainted compressed files", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-tainted-compressed-files"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := packWithTaintedCompressedFiles

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(errs.ErrInsecureZipFileName, err)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	// Test installing a combination of public/non-public local/remote packs
	t.Run("test installing public pack via local file", func(t *testing.T) {
		localTestingDir := "test-add-public-local-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := publicLocalPack123
		addPack(t, packPath, ConfigType{
			IsPublic: true,
		})
	})

	t.Run("test installing public pack via remote file", func(t *testing.T) {
		localTestingDir := "test-add-public-remote-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		zipContent, err := ioutil.ReadFile(publicRemotePack123)
		assert.Nil(err)
		packServer := httptest.NewServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					reader := bytes.NewReader(zipContent)
					_, err := io.Copy(w, reader)
					assert.Nil(err)
				},
			),
		)

		_, packBasePath := filepath.Split(publicRemotePack123)

		packPath := packServer.URL + "/" + packBasePath

		addPack(t, packPath, ConfigType{
			IsPublic: true,
		})
	})

	t.Run("test installing non-public pack via local file", func(t *testing.T) {
		localTestingDir := "test-add-non-public-local-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := nonPublicLocalPack123
		addPack(t, packPath, ConfigType{
			IsPublic: false,
		})
	})

	t.Run("test installing non-public pack via remote file", func(t *testing.T) {
		localTestingDir := "test-add-non-public-remote-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := nonPublicRemotePack123
		addPack(t, packPath, ConfigType{
			IsPublic: false,
		})
	})

	// Test licenses
	t.Run("test installing pack without license", func(t *testing.T) {
		localTestingDir := "test-add-pack-without-license"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := nonPublicLocalPack123
		addPack(t, packPath, ConfigType{
			CheckEula: true,
		})
	})

	t.Run("test installing pack with license disagreed", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-license-disagreed"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithLicense

		info, err := utils.ExtractPackInfo(packPath, false)
		assert.Nil(err)

		// Should NOT be installed if license is not agreed
		ui.LicenseAgreed = &ui.Disagreed
		err = installer.AddPack(packPath, CheckEula, !ExtractEula)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(errs.ErrEula, err)
		assert.False(utils.FileExists(installer.Installation.PackIdx))

		// Check in installer internals
		pack := packInfoToType(info)
		assert.False(installer.Installation.PackIsInstalled(pack))
	})

	t.Run("test installing pack with license agreed", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-license-agreed"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithLicense
		ui.LicenseAgreed = &ui.Agreed
		addPack(t, packPath, ConfigType{
			CheckEula: true,
		})
	})

	t.Run("test installing pack with rtf license agreed", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-rtf-license-agreed"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithRTFLicense
		ui.LicenseAgreed = &ui.Agreed
		addPack(t, packPath, ConfigType{
			CheckEula: true,
		})
	})

	t.Run("test installing pack with license agreement skipped", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-license-skipped"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithLicense
		addPack(t, packPath, ConfigType{
			CheckEula: false,
		})
	})

	t.Run("test installing pack with license extracted", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-license-extracted"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithLicense

		extractedLicensePath := packPath + ".LICENSE.txt"

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
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithMissingLicense

		info, err := utils.ExtractPackInfo(packPath, false)
		assert.Nil(err)

		// Should NOT be installed if license is missing
		err = installer.AddPack(packPath, CheckEula, !ExtractEula)

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
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithMissingLicense

		extractedLicensePath := packPath + ".LICENSE.txt"

		ui.Extract = true
		ui.LicenseAgreed = nil
		err := installer.AddPack(packPath, CheckEula, ExtractEula)
		assert.NotNil(err)
		assert.Equal(errs.ErrLicenseNotFound, err)
		assert.False(utils.FileExists(extractedLicensePath))
		os.Remove(extractedLicensePath)
	})
}

func TestRemovePack(t *testing.T) {

	assert := assert.New(t)

	// Sanity tests
	t.Run("test removing a pack with malformed name", func(t *testing.T) {
		localTestingDir := "test-remove-pack-with-bad-name"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		err := installer.RemovePack("TheVendor.PackName.no-a-valid-version", false)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrBadPackNameInvalidVersion)
	})

	t.Run("test removing a pack that is not installed", func(t *testing.T) {
		localTestingDir := "test-remove-pack-not-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		err := installer.RemovePack("TheVendor.PackName.1.2.3", false)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrPackNotInstalled)
	})

	t.Run("test remove a public pack that was added", func(t *testing.T) {
		localTestingDir := "test-remove-public-pack-that-was-added"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := publicLocalPack123
		config := ConfigType{
			IsPublic: true,
		}

		// Test all possible combinations, with or without version, with or without purging
		addPack(t, packPath, config)
		removePack(t, packPath, true, IsPublic, true) // withVersion=true, purge=true

		addPack(t, packPath, config)
		removePack(t, packPath, true, IsPublic, false) // withVersion=true, purge=false

		addPack(t, packPath, config)
		removePack(t, packPath, false, IsPublic, true) // withVersion=false, purge=true

		addPack(t, packPath, config)
		removePack(t, packPath, false, IsPublic, false) // withVersion=false, purge=false
	})

	t.Run("test remove a non-public pack that was added", func(t *testing.T) {
		localTestingDir := "test-remove-nonpublic-pack-that-was-added"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := nonPublicLocalPack123
		config := ConfigType{
			IsPublic: false,
		}

		// Test all possible combinations, with or without version, with or without purging
		addPack(t, packPath, config)
		removePack(t, packPath, true, NotPublic, true) // withVersion=true, purge=true

		addPack(t, packPath, config)
		removePack(t, packPath, true, NotPublic, false) // withVersion=true, purge=false

		addPack(t, packPath, config)
		removePack(t, packPath, false, NotPublic, true) // withVersion=false, purge=true

		addPack(t, packPath, config)
		removePack(t, packPath, false, IsPublic, false) // withVersion=false, purge=false
	})

	t.Run("test remove version of a pack", func(t *testing.T) {
		localTestingDir := "test-remove-version-of-a-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		// Add a pack, add an updated version of the pack, then remove the first one
		packPath := publicLocalPack123
		updatedPackPath := publicLocalPack124
		config := ConfigType{
			IsPublic: true,
		}
		addPack(t, packPath, config)
		addPack(t, updatedPackPath, config)

		// Remove first one (old pack)
		removePack(t, packPath, true, NotPublic, true) // withVersion=true, purge=true
	})

	t.Run("test remove a pack then purge", func(t *testing.T) {
		localTestingDir := "test-remove-a-pack-then-purge"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		// Add a pack, add an updated version of the pack, then remove the first one
		packPath := publicLocalPack123
		addPack(t, packPath, ConfigType{
			IsPublic: true,
		})

		// Remove it without purge
		removePack(t, packPath, true, NotPublic, false) // withVersion=true, purge=true

		// Now just purge it
		removePack(t, packPath, true, NotPublic, true) // withVersion=true, purge=true

		// Make sure pack is not purgeable
		err := installer.RemovePack(shortenPackPath(packPath, false), true) // withVersion=false, purge=true
		assert.Equal(errs.ErrPackNotPurgeable, err)
	})

	t.Run("test remove all versions at once", func(t *testing.T) {
		localTestingDir := "test-remove-all-versions-at-once"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		// Add a pack, add an updated version of the pack, then remove the first one
		packPath := publicLocalPack123
		updatedPackPath := publicLocalPack124
		config := ConfigType{
			IsPublic: true,
		}
		addPack(t, packPath, config)
		addPack(t, updatedPackPath, config)

		// Remove all packs (withVersion=false), i.e. path will be "TheVendor.PackName"
		removePack(t, packPath, false, IsPublic, true) // withVersion=false, purge=true
	})
}

func TestAddPdsc(t *testing.T) {

	assert := assert.New(t)

	// Sanity tests
	t.Run("test add pdsc with bad name", func(t *testing.T) {
		localTestingDir := "test-add-pdsc-with-bad-name"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		err := installer.AddPdsc(malformedPackName)
		assert.Equal(errs.ErrBadPackNameInvalidExtension, err)
	})

	t.Run("test add pdsc with bad local_repository.pidx", func(t *testing.T) {
		localTestingDir := "test-add-pdsc-with-bad-local-repository"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.LocalPidx = xml.NewPidxXML(badLocalRepositoryPidx)
		defer os.RemoveAll(localTestingDir)

		err := installer.AddPdsc(pdscPack123)
		assert.NotNil(err)
		assert.Equal("XML syntax error on line 3: unexpected EOF", err.Error())
	})

	t.Run("test add a pdsc", func(t *testing.T) {
		localTestingDir := "test-add-a-pdsc"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		err := installer.AddPdsc(pdscPack123)

		// Sanity check
		assert.Nil(err)
	})

	t.Run("test add a pdsc already installed", func(t *testing.T) {
		localTestingDir := "test-add-a-pdsc-already-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		err = installer.AddPdsc(pdscPack123)
		assert.Equal(errs.ErrPdscEntryExists, err)
	})

	t.Run("test add new pdsc version", func(t *testing.T) {
		localTestingDir := "test-add-new-pdsc-version"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		err = installer.AddPdsc(pdscPack124)
		assert.Nil(err)
	})
}

func TestRemovePdsc(t *testing.T) {

	assert := assert.New(t)

	t.Run("test remove pdsc with bad name", func(t *testing.T) {
		localTestingDir := "test-remove-pdsc-with-bad-name"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		err := installer.RemovePdsc(malformedPackName)
		assert.NotNil(err)
		assert.Equal(errs.ErrBadPackName, err)
	})

	t.Run("test remove a pdsc", func(t *testing.T) {
		localTestingDir := "test-remove-pdsc"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		// Add it first
		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		// Remove it
		err = installer.RemovePdsc(shortenPackPath(pdscPack123, true))
		assert.Nil(err)
	})

	t.Run("test remove a pdsc that does not exist", func(t *testing.T) {
		localTestingDir := "test-remove-pdsc-that-does-not-exist"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		err := installer.RemovePdsc(shortenPackPath(pdscPack123, true))
		assert.Equal(errs.ErrPdscEntryNotFound, err)
	})
}

func TestUpdatePublicIndex(t *testing.T) {

	assert := assert.New(t)

	var Overwrite = true

	var NewIndexServer = func(contentBytes []byte) *httptest.Server {
		return httptest.NewTLSServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					reader := bytes.NewReader(contentBytes)
					_, err := io.Copy(w, reader)
					assert.Nil(err)
				},
			),
		)
	}

	t.Run("test add http server index.pidx", func(t *testing.T) {
		localTestingDir := "test-add-http-server-index"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		httpServer := httptest.NewServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				},
			),
		)

		indexPath := httpServer.URL + "/index.pidx"

		err := installer.UpdatePublicIndex(indexPath, !Overwrite)
		assert.NotNil(err)
		assert.Equal(errs.ErrIndexPathNotSafe, err)
	})

	t.Run("test add not found remote index.pidx", func(t *testing.T) {
		localTestingDir := "test-add-not-found-remote-index"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		notFoundServer := httptest.NewTLSServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				},
			),
		)

		indexPath := notFoundServer.URL + "/this-file-does-not-exist"

		currClient := utils.HTTPClient
		utils.HTTPClient = notFoundServer.Client()

		err := installer.UpdatePublicIndex(indexPath, !Overwrite)

		utils.HTTPClient = currClient

		assert.NotNil(err)
		assert.Equal(errs.ErrBadRequest, err)
	})

	t.Run("test add malformed index.pidx", func(t *testing.T) {
		localTestingDir := "test-add-malformed-index"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		indexContent, err := ioutil.ReadFile(malformedPublicIndex)
		assert.Nil(err)

		indexServer := NewIndexServer(indexContent)
		indexPath := indexServer.URL + "/index.pidx"

		currClient := utils.HTTPClient
		utils.HTTPClient = indexServer.Client()

		err = installer.UpdatePublicIndex(indexPath, !Overwrite)

		utils.HTTPClient = currClient

		assert.NotNil(err)
		assert.Equal(err.Error(), "XML syntax error on line 3: unexpected EOF")
	})

	t.Run("test add remote index.pidx", func(t *testing.T) {
		localTestingDir := "test-add-remote-index"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		indexContent, err := ioutil.ReadFile(samplePublicIndex)
		assert.Nil(err)
		indexServer := NewIndexServer(indexContent)

		indexPath := indexServer.URL + "/index.pidx"

		currClient := utils.HTTPClient
		utils.HTTPClient = indexServer.Client()

		err = installer.UpdatePublicIndex(indexPath, !Overwrite)

		utils.HTTPClient = currClient

		assert.Nil(err)

		assert.True(utils.FileExists(installer.Installation.PublicIndex))

		copied, err2 := ioutil.ReadFile(installer.Installation.PublicIndex)
		assert.Nil(err2)

		assert.Equal(copied, indexContent)
	})

	t.Run("test do not overwrite index.pidx", func(t *testing.T) {
		localTestingDir := "test-do-not-overwrite-index"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		_ = utils.TouchFile(installer.Installation.PublicIndex)

		indexContent, err := ioutil.ReadFile(samplePublicIndex)
		assert.Nil(err)
		indexServer := NewIndexServer(indexContent)

		indexPath := indexServer.URL + "/index.pidx"

		currClient := utils.HTTPClient
		utils.HTTPClient = indexServer.Client()

		err = installer.UpdatePublicIndex(indexPath, !Overwrite)

		utils.HTTPClient = currClient

		assert.NotNil(err)
		assert.Equal(errs.ErrCannotOverwritePublicIndex, err)
	})

	t.Run("test overwrite index.pidx", func(t *testing.T) {
		localTestingDir := "test-overwrite-index"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		_ = utils.TouchFile(installer.Installation.PublicIndex)

		indexContent, err := ioutil.ReadFile(samplePublicIndex)
		assert.Nil(err)
		indexServer := NewIndexServer(indexContent)

		indexPath := indexServer.URL + "/index.pidx"

		currClient := utils.HTTPClient
		utils.HTTPClient = indexServer.Client()

		err = installer.UpdatePublicIndex(indexPath, Overwrite)

		utils.HTTPClient = currClient

		assert.Nil(err)
		assert.True(utils.FileExists(installer.Installation.PublicIndex))

		copied, err2 := ioutil.ReadFile(installer.Installation.PublicIndex)
		assert.Nil(err2)

		assert.Equal(copied, indexContent)
	})
}
