/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/xml"
)

var listCmdTests = []TestCase{
	{
		name:        "test help command",
		args:        []string{"help", "list"},
		expectedErr: nil,
	},
	{
		name:           "test listing installed packs default mode",
		args:           []string{"list"},
		createPackRoot: true,
		defaultMode:    true,
		expectedStdout: []string{"Vendor::Pack@1.2.3", "Vendor::PackInstalledViaPdsc@1.2.3"},
		setUpFunc: func(t *TestCase) {
			packRoot := os.Getenv("CMSIS_PACK_ROOT")
			packFolder := filepath.Join(packRoot, "Vendor", "Pack", "1.2.3")
			t.assert.Nil(os.MkdirAll(packFolder, 0700))
			t.assert.Nil(os.WriteFile(filepath.Join(packFolder, "Vendor.Pack.pdsc"), []byte(""), 0600))
			localRepository := installer.Installation.LocalPidx
			t.assert.Nil(localRepository.Read())
			t.assert.Nil(localRepository.AddPdsc(xml.PdscTag{Vendor: "Vendor", Name: "PackInstalledViaPdsc", Version: "1.2.3"}))
			t.assert.Nil(localRepository.Write())
		},
	},
	{
		name:           "test listing installed packs",
		args:           []string{"list"},
		createPackRoot: true,
		expectedStdout: []string{"Vendor::Pack@1.2.3", "Vendor::PackInstalledViaPdsc@1.2.3"},
		setUpFunc: func(t *TestCase) {
			packRoot := os.Getenv("CMSIS_PACK_ROOT")
			packFolder := filepath.Join(packRoot, "Vendor", "Pack", "1.2.3")
			t.assert.Nil(os.MkdirAll(packFolder, 0700))
			t.assert.Nil(os.WriteFile(filepath.Join(packFolder, "Vendor.Pack.pdsc"), []byte(""), 0600))
			localRepository := installer.Installation.LocalPidx
			t.assert.Nil(localRepository.Read())
			t.assert.Nil(localRepository.AddPdsc(xml.PdscTag{Vendor: "Vendor", Name: "PackInstalledViaPdsc", Version: "1.2.3"}))
			t.assert.Nil(localRepository.Write())
		},
	},
}

func TestListCmd(t *testing.T) {
	runTests(t, listCmdTests)
}
