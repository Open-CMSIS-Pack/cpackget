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

func TestAddPdsc(t *testing.T) {

	assert := assert.New(t)

	// Sanity tests
	t.Run("test add pdsc with bad name", func(t *testing.T) {
		localTestingDir := "test-add-pdsc-with-bad-name"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		for i := 0; i < len(malformedPackNames); i++ {
			err := installer.AddPdsc(malformedPackNames[i])
			assert.Equal(errs.ErrBadPackName, err)
		}
	})

	t.Run("test add pdsc with bad local_repository.pidx", func(t *testing.T) {
		localTestingDir := "test-add-pdsc-with-bad-local-repository"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		installer.Installation.LocalPidx = xml.NewPidxXML(badLocalRepositoryPidx)
		defer removePackRoot(localTestingDir)

		err := installer.AddPdsc(pdscPack123)
		assert.NotNil(err)
		assert.Equal("XML syntax error on line 3: unexpected EOF", err.Error())
	})

	t.Run("test add a pdsc", func(t *testing.T) {
		localTestingDir := "test-add-a-pdsc"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		err := installer.AddPdsc(pdscPack123)

		// Sanity check
		assert.Nil(err)
	})

	t.Run("test add a pdsc already installed", func(t *testing.T) {
		localTestingDir := "test-add-a-pdsc-already-installed"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		err = installer.AddPdsc(pdscPack123)
		assert.Nil(err)
	})

	t.Run("test add new pdsc version", func(t *testing.T) {
		localTestingDir := "test-add-new-pdsc-version"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		err := installer.AddPdsc(pdscPack123)
		assert.Nil(err)

		err = installer.AddPdsc(pdscPack124)
		assert.Nil(err)
	})

	t.Run("test add new pdsc version with same path", func(t *testing.T) {
		localTestingDir := "test-add-new-pdsc-version-same-path"
		assert.Nil(installer.SetPackRoot(localTestingDir, CreatePackRoot))
		installer.UnlockPackRoot()
		assert.Nil(installer.ReadIndexFiles())
		defer removePackRoot(localTestingDir)

		// Work on a local copy of the file
		pdscFileName := filepath.Base(pdscPack123)
		assert.Nil(utils.CopyFile(pdscPack123, pdscFileName))
		defer os.Remove(pdscFileName)

		err := installer.AddPdsc(pdscFileName)
		assert.Nil(err)

		// Update the version in PDSC file
		pdscXML := xml.NewPdscXML(pdscFileName)
		assert.Nil(pdscXML.Read())
		releaseTag := xml.ReleaseTag{Version: "1.2.4"}
		pdscXML.ReleasesTag.Releases = append([]xml.ReleaseTag{releaseTag}, pdscXML.ReleasesTag.Releases...)
		assert.Nil(utils.WriteXML(pdscFileName, pdscXML))

		// Attempt to add it the second time
		err = installer.AddPdsc(pdscPack124)
		assert.Nil(err)
	})
}
