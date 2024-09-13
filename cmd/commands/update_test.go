/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"testing"
)

var (
// packFilePath1          = filepath.Join(testingDir, "TheVendor.PublicLocalPack.pack")
// fileWithPacksListed1   = "file_with_listed_packs.txt"
// fileWithNoPacksListed1 = "file_with_no_listed_packs.txt"
)

var updateCmdTests = []TestCase{
	{
		name:        "test help command",
		args:        []string{"help", "update"},
		expectedErr: nil,
	},
	/*{
		name:           "test updating pack file no args",
		args:           []string{"update"},
		createPackRoot: true,
		expectedStdout: []string{"Missing a pack-path or list with pack urls specified via -f/--packs-list-filename"},
		expectedErr:    errs.ErrIncorrectCmdArgs,
	},*/
	/*{
		name:           "test updating pack file default mode",
		args:           []string{"update", packFilePath1},
		createPackRoot: true,
		defaultMode:    true,
		expectedStdout: []string{"updating pack", filepath.Base(packFilePath1)},
	},*/
	/*{
		name:           "test updating pack file default mode no preexisting index",
		args:           []string{"update", packFilePath1},
		createPackRoot: false,
		defaultMode:    true,
		expectedStdout: []string{"updating pack", filepath.Base(packFilePath1)},
	},*/
	{
		name:           "test updating pack missing file",
		args:           []string{"update", "DoesNotExist.Pack"},
		createPackRoot: true,
		//		expectedStdout: []string{"cannot be determined"},
		//		expectedErr:    errs.ErrPackURLCannotBeFound,
	},
	/*	{
			name:           "test updating pack file",
			args:           []string{"update", packFilePath},
			createPackRoot: true,
			expectedStdout: []string{"updating pack", filepath.Base(packFilePath)},
		},
		{
			name:           "test updating packs listed in file",
			args:           []string{"update", "-f", fileWithPacksListed},
			createPackRoot: true,
			expectedStdout: []string{"Parsing packs urls via file " + fileWithPacksListed,
				"Updating pack", filepath.Base(packFilePath)},
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
			name:           "test updating empty packs list file",
			args:           []string{"update", "-f", fileWithNoPacksListed},
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
			name:           "test updating empty packs list file (but whitespace characters)",
			args:           []string{"update", "-f", fileWithNoPacksListed},
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
		},*/
}

func TestUpdateCmd(t *testing.T) {
	runTests(t, updateCmdTests)
}
