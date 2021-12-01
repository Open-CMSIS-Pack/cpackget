/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"os"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/stretchr/testify/assert"
)

func TestRemovePdsc(t *testing.T) {

	assert := assert.New(t)

	t.Run("test remove pdsc with bad name", func(t *testing.T) {
		localTestingDir := "test-remove-pdsc-with-bad-name"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		err := installer.RemovePdsc(malformedPackName)
		assert.NotNil(err)
		assert.Equal(errs.ErrBadPackName, err)
	})

	t.Run("test remove a pdsc", func(t *testing.T) {
		localTestingDir := "test-remove-pdsc"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		// Add it first
		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		// Remove it
		err = installer.RemovePdsc(shortenPackPath(pdscPack123, true))
		assert.Nil(err)
	})

	t.Run("test remove a pdsc that does not exist", func(t *testing.T) {
		localTestingDir := "test-remove-pdsc-that-does-not-exist"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		err := installer.RemovePdsc(shortenPackPath(pdscPack123, true))
		assert.Equal(errs.ErrPdscEntryNotFound, err)
	})
}
