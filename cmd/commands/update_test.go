/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"os"
	"path/filepath"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
)

var updateCmdTests = []TestCase{
	{
		name:        "test help command",
		args:        []string{"help", "update"},
		expectedErr: nil,
	},
	{
		name:           "test updating pack file no args",
		args:           []string{"update"},
		createPackRoot: true,
		expectedErr:    nil,
	},
	{
		name:           "test updating pack missing file",
		args:           []string{"update", "DoesNotExist.Pack"},
		createPackRoot: true,
		expectedStdout: []string{"is not installed"},
	},
	{
		name:           "test updating packs listed in file",
		args:           []string{"update", "-f", fileWithPacksListed},
		createPackRoot: true,
		expectedStdout: []string{"Parsing packs urls via file " + fileWithPacksListed,
			"is not installed", filepath.Base("DoesNotExist.Pack")},
		setUpFunc: func(t *TestCase) {
			f, _ := os.Create(fileWithPacksListed)
			_, _ = f.WriteString("DoesNotExist.Pack")
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
		name:           "test apdating empty packs list file (but whitespace characters)",
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
	},
	{
		name:           "test updating packs listed in missing file",
		args:           []string{"update", "-f", fileWithPacksListed},
		createPackRoot: true,
		expectedErr:    errs.ErrFileNotFound,
		expectedStdout: []string{"Parsing packs urls via file " + fileWithPacksListed},
	},
}

func TestUpdateCmd(t *testing.T) {
	runTests(t, updateCmdTests)
}
