/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"errors"
	"fmt"
	"os"

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
}

// configureInstaller configures cpackget installer for adding or removing pack/pdsc
func configureInstaller(cmd *cobra.Command, args []string) error {
	log.SetOutput(os.Stdout)

	logLevels := []log.Level{log.ErrorLevel, log.InfoLevel, log.DebugLevel}
	maxVerbosiness := len(logLevels) - 1
	verbosiness := viper.GetInt("verbosiness")
	if verbosiness > maxVerbosiness {
		errorMessage := fmt.Sprintf("Max verbosiness count is %v", maxVerbosiness)
		return errors.New(errorMessage)
	}
	log.SetLevel(logLevels[verbosiness])

	return installer.SetPackRoot(viper.GetString("pack-root"))
}
