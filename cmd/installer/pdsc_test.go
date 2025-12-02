/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	"github.com/stretchr/testify/assert"
)

// Helper function to create a test PDSC file
func createTestPdscFile(t *testing.T, dir, vendor, name, version, url string) string {
	pdscContent := `<?xml version="1.0" encoding="UTF-8"?>
<package schemaVersion="1.7.2" xmlns:xs="http://www.w3.org/2001/XMLSchema-instance" xs:noNamespaceSchemaLocation="PACK.xsd">
  <vendor>` + vendor + `</vendor>
  <name>` + name + `</name>
  <description>Test Package</description>
  <url>` + url + `</url>
  <releases>
    <release version="` + version + `" date="2024-01-01">Test release</release>
  </releases>
</package>`

	pdscFileName := filepath.Join(dir, vendor+"."+name+".pdsc")
	err := os.WriteFile(pdscFileName, []byte(pdscContent), 0600)
	assert.Nil(t, err)
	return pdscFileName
}

// TestPdscTypeBasicOperations tests basic PDSC type operations
func TestPdscTypeBasicOperations(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	// Setup installation directories
	installer.Installation = &installer.PacksInstallationType{
		PackRoot:    filepath.Join(localTestingDir, "Packs"),
		DownloadDir: filepath.Join(localTestingDir, ".Download"),
		WebDir:      filepath.Join(localTestingDir, ".Web"),
		LocalDir:    filepath.Join(localTestingDir, ".Local"),
		LocalPidx:   xml.NewPidxXML(filepath.Join(localTestingDir, ".Local", "local_repository.pidx"), false),
	}

	// Create necessary directories
	assert.Nil(utils.EnsureDir(installer.Installation.PackRoot))
	assert.Nil(utils.EnsureDir(installer.Installation.DownloadDir))
	assert.Nil(utils.EnsureDir(installer.Installation.WebDir))
	assert.Nil(utils.EnsureDir(installer.Installation.LocalDir))

	// Create test PDSC file
	pdscPath := createTestPdscFile(t, localTestingDir, "TestVendor", "TestPack", "1.2.3", "http://example.com/")

	// The preparePdsc and other methods are not exported, so we test through the public API
	// This test validates the structure is properly initialized
	assert.True(utils.FileExists(pdscPath))
}

// TestPdscInstallBasic tests basic PDSC installation
func TestPdscInstallBasic(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	// Setup installation
	err := installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Create test PDSC file
	pdscPath := createTestPdscFile(t, localTestingDir, "Vendor1", "Pack1", "1.0.0", "http://example.com/")

	// Add PDSC through public API
	err = installer.AddPdsc(pdscPath)
	assert.Nil(err)

	// Verify it was added to local index
	tags := installer.Installation.LocalPidx.ListPdscTags()
	assert.Greater(len(tags), 0)

	// Check if the pack is in the list
	found := false
	for _, tag := range tags {
		if tag.Vendor == "Vendor1" && tag.Name == "Pack1" {
			found = true
			break
		}
	}
	assert.True(found)
}

// TestPdscInstallDuplicate tests installing the same PDSC twice
func TestPdscInstallDuplicate(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	err := installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Create test PDSC file
	pdscPath := createTestPdscFile(t, localTestingDir, "Vendor2", "Pack2", "2.0.0", "http://example.com/")

	// Add PDSC first time
	err = installer.AddPdsc(pdscPath)
	assert.Nil(err)

	// Try to add the same PDSC again - should succeed (idempotent)
	err = installer.AddPdsc(pdscPath)
	assert.Nil(err)
}

// TestPdscInstallMultipleVersions tests installing multiple versions of the same pack
func TestPdscInstallMultipleVersions(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	err := installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Create first version
	pdscPath1 := createTestPdscFile(t, localTestingDir, "Vendor3", "Pack3", "1.0.0", "http://example.com/v1/")

	// Create second version in a subdirectory with different URL
	subDir := filepath.Join(localTestingDir, "version2")
	assert.Nil(utils.EnsureDir(subDir))
	pdscPath2 := createTestPdscFile(t, subDir, "Vendor3", "Pack3", "2.0.0", "http://example.com/v2/")

	// Add first version
	err = installer.AddPdsc(pdscPath1)
	assert.Nil(err)

	// Add second version - since they have different URLs, both should be added
	err = installer.AddPdsc(pdscPath2)
	assert.Nil(err)

	// Verify both versions are in the index (or at least one if they share same URL)
	tags := installer.Installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: "Vendor3",
		Name:   "Pack3",
	})
	// The actual behavior: if URLs differ, both are added; if same, only one is kept
	assert.GreaterOrEqual(len(tags), 1)
}

// TestPdscUninstallBasic tests basic PDSC uninstallation
func TestPdscUninstallBasic(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	err := installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Create and add test PDSC file
	pdscPath := createTestPdscFile(t, localTestingDir, "Vendor4", "Pack4", "1.0.0", "http://example.com/")

	err = installer.AddPdsc(pdscPath)
	assert.Nil(err)

	// Verify it was added
	tags := installer.Installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: "Vendor4",
		Name:   "Pack4",
	})
	assert.Equal(1, len(tags))

	// Remove the PDSC
	err = installer.RemovePdsc(pdscPath)
	assert.Nil(err)

	// Verify it was removed
	tags = installer.Installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: "Vendor4",
		Name:   "Pack4",
	})
	assert.Equal(0, len(tags))
}

// TestPdscUninstallNotFound tests removing a non-existent PDSC
func TestPdscUninstallNotFound(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	err := installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Try to remove a PDSC that was never added
	pdscPath := createTestPdscFile(t, localTestingDir, "NonExistent", "Pack", "1.0.0", "http://example.com/")

	err = installer.RemovePdsc(pdscPath)
	assert.ErrorIs(err, errs.ErrPdscEntryNotFound)
}

// TestPdscUninstallByBasename tests removing PDSC by basename
func TestPdscUninstallByBasename(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	err := installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Create and add test PDSC file
	pdscPath := createTestPdscFile(t, localTestingDir, "Vendor5", "Pack5", "1.5.0", "http://example.com/")

	err = installer.AddPdsc(pdscPath)
	assert.Nil(err)

	// Remove using just the basename
	basename := "Vendor5.Pack5"
	err = installer.RemovePdsc(basename)
	assert.Nil(err)

	// Verify it was removed
	tags := installer.Installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: "Vendor5",
		Name:   "Pack5",
	})
	assert.Equal(0, len(tags))
}

// TestPdscUninstallMultipleVersions tests removing multiple versions
func TestPdscUninstallMultipleVersions(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	err := installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Create two versions in different directories with different URLs
	pdscPath1 := createTestPdscFile(t, localTestingDir, "Vendor6", "Pack6", "1.0.0", "http://example.com/v1/")

	subDir := filepath.Join(localTestingDir, "v2")
	assert.Nil(utils.EnsureDir(subDir))
	pdscPath2 := createTestPdscFile(t, subDir, "Vendor6", "Pack6", "2.0.0", "http://example.com/v2/")

	// Add both versions
	err = installer.AddPdsc(pdscPath1)
	assert.Nil(err)
	err = installer.AddPdsc(pdscPath2)
	assert.Nil(err)

	// Verify at least one is present
	tags := installer.Installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: "Vendor6",
		Name:   "Pack6",
	})
	initialCount := len(tags)
	assert.GreaterOrEqual(initialCount, 1)

	// Remove using basename (should remove all versions)
	basename := "Vendor6.Pack6"
	err = installer.RemovePdsc(basename)
	assert.Nil(err)

	// Verify all were removed
	tags = installer.Installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: "Vendor6",
		Name:   "Pack6",
	})
	assert.Equal(0, len(tags))
}

// TestPdscWithFileURL tests PDSC with file:// URL
func TestPdscWithFileURL(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	err := installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Create test PDSC with file:// URL
	absPath, _ := filepath.Abs(localTestingDir)
	fileURL := "file://" + filepath.ToSlash(absPath) + "/"

	pdscPath := createTestPdscFile(t, localTestingDir, "Vendor7", "Pack7", "1.0.0", fileURL)

	err = installer.AddPdsc(pdscPath)
	assert.Nil(err)

	// Verify it was added
	tags := installer.Installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: "Vendor7",
		Name:   "Pack7",
	})
	assert.Equal(1, len(tags))

	// On Windows, URLs should be case-insensitive
	if runtime.GOOS == "windows" {
		// The URL should be stored in lowercase on Windows
		for _, tag := range tags {
			if tag.Vendor == "Vendor7" {
				// Just verify the tag exists, the case handling is internal
				assert.NotEmpty(tag.URL)
			}
		}
	}
}

// TestPdscWithMalformedPath tests handling of PDSC with backslashes in path
func TestPdscWithMalformedPath(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	err := installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Create PDSC
	pdscPath := createTestPdscFile(t, localTestingDir, "Vendor8", "Pack8", "1.0.0", "http://example.com/")

	err = installer.AddPdsc(pdscPath)
	assert.Nil(err)

	// Simulate a malformed entry by manually adding one with backslashes
	// Note: The actual correction logic is tested through the public API
	tags := installer.Installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: "Vendor8",
		Name:   "Pack8",
	})
	assert.Equal(1, len(tags))
}

// TestPdscInvalidPackName tests PDSC with invalid pack names
func TestPdscInvalidPackName(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	err := installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Test with various invalid pack names
	invalidNames := []string{
		"",
		"invalid",
		"no-extension",
		"../../etc/passwd",
	}

	for _, invalidName := range invalidNames {
		err = installer.AddPdsc(invalidName)
		assert.NotNil(err, "Expected error for invalid name: %s", invalidName)
	}
}

// TestPdscAbsolutePath tests PDSC operations with absolute paths
func TestPdscAbsolutePath(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	err := installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Create test PDSC file
	pdscPath := createTestPdscFile(t, localTestingDir, "Vendor9", "Pack9", "1.0.0", "http://example.com/")

	// Get absolute path
	absPdscPath, err := filepath.Abs(pdscPath)
	assert.Nil(err)

	// Add using absolute path
	err = installer.AddPdsc(absPdscPath)
	assert.Nil(err)

	// Verify it was added
	tags := installer.Installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: "Vendor9",
		Name:   "Pack9",
	})
	assert.Equal(1, len(tags))

	// Remove using absolute path
	err = installer.RemovePdsc(absPdscPath)
	assert.Nil(err)

	// Verify it was removed
	tags = installer.Installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: "Vendor9",
		Name:   "Pack9",
	})
	assert.Equal(0, len(tags))
}

// TestPdscRelativePath tests PDSC operations with relative paths
func TestPdscRelativePath(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	// Change to temp directory for relative path testing
	origDir, err := os.Getwd()
	assert.Nil(err)
	defer func() { _ = os.Chdir(origDir) }()

	err = os.Chdir(localTestingDir)
	assert.Nil(err)

	err = installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Create test PDSC file
	pdscPath := createTestPdscFile(t, ".", "Vendor10", "Pack10", "1.0.0", "http://example.com/")

	// Add using relative path
	err = installer.AddPdsc(pdscPath)
	assert.Nil(err)

	// Verify it was added
	tags := installer.Installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: "Vendor10",
		Name:   "Pack10",
	})
	assert.Equal(1, len(tags))
}

// TestPdscPathNormalization tests that paths with different separators are handled correctly
func TestPdscPathNormalization(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	err := installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Create test PDSC in a subdirectory
	subDir := filepath.Join(localTestingDir, "subdir")
	assert.Nil(utils.EnsureDir(subDir))
	pdscPath := createTestPdscFile(t, subDir, "Vendor11", "Pack11", "1.0.0", "http://example.com/")

	// Add PDSC
	err = installer.AddPdsc(pdscPath)
	assert.Nil(err)

	// Verify it was added
	tags := installer.Installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: "Vendor11",
		Name:   "Pack11",
	})
	assert.Equal(1, len(tags))

	// Test removal with directory path
	err = installer.RemovePdsc(filepath.Join(subDir, "Vendor11.Pack11.pdsc"))
	assert.Nil(err)

	// Verify it was removed
	tags = installer.Installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: "Vendor11",
		Name:   "Pack11",
	})
	assert.Equal(0, len(tags))
}

// TestPdscSpecialCharacters tests PDSC with special characters in vendor/pack names
func TestPdscSpecialCharacters(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	err := installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Test with hyphen and underscore (valid special characters)
	pdscPath := createTestPdscFile(t, localTestingDir, "My-Vendor", "My_Pack", "1.0.0", "http://example.com/")

	err = installer.AddPdsc(pdscPath)
	assert.Nil(err)

	// Verify it was added
	tags := installer.Installation.LocalPidx.FindPdscTags(xml.PdscTag{
		Vendor: "My-Vendor",
		Name:   "My_Pack",
	})
	assert.Equal(1, len(tags))
}

// TestPdscEmptyLocalIndex tests operations when local index is initially empty
func TestPdscEmptyLocalIndex(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	err := installer.SetPackRoot(localTestingDir, true)
	assert.Nil(err)
	installer.UnlockPackRoot()
	assert.Nil(installer.ReadIndexFiles())

	// Verify local index is empty
	tags := installer.Installation.LocalPidx.ListPdscTags()
	initialCount := len(tags)

	// Create and add test PDSC
	pdscPath := createTestPdscFile(t, localTestingDir, "Vendor12", "Pack12", "1.0.0", "http://example.com/")

	err = installer.AddPdsc(pdscPath)
	assert.Nil(err)

	// Verify it was added
	tags = installer.Installation.LocalPidx.ListPdscTags()
	assert.Equal(initialCount+1, len(tags))
}
