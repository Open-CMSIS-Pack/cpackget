/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/stretchr/testify/assert"
)

// TestPackTypePackID tests the PackID method which returns Vendor.Name format
func TestPackTypePackID(t *testing.T) {
	assert := assert.New(t)

	pack := &installer.PackType{}
	pack.Vendor = "TestVendor"
	pack.Name = "TestPack"

	packID := pack.PackID()
	assert.Equal("TestVendor.TestPack", packID)
}

// TestPackTypePackIDEdgeCases tests PackID with edge cases
func TestPackTypePackIDEdgeCases(t *testing.T) {
	assert := assert.New(t)

	t.Run("empty vendor and name", func(t *testing.T) {
		pack := &installer.PackType{}
		pack.Vendor = ""
		pack.Name = ""

		packID := pack.PackID()
		assert.Equal(".", packID)
	})

	t.Run("special characters in vendor and name", func(t *testing.T) {
		pack := &installer.PackType{}
		pack.Vendor = "My-Vendor"
		pack.Name = "My_Pack"

		packID := pack.PackID()
		assert.Equal("My-Vendor.My_Pack", packID)
	})
}

// TestPackTypePackIDWithVersion tests the PackIDWithVersion method
func TestPackTypePackIDWithVersion(t *testing.T) {
	assert := assert.New(t)

	pack := &installer.PackType{}
	pack.Vendor = "TestVendor"
	pack.Name = "TestPack"
	pack.Version = "1.2.3"

	packIDWithVersion := pack.PackIDWithVersion()
	assert.Equal("TestVendor.TestPack.1.2.3", packIDWithVersion)
}

// TestPackTypePackIDWithVersionMeta tests that metadata is stripped
func TestPackTypePackIDWithVersionMeta(t *testing.T) {
	assert := assert.New(t)

	pack := &installer.PackType{}
	pack.Vendor = "TestVendor"
	pack.Name = "TestPack"
	pack.Version = "1.2.3+build123"

	// PackIDWithVersion should strip metadata
	packIDWithVersion := pack.PackIDWithVersion()
	assert.Equal("TestVendor.TestPack.1.2.3", packIDWithVersion)
}

// TestPackTypePackFileName tests the PackFileName method
func TestPackTypePackFileName(t *testing.T) {
	assert := assert.New(t)

	t.Run("normal version", func(t *testing.T) {
		pack := &installer.PackType{}
		pack.Vendor = "TestVendor"
		pack.Name = "TestPack"
		pack.Version = "1.2.3"

		packFileName := pack.PackFileName()
		assert.Equal("TestVendor.TestPack.1.2.3.pack", packFileName)
	})

	t.Run("version with metadata", func(t *testing.T) {
		pack := &installer.PackType{}
		pack.Vendor = "TestVendor"
		pack.Name = "TestPack"
		pack.Version = "2.0.0+meta-info"

		// PackFileName should strip metadata
		packFileName := pack.PackFileName()
		assert.Equal("TestVendor.TestPack.2.0.0.pack", packFileName)
	})

	t.Run("version with prerelease", func(t *testing.T) {
		pack := &installer.PackType{}
		pack.Vendor = "ARM"
		pack.Name = "CMSIS"
		pack.Version = "5.8.0-rc1"

		packFileName := pack.PackFileName()
		assert.Contains(packFileName, "ARM.CMSIS")
		assert.Contains(packFileName, ".pack")
	})
}

// TestPackTypePdscFileName tests the PdscFileName method
func TestPackTypePdscFileName(t *testing.T) {
	assert := assert.New(t)

	pack := &installer.PackType{}
	pack.Vendor = "TestVendor"
	pack.Name = "TestPack"

	pdscFileName := pack.PdscFileName()
	assert.Equal("TestVendor.TestPack.pdsc", pdscFileName)
}

// TestPackTypePdscFileNameWithVersion tests the PdscFileNameWithVersion method
func TestPackTypePdscFileNameWithVersion(t *testing.T) {
	assert := assert.New(t)

	t.Run("normal version", func(t *testing.T) {
		pack := &installer.PackType{}
		pack.Vendor = "TestVendor"
		pack.Name = "TestPack"
		pack.Version = "1.2.3"

		pdscFileNameWithVersion := pack.PdscFileNameWithVersion()
		assert.Equal("TestVendor.TestPack.1.2.3.pdsc", pdscFileNameWithVersion)
	})

	t.Run("version with metadata", func(t *testing.T) {
		pack := &installer.PackType{}
		pack.Vendor = "ARM"
		pack.Name = "CMSIS"
		pack.Version = "5.8.0+git20210101"

		// PdscFileNameWithVersion should strip metadata
		pdscFileNameWithVersion := pack.PdscFileNameWithVersion()
		assert.Equal("ARM.CMSIS.5.8.0.pdsc", pdscFileNameWithVersion)
	})
}

// TestPackTypeGetVersion tests the GetVersion method
func TestPackTypeGetVersion(t *testing.T) {
	assert := assert.New(t)

	pack := &installer.PackType{}
	pack.Version = "1.2.3"

	version := pack.GetVersion()
	assert.Equal("1.2.3", version)
}

// TestPackTypeGetVersionNoMeta tests the GetVersionNoMeta method
func TestPackTypeGetVersionNoMeta(t *testing.T) {
	assert := assert.New(t)

	t.Run("version without metadata", func(t *testing.T) {
		pack := &installer.PackType{}
		pack.Version = "1.2.3"

		versionNoMeta := pack.GetVersionNoMeta()
		assert.Equal("1.2.3", versionNoMeta)
	})

	t.Run("version with metadata", func(t *testing.T) {
		pack := &installer.PackType{}
		pack.Version = "1.2.3+build123"

		versionNoMeta := pack.GetVersionNoMeta()
		assert.Equal("1.2.3", versionNoMeta)
	})

	t.Run("version with complex metadata", func(t *testing.T) {
		pack := &installer.PackType{}
		pack.Version = "2.1.0+20230101.git.abcdef"

		versionNoMeta := pack.GetVersionNoMeta()
		assert.Equal("2.1.0", versionNoMeta)
	})

	t.Run("version with prerelease info", func(t *testing.T) {
		pack := &installer.PackType{}
		pack.Version = "3.0.0-alpha.1"

		versionNoMeta := pack.GetVersionNoMeta()
		// Prerelease info is not metadata, so it should remain
		assert.Contains(versionNoMeta, "3.0.0")
	})
}

// TestPackTypeRequirementsSatisfied tests the RequirementsSatisfied method
func TestPackTypeRequirementsSatisfied(t *testing.T) {
	assert := assert.New(t)

	pack := &installer.PackType{}

	// By default, requirements should not be satisfied
	assert.False(pack.RequirementsSatisfied())
}

// TestPackTypeLockUnlock tests the Lock and Unlock methods
func TestPackTypeLockUnlock(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	// Setup installation directories
	installer.Installation = &installer.PacksInstallationType{
		PackRoot:    filepath.Join(localTestingDir, "Packs"),
		DownloadDir: filepath.Join(localTestingDir, ".Download"),
		WebDir:      filepath.Join(localTestingDir, ".Web"),
		LocalDir:    filepath.Join(localTestingDir, ".Local"),
	}

	// Create necessary directories
	assert.Nil(utils.EnsureDir(installer.Installation.PackRoot))
	assert.Nil(utils.EnsureDir(installer.Installation.DownloadDir))
	assert.Nil(utils.EnsureDir(installer.Installation.WebDir))
	assert.Nil(utils.EnsureDir(installer.Installation.LocalDir))

	pack := &installer.PackType{}
	pack.Vendor = "TestVendor"
	pack.Name = "TestPack"
	pack.Version = "1.2.3"
	pack.IsPublic = true

	// Create test files
	packHomeDir := filepath.Join(installer.Installation.PackRoot, pack.Vendor, pack.Name, pack.GetVersionNoMeta())
	assert.Nil(utils.EnsureDir(packHomeDir))
	testFile := filepath.Join(packHomeDir, "test.txt")
	assert.Nil(os.WriteFile(testFile, []byte("test"), 0644))

	packBackupPath := filepath.Join(installer.Installation.DownloadDir, pack.PackFileName())
	assert.Nil(os.WriteFile(packBackupPath, []byte("test"), 0644))

	packVersionedPdscPath := filepath.Join(installer.Installation.DownloadDir, pack.PdscFileNameWithVersion())
	assert.Nil(os.WriteFile(packVersionedPdscPath, []byte("test"), 0644))

	packPdscPath := filepath.Join(installer.Installation.WebDir, pack.PdscFileName())
	assert.Nil(os.WriteFile(packPdscPath, []byte("test"), 0644))

	// Test Lock
	pack.Lock()

	// Files should still exist after locking
	assert.True(utils.FileExists(testFile))
	assert.True(utils.FileExists(packBackupPath))
	assert.True(utils.FileExists(packVersionedPdscPath))
	assert.True(utils.FileExists(packPdscPath))

	// Test Unlock
	pack.Unlock()

	// Verify we can write to the file after unlock
	err := os.WriteFile(testFile, []byte("modified"), 0644)
	assert.Nil(err)

	// Verify the content changed
	content, err := os.ReadFile(testFile)
	assert.Nil(err)
	assert.Equal("modified", string(content))
}

// TestPackTypeLockUnlockLocalPack tests Lock/Unlock for local (non-public) packs
func TestPackTypeLockUnlockLocalPack(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	installer.Installation = &installer.PacksInstallationType{
		PackRoot:    filepath.Join(localTestingDir, "Packs"),
		DownloadDir: filepath.Join(localTestingDir, ".Download"),
		WebDir:      filepath.Join(localTestingDir, ".Web"),
		LocalDir:    filepath.Join(localTestingDir, ".Local"),
	}

	// Create necessary directories
	assert.Nil(utils.EnsureDir(installer.Installation.PackRoot))
	assert.Nil(utils.EnsureDir(installer.Installation.DownloadDir))
	assert.Nil(utils.EnsureDir(installer.Installation.WebDir))
	assert.Nil(utils.EnsureDir(installer.Installation.LocalDir))

	pack := &installer.PackType{}
	pack.Vendor = "LocalVendor"
	pack.Name = "LocalPack"
	pack.Version = "0.1.0"
	pack.IsPublic = false // Local pack

	// Create test files
	packHomeDir := filepath.Join(installer.Installation.PackRoot, pack.Vendor, pack.Name, pack.GetVersionNoMeta())
	assert.Nil(utils.EnsureDir(packHomeDir))
	testFile := filepath.Join(packHomeDir, "readme.txt")
	assert.Nil(os.WriteFile(testFile, []byte("readme"), 0644))

	packBackupPath := filepath.Join(installer.Installation.DownloadDir, pack.PackFileName())
	assert.Nil(os.WriteFile(packBackupPath, []byte("pack"), 0644))

	packVersionedPdscPath := filepath.Join(installer.Installation.DownloadDir, pack.PdscFileNameWithVersion())
	assert.Nil(os.WriteFile(packVersionedPdscPath, []byte("pdsc"), 0644))

	// For local packs, PDSC goes to .Local directory
	packPdscPath := filepath.Join(installer.Installation.LocalDir, pack.PdscFileName())
	assert.Nil(os.WriteFile(packPdscPath, []byte("local pdsc"), 0644))

	// Test Lock
	pack.Lock()

	// Files should exist
	assert.True(utils.FileExists(testFile))
	assert.True(utils.FileExists(packBackupPath))

	// Test Unlock
	pack.Unlock()

	// Verify we can write after unlock
	err := os.WriteFile(testFile, []byte("updated readme"), 0644)
	assert.Nil(err)
}

// TestPackTypeNamingConventions tests various naming method combinations
func TestPackTypeNamingConventions(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		name            string
		vendor          string
		packName        string
		version         string
		expectedPackID  string
		expectedPdscExt string
	}{
		{
			name:            "Standard pack",
			vendor:          "ARM",
			packName:        "CMSIS",
			version:         "5.8.0",
			expectedPackID:  "ARM.CMSIS",
			expectedPdscExt: ".pdsc",
		},
		{
			name:            "Pack with hyphen",
			vendor:          "ST",
			packName:        "STM32F4xx-DFP",
			version:         "2.15.0",
			expectedPackID:  "ST.STM32F4xx-DFP",
			expectedPdscExt: ".pdsc",
		},
		{
			name:            "Pack with underscore",
			vendor:          "MyVendor",
			packName:        "My_Custom_Pack",
			version:         "1.0.0",
			expectedPackID:  "MyVendor.My_Custom_Pack",
			expectedPdscExt: ".pdsc",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pack := &installer.PackType{}
			pack.Vendor = tc.vendor
			pack.Name = tc.packName
			pack.Version = tc.version

			// Test PackID
			assert.Equal(tc.expectedPackID, pack.PackID())

			// Test PdscFileName
			assert.Equal(tc.expectedPackID+tc.expectedPdscExt, pack.PdscFileName())

			// Test PackFileName contains expected parts
			packFileName := pack.PackFileName()
			assert.Contains(packFileName, tc.vendor)
			assert.Contains(packFileName, tc.packName)
			assert.Contains(packFileName, ".pack")

			// Test PdscFileNameWithVersion contains version
			pdscWithVer := pack.PdscFileNameWithVersion()
			assert.Contains(pdscWithVer, tc.vendor)
			assert.Contains(pdscWithVer, tc.packName)
			assert.Contains(pdscWithVer, ".pdsc")
		})
	}
}

// TestPackTypeMetadataHandling tests that metadata is correctly stripped across methods
func TestPackTypeMetadataHandling(t *testing.T) {
	assert := assert.New(t)

	testVersions := []struct {
		input    string
		expected string
	}{
		{"1.2.3", "1.2.3"},
		{"1.2.3+build", "1.2.3"},
		{"1.2.3+build.123", "1.2.3"},
		{"1.2.3+20230101", "1.2.3"},
		{"2.0.0+git.abcdef123", "2.0.0"},
	}

	for _, tv := range testVersions {
		t.Run("version "+tv.input, func(t *testing.T) {
			pack := &installer.PackType{}
			pack.Vendor = "TestVendor"
			pack.Name = "TestPack"
			pack.Version = tv.input

			// GetVersionNoMeta should strip metadata
			assert.Equal(tv.expected, pack.GetVersionNoMeta())

			// PackIDWithVersion should use version without metadata
			packIDVer := pack.PackIDWithVersion()
			assert.Equal("TestVendor.TestPack."+tv.expected, packIDVer)

			// PackFileName should use version without metadata
			packFileName := pack.PackFileName()
			assert.Equal("TestVendor.TestPack."+tv.expected+".pack", packFileName)

			// PdscFileNameWithVersion should use version without metadata
			pdscFileName := pack.PdscFileNameWithVersion()
			assert.Equal("TestVendor.TestPack."+tv.expected+".pdsc", pdscFileName)
		})
	}
}
