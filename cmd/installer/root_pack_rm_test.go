/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"os"
	"path/filepath"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/stretchr/testify/assert"
)

func TestRemovePack(t *testing.T) {

	assert := assert.New(t)

	// Sanity tests
	t.Run("test removing a pack with malformed name", func(t *testing.T) {
		localTestingDir := "test-remove-pack-with-bad-name"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		err := installer.RemovePack("TheVendor.PackName.no-a-valid-version", false)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrBadPackName)
	})

	t.Run("test removing a pack that is not installed", func(t *testing.T) {
		localTestingDir := "test-remove-pack-not-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		err := installer.RemovePack("TheVendor.PackName.1.2.3", false)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(err, errs.ErrPackNotInstalled)
	})

	t.Run("test remove a public pack that was added", func(t *testing.T) {
		localTestingDir := "test-remove-public-pack-that-was-added"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.Installation.WebDir = filepath.Join(testDir, "public_index")
		defer os.RemoveAll(localTestingDir)

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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
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
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
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

	t.Run("test remove latest version", func(t *testing.T) {
		localTestingDir := "test-remove-latest-versions"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
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

		// Remove latest pack (withVersion=false), i.e. path will be "TheVendor.PackName"
		removePack(t, packPath, false, IsPublic, true) // withVersion=false, purge=true
	})

	// t.Run("test remove all versions at once", func(t *testing.T) {
	// 	localTestingDir := "test-remove-all-versions-at-once"
	// 	assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
	// 	installer.Installation.WebDir = filepath.Join(testDir, "public_index")
	// 	defer os.RemoveAll(localTestingDir)

	// 	// Add a pack, add an updated version of the pack, then remove the first one
	// 	packPath := publicLocalPack123
	// 	updatedPackPath := publicLocalPack124
	// 	config := ConfigType{
	// 		IsPublic: true,
	// 	}
	// 	addPack(t, packPath, config)
	// 	addPack(t, updatedPackPath, config)

	// 	// Remove latest pack (withVersion=false), i.e. path will be "TheVendor.PackName"
	// 	removePack(t, packPath, false, IsPublic, true) // withVersion=false, purge=true
	// })
}
