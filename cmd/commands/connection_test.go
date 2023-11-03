/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"testing"
)

var (
	urlPath string = "https://www.keil.com"
)

var connectionCmdTests = []TestCase{
	{
		name:        "test help command",
		args:        []string{"help", "connection"},
		expectedErr: nil,
	},
	{
		name:        "test checking connection",
		args:        []string{"connection", urlPath},
		expectedErr: nil,
	},
}

func TestConnectionCmd(t *testing.T) {
	runTests(t, connectionCmdTests)
}
