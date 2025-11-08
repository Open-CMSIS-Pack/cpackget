/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// Copy of cmd/log.go
type LogFormatter struct{}

func (s *LogFormatter) Format(entry *log.Entry) ([]byte, error) {
	level := strings.ToUpper(entry.Level.String())
	msg := fmt.Sprintf("%s: %s\n", level[0:1], entry.Message)
	return []byte(msg), nil
}

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

func getPackIdxModTime(t *testing.T, pushBack bool) time.Time {
	packIdx := installer.Installation.PackIdx
	if !utils.FileExists(packIdx) {
		assert.Nil(t, utils.TouchFile(packIdx))
	}

	// This function helps retrieving mod time of pack.idx file.
	// It is invoked before adding/removing packs to detect if the file really did get touched
	// BUT:
	// Apparently Windows systems update of file modified times
	// happens 64 times per second, and in some cases that is not
	// enough for the time delta  below to show a difference
	// Ref: https://www.lochan.org/2005/keith-cl/useful/win32time.html#timingwin
	// This caused intermittent test failures only on Windows environment.
	// We tried sleeping for 1,2, and 3 seconds before checking for
	// mod time of pack.idx but it still failed unexpectedly.
	// So instead of sleeping only on Windows, we decided now to
	// bring back the original check of pack.idx mod time in 1 day so
	// next time it gets touched, the time delta will be great enough (we hope)
	if pushBack {
		//                              years, months, days
		yesterday := time.Now().AddDate(0, 0, -1)
		err := os.Chtimes(packIdx, yesterday, yesterday)
		assert.Nil(t, err)
	}

	stat, err := os.Stat(packIdx)
	assert.Nil(t, err)
	return stat.ModTime()
}

func checkPackIsInstalled(t *testing.T, pack *installer.PackType) {
	assert := assert.New(t)

	assert.True(installer.Installation.PackIsInstalled(pack, false))

	// Make sure there's a copy of the pack file in .Download/
	assert.True(utils.FileExists(filepath.Join(installer.Installation.DownloadDir, pack.PackFileName())))

	// Make sure there's a versioned copy of the PDSC file in .Download/
	assert.True(utils.FileExists(filepath.Join(installer.Installation.DownloadDir, pack.PdscFileNameWithVersion())))

	if pack.IsPublic {
		// Make sure no PDSC file got copied to .Local/
		assert.False(utils.FileExists(filepath.Join(installer.Installation.LocalDir, pack.PdscFileName())))

		if pack.IsLocallySourced {
			assert.True(utils.FileExists(filepath.Join(installer.Installation.WebDir, pack.PdscFileName())))
		}
	} else {
		// Make sure there's an unversioned copy of the PDSC file in .Local/, in case pack is not public
		assert.True(utils.FileExists(filepath.Join(installer.Installation.LocalDir, pack.PdscFileName())))

		// Make sure no PDSC file got copied to .Web/
		assert.False(utils.FileExists(filepath.Join(installer.Installation.WebDir, pack.PdscFileName())))
	}

	// Make sure the pack.idx file gets created
	assert.True(utils.FileExists(installer.Installation.PackIdx))
}

type ConfigType struct {
	CheckEula      bool
	ExtractEula    bool
	ForceReinstall bool
	IsPublic       bool
	NoRequirements bool
}

func addPack(t *testing.T, packPath string, config ConfigType) {
	assert := assert.New(t)

	// Get pack.idx before removing pack
	packIdxModTime := getPackIdxModTime(t, Start)

	err := installer.AddPack(packPath, config.CheckEula, config.ExtractEula, config.ForceReinstall, config.NoRequirements, true, Timeout)
	assert.Nil(err)

	if config.ExtractEula {
		return
	}

	info, err := utils.ExtractPackInfo(packPath)
	assert.Nil(err)

	// Check in installer internals
	pack := packInfoToType(info)
	pack.IsPublic = config.IsPublic

	checkPackIsInstalled(t, pack)

	// Make sure the pack.idx file gets trouched
	assert.True(packIdxModTime.Before(getPackIdxModTime(t, End)))
}

func removePack(t *testing.T, packPath string, withVersion, isPublic, purge bool) {
	// TODO:Add option to remove ALL

	assert := assert.New(t)

	// Get pack.idx before removing pack
	packIdxModTime := getPackIdxModTime(t, Start)

	// [http://vendor.com|path/to]/TheVendor.PackName.x.y.z -> TheVendor.PackName[.x.y.z]
	shortPackPath := shortenPackPath(packPath, withVersion)

	info, err := utils.ExtractPackInfo(shortPackPath)
	assert.Nil(err)

	// Check in installer internals
	pack := packInfoToType(info)
	isInstalled, _ := installer.Installation.PackIsInstalled(pack, false)

	purgeOnly := !isInstalled && purge

	_, err = installer.RemovePack(shortPackPath, purge, true)
	assert.Nil(err)

	removeAll := false

	if removeAll {
		assert.False(installer.Installation.PackIsInstalled(pack, false))

		if !withVersion {
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
	}

	if withVersion {
		// Make sure files are there (purge=false) or if they no longer exist (purge=true) in .Download/
		assert.Equal(!purge, utils.FileExists(filepath.Join(installer.Installation.DownloadDir, shortPackPath+".pack")))
		assert.Equal(!purge, utils.FileExists(filepath.Join(installer.Installation.DownloadDir, shortPackPath+".pdsc")))
	}

	// No touch on purging only
	if !purgeOnly {
		// Make sure the pack.idx file gets trouched
		assert.True(packIdxModTime.Before(getPackIdxModTime(t, End)))
	}
}

var (
	// Constants to help getting pack.idx mod time
	Start = true
	End   = false

	// Constant telling pack privacy
	CheckEula      = true
	ExtractEula    = true
	ForceReinstall = true
	IsPublic       = true
	NotPublic      = false
	NoRequirements = true
	SubCall        = false
	Timeout        = 0

	CreatePackRoot = true

	// Available testing packs
	testDir = filepath.Join("..", "..", "testdata", "integration")

	malformedPackNames             = []string{"pAck-WiTH-HiFenS", "[$pecialC#aracter£]", "Spaced Pack Name", " "}
	packThatDoesNotExist           = "ThisPack.DoesNotExist.0.0.1.pack"
	packToReinstallFileName        = "TheVendor.PackToReinstall.1.2.3.pack"
	packToReinstall                = filepath.Join(testDir, packToReinstallFileName)
	packWithCorruptZip             = filepath.Join(testDir, "FakeZip.PackName.1.2.3.pack")
	packWithMalformedURL           = "http://:malformed-url*/TheVendor.PackName.1.2.3.pack"
	packWithoutPdscFileInside      = filepath.Join(testDir, "PackWithout.PdscFileInside.1.2.3.pack")
	packWithTaintedCompressedFiles = filepath.Join(testDir, "PackWith.TaintedFiles.1.2.3.pack")
	packWithParentDirectoryFiles   = filepath.Join(testDir, "PackWith.ParentDirectoryFiles.1.2.3.pack")
	packToUpdateFileName           = "TheVendor.PackToUpdate.1.2.3.pack"
	packToUpdate                   = filepath.Join(testDir, packToUpdateFileName)

	// Packs with packid names only
	publicRemotePackPackID         = "TheVendor.PublicRemotePack"
	publicRemotePack123PackID      = publicRemotePackPackID + ".1.2.3"
	publicRemotePack123PackIDAlpha = publicRemotePackPackID + ".1.2.3-alpha.1.0"
	publicRemotePack123PackIDMeta  = publicRemotePackPackID + ".1.2.3+meta"
	nonPublicLocalPackPackID       = "TheVendor.NonPublicLocalPack"
	nonPublicLocalPack123PackID    = nonPublicLocalPackPackID + ".1.2.3"
	nonPublicLocalPack124PackID    = nonPublicLocalPackPackID + ".1.2.4"

	// Packs with legacy packid names
	publicRemotePackLegacyPackID                               = "TheVendor::PublicRemotePack"
	publicLocalPackLegacyPackID                                = "TheVendor::PublicLocalPack"
	publicRemotePack123LegacyPackID                            = publicRemotePackLegacyPackID + "@1.2.3"
	publicLocalPack123WithMinimumVersionLegacyPackID           = publicLocalPackLegacyPackID + ">=1.2.3"
	publicLocalPack125WithMinimumVersionLegacyPackID           = publicLocalPackLegacyPackID + ">=1.2.5"
	publicLocalPack010WithMinimumCompatibleVersionLegacyPackID = publicLocalPackLegacyPackID + "@^0.1.0"
	publicLocalPack011WithMinimumCompatibleVersionLegacyPackID = publicLocalPackLegacyPackID + "@^0.1.1"
	publicLocalPack211WithMinimumCompatibleVersionLegacyPackID = publicLocalPackLegacyPackID + "@^2.1.1"
	publicLocalPack010WithPatchVersionLegacyPackID             = publicLocalPackLegacyPackID + "@~0.1.0"
	publicLocalPack011WithPatchVersionLegacyPackID             = publicLocalPackLegacyPackID + "@~0.1.1"
	publicLocalPack211WithPatchVersionLegacyPackID             = publicLocalPackLegacyPackID + "@~2.1.1"
	publicLocalPackLatestVersionLegacyPackID                   = publicLocalPackLegacyPackID + "@latest"

	// Pdsc files to test out installing packs with pack id only
	pdscPack123MissingVersion = filepath.Join(testDir, "TheVendor.PublicRemotePack_VersionNotAvailable.pdsc")
	pack123MissingVersion     = filepath.Join(testDir, "TheVendor.LocalPackWithMissingVersion.1.2.3.pack")
	pack123VersionNotLatest   = filepath.Join(testDir, "TheVendor.LocalPackWithVersionNotLatest.1.2.3.pack")

	// Public packs
	publicLocalPack010       = filepath.Join(testDir, "0.1.0", "TheVendor.PublicLocalPack.0.1.0.pack")
	publicLocalPack011       = filepath.Join(testDir, "0.1.1", "TheVendor.PublicLocalPack.0.1.1.pack")
	publicLocalPack122       = filepath.Join(testDir, "1.2.2", "TheVendor.PublicLocalPack.1.2.2.pack")
	publicLocalPack123       = filepath.Join(testDir, "1.2.3", "TheVendor.PublicLocalPack.1.2.3.pack")
	publicLocalPack124       = filepath.Join(testDir, "1.2.4", "TheVendor.PublicLocalPack.1.2.4.pack")
	publicLocalPack123Pdsc   = filepath.Join(testDir, "1.2.3", "TheVendor.PublicLocalPack.pdsc")
	publicLocalPack124Pdsc   = filepath.Join(testDir, "1.2.4", "TheVendor.PublicLocalPack.pdsc")
	publicLocalPack123meta   = filepath.Join(testDir, "1.2.3+meta", "TheVendor.PublicLocalPack.1.2.3+meta.pack")
	publicRemotePack123      = filepath.Join(testDir, "1.2.3", publicRemotePack123PackID+".pack")
	publicRemotePack123alpha = filepath.Join(testDir, "1.2.3-alpha.1.0", publicRemotePack123PackIDAlpha+".pack")
	publicLocalPackCASE123   = filepath.Join(testDir, "1.2.3", "TheVendor.PublicLocalPackCASE.1.2.3.pack")

	// Private packs
	nonPublicLocalPack123  = filepath.Join(testDir, "1.2.3", nonPublicLocalPack123PackID+".pack")
	nonPublicLocalPack124  = filepath.Join(testDir, "1.2.4", nonPublicLocalPack124PackID+".pack")
	nonPublicRemotePack123 = filepath.Join(testDir, "1.2.3", "TheVendor.NonPublicRemotePack.1.2.3.pack")

	// Packs with license
	packWithLicense        = filepath.Join(testDir, "TheVendor.PackWithLicense.1.2.3.pack")
	packWithRTFLicense     = filepath.Join(testDir, "TheVendor.PackWithRTFLicense.1.2.3.pack")
	packWithMissingLicense = filepath.Join(testDir, "TheVendor.PackWithMissingLicense.1.2.3.pack")

	// Pack with subfolder in it, pdsc not in root folder
	packWithSubFolder    = filepath.Join(testDir, "TheVendor.PackWithSubFolder.1.2.3.pack")
	packWithSubSubFolder = filepath.Join(testDir, "TheVendor.PackWithSubSubFolder.1.2.3.pack")

	// Packs with dependencies
	packWithSingleDependency      = filepath.Join(testDir, "dependencies", "TheVendor.SingleDependency.1.2.3.pack")
	packWithSingleDependencyAlpha = filepath.Join(testDir, "dependencies", "TheVendor.SingleDependency.1.2.3-alpha.1.0.pack")

	// Concurrent download PDSC base name
	publicConcurrentLocalPdscBase = "TheVendor.PublicLocalPack"

	// PDSC packs
	pdscPack123         = filepath.Join(testDir, "1.2.3", "TheVendor.PackName.pdsc")
	pdscPack124         = filepath.Join(testDir, "1.2.4", "TheVendor.PackName.pdsc")
	pdscPublicLocalPack = filepath.Join(testDir, "public_index", "TheVendor.PublicLocalPack.pdsc")
	pdscPackNotInIndex  = filepath.Join(testDir, "TheVendor.PackNotInIndex.pdsc")

	// Bad local_repository.pidx
	badLocalRepositoryPidx = filepath.Join(testDir, "bad_local_repository.pidx")

	// Sample public index.pidx
	samplePublicIndex = filepath.Join(testDir, "SamplePublicIndex.pidx")
	// Sample public index.pdix with a localhost pdsc url
	samplePublicIndexLocalhostPdsc = filepath.Join(testDir, "SamplePublicIndexLocalhostUrl.pidx")
	// Sample public index.pdix with several localhost pdsc url for concurrent download
	samplePublicIndexConcurrentLocalhostPdsc = filepath.Join(testDir, "concurrent", "SamplePublicIndex.pidx")

	// Malformed index.pidx
	malformedPublicIndex = filepath.Join("..", "..", "testdata", "MalformedPack.pidx")
)

type Server struct {
	routes      map[string][]byte
	httpsServer *httptest.Server
}

func (s *Server) URL() string {
	return s.httpsServer.URL + "/"
}

func (s *Server) AddRoute(route string, content []byte) {
	s.routes[route] = content
}

// NewServer is a generic dev server that takes in a routes map and returns 404 if the route[path] is nil
// Ex:
//
//	server := NewHttpsServer(map[string][]byte{
//		"*": []byte("Default content"),
//		"should-return-404": nil,
//	})
//
// Acessing server.URL should return "Default content"
// Acessing server.URL + "/should-return-404" should return HTTP 404
func NewServer() Server {
	server := Server{}
	server.routes = make(map[string][]byte)
	server.httpsServer = httptest.NewTLSServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				path := r.URL.Path
				if len(path) > 1 {
					path = path[1:]
				}
				content, ok := server.routes[path]
				if !ok {
					defaultContent, ok := server.routes["*"]
					if !ok {
						w.WriteHeader(http.StatusNotFound)
						return
					}

					content = defaultContent
				}

				if content == nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}

				reader := bytes.NewReader(content)
				_, _ = io.Copy(w, reader)
			},
		),
	)

	utils.HTTPClient = server.httpsServer.Client()
	return server
}

func TestGetDefaultCmsisPackRoot(t *testing.T) {
	tests := []struct {
		name           string
		goos           string
		envVars        map[string]string
		expectedResult string
	}{
		{
			name: "default mode path set",
			envVars: map[string]string{
				"CPACKGET_DEFAULT_MODE_PATH": "/custom/default/path",
			},
			expectedResult: filepath.Clean("/custom/default/path"),
		},
		{
			name: "windows LOCALAPPDATA set",
			goos: "windows",
			envVars: map[string]string{
				"LOCALAPPDATA": "C:\\Users\\TestUser\\AppData\\Local",
			},
			expectedResult: filepath.Clean("C:\\Users\\TestUser\\AppData\\Local\\Arm\\Packs"),
		},
		{
			name: "windows USERPROFILE set",
			goos: "windows",
			envVars: map[string]string{
				"LOCALAPPDATA": "",
				"USERPROFILE":  "C:\\Users\\TestUser",
			},
			expectedResult: filepath.Clean("C:\\Users\\TestUser\\AppData\\Local\\Arm\\Packs"),
		},
		{
			name: "linux XDG_CACHE_HOME set",
			goos: "linux",
			envVars: map[string]string{
				"XDG_CACHE_HOME": "/home/testuser/.cache",
			},
			expectedResult: filepath.Clean("/home/testuser/.cache/arm/packs"),
		},
		{
			name: "linux HOME set",
			goos: "linux",
			envVars: map[string]string{
				"XDG_CACHE_HOME": "",
				"HOME":           "/home/testuser",
			},
			expectedResult: filepath.Clean("/home/testuser/.cache/arm/packs"),
		},
		{
			name: "no environment variables set",
			envVars: map[string]string{
				"CPACKGET_DEFAULT_MODE_PATH": "",
				"LOCALAPPDATA":               "",
				"USERPROFILE":                "",
				"XDG_CACHE_HOME":             "",
				"HOME":                       "",
			},
			expectedResult: ".",
		},
	}

	for _, test := range tests {
		if test.goos == "" || test.goos == runtime.GOOS {
			t.Run(test.name, func(t *testing.T) {
				// Backup and restore environment variables
				originalEnv := make(map[string]string)
				for key := range test.envVars {
					originalEnv[key] = os.Getenv(key)
					os.Setenv(key, test.envVars[key])
				}
				defer func() {
					for key, value := range originalEnv {
						os.Setenv(key, value)
					}
				}()
				// Call the function and validate the result
				result := installer.GetDefaultCmsisPackRoot()
				assert.Equal(t, test.expectedResult, result)
			})
		}
	}
}

func TestUpdatePublicIndex(t *testing.T) {

	assert := assert.New(t)

	var Sparse = true
	var DownloadPdsc = false
	var DownloadRemainingPdscFiles = true
	var UpdatePrivatePdsc = true
	var ShowInfo = true
	var Concurrency = 0

	// Re-enable this test when a flag --enforce-security is implemented
	// t.Run("test add http server "+installer.PublicIndex, func(t *testing.T) {
	// 	localTestingDir := "test-add-http-server-index"
	// 	assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
	//  installer.UnlockPackRoot()
	//	assert.Nil(installer.ReadIndexFiles())
	// 	defer os.RemoveAll(localTestingDir)

	// 	httpServer := httptest.NewServer(
	// 		http.HandlerFunc(
	// 			func(w http.ResponseWriter, r *http.Request) {
	// 				w.WriteHeader(http.StatusNotFound)
	// 			},
	// 		),
	// 	)

	// 	indexPath := httpServer.URL + "/" + installer.PublicIndexindex.pidx

	// 	err := installer.UpdatePublicIndex(indexPath)
	// 	assert.NotNil(err)
	// 	assert.Equal(errs.ErrIndexPathNotSafe, err)
	// })

	t.Run("test add not found remote "+installer.PublicIndexName, func(t *testing.T) {
		localTestingDir := "test-add-not-found-remote-index"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer os.RemoveAll(localTestingDir)

		server := NewServer()

		indexPath := server.URL() + "this-file-does-not-exist"

		err := installer.UpdatePublicIndex(indexPath, Sparse, DownloadPdsc, !DownloadRemainingPdscFiles, !UpdatePrivatePdsc, ShowInfo, Concurrency, Timeout)

		assert.NotNil(err)
		assert.Equal(errors.Unwrap(err), errs.ErrBadRequest)
	})

	t.Run("test add malformed "+installer.PublicIndexName, func(t *testing.T) {
		localTestingDir := "test-add-malformed-index"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer os.RemoveAll(localTestingDir)

		indexContent, err := os.ReadFile(malformedPublicIndex)
		assert.Nil(err)

		indexServer := NewServer()
		indexServer.AddRoute(installer.PublicIndexName, indexContent)
		indexPath := indexServer.URL() + installer.PublicIndexName

		err = installer.UpdatePublicIndex(indexPath, Sparse, DownloadPdsc, !DownloadRemainingPdscFiles, !UpdatePrivatePdsc, ShowInfo, Concurrency, Timeout)

		assert.NotNil(err)
		assert.Equal(err.Error(), "XML syntax error on line 3: unexpected EOF")
	})

	t.Run("test add remote "+installer.PublicIndexName, func(t *testing.T) {
		localTestingDir := "test-add-remote-index"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer os.RemoveAll(localTestingDir)

		indexContent, err := os.ReadFile(samplePublicIndex)
		assert.Nil(err)
		indexServer := NewServer()
		indexServer.AddRoute(installer.PublicIndexName, indexContent)
		indexPath := indexServer.URL() + installer.PublicIndexName

		err = installer.UpdatePublicIndex(indexPath, Sparse, DownloadPdsc, !DownloadRemainingPdscFiles, !UpdatePrivatePdsc, ShowInfo, Concurrency, Timeout)

		assert.Nil(err)

		assert.True(utils.FileExists(installer.Installation.PublicIndex))

		copied, err2 := os.ReadFile(installer.Installation.PublicIndex)
		assert.Nil(err2)

		assert.Equal(copied, indexContent)
	})

	t.Run("test add remaining PDSC from "+installer.PublicIndexName, func(t *testing.T) {
		localTestingDir := "test-add-remaining-PDSC-from-index"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer os.RemoveAll(localTestingDir)

		indexContent, err := os.ReadFile(samplePublicIndexLocalhostPdsc)
		assert.Nil(err)
		indexServer := NewServer()
		// The psd URL needs to be updated as it's not known beforehand
		updatedIndex := []byte(strings.ReplaceAll(string(indexContent), "https://127.0.0.1", indexServer.URL()))
		indexServer.AddRoute(installer.PublicIndexName, updatedIndex)
		indexPath := indexServer.URL() + installer.PublicIndexName

		pdscContent, err := os.ReadFile(publicLocalPack123Pdsc)
		assert.Nil(err)
		indexServer.AddRoute("TheVendor.PublicLocalPack.pdsc", pdscContent)

		err = installer.UpdatePublicIndex(indexPath, Sparse, DownloadPdsc, DownloadRemainingPdscFiles, !UpdatePrivatePdsc, ShowInfo, Concurrency, Timeout)
		assert.Nil(err)

		assert.True(utils.FileExists(path.Join(localTestingDir, ".Web", "TheVendor.PublicLocalPack.pdsc")))
	})

	// TODO: this test currently fails because the pdsc file is not found in the public index
	t.Run("test update-index delete pdsc when not in "+installer.PublicIndexName, func(t *testing.T) {
		localTestingDir := "test-update-index-delete-pdsc-when-not-in-index"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer os.RemoveAll(localTestingDir)

		indexContent, err := os.ReadFile(samplePublicIndex)
		assert.Nil(err)
		indexServer := NewServer()
		indexServer.AddRoute(installer.PublicIndexName, indexContent)
		indexPath := indexServer.URL() + installer.PublicIndexName

		err = installer.UpdatePublicIndex(indexPath, Sparse, DownloadPdsc, !DownloadRemainingPdscFiles, !UpdatePrivatePdsc, ShowInfo, Concurrency, Timeout)
		assert.Nil(err)

		publicIndex := installer.Installation.PublicIndex
		assert.True(utils.FileExists(publicIndex))

		//copy pdscPack123
		err = utils.CopyFile(pdscPack123, filepath.Join(localTestingDir, ".Web", "TheVendor.PackName.pdsc"))
		assert.Nil(err)

		//copy pdscPackNotInIndex
		err = utils.CopyFile(pdscPackNotInIndex, filepath.Join(localTestingDir, ".Web", "TheVendor.PackNotInIndex.pdsc"))
		assert.Nil(err)

		err = installer.UpdatePublicIndex(indexPath, false, DownloadPdsc, !DownloadRemainingPdscFiles, !UpdatePrivatePdsc, ShowInfo, Concurrency, Timeout)
		assert.Nil(err)

		// assert.False(utils.FileExists(filepath.Join(localTestingDir, ".Web", "TheVendor.PackNotInIndex.pdsc")))
	})

	t.Run("test add local file "+installer.PublicIndexName, func(t *testing.T) {
		localTestingDir := "test-add-local-file-index"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer os.RemoveAll(localTestingDir)

		indexContent, err := os.ReadFile(samplePublicIndex)
		assert.Nil(err)

		assert.Nil(installer.UpdatePublicIndex(samplePublicIndex, Sparse, DownloadPdsc, !DownloadRemainingPdscFiles, !UpdatePrivatePdsc, ShowInfo, Concurrency, Timeout))

		assert.True(utils.FileExists(installer.Installation.PublicIndex))

		copied, err2 := os.ReadFile(installer.Installation.PublicIndex)
		assert.Nil(err2)

		assert.Equal(copied, indexContent)
	})

	t.Run("test check concurrency function call", func(t *testing.T) {
		assert.Equal(0, installer.CheckConcurrency(0))
		assert.Equal(2, installer.CheckConcurrency(2))
		assert.NotEqual(999999, installer.CheckConcurrency(999999))
	})

	t.Run("test add remote "+installer.PublicIndexName+" and dowload pdsc files", func(t *testing.T) {
		localTestingDir := "test-add-remote-index-download-pdsc"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer os.RemoveAll(localTestingDir)

		indexContent, err := os.ReadFile(samplePublicIndexLocalhostPdsc)
		assert.Nil(err)
		indexServer := NewServer()
		// The psd URL needs to be updated as it's not known beforehand
		updatedIndex := []byte(strings.ReplaceAll(string(indexContent), "https://127.0.0.1", indexServer.URL()))
		indexServer.AddRoute(installer.PublicIndexName, updatedIndex)
		indexPath := indexServer.URL() + installer.PublicIndexName

		pdscContent, err := os.ReadFile(publicLocalPack123Pdsc)
		assert.Nil(err)
		indexServer.AddRoute("TheVendor.PublicLocalPack.pdsc", pdscContent)

		err = installer.UpdatePublicIndex(indexPath, Sparse, !DownloadPdsc, !DownloadRemainingPdscFiles, !UpdatePrivatePdsc, ShowInfo, Concurrency, Timeout)

		assert.Nil(err)
		assert.True(utils.FileExists(installer.Installation.PublicIndex))

		copied, err := os.ReadFile(installer.Installation.PublicIndex)
		assert.Nil(err)
		assert.Equal(copied, updatedIndex)
		// Check if referenced pdsc was downloaded
		assert.True(utils.FileExists(installer.Installation.WebDir + string(filepath.Separator) + "TheVendor.PublicLocalPack.pdsc"))
	})

	t.Run("test add remote "+installer.PublicIndexName+" and concurrent dowload pdsc files", func(t *testing.T) {
		localTestingDir := "test-add-remote-index-concurrent-download-pdsc"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer os.RemoveAll(localTestingDir)

		indexContent, err := os.ReadFile(samplePublicIndexConcurrentLocalhostPdsc)
		assert.Nil(err)
		indexServer := NewServer()
		// The psd URL needs to be updated as it's not known beforehand
		updatedIndex := []byte(strings.ReplaceAll(string(indexContent), "https://127.0.0.1", indexServer.URL()))
		indexServer.AddRoute(installer.PublicIndexName, updatedIndex)
		indexPath := indexServer.URL() + installer.PublicIndexName

		for i := 1; i < 11; i++ {
			pdsc := publicConcurrentLocalPdscBase + fmt.Sprint(i) + ".pdsc"
			pdscContent, err := os.ReadFile(filepath.Join(testDir, "concurrent", "1.2.3", pdsc))
			assert.Nil(err)
			indexServer.AddRoute(publicConcurrentLocalPdscBase+fmt.Sprint(i)+".pdsc", pdscContent)
		}

		err = installer.UpdatePublicIndex(indexPath, Sparse, !DownloadPdsc, !DownloadRemainingPdscFiles, !UpdatePrivatePdsc, ShowInfo, 5, Timeout)

		assert.Nil(err)
		assert.True(utils.FileExists(installer.Installation.PublicIndex))

		copied, err := os.ReadFile(installer.Installation.PublicIndex)
		assert.Nil(err)
		assert.Equal(copied, updatedIndex)

		// Needed as the routines might not finish before the assert
		time.Sleep(400 * time.Millisecond)
		for i := 1; i < 11; i++ {
			pdsc := publicConcurrentLocalPdscBase + fmt.Sprint(i) + ".pdsc"
			assert.True(utils.FileExists(installer.Installation.WebDir + string(filepath.Separator) + pdsc))
		}
	})

	t.Run("test full update when sparse is false", func(t *testing.T) {
		localTestingDir := "test-sparse-update"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer os.RemoveAll(localTestingDir)

		// A non-sparse/full update should detect all
		// pdsc files under .Web/ that need update

		indexServer := NewServer()

		// Inject 1.2.3 in index.pidx
		pack123Info, err := utils.ExtractPackInfo(publicLocalPack123)
		assert.Nil(err)
		assert.Nil(installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
			Vendor:  pack123Info.Vendor,
			Name:    pack123Info.Pack,
			Version: pack123Info.Version,
		}))
		// Write the index.pidx
		assert.Nil(installer.Installation.PublicIndexXML.Write())

		// Inject the testing server URL
		installer.Installation.PublicIndexXML.URL = indexServer.URL()

		// Place 1.2.3 pdsc in .Web/
		pdscFile := filepath.Join(installer.Installation.WebDir, filepath.Base(publicLocalPack123Pdsc))
		assert.Nil(utils.CopyFile(publicLocalPack123Pdsc, pdscFile))
		assert.Nil(installer.InitializeCache())

		// Now get a new index.pidx and the 1.2.4 pdsc into the server and attempt updating with sparse=false
		pack124Info, err := utils.ExtractPackInfo(publicLocalPack124)
		assert.Nil(err)

		// Create a temp index file to serve it for update
		tempIndexFile := filepath.Join(localTestingDir, installer.PublicIndexName)
		assert.Nil(utils.CopyFile(samplePublicIndex, tempIndexFile))
		indexXML := xml.NewPidxXML(tempIndexFile)
		assert.Nil(indexXML.Read())
		assert.Nil(indexXML.AddPdsc(xml.PdscTag{
			URL:     indexServer.URL(),
			Vendor:  pack124Info.Vendor,
			Name:    pack124Info.Pack,
			Version: pack124Info.Version,
		}))

		assert.Nil(indexXML.Write())
		indexContent, err := os.ReadFile(tempIndexFile)
		assert.Nil(err)
		indexServer.AddRoute(installer.PublicIndexName, indexContent)

		// Add the path to the pack's pdsc
		pdscContent, err := os.ReadFile(publicLocalPack124Pdsc)
		assert.Nil(err)

		indexServer.AddRoute(filepath.Base(publicLocalPack124Pdsc), pdscContent)

		assert.Nil(installer.UpdatePublicIndex("", !Sparse, DownloadPdsc, !DownloadRemainingPdscFiles, !UpdatePrivatePdsc, ShowInfo, Concurrency, Timeout))

		// Make sure index.pidx exists and it is updated
		assert.FileExists(installer.Installation.PublicIndex)
		copied, err2 := os.ReadFile(installer.Installation.PublicIndex)
		assert.Nil(err2)
		assert.Equal(copied, indexContent)

		// Make sure the pdsc under .Web/ is updated
		assert.FileExists(pdscFile)
		pdscXML := xml.NewPdscXML(pdscFile)
		assert.Nil(pdscXML.Read())
		assert.Equal(pack124Info.Version, pdscXML.LatestVersion())
	})

	t.Run("test full concurrent update when sparse is false", func(t *testing.T) {
		localTestingDir := "test-concurrent-sparse-update"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer os.RemoveAll(localTestingDir)

		indexServer := NewServer()

		// // Inject all 1.2.3 pdscs in index.pidx
		for i := 1; i < 11; i++ {
			assert.Nil(installer.Installation.PublicIndexXML.AddPdsc(xml.PdscTag{
				Vendor:  "TheVendor",
				Name:    "PublicLocalPack" + fmt.Sprint(i),
				Version: "1.2.3",
			}))
		}
		// Write the index.pidx
		assert.Nil(installer.Installation.PublicIndexXML.Write())

		// Inject the testing server URL
		installer.Installation.PublicIndexXML.URL = indexServer.URL()

		// Place all 1.2.3 pdscs in .Web/
		for i := 1; i < 11; i++ {
			pdsc := publicConcurrentLocalPdscBase + fmt.Sprint(i) + ".pdsc"
			assert.Nil(utils.CopyFile(filepath.Join(testDir, "concurrent", "1.2.3", pdsc), filepath.Join(installer.Installation.WebDir, pdsc)))
		}
		assert.Nil(installer.InitializeCache())

		// Now get a new index.pidx and the 1.2.4 pdscs into the server and attempt updating with sparse=false
		tempIndexFile := filepath.Join(localTestingDir, installer.PublicIndexName)
		assert.Nil(utils.CopyFile(samplePublicIndex, tempIndexFile))
		indexXML := xml.NewPidxXML(tempIndexFile)
		assert.Nil(indexXML.Read())

		for i := 1; i < 11; i++ {
			assert.Nil(indexXML.AddPdsc(xml.PdscTag{
				URL:     indexServer.URL(),
				Vendor:  "TheVendor",
				Name:    "PublicLocalPack" + fmt.Sprint(i),
				Version: "1.2.4",
			}))
		}

		assert.Nil(indexXML.Write())
		indexContent, err := os.ReadFile(tempIndexFile)
		assert.Nil(err)
		indexServer.AddRoute(installer.PublicIndexName, indexContent)

		// Add the path to the pack's pdsc
		for i := 1; i < 11; i++ {
			pdsc := publicConcurrentLocalPdscBase + fmt.Sprint(i) + ".pdsc"
			pdscContent, err := os.ReadFile(filepath.Join(testDir, "concurrent", "1.2.4", pdsc))
			assert.Nil(err)
			indexServer.AddRoute(pdsc, pdscContent)
		}

		assert.Nil(installer.UpdatePublicIndex("", !Sparse, DownloadPdsc, !DownloadRemainingPdscFiles, !UpdatePrivatePdsc, ShowInfo, 5, Timeout))

		// Make sure index.pidx exists and it is updated
		assert.FileExists(installer.Installation.PublicIndex)
		copied, err2 := os.ReadFile(installer.Installation.PublicIndex)
		assert.Nil(err2)
		assert.Equal(copied, indexContent)

		// Make sure the pdsc under .Web/ is updated
		time.Sleep(400 * time.Millisecond)
		for i := 1; i < 11; i++ {
			pdsc := publicConcurrentLocalPdscBase + fmt.Sprint(i) + ".pdsc"
			pdscFile := filepath.Join(installer.Installation.WebDir, pdsc)
			assert.FileExists(pdscFile)
			pdscXML := xml.NewPdscXML(pdscFile)
			assert.Nil(pdscXML.Read())
			assert.Equal("1.2.4", pdscXML.LatestVersion())
		}
	})
}

func checkPackRoot(t *testing.T, path string) {
	assert := assert.New(t)

	assert.True(utils.DirExists(path))
	assert.True(utils.DirExists(installer.Installation.DownloadDir))
	assert.True(utils.DirExists(installer.Installation.WebDir))
	assert.True(utils.DirExists(installer.Installation.LocalDir))
}

func removePackRoot(packRoot string) {
	utils.UnsetReadOnlyR(packRoot)
	os.RemoveAll(packRoot)
}

func TestSetPackRoot(t *testing.T) {

	assert := assert.New(t)

	// Sanity tests
	t.Run("test fail to initialize empty pack root", func(t *testing.T) {
		localTestingDir := ""
		err := installer.SetPackRoot(localTestingDir, !CreatePackRoot)
		assert.Equal(errs.ErrPackRootNotFound, err)

		err = installer.SetPackRoot(localTestingDir, CreatePackRoot)
		assert.Equal(errs.ErrPackRootNotFound, err)
	})

	t.Run("test fail to use non-existing directory", func(t *testing.T) {
		localTestingDir := "non-existing-dir"
		err := installer.SetPackRoot(localTestingDir, !CreatePackRoot)
		assert.Equal(errs.ErrPackRootDoesNotExist, err)
	})

	t.Run("test initialize pack root", func(t *testing.T) {
		localTestingDir := "valid-pack-root"
		defer removePackRoot(localTestingDir)
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))

		checkPackRoot(t, localTestingDir)

		// Now just make sure it's usable, even when not forced to initialize
		assert.Nil(installer.SetPackRoot(localTestingDir, !CreatePackRoot))
	})

	// Define a few paths to try out per operating system
	paths := generatePaths(t)
	for description, path := range paths {
		t.Run("test "+description, func(t *testing.T) {
			defer os.RemoveAll(path)
			assert.Nil(installer.SetPackRoot(path, CreatePackRoot))

			checkPackRoot(t, path)

			// Now just make sure it's usable, even when not forced to initialize
			assert.Nil(installer.SetPackRoot(path, !CreatePackRoot))
		})
	}
}

func init() {
	logLevel := log.InfoLevel
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = log.DebugLevel
	}
	log.SetLevel(logLevel)
	log.SetFormatter(new(LogFormatter))
}
