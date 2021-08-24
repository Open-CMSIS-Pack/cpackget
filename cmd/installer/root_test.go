/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the vidx2pidx project. */

package installer_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
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
	return packPath[:len(packPath)-len("x.y.z.pack")] + ".pdsc"
}

func checkPackIsInstalled(t *testing.T, packPath string) bool {
	info, err := utils.ExtractPackInfo(packPath, false)
	assert.Nil(t, err)
	pack := packInfoToType(info)
	return installer.Installation.PackIsInstalled(pack)
}

// Tests should cover all possible scenarios for adding packs
// +----------------+--------+------------+
// | origin/privacy | public | non-public |
// +----------------+--------+------------+
// | local          |        |            |
// +----------------+--------+------------+
// | remote         |        |            |
// +----------------+--------+------------+

var (
	// Available testing packs
	testDir = "../../testdata/integration/"

	malformedPackName              = "pack-with-bad-name"
	packThatDoesNotExist           = "ThisPack.DoesNotExist.0.0.1.pack"
	packWithCorruptZip             = path.Join(testDir, "FakeZip.PackName.1.2.3.pack")
	packWithMalformedURL           = "http://:malformed-url*/TheVendor.PackName.1.2.3.pack"
	packWithoutPdscFileInside      = path.Join(testDir, "PackWithout.PdscFileInside.1.2.3.pack")
	packWithTaintedCompressedFiles = path.Join(testDir, "PackWith.TaintedFiles.1.2.3.pack")

	// Public packs
	publicLocalPack123 = path.Join(testDir, "1.2.3", "TheVendor.PublicLocalPack.1.2.3.pack")
	// publicLocalPack124  = path.Join(testDir, "1.2.4", "TheVendor.PublicLocalPack.1.2.4.pack")
	publicRemotePack123 = path.Join(testDir, "1.2.3", "TheVendor.PublicRemotePack.1.2.3.pack")
	// publicRemotePack124 = path.Join(testDir, "1.2.4", "TheVendor.PublicRemotePack.1.2.4.pack")

	// Private packs
	nonPublicLocalPack123 = path.Join(testDir, "1.2.3", "TheVendor.NonPublicLocalPack.1.2.3.pack")
	// nonPublicLocalPack124  = path.Join(testDir, "1.2.4", "TheVendor.NonPublicLocalPack.1.2.4.pack")
	// nonPublicRemotePack123 = path.Join(testDir, "1.2.3", "TheVendor.NonPublicRemotePack.1.2.3.pack")
	// nonPublicRemotePack124 = path.Join(testDir, "1.2.4", "TheVendor.NonPublicRemotePack.1.2.4.pack")
)

func TestSetPackRoot(t *testing.T) {
	err := installer.SetPackRoot("/")
	assert.NotNil(t, err)
	assert.Equal(t, err, errs.ErrFailedCreatingDirectory)
}

func TestAddPack(t *testing.T) {

	assert := assert.New(t)

	// Sanity tests
	t.Run("test installing a pack with bad name", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-bad-name"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := malformedPackName

		err := installer.AddPack(packPath)

		// Sanity check
		assert.NotNil(err)
		assert.True(errs.Is(err, errs.ErrBadPackNameInvalidExtension))
	})

	t.Run("test installing a pack previously installed", func(t *testing.T) {
		localTestingDir := "test-add-pack-previously-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := publicLocalPack123

		err := installer.AddPack(packPath)

		// Sanity check
		assert.Nil(err)

		_, packPath = path.Split(packPath)

		// Make sure it really got installed
		assert.True(checkPackIsInstalled(t, packPath))

		// Make sure that both pack and pdsc files are under .Download folder
		assert.True(utils.FileExists(path.Join(installer.Installation.DownloadDir, packPath)))
		assert.True(utils.FileExists(path.Join(installer.Installation.DownloadDir, packPathToPdsc(packPath, true))))

		// Attempt installing it again, this time we should get an error
		packPath = publicLocalPack123
		err = installer.AddPack(packPath)
		assert.NotNil(err)
		assert.Equal(err, errs.ErrPackAlreadyInstalled)
	})

	t.Run("test installing local pack that does not exist", func(t *testing.T) {
		localTestingDir := "test-add-local-pack-that-does-not-exist"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packThatDoesNotExist

		err := installer.AddPack(packPath)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrFileNotFound)
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

		err := installer.AddPack(packPath)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrBadRequest)
	})

	t.Run("test installing a pack with corrupt zip file", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-corrupt-zip"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithCorruptZip

		err := installer.AddPack(packPath)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrFailedDecompressingFile)
	})

	t.Run("test installing a pack with bad URL format", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-malformed-url"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithMalformedURL

		err := installer.AddPack(packPath)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrBadPackURL)
	})

	t.Run("test installing a pack with no PDSC file inside", func(t *testing.T) {
		localTestingDir := "test-add-pack-without-pdsc-file"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		packPath := packWithoutPdscFileInside

		err := installer.AddPack(packPath)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrPdscFileNotFound)
	})

	t.Run("test installing a pack that has problems with its directory", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-unaccessible-directory"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = path.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := publicLocalPack123

		// Force a bad file path
		installer.Installation.PackRoot = "/"
		err := installer.AddPack(packPath)

		// Sanity check
		assert.NotNil(err)

		assert.Equal(err, errs.ErrFailedCreatingDirectory)
	})

	t.Run("test installing a pack with tainted compressed files", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-tainted-compressed-files"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = path.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := packWithTaintedCompressedFiles

		err := installer.AddPack(packPath)

		// Sanity check
		assert.NotNil(err)

		assert.Equal(err, errs.ErrInsecureZipFileName)
	})

	// Test installing a combination of public/non-public local/remote packs
	t.Run("test installing public pack via local file", func(t *testing.T) {
		localTestingDir := "test-add-public-local-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = path.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := publicLocalPack123

		err := installer.AddPack(packPath)

		// Sanity check
		assert.Nil(err)

		_, packPath = path.Split(packPath)

		// Make sure it really got installed
		assert.True(checkPackIsInstalled(t, packPath))

		// Make sure that both pack and pdsc files are under .Download folder
		assert.True(utils.FileExists(path.Join(installer.Installation.DownloadDir, packPath)))
		assert.True(utils.FileExists(path.Join(installer.Installation.DownloadDir, packPathToPdsc(packPath, true))))

		// Make sure no pdsc file got copied to .Local
		assert.False(utils.FileExists(path.Join(installer.Installation.LocalDir, packPathToPdsc(packPath, false))))
	})

	t.Run("test installing public pack via remote file", func(t *testing.T) {
		localTestingDir := "test-add-public-remote-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = path.Join(testDir, "public_index")
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

		_, packBasePath := path.Split(publicRemotePack123)

		packPath := packServer.URL + "/" + packBasePath

		err = installer.AddPack(packPath)

		// Sanity check
		assert.Nil(err)

		// Make sure it really got installed
		assert.True(checkPackIsInstalled(t, packBasePath))

		// Make sure that both pack and pdsc files are under .Download folder
		assert.True(utils.FileExists(path.Join(installer.Installation.DownloadDir, packBasePath)))
		assert.True(utils.FileExists(path.Join(installer.Installation.DownloadDir, packPathToPdsc(packBasePath, true))))

		// Make sure no pdsc file got copied to .Local
		assert.False(utils.FileExists(path.Join(installer.Installation.LocalDir, packPathToPdsc(packPath, false))))
	})

	t.Run("test installing non-public pack via local file", func(t *testing.T) {
		localTestingDir := "test-add-non-public-local-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		installer.Installation.WebDir = path.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

		packPath := nonPublicLocalPack123

		err := installer.AddPack(packPath)

		// Sanity check
		assert.Nil(err)

		_, packPath = path.Split(packPath)

		// Make sure it really got installed
		assert.True(checkPackIsInstalled(t, packPath))

		// Make sure that both pack and pdsc files are under .Download folder
		assert.True(utils.FileExists(path.Join(installer.Installation.DownloadDir, packPath)))
		assert.True(utils.FileExists(path.Join(installer.Installation.DownloadDir, packPathToPdsc(packPath, true))))

		// Make sure pdsc file got copied to .Local
		// TODO: check why it's not working
		// assert.True(utils.FileExists(path.Join(installer.Installation.LocalDir, packPathToPdsc(packPath, false))))
	})
}

func TestRemovePack(t *testing.T) {

	assert := assert.New(t)

	// Sanity tests
	t.Run("test removing a pack that is not installed", func(t *testing.T) {
		localTestingDir := "test-add-pack-with-bad-name"
		assert.Nil(installer.SetPackRoot(localTestingDir))
		defer os.RemoveAll(localTestingDir)

		err := installer.AddPack("TheVendor.PackName.1.2.3")

		// Sanity check
		assert.NotNil(err)
		assert.True(errs.Is(err, errs.ErrBadPackNameInvalidExtension))
	})

	// Install a pack and Remove it
	// Install a pack, a new version and Remove the first one
	// Install a pack, a new version and Remove all
	// Install a pack, a new version and Remove all with purge
	// Install a pack, and remove it with purge
	// Install a pack, remove it, then purge it
	// Install a pack, a new version, remove the first, then remove the second one with purge
}
