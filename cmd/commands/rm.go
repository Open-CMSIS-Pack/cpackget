/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rmCmdFlags struct {
	// skipEula tells whether pack's license should be presented to the user or not for a yay-or-nay acceptance
	purge bool
}

var RmCmd = &cobra.Command{
	Use:   "rm <pack reference>",
	Short: "Remove Open-CMSIS-Pack packages",
	Long: `
Remove a pack using the reference "Vendor.Pack[.x.y.z]" or "Vendor::Pack[@x.y.z]".

  $ cpackget rm Vendor.Pack.1.2.3
  $ cpackget rm Vendor::Pack@1.2.3
  
  Use the syntax above to let cpackget determine
  the location of pack files prior to removing them.
  
  $ cpackget rm Vendor.LocalPackInstalledViaPdsc.1.2.3
  
  cpackget also identifies if the pack was installed via
  PDSC file. In this case, cpackget will remove its reference
  from ".Local/local_repository.pidx".

The version "x.y.z" is optional.
Cache files (i.e. under CMSIS_PACK_ROOT/.Download/)
are *NOT* removed. If cache files need to be actually removed,
please use "--purge".`,
	Args:              cobra.MinimumNArgs(1),
	PersistentPreRunE: configureInstaller,
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Infof("Removing %v", args)
		var lastErr error
		for _, packPath := range args {
			err := installer.RemovePack(packPath, purge)
			if err == errs.ErrPackNotInstalled {
				err = installer.RemovePdsc(packPath)
				if err == errs.ErrPdscEntryNotFound {
					err = errs.ErrPackNotInstalled
				}
			}
			if err != nil {
				lastErr = err
				if err != errs.ErrAlreadyLogged {
					log.Error(err)
				}

			}
		}

		return lastErr
	},
}

func init() {
	RmCmd.Flags().BoolVarP(&rmCmdFlags.purge, "purge", "p", false, "forces deletion of cached pack files")
}
