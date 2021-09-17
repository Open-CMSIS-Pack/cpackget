/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var PackCmd = &cobra.Command{
	Use:               "pack",
	Short:             "add/rm Open-CMSIS-Pack packages",
	Long:              "Add or remove an Open-CMSIS-Pack from a local file or a file hosted somewhere else on the Internet.",
	PersistentPreRunE: configureInstaller,
}

// skipEula tells whether pack's license should be presented to the user or not for a yay-or-nay acceptance
var skipEula bool

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
		for _, packPath := range args {
			if err := installer.AddPack(packPath, !skipEula); err != nil {
				return err
			}
		}

		return nil
	},
}

// purge stores the value of "--purge" flag for the "pack rm" command
var purge bool

var packRmCmd = &cobra.Command{
	Use:   "rm <pack reference>",
	Short: "Uninstalls Open-CMSIS-Pack packages",
	Long: `Uninstalls a pack using the reference "PackVendor.PackName[.x.y.z]",
where the version "x.y.z" is optional. This will remove
the pack from the reference index files. If files need
to be actually removed, please use "--purge".`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Infof("Removing %v", args)
		for _, packPath := range args {
			if err := installer.RemovePack(packPath, purge); err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	packRmCmd.Flags().BoolVarP(&purge, "purge", "p", false, "forces deletion of cached pack files")
	packAddCmd.Flags().BoolVarP(&skipEula, "agree-embedded-license", "a", false, "agree with the embedded license of the pack")
	PackCmd.AddCommand(packAddCmd, packRmCmd)
}
