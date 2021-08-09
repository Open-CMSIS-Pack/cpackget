/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

func main() {
	cmd := NewCli()
	ExitOnError(cmd.Execute())
}

func init() {
	log.SetOutput(os.Stdout)
	// TODO: put it back to info, set to debug only when --verbosiness is specified
	log.SetLevel(log.DebugLevel)
}
