/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// All contains all available commands for cpackget
var All = []*cobra.Command{
	PackCmd,
	PdscCmd,
}

// configureInstaller configures cpackget installer for adding or removing pack/pdsc
func configureInstaller(cmd *cobra.Command, args []string) {
	installer.SetPackRoot(viper.GetString("pack-root"))
}
