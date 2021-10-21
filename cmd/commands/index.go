/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var IndexCmd = &cobra.Command{
	Use:               "index <index url>",
	Short:             "Updates public index",
	Long:              "Updates the public index in CMSIS_PACK_ROOT/.Web/index.pidx using the file specified by the given url.",
	PersistentPreRunE: configureInstaller,
	Args:              cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Infof("Updating index %v", args)
		indexPath := args[0]
		return installer.UpdatePublicIndex(indexPath)
	},
}

func init() {
}
