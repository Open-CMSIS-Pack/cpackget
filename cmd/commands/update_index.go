/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"os"

	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var updateIndexCmdFlags struct {
	// sparse indicates whether the update should update all installed pack's pdscs (false) or simply update the index (true)
	sparse bool
	// downloadPdscFiles forces all pdsc files from the public index to be downloaded
	downloadUpdatePdscFiles bool

	// Reports encoded progress for files and download when used by other tools
	encodedProgress bool

	// skipTouch does not touch pack.idx after adding
	skipTouch bool
}

var UpdateIndexCmd = &cobra.Command{
	Use:               "update-index",
	Short:             "Update the public index",
	Long:              getLongUpdateDescription(),
	PersistentPreRunE: configureInstaller,
	Args:              cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		utils.SetEncodedProgress(updateIndexCmdFlags.encodedProgress)
		utils.SetSkipTouch(updateIndexCmdFlags.skipTouch)
		log.Infof("Updating public index")
		installer.UnlockPackRoot()
		err := installer.UpdatePublicIndex("", true, updateIndexCmdFlags.sparse, false, updateIndexCmdFlags.downloadUpdatePdscFiles, viper.GetInt("concurrent-downloads"), viper.GetInt("timeout"))
		installer.LockPackRoot()
		return err
	},
}

func getLongUpdateDescription() string {
	return `Updates the public index in ` + os.Getenv("CMSIS_PACK_ROOT") + "/.Web/" + installer.PublicIndex + " using the URL in <url> tag inside " + installer.PublicIndex + `.
By default it will also check if all PDSC files under .Web/ need update as well. This can be disabled via the "--sparse" flag.`
}

func init() {
	UpdateIndexCmd.Flags().BoolVarP(&updateIndexCmdFlags.sparse, "sparse", "s", false, "avoid updating the pdsc files within .Web/ folder")
	UpdateIndexCmd.Flags().BoolVarP(&updateIndexCmdFlags.downloadUpdatePdscFiles, "all-pdsc-files", "a", false, "updates/downloads all the latest .pdsc files from the public index")
	UpdateIndexCmd.Flags().BoolVarP(&updateIndexCmdFlags.encodedProgress, "encoded-progress", "E", false, "Reports encoded progress for files and download when used by other tools")
	UpdateIndexCmd.Flags().BoolVar(&updateIndexCmdFlags.skipTouch, "skip-touch", false, "do not touch pack.idx")
}
