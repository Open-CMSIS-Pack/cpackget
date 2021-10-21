/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var PdscCmd = &cobra.Command{
	Use:               "pdsc",
	Short:             "Adds or removes Open-CMSIS-Pack packages in the local file system via PDSC files.",
	PersistentPreRunE: configureInstaller,
}

var pdscAddCmd = &cobra.Command{
	Use:   "add <path/to/TheVendor.ThePack.x.y.z.pdsc>",
	Short: "Adds a pack via pdsc file to the local repository index",
	Long: `Adds a pack via pdsc file specified in <path/to/TheVendor.ThePack.x.y.z.pdsc>.
cpackget will add the pdsc entry to CMSIS_PACK_ROOT/.Local/local_repository.pidx.`,
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
	Short: "Removes a pack installed via pdsc file from the local repository index",
	Long: `Removes the pack referenced by the pdsc file specified in <pack-name>, e.g. "PackVendor.PackName.x.y.z".
cpackget will remove the pdsc entry from CMSIS_PACK_ROOT/.Local/local_repository.pidx."`,
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
