/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
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
		defer os.RemoveAll(localTestingDir)

		packPath := malformedPackName

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrBadPackName)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack previously installed", func(t *testing.T) {
		localTestingDir := "test-add-pack-previously-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		notFoundServer := NewServer()

		packPath := notFoundServer.URL() + packThatDoesNotExist

		err := installer.AddPack(packPath, !CheckEula, !ExtractEula)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrBadRequest)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test installing a pack with corrupt zip file", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-corrupt-zip"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := publicLocalPack123
		addPack(t, packPath, ConfigType{
			IsPublic: true,
		})
	})

	t.Run("test installing public pack via remote file", func(t *testing.T) {
		localTestingDir := "test-add-public-remote-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

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
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := nonPublicLocalPack123
		addPack(t, packPath, ConfigType{
			IsPublic: false,
		})
	})

	t.Run("test installing non-public pack via remote file", func(t *testing.T) {
		localTestingDir := "test-add-non-public-remote-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := nonPublicRemotePack123
		addPack(t, packPath, ConfigType{
			IsPublic: false,
		})
	})

	// Test that cpackget will attempt to retrieve the PDSC file of public packs and place it under .Web/
	t.Run("test installing public pack retrieving pdsc file", func(t *testing.T) {
		localTestingDir := "test-add-public-pack-retrieving-pdsc-file"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		packPath := publicLocalPack123
		packInfo, err := utils.ExtractPackInfo(packPath, false)
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
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := nonPublicLocalPack123
		addPack(t, packPath, ConfigType{
			CheckEula: true,
		})
	})

	t.Run("test installing pack with license disagreed", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-license-disagreed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithLicense
		ui.LicenseAgreed = &ui.Agreed
		addPack(t, packPath, ConfigType{
			CheckEula: true,
		})
	})

	t.Run("test installing pack with rtf license agreed", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-rtf-license-agreed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithRTFLicense
		ui.LicenseAgreed = &ui.Agreed
		addPack(t, packPath, ConfigType{
			CheckEula: true,
		})
	})

	t.Run("test installing pack with license agreement skipped", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-license-skipped"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithLicense
		addPack(t, packPath, ConfigType{
			CheckEula: false,
		})
	})

	t.Run("test installing pack with license extracted", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-license-extracted"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
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

	// Pack with the entire pack structure within another folder
	t.Run("test installing pack within subfolder", func(t *testing.T) {
		localTestingDir := "test-add-pack-within-subfolder"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithSubFolder
		addPack(t, packPath, ConfigType{})
	})

	t.Run("test installing pack within too many subfolders", func(t *testing.T) {
		localTestingDir := "test-add-pack-within-too-many-subfolder"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithSubSubFolder
		err := installer.AddPack(packPath, !CheckEula, !ExtractEula)
		assert.NotNil(err)
		assert.Equal(err, errs.ErrPdscFileTooDeepInPack)
	})

	// Install packs with pack id: Vendor.PackName[.x.y.z]
	for _, packPath := range []string{publicRemotePack123PackID, publicRemotePackPackID} {
		packBasePath := filepath.Base(packPath)

		t.Run("test installing pack with pack id pdsc file not found "+packBasePath, func(t *testing.T) {
			localTestingDir := "test-add-pack-with-pack-id-pdsc-file-not-found-" + packBasePath
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			defer os.RemoveAll(localTestingDir)

			// Fake public index server
			publicIndexServer := NewServer()

			// Tweak the URL for the pack's pdsc
			packInfo, err := utils.ExtractPackInfo(packPath, true)
			assert.Nil(err)
			packPdscTag := xml.PdscTag{Vendor: packInfo.Vendor, Name: packInfo.Pack, Version: packInfo.Version}
			packPdscTag.URL = publicIndexServer.URL()
			err = installer.Installation.PublicIndexXML.AddPdsc(packPdscTag)
			assert.Nil(err)

			err = installer.AddPack(packPath, !CheckEula, !ExtractEula)

			assert.NotNil(err)
			assert.Equal(err, errs.ErrPackPdscCannotBeFound)

			// Make sure pack.idx never got touched
			assert.False(utils.FileExists(installer.Installation.PackIdx))
		})

		// This also tests the case where the URL in the pdsc tag serves the correct
		// pdsc file, but DOES NOT serve a pack file
		t.Run("test installing pack with pack id version not found "+packBasePath, func(t *testing.T) {
			localTestingDir := "test-add-pack-with-pack-id-version-not-found-" + packBasePath
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			defer os.RemoveAll(localTestingDir)

			packInfo, err := utils.ExtractPackInfo(packPath, true)
			assert.Nil(err)
			pack := packInfoToType(packInfo)

			// Place the bogus pdsc file in .Web/
			assert.Nil(utils.CopyFile(pdscPack123MissingVersion, filepath.Join(installer.Installation.WebDir, pack.PdscFileName())))

			err = installer.AddPack(packPath, !CheckEula, !ExtractEula)

			assert.NotNil(err)
			assert.Equal(err, errs.ErrPackVersionNotFoundInPdsc)

			// Make sure pack.idx never got touched
			assert.False(utils.FileExists(installer.Installation.PackIdx))
		})

		t.Run("test installing pack with pack id using release url"+packBasePath, func(t *testing.T) {
			localTestingDir := "test-add-pack-with-pack-id-using-release-url" + packBasePath
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			defer os.RemoveAll(localTestingDir)

			// Prep pack info
			packInfo, err := utils.ExtractPackInfo(packPath, true)
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

			err = installer.AddPack(packPath, !CheckEula, !ExtractEula)
			assert.Nil(err)

			pack.Version = "1.2.3"
			pack.IsPublic = true
			checkPackIsInstalled(t, pack)
		})

		t.Run("test installing pack with pack id using pdsc url "+packBasePath, func(t *testing.T) {
			localTestingDir := "test-add-pack-with-pack-id-using-pdsc-url-" + packBasePath
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			defer os.RemoveAll(localTestingDir)

			// Prep pack info
			packInfo, err := utils.ExtractPackInfo(packPath, true)
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

			err = installer.AddPack(packPath, !CheckEula, !ExtractEula)
			assert.Nil(err)

			pack.IsPublic = true
			checkPackIsInstalled(t, pack)
		})

	}

	t.Run("test installing non-public pack via packID", func(t *testing.T) {
		localTestingDir := "test-add-non-public-local-packid"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		pack123Path := nonPublicLocalPack123
		pack124Path := nonPublicLocalPack124
		pack123ID := nonPublicLocalPack123PackID
		//pack124ID := nonPublicLocalPack124PackID

		pack124Info, err := utils.ExtractPackInfo(pack124Path, false)
		assert.Nil(err)
		pack124 := packInfoToType(pack124Info)

		pack123Info, err := utils.ExtractPackInfo(pack123Path, false)
		assert.Nil(err)
		pack123 := packInfoToType(pack123Info)

		pack123Content, err := ioutil.ReadFile(pack123Path)
		assert.Nil(err)

		server := NewServer()
		server.AddRoute(pack123.PackFileName(), pack123Content)

		// Attempt to install with PackID only first time, with no success (no pdsc in .Local)
		err = installer.AddPack(pack123ID, !CheckEula, !ExtractEula)
		assert.Equal(err, errs.ErrPackURLCannotBeFound)

		// Add the pack via file, then remove it just to leave the pdsc in .Local
		addPack(t, pack124Path, ConfigType{
			IsPublic: false,
		})

		// The 1.2.4 pack's PDSC does NOT contain the 1.2.3 release tag on purpose
		// so an attemp to install it should raise an error
		err = installer.AddPack(pack123ID, !CheckEula, !ExtractEula)
		assert.Equal(err, errs.ErrPackVersionNotFoundInPdsc)

		// Tweak the URL to retrieve version 1.2.3 and inject the 1.2.3 tag
		pdscXML := xml.NewPdscXML(filepath.Join(installer.Installation.LocalDir, pack124.PdscFileName()))
		assert.Nil(pdscXML.Read())
		pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, xml.ReleaseTag{Version: "1.2.3"})
		pdscXML.URL = server.URL()
		assert.Nil(utils.WriteXML(pdscXML.FileName, pdscXML))

		err = installer.AddPack(pack123ID, !CheckEula, !ExtractEula)
		assert.Nil(err)
		checkPackIsInstalled(t, pack123)
	})
}
