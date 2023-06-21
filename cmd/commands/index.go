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

// overwrite is a flag that tells whether or not to overwrite current CMSIS_PACK_ROOT/.Web/index.pidx
var overwrite bool

var IndexCmd = &cobra.Command{
	Deprecated:        "Consider running `cpackget update-index` instead",
	Use:               "index <index url>",
	Short:             "Updates public index",
	Long:              getLongIndexDescription(),
	PersistentPreRunE: configureInstaller,
	Args:              cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Infof("Updating index %v", args)
		indexPath := args[0]
		installer.UnlockPackRoot()
		err := installer.UpdatePublicIndex(indexPath, overwrite, true, false, false, viper.GetInt("concurrent-downloads"), viper.GetInt("timeout"))
		installer.LockPackRoot()
		return err
	},
}

// getLongIndexDescription prints a "Windows friendly" long description,
// using the correct path slashes
func getLongIndexDescription() string {
	if runtime.GOOS == "windows" {
		return `Updates the public index in ` + os.Getenv("CMSIS_PACK_ROOT") + `\.Web\index.pidx using the file specified by the given url.
If there's already an index file, cpackget won't overwrite it. Use "-f" to do so.`
	} else {
		return `Updates the public index in ` + os.Getenv("CMSIS_PACK_ROOT") + `/.Web/index.pidx using the file specified by the given url.
If there's already an index file, cpackget won't overwrite it. Use "-f" to do so.`
	}
}

func init() {
	IndexCmd.Flags().BoolVarP(&overwrite, "force", "f", false, "forces cpackget to overwrite an existing public index file")
}
