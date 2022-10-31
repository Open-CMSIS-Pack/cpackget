/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
)

var (
	packFilePath        = filepath.Join(testingDir, "1.2.3", "TheVendor.PublicLocalPack.1.2.3.pack")
	fileWithPacksListed = "file_with_listed_packs.txt"
)

var packCmdTests = []TestCase{
	{
		name:           "test no parameter given",
		args:           []string{"pack"},
		expectedStdout: []string{"Adds/Removes Open-CMSIS-Pack packages from a local file or a file hosted somewhere else on the Internet"},
	},

	// Add
	{
		name:        "test help command",
		args:        []string{"help", "pack", "add"},
		expectedErr: nil,
	},
	{
		name:           "test adding pack file no args",
		args:           []string{"pack", "add"},
		createPackRoot: true,
		expectedStdout: []string{"Missing a pack-path or list with pack urls specified via -f/--packs-list-filename"},
		expectedErr:    errs.ErrIncorrectCmdArgs,
	},
	{
		name:           "test adding pack missing file",
		args:           []string{"pack", "add", "DoesNotExist.Pack.1.2.3.pack"},
		createPackRoot: true,
		expectedStdout: []string{"File", "DoesNotExist.Pack.1.2.3.pack", "does't exist"},
		expectedErr:    errs.ErrAlreadyLogged,
	},
	{
		name:           "test adding pack file default mode",
		args:           []string{"pack", "add", packFilePath},
		createPackRoot: true,
		defaultMode:    true,
		expectedStdout: []string{"Adding pack", filepath.Base(packFilePath)},
	},
	{
		name:           "test adding pack file default mode no preexisting index",
		args:           []string{"pack", "add", packFilePath},
		createPackRoot: false,
		defaultMode:    true,
		expectedStdout: []string{"Adding pack", filepath.Base(packFilePath)},
	},
	{
		name:           "test adding pack file",
		args:           []string{"pack", "add", packFilePath},
		createPackRoot: true,
		expectedStdout: []string{"Adding pack", filepath.Base(packFilePath)},
	},
	{
		name:           "test adding packs listed in file",
		args:           []string{"pack", "add", "-f", fileWithPacksListed},
		createPackRoot: true,
		expectedStdout: []string{"Parsing packs urls via file " + fileWithPacksListed,
			"Adding pack", filepath.Base(packFilePath)},
		setUpFunc: func(t *TestCase) {
			f, _ := os.Create(fileWithPacksListed)
			_, _ = f.WriteString(packFilePath)
			f.Close()
		},
		tearDownFunc: func() {
			os.Remove(fileWithPacksListed)
		},
	},

	// Rm
	{
		name:        "test removing pack file no args",
		args:        []string{"pack", "rm"},
		expectedErr: errors.New("requires at least 1 arg(s), only received 0"),
	},
	{
		name:        "test help command",
		args:        []string{"help", "pack", "rm"},
		expectedErr: nil,
	},
	{
		name:           "test removing pack default mode",
		args:           []string{"pack", "rm", "TheVendor.PublicLocalPack.1.2.3"},
		createPackRoot: true,
		defaultMode:    true,
		expectedStdout: []string{"Removing [TheVendor.PublicLocalPack.1.2.3]"},
		expectedErr:    errs.ErrPackNotInstalled,
	},
	{
		name:           "test removing pack default mode no preexisting index",
		args:           []string{"pack", "rm", "TheVendor.PublicLocalPack.1.2.3"},
		createPackRoot: false,
		defaultMode:    true,
		expectedStdout: []string{"Removing [TheVendor.PublicLocalPack.1.2.3]"},
		expectedErr:    errs.ErrPackNotInstalled,
	},
	{
		name:           "test removing pack",
		args:           []string{"pack", "rm", "TheVendor.PublicLocalPack.1.2.3"},
		createPackRoot: true,
		expectedStdout: []string{"Removing [TheVendor.PublicLocalPack.1.2.3]"},
		expectedErr:    errs.ErrPackNotInstalled,
	},

	// List
	{
		name:        "test help command",
		args:        []string{"help", "pack", "list"},
		expectedErr: nil,
	},
	{
		name:           "test listing installed packs default mode",
		args:           []string{"pack", "list"},
		createPackRoot: true,
		defaultMode:    true,
		expectedStdout: []string{"Listing installed packs"},
	},
	{
		name:           "test listing installed packs default mode no preexisting index",
		args:           []string{"pack", "list"},
		createPackRoot: false,
		defaultMode:    true,
		expectedStdout: []string{"Listing installed packs"},
	},
	{
		name:           "test listing installed packs",
		args:           []string{"pack", "list"},
		createPackRoot: true,
		expectedStdout: []string{"Listing installed packs"},
	},
}

func TestPackCmd(t *testing.T) {
	runTests(t, packCmdTests)
}
