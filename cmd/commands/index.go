/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var IndexCmd = &cobra.Command{
	Use:               "index <index-path>",
	Short:             "Add public index",
	Long:              "Add/overwrite the public index using the index path specified, it can be an local file or a URL",
	PersistentPreRunE: configureInstaller,
	Args:              cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Infof("Changing index %v", args)
		indexPath := args[0]
		return installer.UpdatePublicIndex(indexPath)
	},
}

func init() {
}
