/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/open-cmsis-pack/cpackget/cmd/installer"
)

var PackCmd = &cobra.Command{
	Use:   "pack",
	Short: "add/rm Open-CMSIS-Pack packages",
	Long: "Add or remove an Open-CMSIS-Pack from a local file or a file hosted somewhere else on the Internet.",
}

var packAddCmd = &cobra.Command{
	Use:   "add <pack path>",
	Short: "Installs Open-CMSIS-Pack packages",
	Long: `Installs a pack using the file specified in "<pack path>".
The file can be a local file or a file hosted somewhere else on the Internet.
If it's hosted somewhere, cpackget will first download it. 
The process consists of extracting all pack files into "CMSIS_PACK_ROOT/<vendor>/<packName>/<packVersion>/"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Infof("Adding %v", args)
		installer.SetPackRoot(viper.GetString("pack-root"))
		for _, packPath := range args {
			if err := installer.AddPack(packPath); err != nil {
				return err
			}
		}

		return nil
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
