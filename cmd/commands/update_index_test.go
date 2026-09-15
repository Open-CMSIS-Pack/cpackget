/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
)

var updateIndexServer Server
var updateIndexCmdTests = []TestCase{
	{
		name:        "test no parameter is required",
		args:        []string{"update-index", installer.PublicIndexName},
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
		expectedStdout: []string{"Updating public index", "Downloading " + installer.PublicIndexName},
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
			_ = os.WriteFile(installer.Installation.PublicIndex, []byte(indexContent), 0600)
			oldDate := time.Now().AddDate(0, 0, -2).Format("2-1-2006")
			updateCfgPath := filepath.Join(installer.Installation.WebDir, "update.cfg")
			_ = os.WriteFile(updateCfgPath, []byte("Date="+oldDate+"\nAuto=false\n"), 0600)

			updateIndexServer.AddRoute(installer.PublicIndexName, []byte(indexContent))
		},
		validationFunc: func(t *testing.T) {
			updateCfgPath := filepath.Join(installer.Installation.WebDir, "update.cfg")
			content, err := os.ReadFile(updateCfgPath)
			if err != nil {
				t.Fatal(err)
			}
			expected := "Date=" + time.Now().Format("2-1-2006") + "\nAuto=false\n"
			if string(content) != expected {
				t.Fatalf("unexpected update.cfg content: %q", content)
			}
		},
	},
	{
		name:           "test updating index",
		args:           []string{"update-index"},
		createPackRoot: true,
		expectedStdout: []string{"Updating public index", "Downloading " + installer.PublicIndexName},
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
			_ = os.WriteFile(installer.Installation.PublicIndex, []byte(indexContent), 0600)

			updateIndexServer.AddRoute(installer.PublicIndexName, []byte(indexContent))
		},
		validationFunc: func(t *testing.T) {
			updateCfgPath := filepath.Join(installer.Installation.WebDir, "update.cfg")
			content, err := os.ReadFile(updateCfgPath)
			if err != nil {
				t.Fatal(err)
			}
			expected := "Date=" + time.Now().Format("2-1-2006") + "\nAuto=true\n"
			if string(content) != expected {
				t.Fatalf("unexpected update.cfg content: %q", content)
			}
		},
	},
	{
		name:           "test updating index with insecure-skip-verify flag",
		args:           []string{"update-index", "--insecure-skip-verify"},
		createPackRoot: true,
		expectedStdout: []string{"Updating public index", "Downloading " + installer.PublicIndexName},
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
			_ = os.WriteFile(installer.Installation.PublicIndex, []byte(indexContent), 0600)

			updateIndexServer.AddRoute(installer.PublicIndexName, []byte(indexContent))
		},
	},
	{
		name:           "test failed update preserves update config",
		args:           []string{"update-index"},
		createPackRoot: true,
		expectedErr:    errs.ErrBadRequest,
		expErrUnwrap:   true,
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
			_ = os.WriteFile(installer.Installation.PublicIndex, []byte(indexContent), 0600)
			updateCfgPath := filepath.Join(installer.Installation.WebDir, "update.cfg")
			_ = os.WriteFile(updateCfgPath, []byte("Date=1-1-2000\nAuto=false\n"), 0600)
			updateIndexServer.AddRoute(installer.PublicIndexName, nil)
		},
		validationFunc: func(t *testing.T) {
			updateCfgPath := filepath.Join(installer.Installation.WebDir, "update.cfg")
			content, err := os.ReadFile(updateCfgPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != "Date=1-1-2000\nAuto=false\n" {
				t.Fatalf("failed update modified update.cfg: %q", content)
			}
		},
	},
}

func TestUpdateIndexCmd(t *testing.T) {
	updateIndexServer = NewServer()
	runTests(t, updateIndexCmdTests)
}
