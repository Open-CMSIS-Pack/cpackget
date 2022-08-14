/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"os"
	"runtime"

	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var updateIndexCmdFlags struct {
	// sparse indicates whether the update should update all installed pack's pdscs (false) or simply update the index (true)
	sparse bool
}

var UpdateIndexCmd = &cobra.Command{
	Use:               "update-index",
	Short:             "Update the public index",
	Long:              getLongUpdateDescription(),
	PersistentPreRunE: configureInstaller,
	Args:              cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Infof("Updating public index")
		installer.UnlockPackRoot()
		err := installer.UpdatePublicIndex("", true, updateIndexCmdFlags.sparse, false, viper.GetInt("concurrent-downloads"))
		installer.LockPackRoot()
		return err
	},
}

// getLongUpdateDescription prints a "Windows friendly" long description,
// using the correct path slashes
func getLongUpdateDescription() string {
	if runtime.GOOS == "windows" {
		return `Updates the public index in ` + os.Getenv("CMSIS_PACK_ROOT") + `\.Web\index.pidx using the URL in <url> tag inside index.pidx.
By default it will also check if all PDSC files under .Web/ need update as well. This can be disabled via the "--sparse" flag.`
	} else {
		return `Updates the public index in ` + os.Getenv("CMSIS_PACK_ROOT") + `/.Web/index.pidx using the URL in <url> tag inside index.pidx.
By default it will also check if all PDSC files under .Web/ need update as well. This can be disabled via the "--sparse" flag.`
	}
}

func init() {
	UpdateIndexCmd.Flags().BoolVarP(&updateIndexCmdFlags.sparse, "sparse", "s", false, "avoid updating the pdsc files within .Web/ folder")
}
