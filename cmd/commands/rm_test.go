/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
)

var rmCmdTests = []TestCase{
	{
		name:           "test removing pack no args",
		args:           []string{"rm"},
		createPackRoot: true,
		expectedErr:    errors.New("requires at least 1 arg(s), only received 0"),
	},
	{
		name:        "test help command",
		args:        []string{"help", "rm"},
		expectedErr: nil,
	},
	{
		name:           "test removing pack that does not exist",
		args:           []string{"rm", "DoesNotExist.Pack.1.2.3"},
		createPackRoot: true,
		expectedStdout: []string{"Removing [DoesNotExist.Pack.1.2.3]", "pack not installed"},
		expectedErr:    errs.ErrAlreadyLogged,
	},
	{
		name:           "test removing pack default mode",
		args:           []string{"rm", "Vendor.Pack.1.2.3", "Vendor.PackInstalledViaPdsc.pdsc"},
		createPackRoot: true,
		defaultMode:    true,
		expectedStdout: []string{"Removing [Vendor.Pack.1.2.3 Vendor.PackInstalledViaPdsc.pdsc]"},
		setUpFunc: func(t *TestCase) {
			packRoot := os.Getenv("CMSIS_PACK_ROOT")
			packFolder := filepath.Join(packRoot, "Vendor", "Pack", "1.2.3")
			t.assert.Nil(os.MkdirAll(packFolder, 0700))
			t.assert.Nil(os.WriteFile(filepath.Join(packFolder, "Vendor.Pack.pdsc"), []byte(""), 0600))
			t.assert.Nil(os.WriteFile(filepath.Join(packRoot, ".Local", "Vendor.Pack.pdsc"), []byte(""), 0600))
			localRepository := installer.Installation.LocalPidx
			t.assert.Nil(localRepository.Read())
			t.assert.Nil(localRepository.AddPdsc(xml.PdscTag{Vendor: "Vendor", Name: "PackInstalledViaPdsc", Version: "1.2.3"}))
			t.assert.Nil(localRepository.Write())
		},
	},
	{
		name:           "test removing pack",
		args:           []string{"rm", "Vendor.Pack.1.2.3", "Vendor.PackInstalledViaPdsc.pdsc"},
		createPackRoot: true,
		expectedStdout: []string{"Removing [Vendor.Pack.1.2.3 Vendor.PackInstalledViaPdsc.pdsc]"},
		setUpFunc: func(t *TestCase) {
			packRoot := os.Getenv("CMSIS_PACK_ROOT")
			packFolder := filepath.Join(packRoot, "Vendor", "Pack", "1.2.3")
			t.assert.Nil(os.MkdirAll(packFolder, 0700))
			t.assert.Nil(os.WriteFile(filepath.Join(packFolder, "Vendor.Pack.pdsc"), []byte(""), 0600))
			t.assert.Nil(os.WriteFile(filepath.Join(packRoot, ".Local", "Vendor.Pack.pdsc"), []byte(""), 0600))
			localRepository := installer.Installation.LocalPidx
			t.assert.Nil(localRepository.Read())
			t.assert.Nil(localRepository.AddPdsc(xml.PdscTag{Vendor: "Vendor", Name: "PackInstalledViaPdsc", Version: "1.2.3"}))
			t.assert.Nil(localRepository.Write())
		},
	},
}

func TestRmCmd(t *testing.T) {
	runTests(t, rmCmdTests)
}
