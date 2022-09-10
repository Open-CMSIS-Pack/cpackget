/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var listCmdFlags struct {
	// listPublic tells whether listing all packs in the public index
	listPublic bool

	// listCached tells whether listing all cached packs
	listCached bool

	// listFilter is a set of words by which to filter listed packs
	listFilter string
}

var ListCmd = &cobra.Command{
	Use:               "list [--cached|--public]",
	Short:             "List installed packs",
	Long:              `List all installed packs and optionally cached pack files`,
	Args:              cobra.MaximumNArgs(0),
	PersistentPreRunE: configureInstaller,
	RunE: func(cmd *cobra.Command, args []string) error {
		return installer.ListInstalledPacks(listCmdFlags.listCached, listCmdFlags.listPublic, listCmdFlags.listFilter)
	},
}

func init() {
	ListCmd.Flags().BoolVarP(&listCmdFlags.listCached, "cached", "c", false, "list only cached packs")
	ListCmd.Flags().BoolVarP(&listCmdFlags.listPublic, "public", "p", false, "list packs in the public index")
	ListCmd.Flags().StringVarP(&listCmdFlags.listFilter, "filter", "f", "", "filter results (case sensitive, accepts several expressions)")

	ListCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		err := command.Flags().MarkHidden("concurrent-downloads")
		_ = command.Flags().MarkHidden("timeout")
		log.Debug(err)
		command.Parent().HelpFunc()(command, strings)
	})
}
