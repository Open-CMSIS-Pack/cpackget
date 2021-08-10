/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func main() {
	cmd := NewCli()
	err := cmd.Execute()
	if err != nil {
		log.Error(err.Error())
		os.Exit(-1)
	}
}

func init() {
	log.SetOutput(os.Stdout)
	// TODO: put it back to info, set to debug only when --verbosiness is specified
	log.SetLevel(log.DebugLevel)
}
