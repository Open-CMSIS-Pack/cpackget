/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var PdscCmd = &cobra.Command{
	Use:   "pdsc",
	Short: "Add/remove packs in the local file system via PDSC files.",
	Long: `<pack-path> can be a local file or a file hosted somewhere else on the Internet.
cpack will extract information from it and install the files in specific directories inside this machine.`,
}

var pdscAddCmd = &cobra.Command{
	Use:   "add </path/to/TheVendor.ThePack.x.y.z.pdsc>",
	Short: "Adds the pdsc to the local repository index",
	Long: `<pack-path> can be a local file or a file hosted somewhere else on the Internet.
cpack will extract information from it and install the files in specific directories inside this machine.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Adding pdsc")
	},
}

var pdscRmCmd = &cobra.Command{
	Use:   "rm <pack-name>",
	Short: "Remove ",
	Long: `<pack-name> should be in the format of "PackVendor.PackName.PackVersion".
This will remove the pack from the reference index files. If files need to be actually removed,
please use "cpackget purge <pack-name>"`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Removing pdsc")
	},
}

func init() {
	PdscCmd.AddCommand(pdscAddCmd, pdscRmCmd)
}
