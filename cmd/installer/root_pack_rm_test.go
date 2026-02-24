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
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
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

		_, err := installer.RemovePack("TheVendor.PackName.no-a-valid-version", false, true)

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

		_, err := installer.RemovePack("TheVendor.PackName.1.2.3", false, true)

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

		packInfo, err := utils.ExtractPackInfo(packPath)
		assert.Nil(err)
		pack := packInfoToType(packInfo)
		packPdscTag := xml.PdscTag{
			Vendor:  pack.Vendor,
			Name:    pack.Name,
			Version: pack.Version,
		}
		assert.Nil(installer.Installation.PublicIndexXML.AddPdsc(packPdscTag))
		assert.Nil(installer.Installation.PublicIndexXML.Write())

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

		packInfo, err := utils.ExtractPackInfo(packPath)
		assert.Nil(err)
		pack := packInfoToType(packInfo)
		packPdscTag := xml.PdscTag{
			Vendor:  pack.Vendor,
			Name:    pack.Name,
			Version: pack.Version,
		}
		assert.Nil(installer.Installation.PublicIndexXML.AddPdsc(packPdscTag))
		assert.Nil(installer.Installation.PublicIndexXML.Write())

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

		packInfo, err := utils.ExtractPackInfo(packPath)
		assert.Nil(err)
		pack := packInfoToType(packInfo)
		packPdscTag := xml.PdscTag{
			Vendor:  pack.Vendor,
			Name:    pack.Name,
			Version: pack.Version,
		}
		assert.Nil(installer.Installation.PublicIndexXML.AddPdsc(packPdscTag))
		assert.Nil(installer.Installation.PublicIndexXML.Write())

		addPack(t, packPath, ConfigType{
			IsPublic: true,
		})

		// Remove it without purge
		removePack(t, packPath, true, NotPublic, false) // withVersion=true, purge=true

		// Now just purge it
		removePack(t, packPath, true, NotPublic, true) // withVersion=true, purge=true

		// Make sure pack is not purgeable
		ok, err := installer.RemovePack(shortenPackPath(packPath, false), true, true) // withVersion=false, purge=true
		assert.Nil(err)
		assert.Equal(ok, true)
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
		ok, err := installer.RemovePack(shortPackPath, true, true) // purge=true
		assert.Nil(err)
		assert.Equal(ok, true)

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

		packInfo, err := utils.ExtractPackInfo(packPath)
		assert.Nil(err)
		pack := packInfoToType(packInfo)
		packPdscTag := xml.PdscTag{
			Vendor:  pack.Vendor,
			Name:    pack.Name,
			Version: pack.Version,
		}
		assert.Nil(installer.Installation.PublicIndexXML.AddPdsc(packPdscTag))
		assert.Nil(installer.Installation.PublicIndexXML.Write())

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
		pack := packInfoToType(packInfo)
		packPdscTag := xml.PdscTag{
			Vendor:  pack.Vendor,
			Name:    pack.Name,
			Version: pack.Version,
		}
		assert.Nil(installer.Installation.PublicIndexXML.AddPdsc(packPdscTag))
		assert.Nil(installer.Installation.PublicIndexXML.Write())

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

	t.Run("test remove pack registered only via AddPdsc using Vendor::Name", func(t *testing.T) {
		localTestingDir := "test-remove-pack-pdsc-only-vendor-name"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		// Register a pack via AddPdsc (only creates entry in local_repository.pidx)
		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		// Verify it is registered
		tags := installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(1, len(tags))

		// RemovePack using legacy PackID format Vendor::Name
		_, err = installer.RemovePack("TheVendor::PackName", false, true)
		assert.Nil(err)

		// Verify the entry was removed from local_repository.pidx
		tags = installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(0, len(tags))
	})

	t.Run("test remove pack registered only via AddPdsc using Vendor::Name@Version", func(t *testing.T) {
		localTestingDir := "test-remove-pack-pdsc-only-vendor-name-version"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		// Register a pack via AddPdsc
		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		// Verify it is registered
		tags := installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(1, len(tags))

		// RemovePack using legacy PackID format Vendor::Name@Version
		_, err = installer.RemovePack("TheVendor::PackName@1.2.3", false, true)
		assert.Nil(err)

		// Verify the entry was removed from local_repository.pidx
		tags = installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(0, len(tags))
	})

	t.Run("test remove pack registered only via AddPdsc using Vendor.Name", func(t *testing.T) {
		localTestingDir := "test-remove-pack-pdsc-only-dotted-name"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		// Register a pack via AddPdsc
		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		// Verify it is registered
		tags := installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(1, len(tags))

		// RemovePack using dotted PackID format Vendor.Name
		_, err = installer.RemovePack("TheVendor.PackName", false, true)
		assert.Nil(err)

		// Verify the entry was removed from local_repository.pidx
		tags = installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(0, len(tags))
	})

	t.Run("test remove pack registered only via AddPdsc whose source file no longer exists", func(t *testing.T) {
		localTestingDir := "test-remove-pack-pdsc-only-source-missing"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		// Create a temporary copy of a PDSC file so we can delete it later
		tempDir := filepath.Join(localTestingDir, "temp-pdsc")
		assert.Nil(os.MkdirAll(tempDir, 0700))
		tempPdsc := filepath.Join(tempDir, "TheVendor.PackName.pdsc")
		assert.Nil(utils.CopyFile(pdscPack123, tempPdsc))

		// Register via AddPdsc
		err := installer.AddPdsc(tempPdsc)
		assert.Nil(err)

		// Verify it is registered
		tags := installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(1, len(tags))

		// Delete the source file
		assert.Nil(os.Remove(tempPdsc))
		assert.False(utils.FileExists(tempPdsc))

		// RemovePack should still succeed
		_, err = installer.RemovePack("TheVendor::PackName", false, true)
		assert.Nil(err)

		// Verify the entry was removed from local_repository.pidx
		tags = installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(0, len(tags))
	})

	t.Run("test remove pack registered via AddPdsc with wrong version returns error", func(t *testing.T) {
		localTestingDir := "test-remove-pack-pdsc-only-wrong-version"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		// Register a pack via AddPdsc (version in pdsc is 1.2.3)
		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		// Verify it is registered
		tags := installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(1, len(tags))

		// RemovePack with a non-matching version should fail
		_, err = installer.RemovePack("TheVendor::PackName@9.9.9", false, true)
		assert.NotNil(err)
		assert.Equal(errs.ErrPdscEntryNotFound, err)

		// Verify the entry is still present in local_repository.pidx
		tags = installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(1, len(tags))
	})
}
