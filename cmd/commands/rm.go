/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"path/filepath"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rmCmdFlags struct {
	// purge stores the value of "--purge" flag for the "pack rm" command
	purge bool
}

var RmCmd = &cobra.Command{
	Use:   "rm <pack reference>",
	Short: "Remove Open-CMSIS-Pack packages",
	Long: `
Remove a pack using the reference "Vendor.Pack[.x.y.z]", "Vendor::Pack[@x.y.z]" or "[path/to/]Vendor.Pack.pdsc".

  $ cpackget rm Vendor.Pack.1.2.3
  $ cpackget rm Vendor::Pack@1.2.3

  Use the syntax above to let cpackget determine
  the location of pack files prior to removing them.

  $ cpackget rm Vendor.LocalPackInstalledViaPdsc.pdsc
  $ cpackget rm path/to/Vendor.LocalPackInstalledViaPdsc.pdsc

  cpackget also identifies if the pack was installed via
  PDSC file. In this case, cpackget will remove its reference
  from ".Local/local_repository.pidx".

  In the first example, since just the basename of the PDSC file
  path was specified, cpackget will remove *ALL* references of the
  PDSC file it might find. Since it is possible to have many versions
  of the same pack installed via different PDSC file paths, one may
  wish to remove a specific one by specifying a more complete
  PDSC file path, as shown in the second example.

The version "x.y.z" is optional.
Cache files (i.e. under CMSIS_PACK_ROOT/.Download/)
are *NOT* removed. If cache files need to be actually removed,
please use "--purge".`,
	Args:              cobra.MinimumNArgs(1),
	PersistentPreRunE: configureInstaller,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Infof("Removing %v", args)
		var lastErr error
		installer.UnlockPackRoot()
		for _, packPath := range args {
			var err error
			if filepath.Ext(packPath) == ".pdsc" {
				err = installer.RemovePdsc(packPath)
				if err == errs.ErrPdscEntryNotFound {
					err = errs.ErrPackNotInstalled
				}
			} else {
				err = installer.RemovePack(packPath, rmCmdFlags.purge, viper.GetInt("timeout"))
			}
			if err != nil {
				if err != errs.ErrAlreadyLogged {
					log.Error(err)
					err = errs.ErrAlreadyLogged
				}
				lastErr = err

			}
		}
		installer.LockPackRoot()

		return lastErr
	},
}

func init() {
	RmCmd.Flags().BoolVarP(&rmCmdFlags.purge, "purge", "p", false, "forces deletion of cached pack files")
}
