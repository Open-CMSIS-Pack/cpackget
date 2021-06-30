/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	log "github.com/sirupsen/logrus"
)

func main() {
	cmd := NewCli()
	ExitOnError(cmd.Execute())
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.WarnLevel)
}
