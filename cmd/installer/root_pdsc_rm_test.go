/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"path/filepath"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/stretchr/testify/assert"
)

func TestRemovePdsc(t *testing.T) {

	assert := assert.New(t)

	t.Run("test remove pdsc with bad name", func(t *testing.T) {
		localTestingDir := "test-remove-pdsc-with-bad-name"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot, false))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		for i := 0; i < len(malformedPackNames); i++ {
			err := installer.RemovePdsc(malformedPackNames[i])
			assert.NotNil(err)
			assert.Equal(errs.ErrBadPackName, err)
		}
	})

	t.Run("test remove a pdsc", func(t *testing.T) {
		localTestingDir := "test-remove-pdsc"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot, false))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Add it first
		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		tags := installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(1, len(tags))

		// Remove it
		err = installer.RemovePdsc(shortenPackPath(pdscPack123, true))
		assert.Nil(err)

		// Make sure there is no tags in local_repository.pidx
		tags = installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(0, len(tags))
	})

	t.Run("test remove multiple pdscs using basename PDSC file name", func(t *testing.T) {
		localTestingDir := "test-remove-multiple-pdscs-using-basename-pdsc-file-name"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot, false))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Add it first
		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		// Add a new version of the same pack
		err = installer.AddPdsc(pdscPack124)
		assert.Nil(err)

		tags := installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(2, len(tags))

		// Remove it
		err = installer.RemovePdsc(shortenPackPath(pdscPack123, true))
		assert.Nil(err)

		// Make sure there is no tags in local_repository.pidx
		tags = installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(0, len(tags))
	})

	t.Run("test remove a pdsc using full path", func(t *testing.T) {
		localTestingDir := "test-remove-pdsc-using-full-path"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot, false))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Add it first
		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		tags := installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(1, len(tags))

		// Remove it
		absPath, _ := filepath.Abs(pdscPack123)
		err = installer.RemovePdsc(absPath)
		assert.Nil(err)

		// Make sure there is no tags in local_repository.pidx
		tags = installer.Installation.LocalPidx.ListPdscTags()
		assert.Equal(0, len(tags))
	})

	t.Run("test remove one pdsc using full path and leave others untouched", func(t *testing.T) {
		localTestingDir := "test-remove-one-pdsc-using-full-path-and-leave-others-untouched"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot, false))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		// Add it first
		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		// Add a new version of the same pack
		err = installer.AddPdsc(pdscPack124)
		assert.Nil(err)

		// Remove only the first one
		absPath, _ := filepath.Abs(pdscPack123)
		err = installer.RemovePdsc(absPath)
		assert.Nil(err)

		// Make sure 1.2.4 is still present in local_repository.pidx
		tags := installer.Installation.LocalPidx.ListPdscTags()
		assert.Greater(len(tags), 0)
	})

	t.Run("test remove a pdsc that does not exist", func(t *testing.T) {
		localTestingDir := "test-remove-pdsc-that-does-not-exist"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot, false))
		installer.UnlockPackRoot()
		defer removePackRoot(localTestingDir)

		err := installer.RemovePdsc(shortenPackPath(pdscPack123, true))
		assert.Equal(errs.ErrPdscEntryNotFound, err)
	})
}
