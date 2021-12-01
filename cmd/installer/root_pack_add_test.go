/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/ui"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	"github.com/stretchr/testify/assert"
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

	var newServer = func(contentBytes []byte) *httptest.Server {
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

	var new404Server = func() *httptest.Server {
		return httptest.NewServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				},
			),
		)
	}

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

	// Install packs with pack id: Vendor.PackName[.x.y.z]
	for _, packPath := range []string{publicRemotePack123PackID, publicRemotePackPackID} {
		packBasePath := filepath.Base(packPath)

		t.Run("test installing pack with pack id not found in public index "+packBasePath, func(t *testing.T) {
			localTestingDir := "test-add-pack-with-pack-id-not-found-in-public-index-" + packBasePath
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			defer os.RemoveAll(localTestingDir)

			// Fake public index server
			publicIndexContent, err := ioutil.ReadFile(emptyPublicIndex)
			assert.Nil(err)
			publicIndexServer := newServer(publicIndexContent)

			// Swap public index url and http client
			currentPublicIndexURL := installer.PublicIndexURL
			currentClient := utils.HTTPClient
			utils.HTTPClient = publicIndexServer.Client()
			installer.PublicIndexURL = publicIndexServer.URL + "/index.pidx"
			installer.PublicIndexUpdated = false

			err = installer.AddPack(packPath, !CheckEula, !ExtractEula)

			// Restore public index url and http client
			installer.PublicIndexURL = currentPublicIndexURL
			utils.HTTPClient = currentClient

			assert.NotNil(err)
			assert.Equal(err, errs.ErrPackNotFoundInPublicIndex)

			// Check that the index got updated
			assert.True(installer.PublicIndexUpdated)

			// Make sure pack.idx never got touched
			assert.False(utils.FileExists(installer.Installation.PackIdx))
		})

		t.Run("test installing pack with pack id pdsc file not found "+packBasePath, func(t *testing.T) {
			localTestingDir := "test-add-pack-with-pack-id-pdsc-file-not-found-" + packBasePath
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			defer os.RemoveAll(localTestingDir)

			// Fake public index server
			publicIndexContent, err := ioutil.ReadFile(emptyPublicIndex)
			assert.Nil(err)
			publicIndexServer := newServer(publicIndexContent)

			// Fake pdsc file server
			notFoundPdscServer := new404Server()

			// Swap public index url and http client
			currentPublicIndexURL := installer.PublicIndexURL
			currentClient := utils.HTTPClient
			utils.HTTPClient = publicIndexServer.Client()
			installer.PublicIndexURL = publicIndexServer.URL + "/index.pidx"
			installer.PublicIndexUpdated = false

			// Force updating the public index
			assert.Nil(installer.EnsurePublicIndexIsUpdated(true))

			// Tweak the URL for the pack's pdsc
			packInfo, err := utils.ExtractPackInfo(packPath, true)
			assert.Nil(err)
			packPdscTag := xml.PdscTag{Vendor: packInfo.Vendor, Name: packInfo.Pack, Version: packInfo.Version}
			packPdscTag.URL = notFoundPdscServer.URL + "/"
			err = installer.Installation.PublicIndexXML.AddPdsc(packPdscTag)
			assert.Nil(err)

			err = installer.AddPack(packPath, !CheckEula, !ExtractEula)

			// Restore public index url and http client
			installer.PublicIndexURL = currentPublicIndexURL
			utils.HTTPClient = currentClient

			assert.NotNil(err)
			assert.Equal(err, errs.ErrPackPdscCannotBeFound)

			// Check that the index got updated
			assert.True(installer.PublicIndexUpdated)

			// Make sure pack.idx never got touched
			assert.False(utils.FileExists(installer.Installation.PackIdx))
		})

		// This also tests the case where the URL in the pdsc tag serves the correct
		// pdsc file, but DOES NOT serve a pack file
		t.Run("test installing pack with pack id version not found "+packBasePath, func(t *testing.T) {
			localTestingDir := "test-add-pack-with-pack-id-version-not-found-" + packBasePath
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			defer os.RemoveAll(localTestingDir)

			// Get pack info
			packInfo, err := utils.ExtractPackInfo(packPath, true)
			assert.Nil(err)

			// Fake public index server
			publicIndexContent, err := ioutil.ReadFile(emptyPublicIndex)
			assert.Nil(err)
			publicIndexServer := newServer(publicIndexContent)

			// Fake pdsc file server
			// should serve pdsc file
			// should return 404 on packPath.pack
			pdscContent, err := ioutil.ReadFile(pdscPack123MissingVersion)
			assert.Nil(err)
			pdscServer := httptest.NewTLSServer(
				http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						// Fail if trying to get url directly from pdsc.URL + packID
						if strings.Contains(r.URL.Path, ".pack") {
							w.WriteHeader(http.StatusNotFound)
						} else {
							reader := bytes.NewReader(pdscContent)
							_, err := io.Copy(w, reader)
							assert.Nil(err)
						}
					},
				),
			)

			// Swap public index url and http client
			currentPublicIndexURL := installer.PublicIndexURL
			currentClient := utils.HTTPClient
			utils.HTTPClient = publicIndexServer.Client()
			installer.PublicIndexURL = publicIndexServer.URL + "/index.pidx"
			installer.PublicIndexUpdated = false

			// Force updating the public index
			assert.Nil(installer.EnsurePublicIndexIsUpdated(true))

			// Tweak the URL for the pack's pdsc
			version := packInfo.Version
			if version == "" {
				version = "1.2.3"
			}
			packPdscTag := xml.PdscTag{Vendor: packInfo.Vendor, Name: packInfo.Pack, Version: version}
			packPdscTag.URL = pdscServer.URL + "/"
			err = installer.Installation.PublicIndexXML.AddPdsc(packPdscTag)
			assert.Nil(err)

			err = installer.AddPack(packPath, !CheckEula, !ExtractEula)

			// Restore public index url and http client
			installer.PublicIndexURL = currentPublicIndexURL
			utils.HTTPClient = currentClient

			assert.NotNil(err)
			assert.Equal(err, errs.ErrPackVersionNotFoundInPdsc)

			// Check that the index got updated
			assert.True(installer.PublicIndexUpdated)

			// Make sure pack.idx never got touched
			assert.False(utils.FileExists(installer.Installation.PackIdx))
		})

		t.Run("test installing pack with pack id empty release url "+packBasePath, func(t *testing.T) {
			localTestingDir := "test-add-pack-with-pack-id-empty-release-url-" + packBasePath
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			defer os.RemoveAll(localTestingDir)

			// Fake public index server
			publicIndexContent, err := ioutil.ReadFile(emptyPublicIndex)
			assert.Nil(err)
			publicIndexServer := newServer(publicIndexContent)

			// Fake pdsc file server
			pdscContent, err := ioutil.ReadFile(pdscPack123EmptyURL)
			assert.Nil(err)
			pdscServer := httptest.NewTLSServer(
				http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						// Fail if trying to get url directly from pdsc.URL + packID
						if strings.Contains(r.URL.Path, ".pack") {
							w.WriteHeader(http.StatusNotFound)
						} else {
							reader := bytes.NewReader(pdscContent)
							_, err := io.Copy(w, reader)
							assert.Nil(err)
						}
					},
				),
			)

			// Swap public index url and http client
			currentPublicIndexURL := installer.PublicIndexURL
			currentClient := utils.HTTPClient
			utils.HTTPClient = publicIndexServer.Client()
			installer.PublicIndexURL = publicIndexServer.URL + "/index.pidx"
			installer.PublicIndexUpdated = false

			// Force updating the public index
			assert.Nil(installer.EnsurePublicIndexIsUpdated(true))

			// Tweak the URL for the pack's pdsc
			packInfo, err := utils.ExtractPackInfo(packPath, true)
			assert.Nil(err)
			packPdscTag := xml.PdscTag{Vendor: packInfo.Vendor, Name: packInfo.Pack, Version: packInfo.Version}
			packPdscTag.URL = pdscServer.URL + "/"
			err = installer.Installation.PublicIndexXML.AddPdsc(packPdscTag)
			assert.Nil(err)

			err = installer.AddPack(packPath, !CheckEula, !ExtractEula)

			// Restore public index url and http client
			installer.PublicIndexURL = currentPublicIndexURL
			utils.HTTPClient = currentClient

			assert.NotNil(err)
			assert.Equal(err, errs.ErrPackURLCannotBeFound)

			// Check that the index got updated
			assert.True(installer.PublicIndexUpdated)

			// Make sure pack.idx never got touched
			assert.False(utils.FileExists(installer.Installation.PackIdx))
		})

		t.Run("test installing pack with pack id "+packBasePath, func(t *testing.T) {
			localTestingDir := "test-add-pack-with-pack-id-" + packBasePath
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			defer os.RemoveAll(localTestingDir)

			// Fake public index server
			publicIndexContent, err := ioutil.ReadFile(emptyPublicIndex)
			assert.Nil(err)
			publicIndexServer := newServer(publicIndexContent)

			// Fake pack server, used to define url in release tag
			packContent, err := ioutil.ReadFile(publicRemotePack123)
			assert.Nil(err)
			packServer := newServer(packContent)

			// Prep pack info
			packInfo, err := utils.ExtractPackInfo(packPath, true)
			assert.Nil(err)

			// Prep the release tag
			releaseTag := xml.ReleaseTag{URL: packServer.URL + "/pack.zip", Version: "1.2.3"}
			if packInfo.Version != "" {
				releaseTag.Version = packInfo.Version
			}

			// Prepare the pdsc file
			pdscXML := xml.NewPdscXML(pdscPack123MissingVersion)
			pdscXML.ReleasesTag.Releases = append(pdscXML.ReleasesTag.Releases, releaseTag)
			pdscFilePath := filepath.Join(localTestingDir, filepath.Base(pdscPack123MissingVersion))
			err = utils.WriteXML(pdscFilePath, pdscXML)
			assert.Nil(err)
			pdscContent, err := ioutil.ReadFile(pdscFilePath)
			assert.Nil(err)
			pdscServer := httptest.NewTLSServer(
				http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						// Fail if trying to get url directly from pdsc.URL + packID
						if strings.Contains(r.URL.Path, ".pack") {
							w.WriteHeader(http.StatusNotFound)
						} else {
							reader := bytes.NewReader(pdscContent)
							_, err := io.Copy(w, reader)
							assert.Nil(err)
						}
					},
				),
			)

			// Swap public index url and http client
			currentPublicIndexURL := installer.PublicIndexURL
			currentClient := utils.HTTPClient
			utils.HTTPClient = publicIndexServer.Client()
			installer.PublicIndexURL = publicIndexServer.URL + "/index.pidx"
			installer.PublicIndexUpdated = false

			// Force updating the public index
			assert.Nil(installer.EnsurePublicIndexIsUpdated(true))

			// Tweak the URL for the pack's pdsc
			packPdscTag := xml.PdscTag{Vendor: packInfo.Vendor, Name: packInfo.Pack, Version: packInfo.Version}
			packPdscTag.URL = pdscServer.URL + "/"
			err = installer.Installation.PublicIndexXML.AddPdsc(packPdscTag)
			assert.Nil(err)

			err = installer.AddPack(packPath, !CheckEula, !ExtractEula)
			assert.Nil(err)

			pack := packInfoToType(packInfo)
			assert.True(installer.Installation.PackIsInstalled(pack))

			// Fill in pack.Version
			_, err = installer.FindPackURL(pack)
			assert.Nil(err)

			packID := fmt.Sprintf("%s.%s.%s", pack.Vendor, pack.Name, pack.Version)

			// Make sure there's a copy of the pack file in .Download/
			assert.True(utils.FileExists(filepath.Join(installer.Installation.DownloadDir, packID+".pack")))

			// Make sure there's a versioned copy of the PDSC file in .Download/
			assert.True(utils.FileExists(filepath.Join(installer.Installation.DownloadDir, packID+".pdsc")))

			// Make sure the pack.idx file gets created
			assert.True(utils.FileExists(installer.Installation.PackIdx))

			// Check that the index got updated
			assert.True(installer.PublicIndexUpdated)

			// Restore public index url and http client
			installer.PublicIndexURL = currentPublicIndexURL
			utils.HTTPClient = currentClient
		})

		t.Run("test installing pack with pack id directly from index.pidx "+packBasePath, func(t *testing.T) {
			localTestingDir := "test-add-pack-with-pack-id-directly-from-index.pidx-" + packBasePath
			assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
			defer os.RemoveAll(localTestingDir)

			// Fake public index server
			publicIndexContent, err := ioutil.ReadFile(samplePublicIndex)
			assert.Nil(err)
			publicIndexServer := newServer(publicIndexContent)

			// Fake pack server, used to define url in release tag
			packContent, err := ioutil.ReadFile(publicRemotePack123)
			assert.Nil(err)
			packServer := newServer(packContent)

			// Prep pack info
			packInfo, err := utils.ExtractPackInfo(packPath, true)
			assert.Nil(err)
			if packInfo.Version == "" {
				packInfo.Version = "1.2.3"
			}

			// Swap public index url and http client
			currentPublicIndexURL := installer.PublicIndexURL
			currentClient := utils.HTTPClient
			utils.HTTPClient = publicIndexServer.Client()
			installer.PublicIndexURL = publicIndexServer.URL + "/index.pidx"
			installer.PublicIndexUpdated = false

			// Force updating the public index
			assert.Nil(installer.EnsurePublicIndexIsUpdated(true))

			// Tweak the URL for the pack's pdsc
			packPdscTag := xml.PdscTag{Vendor: packInfo.Vendor, Name: packInfo.Pack, Version: packInfo.Version}
			packPdscTag.URL = packServer.URL + "/"
			err = installer.Installation.PublicIndexXML.AddPdsc(packPdscTag)
			assert.Nil(err)

			err = installer.AddPack(packPath, !CheckEula, !ExtractEula)
			assert.Nil(err)

			pack := packInfoToType(packInfo)
			assert.True(installer.Installation.PackIsInstalled(pack))

			packID := fmt.Sprintf("%s.%s.%s", pack.Vendor, pack.Name, pack.Version)

			// Make sure there's a copy of the pack file in .Download/
			assert.True(utils.FileExists(filepath.Join(installer.Installation.DownloadDir, packID+".pack")))

			// Make sure there's a versioned copy of the PDSC file in .Download/
			assert.True(utils.FileExists(filepath.Join(installer.Installation.DownloadDir, packID+".pdsc")))

			// Make sure the pack pdsc file did NOT get downloaded
			assert.False(utils.FileExists(filepath.Join(installer.Installation.WebDir, pack.Vendor+"."+pack.Name+".pdsc")))

			// Make sure the pack.idx file gets created
			assert.True(utils.FileExists(installer.Installation.PackIdx))

			// Check that the index got updated
			assert.True(installer.PublicIndexUpdated)

			// Restore public index url and http client
			installer.PublicIndexURL = currentPublicIndexURL
			utils.HTTPClient = currentClient
		})

	}
}
