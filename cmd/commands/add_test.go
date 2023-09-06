/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"os"
	"path/filepath"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
)

var (
	packFilePath          = filepath.Join(testingDir, "1.2.3", "TheVendor.PublicLocalPack.1.2.3.pack")
	fileWithPacksListed   = "file_with_listed_packs.txt"
	fileWithNoPacksListed = "file_with_no_listed_packs.txt"
	pdscFilePath          = filepath.Join(testingDir, "1.2.3", "TheVendor.PackName.pdsc")
)

var addCmdTests = []TestCase{
	{
		name:        "test help command",
		args:        []string{"help", "add"},
		expectedErr: nil,
	},
	{
		name:           "test adding pack file no args",
		args:           []string{"add"},
		createPackRoot: true,
		expectedStdout: []string{"Missing a pack-path or list with pack urls specified via -f/--packs-list-filename"},
		expectedErr:    errs.ErrIncorrectCmdArgs,
	},
	{
		name:           "test adding pack file default mode",
		args:           []string{"add", packFilePath},
		createPackRoot: true,
		defaultMode:    true,
		expectedStdout: []string{"Adding pack", filepath.Base(packFilePath)},
	},
	{
		name:           "test adding pack file default mode no preexisting index",
		args:           []string{"add", packFilePath},
		createPackRoot: false,
		defaultMode:    true,
		expectedStdout: []string{"Adding pack", filepath.Base(packFilePath)},
	},
	{
		name:           "test adding pack missing file",
		args:           []string{"add", "DoesNotExist.Pack.1.2.3.pack"},
		createPackRoot: true,
		expectedStdout: []string{"File", "DoesNotExist.Pack.1.2.3.pack", "doesn't exist"},
		expectedErr:    errs.ErrFileNotFound,
	},
	{
		name:           "test adding pack file",
		args:           []string{"add", packFilePath},
		createPackRoot: true,
		expectedStdout: []string{"Adding pack", filepath.Base(packFilePath)},
	},
	{
		name:           "test adding pack via pdsc file",
		args:           []string{"add", pdscFilePath},
		createPackRoot: true,
		expectedStdout: []string{"Adding pdsc", filepath.Base(pdscFilePath)},
	},
	{
		name:           "test adding packs listed in file",
		args:           []string{"add", "-f", fileWithPacksListed},
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
	{
		name:           "test adding empty packs list file",
		args:           []string{"add", "-f", fileWithNoPacksListed},
		createPackRoot: true,
		expectedStdout: []string{"Parsing packs urls via file " + fileWithNoPacksListed},
		expectedErr:    nil,
		setUpFunc: func(t *TestCase) {
			f, _ := os.Create(fileWithNoPacksListed)
			_, _ = f.WriteString("")
			f.Close()
		},
		tearDownFunc: func() {
			os.Remove(fileWithNoPacksListed)
		},
	},
	{
		name:           "test adding empty packs list file (but whitespace characters)",
		args:           []string{"add", "-f", fileWithNoPacksListed},
		createPackRoot: true,
		expectedStdout: []string{"Parsing packs urls via file " + fileWithNoPacksListed},
		expectedErr:    nil,
		setUpFunc: func(t *TestCase) {
			f, _ := os.Create(fileWithNoPacksListed)
			_, _ = f.WriteString("  \n  \t  \n")
			f.Close()
		},
		tearDownFunc: func() {
			os.Remove(fileWithNoPacksListed)
		},
	},
}

func TestAddCmd(t *testing.T) {
	runTests(t, addCmdTests)
}
