/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/open-cmsis-pack/cpackget/cmd/installer"
)

var updateIndexServer Server
var updateIndexCmdTests = []TestCase{
	{
		name:        "test no parameter is required",
		args:        []string{"update-index", "index.pidx"},
		expectedErr: errors.New("accepts 0 arg(s), received 1"),
	},
	{
		name:        "test help command",
		args:        []string{"help", "index"},
		expectedErr: nil,
	},
	{
		name:           "test updating index default mode",
		args:           []string{"update-index"},
		createPackRoot: true,
		defaultMode:    true,
		expectedStdout: []string{"Updating public index", "Downloading index.pidx"},
		setUpFunc: func(t *TestCase) {
			indexContent := `<?xml version="1.0" encoding="UTF-8" ?>
<index schemaVersion="1.1.0" xs:noNamespaceSchemaLocation="PackIndex.xsd" xmlns:xs="http://www.w3.org/2001/XMLSchema-instance">
<vendor>TheVendor</vendor>
<url>%s</url>
<timestamp>2021-10-17T12:21:59.1747971+00:00</timestamp>
<pindex>
  <pdsc url="http://the.vendor/" vendor="TheVendor" name="PackName" version="1.2.3" />
</pindex>
</index>`
			indexContent = fmt.Sprintf(indexContent, updateIndexServer.URL())
			_ = ioutil.WriteFile(installer.Installation.PublicIndex, []byte(indexContent), 0600)

			updateIndexServer.AddRoute("index.pidx", []byte(indexContent))
		},
	},
	{
		name:           "test updating index",
		args:           []string{"update-index"},
		createPackRoot: true,
		expectedStdout: []string{"Updating public index", "Downloading index.pidx"},
		setUpFunc: func(t *TestCase) {
			indexContent := `<?xml version="1.0" encoding="UTF-8" ?>
<index schemaVersion="1.1.0" xs:noNamespaceSchemaLocation="PackIndex.xsd" xmlns:xs="http://www.w3.org/2001/XMLSchema-instance">
<vendor>TheVendor</vendor>
<url>%s</url>
<timestamp>2021-10-17T12:21:59.1747971+00:00</timestamp>
<pindex>
  <pdsc url="http://the.vendor/" vendor="TheVendor" name="PackName" version="1.2.3" />
</pindex>
</index>`
			indexContent = fmt.Sprintf(indexContent, updateIndexServer.URL())
			_ = ioutil.WriteFile(installer.Installation.PublicIndex, []byte(indexContent), 0600)

			updateIndexServer.AddRoute("index.pidx", []byte(indexContent))
		},
	},
}

func TestUpdateIndexCmd(t *testing.T) {
	updateIndexServer = NewServer()
	runTests(t, updateIndexCmdTests)
}
