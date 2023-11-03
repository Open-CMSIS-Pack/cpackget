/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"errors"
	"os"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
)

var connectionCmdTests = []TestCase{
	{
		name:        "test no parameter given",
		args:        []string{"connection"},
		expectedErr: errors.New("accepts 1 arg(s), received 0"),
	},
	{
		name:        "test help command",
		args:        []string{"help", "connection"},
		expectedErr: nil,
	},
	{
		name: "test create using an index.pidx",
		args: []string{"connection"},
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
		args:           []string{"connection", pidxFilePath},
		createPackRoot: true,
	},
	{
		name:           "test create using local index.pidx that do not exist",
		args:           []string{"connection", notFoundPidxFilePath},
		createPackRoot: true,
		expectedErr:    errs.ErrFileNotFound,
	},
	{
		name:           "test create using directory as path",
		args:           []string{"connection", "foo/"},
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

func TestConnectionCmd(t *testing.T) {
	runTests(t, connectionCmdTests)
}
