/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"path/filepath"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/stretchr/testify/assert"
)

// Tests should cover all possible scenarios for updating packs. Here are all possible ones:
// cpackget pack update Vendor.PackName                            # packID without version
// cpackget pack add Vendor.PackName.x.y.z                         # packID with version
// cpackget pack add Vendor::PackName                              # packID using legacy syntax
// cpackget pack add Vendor::PackName@x.y.z                        # packID using legacy syntax specifying an exact version
// cpackget pack add Vendor::PackName@^x.y.z                       # packID using legacy syntax specifying a minimum compatible version
// cpackget pack add Vendor::PackName@~x.y.z                       # packID using legacy syntax specifying a patch version
// cpackget pack add Vendor::PackName>=x.y.z                       # packID using legacy syntax specifying a minimum version
// cpackget pack add Vendor.PackName.x.y.z.pack                    # pack file name
// cpackget pack add https://vendor.com/Vendor.PackName.x.y.z.pack # pack URL
//
// So it doesn't really matter how the pack is specified, cpackget should
// handle is as normal.
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
			err := installer.UpdatePack(malformedPackNames[i], !CheckEula, !NoRequirements, SubCall, true, Timeout)
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
		assert.Nil(installer.UpdatePack(packPath, !CheckEula, !NoRequirements, SubCall, true, Timeout))
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
		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, !ForceReinstall, !NoRequirements, true, Timeout)
		assert.Nil(err)

		// ensure downloaded pack remains valid
		err = installer.UpdatePack(packPath, !CheckEula, !NoRequirements, SubCall, true, Timeout)
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
		err := installer.AddPack(packPath, !CheckEula, !ExtractEula, ForceReinstall, !NoRequirements, true, Timeout)
		assert.Nil(err)

		err = installer.UpdatePack(packPath, !CheckEula, !NoRequirements, SubCall, true, Timeout)
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

		err := installer.UpdatePack(packPath, !CheckEula, !NoRequirements, SubCall, true, Timeout)
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

		err := installer.UpdatePack(packPath, !CheckEula, !NoRequirements, SubCall, true, Timeout)
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

		err := installer.UpdatePack(packPath, !CheckEula, !NoRequirements, SubCall, true, Timeout)

		// Sanity check
		assert.NotNil(err)
		assert.Equal(errs.ErrBadPackURL, err)

		// Make sure pack.idx never got touched
		assert.False(utils.FileExists(installer.Installation.PackIdx))
	})

}
