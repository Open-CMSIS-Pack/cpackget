/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var listCmdFlags struct {
	// listUpdates tells whether listing all packs for which updates exist
	listUpdates bool

	// listPublic tells whether listing all packs in the public index
	listPublic bool

	// listCached tells whether listing all cached packs
	listCached bool

	// listFilter is a set of words by which to filter listed packs
	listFilter string
}

var ListCmd = &cobra.Command{
	Use:               "list [--cached|--public|--updates]",
	Short:             "List installed packs",
	Long:              "List all installed packs and optionally cached packs or those for which updates are available",
	Args:              cobra.MaximumNArgs(0),
	PersistentPreRunE: configureInstaller,
	RunE: func(cmd *cobra.Command, args []string) error {
		return installer.ListInstalledPacks(listCmdFlags.listCached, listCmdFlags.listPublic, listCmdFlags.listUpdates, false, false, listCmdFlags.listFilter)
	},
}

var listRequiredCmd = &cobra.Command{
	Use:               "required",
	Short:             "List dependencies of installed packs",
	Long:              "List dependencies of all installed packs, public and local",
	Args:              cobra.MaximumNArgs(0),
	PersistentPreRunE: configureInstaller,
	RunE: func(cmd *cobra.Command, args []string) error {
		return installer.ListInstalledPacks(listCmdFlags.listCached, listCmdFlags.listPublic, listCmdFlags.listUpdates, true, false, listCmdFlags.listFilter)
	},
}

func init() {
	ListCmd.Flags().BoolVarP(&listCmdFlags.listCached, "cached", "c", false, "list only cached packs")
	ListCmd.Flags().BoolVarP(&listCmdFlags.listPublic, "public", "p", false, "list packs in the public index")
	ListCmd.Flags().BoolVarP(&listCmdFlags.listUpdates, "updates", "u", false, "list packs which have newer versions")
	ListCmd.Flags().StringVarP(&listCmdFlags.listFilter, "filter", "f", "", "filter results (case sensitive, accepts several expressions)")
	ListCmd.AddCommand(listRequiredCmd)

	listRequiredCmd.SetHelpFunc(ListCmd.HelpFunc())
	ListCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		err := command.Flags().MarkHidden("concurrent-downloads")
		_ = command.Flags().MarkHidden("timeout")
		log.Debug(err)
		command.Parent().HelpFunc()(command, strings)
	})
}
