/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"os"
)

func main() {
	cmd := NewCli()
	err := cmd.Execute()
	if err != nil {
		os.Exit(-1)
	}
}
