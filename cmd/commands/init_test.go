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

// Tests for init command are placed here because there was something wrong
// while putting them into a file init_test.go

var (
	pidxFilePath         = filepath.Join(testingDir, "SamplePublicIndex.pidx")
	notFoundPidxFilePath = filepath.Join("path", "to", "index.pidx")
)

var initCmdTests = []TestCase{
	{
		name:        "test no parameter given",
		args:        []string{"init"},
		expectedErr: errors.New("accepts 1 arg(s), received 0"),
	},
	{
		name:        "test help command",
		args:        []string{"help", "init"},
		expectedErr: nil,
	},
	{
		name: "test create using an index.pidx",
		args: []string{"init"},
		setUpFunc: func(t *TestCase) {
			server := NewServer()
			t.args = append(t.args, server.URL()+"index.pidx")
			server.AddRoute("index.pidx", []byte(`<?xml version="1.0" encoding="UTF-8" ?>
<index schemaVersion="1.1.0" xs:noNamespaceSchemaLocation="PackIndex.xsd" xmlns:xs="https://www.w3.org/2001/XMLSchema-instance">
<vendor>TheVendor</vendor>
<url>https://the.vendor/</url>
<timestamp>2021-10-17T12:21:59.1747971+00:00</timestamp>
<pindex>
  <pdsc url="https://the.vendor/" vendor="TheVendor" name="PackName" version="1.2.3" />
</pindex>
</index>`))
		},
	},
	{
		name:           "test create using local index.pidx",
		args:           []string{"init", pidxFilePath},
		createPackRoot: true,
	},
	{
		name:           "test create using local index.pidx that do not exist",
		args:           []string{"init", notFoundPidxFilePath},
		createPackRoot: true,
		expectedErr:    errs.ErrFileNotFoundUseInit,
	},
	{
		name:           "test create using directory as path",
		args:           []string{"init", "foo/"},
		createPackRoot: true,
		expectedErr:    errs.ErrInvalidPublicIndexReference,
		setUpFunc: func(t *TestCase) {
			t.assert.Nil(os.Mkdir("foo/", 0777))
		},
		tearDownFunc: func() {
			os.Remove("foo/")
		},
	},
}

func TestInitCmd(t *testing.T) {
	runTests(t, initCmdTests)
}
