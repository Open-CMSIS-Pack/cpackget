/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/spf13/cobra"
)

var connectionCmdFlags struct {
	// downloadPdscFiles forces all pdsc files from the public index to be downloaded
	downloadPdscFiles bool

	// Reports encoded progress for files and download when used by other tools
	encodedProgress bool

	// skipTouch does not touch pack.idx after adding
	skipTouch bool

	// check connection status
	checkConnection bool
}

var ConnectionCmd = &cobra.Command{
	Use:   "connection [<url>]",
	Short: "Check online connection to default or given URL",
	Long: `Checks if the given or default url is accessible
The url is optional. Ex "cpackget connection https://www.keil.com/pack"`,
	Args: cobra.MinimumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		utils.SetEncodedProgress(connectionCmdFlags.encodedProgress)
		utils.SetSkipTouch(connectionCmdFlags.skipTouch)

		var indexPath string
		if len(args) > 0 {
			indexPath = args[0]
		}
		createPackRoot = false
		var err error

		if indexPath == "" { // try to fetch from environment
			err = configureInstaller(cmd, args)
			if err != nil {
				return err
			}
		}

		indexPath, err = installer.GetIndexPath(indexPath)
		if err != nil {
			return err
		}

		err = utils.CheckConnection(indexPath, viper.GetInt("timeout"))
		return err
	},
}

func init() {
	ConnectionCmd.Flags().BoolVarP(&connectionCmdFlags.encodedProgress, "encoded-progress", "E", false, "Reports encoded progress for files and download when used by other tools")
}
