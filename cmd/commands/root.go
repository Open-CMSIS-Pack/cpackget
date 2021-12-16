/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"errors"

	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// All contains all available commands for cpackget
var All = []*cobra.Command{
	PackCmd,
	PdscCmd,
	IndexCmd,
	InitCmd,
}

// createPackRoot is a flag that determines if the pack root should be created or not
var createPackRoot bool

// configureInstaller configures cpackget installer for adding or removing pack/pdsc
func configureInstaller(cmd *cobra.Command, args []string) error {
	verbosiness := viper.GetBool("verbosiness")
	quiet := viper.GetBool("quiet")
	if quiet && verbosiness {
		return errors.New("both \"-q\" and \"-v\" were specified, please pick only one verbosiness option")
	}

	log.SetLevel(log.InfoLevel)

	if quiet {
		log.SetLevel(log.ErrorLevel)
	}

	if verbosiness {
		log.SetLevel(log.DebugLevel)
	}

	return installer.SetPackRoot(viper.GetString("pack-root"), createPackRoot)
}
