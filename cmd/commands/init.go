/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var InitCmd = &cobra.Command{
	Use:   "init [--pack-root <pack root>] <index-url>",
	Short: "Initializes a pack root folder",
	Long: `Initializes a pack root folder specified by -R/--pack-root command line
or via the CMSIS_PACK_ROOT environment variable with the following contents:
  - .Download/
  - .Local/
  - .Web/
  - .Web/index.pidx (downloaded from <index-url>)
The index-url is mandatory. Ex "cpackget init --pack-root path/to/mypackroot https://www.keil.com/pack/index.pidx"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		packRoot := viper.GetString("pack-root")
		indexPath := args[0]

		log.Debugf("Initializing a new pack root in \"%v\" using index url \"%v\"", packRoot, indexPath)

		createPackRoot = true
		err := configureInstaller(cmd, args)
		if err != nil {
			return err
		}

		return installer.UpdatePublicIndex(indexPath, true, true)
	},
}
