/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
)

func main() {
	cmd := NewCli()
	utils.ExitOnError(cmd.Execute())
}

func init() {
	log.SetOutput(os.Stdout)
	// TODO: put it back to info, set to debug only when --verbosiness is specified
	log.SetLevel(log.DebugLevel)
}
