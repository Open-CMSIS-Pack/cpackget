/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

func main() {
	cmd := NewCli()
	ExitOnError(cmd.Execute())
}
