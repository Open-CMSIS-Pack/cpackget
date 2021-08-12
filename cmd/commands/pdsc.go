/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/open-cmsis-pack/cpackget/cmd/installer"
)

var PdscCmd = &cobra.Command{
	Use:   "pdsc",
	Short: "Add/remove packs in the local file system via PDSC files.",
	Long: `<pack-path> can be a local file or a file hosted somewhere else on the Internet.
cpack will extract information from it and install the files in specific directories inside this machine.`,
	PersistentPreRun: configureInstaller,
}

var pdscAddCmd = &cobra.Command{
	Use:   "add </path/to/TheVendor.ThePack.x.y.z.pdsc>",
	Short: "Adds the pdsc to the local repository index",
	Long: `<pack-path> can be a local file or a file hosted somewhere else on the Internet.
cpack will extract information from it and install the files in specific directories inside this machine.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Info("Adding pdsc")
		for _, pdscPath := range args {
			if err := installer.AddPdsc(pdscPath); err != nil {
				return err
			}
		}

		return nil
	},
}

var pdscRmCmd = &cobra.Command{
	Use:   "rm <pack-name>",
	Short: "Remove ",
	Long: `<pack-name> should be in the format of "PackVendor.PackName.PackVersion".
This will remove the pack from the reference index files. If files need to be actually removed,
please use "cpackget purge <pack-name>"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Info("Removing pdsc")
		for _, pdscPath := range args {
			if err := installer.RemovePdsc(pdscPath); err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	PdscCmd.AddCommand(pdscAddCmd, pdscRmCmd)
}
