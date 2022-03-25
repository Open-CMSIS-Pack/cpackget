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
	"runtime"
	"strings"
	"testing"
	"time"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
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

func getPackIdxModTime() time.Time {
	packIdx, _ := os.Stat(installer.Installation.PackIdx)
	return packIdx.ModTime()
}

func checkPackIsInstalled(t *testing.T, pack *installer.PackType) {
	assert := assert.New(t)

	assert.True(installer.Installation.PackIsInstalled(pack))

	// Make sure there's a copy of the pack file in .Download/
	assert.True(utils.FileExists(filepath.Join(installer.Installation.DownloadDir, pack.PackFileName())))

	// Make sure there's a versioned copy of the PDSC file in .Download/
	assert.True(utils.FileExists(filepath.Join(installer.Installation.DownloadDir, pack.PdscFileNameWithVersion())))

	if pack.IsPublic {
		// Make sure no PDSC file got copied to .Local/
		assert.False(utils.FileExists(filepath.Join(installer.Installation.LocalDir, pack.PdscFileName())))

		// Make sure there is a PDSC file under .Web/
		assert.True(utils.FileExists(filepath.Join(installer.Installation.WebDir, pack.PdscFileName())))
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

	info, err := utils.ExtractPackInfo(packPath)
	assert.Nil(err)

	// Check in installer internals
	pack := packInfoToType(info)
	pack.IsPublic = config.IsPublic

	checkPackIsInstalled(t, pack)
}

func removePack(t *testing.T, packPath string, withVersion, isPublic, purge bool) {
	// TODO:Add option to remove ALL

	assert := assert.New(t)

	// Get pack.idx before removing pack
	packIdxModTime := getPackIdxModTime()

	// [http://vendor.com|path/to]/TheVendor.PackName.x.y.z -> TheVendor.PackName[.x.y.z]
	shortPackPath := shortenPackPath(packPath, withVersion)

	info, err := utils.ExtractPackInfo(shortPackPath)
	assert.Nil(err)

	// Check in installer internals
	pack := packInfoToType(info)
	isInstalled := installer.Installation.PackIsInstalled(pack)

	purgeOnly := !isInstalled && purge

	err = installer.RemovePack(shortPackPath, purge)
	assert.Nil(err)

	removeAll := false

	if removeAll {
		assert.False(installer.Installation.PackIsInstalled(pack))

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
		if runtime.GOOS == "windows" {
			// Apparently Windows systems update of file modified times
			// happens 64 times per second, and in some cases that is not
			// enough for the time delta below to show a difference
			// Ref: https://www.lochan.org/2005/keith-cl/useful/win32time.html#timingwin
			// So let's sleep a bit before checking for file mod times
			// Sleeping for 1 second still caused interemittent results on Windows,
			// bumping this up to 3 seconds
			time.Sleep(3 * time.Second)
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

	CreatePackRoot = true

	// Available testing packs
	testDir = filepath.Join("..", "..", "testdata", "integration")

	malformedPackName              = "pack-with-bad-name"
	packThatDoesNotExist           = "ThisPack.DoesNotExist.0.0.1.pack"
	packWithCorruptZip             = filepath.Join(testDir, "FakeZip.PackName.1.2.3.pack")
	packWithMalformedURL           = "http://:malformed-url*/TheVendor.PackName.1.2.3.pack"
	packWithoutPdscFileInside      = filepath.Join(testDir, "PackWithout.PdscFileInside.1.2.3.pack")
	packWithTaintedCompressedFiles = filepath.Join(testDir, "PackWith.TaintedFiles.1.2.3.pack")

	// Packs with packid names only
	publicRemotePackPackID      = "TheVendor.PublicRemotePack"
	publicRemotePack123PackID   = publicRemotePackPackID + ".1.2.3"
	nonPublicLocalPackPackID    = "TheVendor.NonPublicLocalPack"
	nonPublicLocalPack123PackID = nonPublicLocalPackPackID + ".1.2.3"
	nonPublicLocalPack124PackID = nonPublicLocalPackPackID + ".1.2.4"

	// Pdsc files to test out installing packs with pack id only
	pdscPack123MissingVersion = filepath.Join(testDir, "TheVendor.PublicRemotePack_VersionNotAvailable.pdsc")

	// Public packs
	publicLocalPack123  = filepath.Join(testDir, "1.2.3", "TheVendor.PublicLocalPack.1.2.3.pack")
	publicLocalPack124  = filepath.Join(testDir, "1.2.4", "TheVendor.PublicLocalPack.1.2.4.pack")
	publicRemotePack123 = filepath.Join(testDir, "1.2.3", publicRemotePack123PackID+".pack")

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
// server := NewHttpsServer(map[string][]byte{
// 	"*": []byte("Default content"),
// 	"should-return-404": nil,
// })
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

func TestUpdatePublicIndex(t *testing.T) {

	assert := assert.New(t)

	var Overwrite = true

	t.Run("test add http server index.pidx", func(t *testing.T) {
		localTestingDir := "test-add-http-server-index"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		httpServer := httptest.NewServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				},
			),
		)

		indexPath := httpServer.URL + "/index.pidx"

		err := installer.UpdatePublicIndex(indexPath, Overwrite)
		assert.NotNil(err)
		assert.Equal(errs.ErrIndexPathNotSafe, err)
	})

	t.Run("test add not found remote index.pidx", func(t *testing.T) {
		localTestingDir := "test-add-not-found-remote-index"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		server := NewServer()

		indexPath := server.URL() + "this-file-does-not-exist"

		err := installer.UpdatePublicIndex(indexPath, Overwrite)

		assert.NotNil(err)
		assert.Equal(errs.ErrBadRequest, err)
	})

	t.Run("test add malformed index.pidx", func(t *testing.T) {
		localTestingDir := "test-add-malformed-index"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		indexContent, err := ioutil.ReadFile(malformedPublicIndex)
		assert.Nil(err)

		indexServer := NewServer()
		indexServer.AddRoute("index.pidx", indexContent)
		indexPath := indexServer.URL() + "index.pidx"

		err = installer.UpdatePublicIndex(indexPath, Overwrite)

		assert.NotNil(err)
		assert.Equal(err.Error(), "XML syntax error on line 3: unexpected EOF")
	})

	t.Run("test add remote index.pidx", func(t *testing.T) {
		localTestingDir := "test-add-remote-index"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		indexContent, err := ioutil.ReadFile(samplePublicIndex)
		assert.Nil(err)
		indexServer := NewServer()
		indexServer.AddRoute("index.pidx", indexContent)
		indexPath := indexServer.URL() + "index.pidx"

		err = installer.UpdatePublicIndex(indexPath, Overwrite)

		assert.Nil(err)

		assert.True(utils.FileExists(installer.Installation.PublicIndex))

		copied, err2 := ioutil.ReadFile(installer.Installation.PublicIndex)
		assert.Nil(err2)

		assert.Equal(copied, indexContent)
	})

	t.Run("test do not overwrite index.pidx", func(t *testing.T) {
		localTestingDir := "test-do-not-overwrite-index"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		_ = utils.TouchFile(installer.Installation.PublicIndex)

		indexContent, err := ioutil.ReadFile(samplePublicIndex)
		assert.Nil(err)
		indexServer := NewServer()
		indexServer.AddRoute("index.pidx", indexContent)
		indexPath := indexServer.URL() + "index.pidx"

		err = installer.UpdatePublicIndex(indexPath, !Overwrite)

		assert.NotNil(err)
		assert.Equal(errs.ErrCannotOverwritePublicIndex, err)
	})

	t.Run("test overwrite index.pidx", func(t *testing.T) {
		localTestingDir := "test-overwrite-index"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		_ = utils.TouchFile(installer.Installation.PublicIndex)

		indexContent, err := ioutil.ReadFile(samplePublicIndex)
		assert.Nil(err)
		indexServer := NewServer()
		indexServer.AddRoute("index.pidx", indexContent)
		indexPath := indexServer.URL() + "index.pidx"

		err = installer.UpdatePublicIndex(indexPath, Overwrite)

		assert.Nil(err)
		assert.True(utils.FileExists(installer.Installation.PublicIndex))

		copied, err2 := ioutil.ReadFile(installer.Installation.PublicIndex)
		assert.Nil(err2)

		assert.Equal(copied, indexContent)
	})
}

func checkPackRoot(t *testing.T, path string) {
	assert := assert.New(t)

	assert.True(utils.DirExists(path))
	assert.True(utils.DirExists(installer.Installation.DownloadDir))
	assert.True(utils.DirExists(installer.Installation.WebDir))
	assert.True(utils.DirExists(installer.Installation.LocalDir))
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
		defer os.RemoveAll(localTestingDir)
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
	log.SetLevel(log.InfoLevel)
	log.SetFormatter(new(LogFormatter))
}
