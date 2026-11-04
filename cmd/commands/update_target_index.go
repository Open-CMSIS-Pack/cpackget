/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/spf13/cobra"
)

var UpdateTargetIndexCmd = &cobra.Command{
	Use:   "update-target-index",
	Short: "Rebuild the installed target index offline",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := configureInstallerGlobalCmd(cmd, args); err != nil {
			return err
		}
		if err := installer.SetPackRoot(viper.GetString("pack-root"), false); err != nil {
			return err
		}
		installer.UnlockPackRoot()
		defer installer.LockPackRoot()
		return installer.UpdateTargetIndex()
	},
}

func init() {
	AllCommands = append(AllCommands, UpdateTargetIndexCmd)
}
