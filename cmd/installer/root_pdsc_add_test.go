/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package installer_test

import (
	"os"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
	"github.com/stretchr/testify/assert"
)

func TestAddPdsc(t *testing.T) {

	assert := assert.New(t)

	// Sanity tests
	t.Run("test add pdsc with bad name", func(t *testing.T) {
		localTestingDir := "test-add-pdsc-with-bad-name"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		err := installer.AddPdsc(malformedPackName)
		assert.Equal(errs.ErrBadPackName, err)
	})

	t.Run("test add pdsc with bad local_repository.pidx", func(t *testing.T) {
		localTestingDir := "test-add-pdsc-with-bad-local-repository"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.Installation.LocalPidx = xml.NewPidxXML(badLocalRepositoryPidx)
		defer os.RemoveAll(localTestingDir)

		err := installer.AddPdsc(pdscPack123)
		assert.NotNil(err)
		assert.Equal("XML syntax error on line 3: unexpected EOF", err.Error())
	})

	t.Run("test add a pdsc", func(t *testing.T) {
		localTestingDir := "test-add-a-pdsc"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		err := installer.AddPdsc(pdscPack123)

		// Sanity check
		assert.Nil(err)
	})

	t.Run("test add a pdsc already installed", func(t *testing.T) {
		localTestingDir := "test-add-a-pdsc-already-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		err = installer.AddPdsc(pdscPack123)
		assert.Equal(errs.ErrPdscEntryExists, err)
	})

	t.Run("test add new pdsc version", func(t *testing.T) {
		localTestingDir := "test-add-new-pdsc-version"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		defer os.RemoveAll(localTestingDir)

		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		err = installer.AddPdsc(pdscPack124)
		assert.Nil(err)
	})
}
