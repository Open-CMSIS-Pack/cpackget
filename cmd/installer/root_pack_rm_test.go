/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/stretchr/testify/assert"
)

func TestRemovePack(t *testing.T) {

	assert := assert.New(t)

	// Sanity tests
	t.Run("test removing a pack with malformed name", func(t *testing.T) {
		localTestingDir := "test-remove-pack-with-bad-name"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		err := installer.RemovePack("TheVendor.PackName.no-a-valid-version", false, true, Timeout)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrBadPackName)
	})

	t.Run("test removing a pack that is not installed", func(t *testing.T) {
		localTestingDir := "test-remove-pack-not-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		err := installer.RemovePack("TheVendor.PackName.1.2.3", false, true, Timeout)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrPackNotInstalled)
	})

	t.Run("test remove a public pack that was added", func(t *testing.T) {
		localTestingDir := "test-remove-public-pack-that-was-added"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

		packPath := publicLocalPack124
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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

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
		err := installer.RemovePack(shortenPackPath(packPath, false), true, true, Timeout) // withVersion=false, purge=true
		assert.Equal(errs.ErrPackNotPurgeable, err)
	})

	t.Run("test purge a pack with license", func(t *testing.T) {
		localTestingDir := "test-purge-pack-with-license"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		packPath := packWithLicense

		shortPackPath := shortenPackPath(packPath, false) // withVersion=true

		licenseFilePath := filepath.Join(installer.Installation.DownloadDir, filepath.Base(packPath)+".LICENSE.txt")

		// Add a pack
		addPack(t, packPath, ConfigType{})
		assert.False(utils.FileExists(licenseFilePath))

		// Now extract its license license
		addPack(t, packPath, ConfigType{
			ExtractEula: true,
		})
		assert.True(utils.FileExists(licenseFilePath))

		// Purge it
		removePack(t, packPath, true, NotPublic, true) // withVersion=true, purge=true

		// Make sure pack is not purgeable
		err := installer.RemovePack(shortPackPath, true, true, Timeout) // purge=true
		assert.Equal(errs.ErrPackNotPurgeable, err)

		assert.False(utils.FileExists(licenseFilePath))
	})

	t.Run("test remove latest version", func(t *testing.T) {
		localTestingDir := "test-remove-latest-versions"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer removePackRoot(localTestingDir)

		// Add a pack, add an updated version of the pack, then remove the first one
		packPath := publicLocalPack123
		updatedPackPath := publicLocalPack124
		config := ConfigType{
			IsPublic: true,
		}
		addPack(t, packPath, config)
		addPack(t, updatedPackPath, config)

		// Remove latest pack (withVersion=false), i.e. path will be "TheVendor.PackName"
		removePack(t, packPath, false, IsPublic, true) // withVersion=false, purge=true
	})

	t.Run("test remove public pack without pdsc file in .Web folder", func(t *testing.T) {
		localTestingDir := "test-remove-public-pack-without-pdsc-in-web-folder"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		packPath := publicLocalPack123
		packInfo, err := utils.ExtractPackInfo(packPath)
		assert.Nil(err)

		// Make sure there's a pdsc in .Web
		pdscFilePath := filepath.Join(installer.Installation.WebDir, fmt.Sprintf("%s.%s.pdsc", packInfo.Vendor, packInfo.Pack))
		assert.Nil(utils.TouchFile(pdscFilePath))

		config := ConfigType{
			IsPublic: true,
		}
		addPack(t, packPath, config)

		// Make sure there is no PDSC file in .Web/
		os.Remove(pdscFilePath)

		removePack(t, packPath, true, IsPublic, false) // withVersion=true, purge=false

		// Assert that the file did not get created during the operation
		assert.False(utils.FileExists(pdscFilePath))
	})
}
