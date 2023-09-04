/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"os"
	"time"

	"github.com/open-cmsis-pack/cpackget/cmd/commands"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(new(LogFormatter))
	log.SetOutput(os.Stdout)

	utils.StartSignalWatcher()
	start := time.Now()

	commands.Version = version
	commands.CopyRight = copyRight
	cmd := commands.NewCli()
	err := cmd.Execute()
	if err != nil {
		if !errs.AlreadyLogged(err) {
			log.Error(err)
		}
		os.Exit(-1)
	}

	log.Debugf("took %v", time.Since(start))
	utils.StopSignalWatcher()
}
