/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"errors"
	"path/filepath"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
)

var (
	pdscFilePath = filepath.Join(testingDir, "1.2.3", "TheVendor.PackName.pdsc")
)

var pdscCmdTests = []TestCase{
	{
		name:        "test help command",
		args:        []string{"help", "pdsc"},
		expectedErr: nil,
	},
	{
		name:           "test no parameter given",
		args:           []string{"pdsc"},
		expectedStdout: []string{"Adds or removes Open-CMSIS-Pack packages in the local file system via PDSC files"},
	},

	// Add
	{
		name:           "test adding pdsc file no args",
		args:           []string{"pdsc", "add"},
		createPackRoot: true,
		expectedErr:    errors.New("requires at least 1 arg(s), only received 0"),
	},
	{
		name:        "test help command",
		args:        []string{"help", "pdsc", "add"},
		expectedErr: nil,
	},
	{
		name:           "test adding pdsc file default mode",
		args:           []string{"pdsc", "add", pdscFilePath},
		createPackRoot: true,
		defaultMode:    true,
		expectedStdout: []string{"Adding pdsc"},
	},
	{
		name:           "test adding pdsc file default mode no preexisting index",
		args:           []string{"pdsc", "add", pdscFilePath},
		createPackRoot: false,
		defaultMode:    true,
		expectedStdout: []string{"Adding pdsc"},
	},
	{
		name:           "test adding pdsc file",
		args:           []string{"pdsc", "add", pdscFilePath},
		createPackRoot: true,
		expectedStdout: []string{"Adding pdsc"},
	},

	// Rm
	{
		name:        "test removing pdsc file no args",
		args:        []string{"pdsc", "rm"},
		expectedErr: errors.New("requires at least 1 arg(s), only received 0"),
	},
	{
		name:        "test help command",
		args:        []string{"help", "pdsc", "rm"},
		expectedErr: nil,
	},
	{
		name:           "test removing pdsc default mode",
		args:           []string{"pdsc", "rm", "TheVendor.PublicLocalPack.1.2.3"},
		createPackRoot: true,
		defaultMode:    true,
		expectedStdout: []string{"Removing pdsc"},
		expectedErr:    errs.ErrPdscEntryNotFound,
	},
	{
		name:           "test removing pdsc default mode no preexisting index",
		args:           []string{"pdsc", "rm", "TheVendor.PublicLocalPack.1.2.3"},
		createPackRoot: false,
		defaultMode:    true,
		expectedStdout: []string{"Removing pdsc"},
		expectedErr:    errs.ErrPdscEntryNotFound,
	},
	{
		name:           "test removing pdsc",
		args:           []string{"pdsc", "rm", "TheVendor.PublicLocalPack.1.2.3"},
		createPackRoot: true,
		expectedStdout: []string{"Removing pdsc"},
		expectedErr:    errs.ErrPdscEntryNotFound,
	},
}

func TestPdscCmd(t *testing.T) {
	runTests(t, pdscCmdTests)
}
