/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var PackCmd = &cobra.Command{
	Use:   "pack",
	Short: "Download and install Open-CMSIS-Pack packages",
	Long: `<pack-path> can be a local file or a file hosted somewhere else on the Internet.
cpack will extract information from it and install the files in specific directories inside this machine.`,
	Args: cobra.MinimumNArgs(1),
}

var packAddCmd = &cobra.Command{
	Use:   "add <pack-path>",
	Short: "Download and install Open-CMSIS-Pack packages",
	Long: `<pack-path> can be a local file or a file hosted somewhere else on the Internet.
cpack will extract information from it and install the files in specific directories inside this machine.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Infof("Adding %v", args)
	},
}

var packRmCmd = &cobra.Command{
	Use:   "rm <pack-name>",
	Short: "Uninstall Open-CMSIS-Pack packages",
	Long: `<pack-name> should be in the format of "PackVendor.PackName.PackVersion".
This will remove the pack from the reference index files. If files need to be actually removed,
please use "cpackget purge <pack-name>"`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Infof("Removing %v", args)
	},
}

func init() {
	PackCmd.AddCommand(packAddCmd, packRmCmd)
}
