/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// overwrite is a flag that tells whether or not to overwrite current CMSIS_PACK_ROOT/.Web/index.pidx
var overwrite bool

var IndexCmd = &cobra.Command{
	Use:   "index <index url>",
	Short: "Updates public index",
	Long: `Updates the public index in CMSIS_PACK_ROOT/.Web/index.pidx using the file specified by the given url.
If there's already an index file, cpackget won't overwrite it. Use "-f" to do so.`,
	PersistentPreRunE: configureInstaller,
	Args:              cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Infof("Updating index %v", args)
		indexPath := args[0]
		err := installer.UpdatePublicIndex(indexPath, overwrite)
		if err != nil {
			log.Error(err)
		}

		return err
	},
}

func init() {
	IndexCmd.Flags().BoolVarP(&overwrite, "force", "f", false, "forces cpackget to overwrite an existing public index file")
}
