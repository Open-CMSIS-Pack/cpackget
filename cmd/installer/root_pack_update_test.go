/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"os"
	"path/filepath"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	"github.com/stretchr/testify/assert"
)

// cpackget pack update Vendor.PackName                            # packID without version
func TestUpdatePack(t *testing.T) {

	assert := assert.New(t)

	// Sanity tests
	t.Run("test updating a pack with bad name", func(t *testing.T) {
		localTestingDir := "test-update-pack-with-bad-name"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		for i := range malformedPackNames {
			err := installer.UpdatePack(malformedPackNames[i], !CheckEula, !NoRequirements, SubCall, !InsecureSkipVerify, true, Timeout)
			// Sanity check
			assert.NotNil(err)
			assert.Equal(err, errs.ErrBadPackName)
		}

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test updating a pack not installed", func(t *testing.T) {
		localTestingDir := "test-update-pack-not-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		packPath := publicLocalPack123
		assert.Nil(installer.UpdatePack(packPath, !CheckEula, !NoRequirements, SubCall, !InsecureSkipVerify, true, Timeout))
	})

	t.Run("test updating downloaded pack", func(t *testing.T) {
		localTestingDir := "test-update-downloaded-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		packPath := packToUpdate
		addPack(t, packPath, ConfigType{})
		removePack(t, packPath, true, true, false)
		packPath = filepath.Join(installer.Installation.DownloadDir, packToUpdateFileName)
		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, !NoRequirements, !InsecureSkipVerify, true, Timeout)
		assert.Nil(err)

		// ensure downloaded pack remains valid
		err = installer.UpdatePack(packPath, !CheckEula, !NoRequirements, SubCall, !InsecureSkipVerify, true, Timeout)
		assert.Nil(err)

		packToUpdate, err := utils.ExtractPackInfo(packPath)
		assert.Nil(err)
		checkPackIsInstalled(t, packInfoToType(packToUpdate))
	})

	t.Run("test updating a pack with metadata", func(t *testing.T) {
		localTestingDir := "test-update-pack-metadata"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		packPath := publicLocalPack123meta
		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, ForceReinstall, !NoRequirements, !InsecureSkipVerify, true, Timeout)
		assert.Nil(err)

		err = installer.UpdatePack(packPath, !CheckEula, !NoRequirements, SubCall, !InsecureSkipVerify, true, Timeout)
		assert.Nil(err)

		packToReinstall, err := utils.ExtractPackInfo(packPath)
		assert.Nil(err)
		checkPackIsInstalled(t, packInfoToType(packToReinstall))
	})

	t.Run("test updating local pack", func(t *testing.T) {
		localTestingDir := "test-update-local-pack"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		packPath := packThatDoesNotExist

		err := installer.UpdatePack(packPath, !CheckEula, !NoRequirements, SubCall, !InsecureSkipVerify, true, Timeout)
		assert.Nil(err)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test updating remote pack that does not exist", func(t *testing.T) {
		localTestingDir := "test-update-remote-pack-that-does-not-exist"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		notFoundServer := NewServer()

		packPath := notFoundServer.URL() + packThatDoesNotExist

		err := installer.UpdatePack(packPath, !CheckEula, !NoRequirements, SubCall, !InsecureSkipVerify, true, Timeout)
		assert.Nil(err)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test updating a pack with bad URL format", func(t *testing.T) {
		localTestingDir := "test-update-pack-with-malformed-url"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		packPath := packWithMalformedURL

		err := installer.UpdatePack(packPath, !CheckEula, !NoRequirements, SubCall, !InsecureSkipVerify, true, Timeout)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(errs.ErrBadPackURL, err)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

	t.Run("test updating all installed packs", func(t *testing.T) {
		localTestingDir := "test-update-all-installed-packs"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		// packPath := publicLocalPack123
		// err := installer.AddPack(packPath, !CheckEula, !ExtractEula, ForceReinstall, !NoRequirements, !InsecureSkipVerify, true, Timeout)
		// assert.Nil(err)

		// packPath := packToUpdate
		// addPack(t, packPath, ConfigType{})
		// removePack(t, packPath, true, true, false)
		// packPath = filepath.Join(installer.Installation.DownloadDir, packToUpdateFileName)
		// err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, !NoRequirements, !InsecureSkipVerify, true, Timeout)
		// assert.Nil(err)

		// Inject pdsc into .Web folder
		packPdscFilePath := filepath.Join(installer.Installation.WebDir, filepath.Base(pdscPublicLocalPack))
		assert.Nil(utils.CopyFile(pdscPublicLocalPack, packPdscFilePath))

		packPdscTag := xml.PdscTag{
			Vendor:  "TheVendor",
			Name:    "PublicLocalPack",
			Version: "1.2.2",
		}
		assert.Nil(installer.Installation.PublicIndexXML.AddPdsc(packPdscTag))
		assert.Nil(installer.Installation.PublicIndexXML.Write())

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
		pack124Content, err := os.ReadFile(publicLocalPack124)
		assert.Nil(err)
		server := NewServer()
		server.AddRoute(pack124.PackFileName(), pack124Content)

		// Inject URL into pdsc
		pdscXML := xml.NewPdscXML(packPdscFilePath)
		utils.UnsetReadOnly(packPdscFilePath)
		assert.Nil(pdscXML.Read())
		pdscXML.URL = server.URL()
		assert.Nil(utils.WriteXML(packPdscFilePath, pdscXML))

		err = installer.UpdatePack("", !CheckEula, !NoRequirements, SubCall, !InsecureSkipVerify, true, Timeout)

		// Sanity check
		assert.Nil(err)
	})

	t.Run("test updating pack with insecureSkipVerify parameter", func(t *testing.T) {
		localTestingDir := "test-update-pack-insecure-skip-verify"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		packPath := publicLocalPack123

		// First, install the pack
		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, !NoRequirements, !InsecureSkipVerify, true, Timeout)
		assert.Nil(err)

		// Test update with insecureSkipVerify = true
		err = installer.UpdatePack(packPath, !CheckEula, !NoRequirements, SubCall, InsecureSkipVerify, true, Timeout)
		assert.Nil(err)

		// Test update with insecureSkipVerify = false (default)
		err = installer.UpdatePack(packPath, !CheckEula, !NoRequirements, SubCall, !InsecureSkipVerify, true, Timeout)
		assert.Nil(err)
	})

}
