/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"path/filepath"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var addCmdFlags struct {
	// extractEula forces extraction of the embedded license only, not installing the pack
	extractEula bool

	// forceReinstall forces installation of an already installed pack
	forceReinstall bool

	// noRequirements skips installing package requirements
	noRequirements bool

	// packsListFileName is the file name where a list of pack urls is present
	packsListFileName string

	// skipEula tells whether pack's license should be presented to the user or not for a yay-or-nay acceptance
	skipEula bool

	// skipTouch does not touch pack.idx after adding
	skipTouch bool

	// Reports encoded progress for files and download when used by other tools
	encodedProgress bool
}

var AddCmd = &cobra.Command{
	Use:   "add [<pack> | -f <packs list>]",
	Short: "Add Open-CMSIS-Pack packages",
	Long: `
Add a pack using the following "<pack>" specification or using packs provided by "-f <packs list>":

  $ cpackget add Vendor.Pack.1.2.3
  $ cpackget add Vendor::Pack@1.2.3

  Use the syntax above to let cpackget determine
  the location of pack files prior to installing it locally.

  $ cpackget add Vendor.Pack.1.2.3.pack

  Use this syntax if you already have a pack at hand and simply
  want to install it under pack-root folder.

  $ cpackget add path/to/Vendor.Pack.pdsc

  Use this syntax if you are installing a pack that has not
  been released yet. This will add a reference in ".Local/local_repository.pidx".

  To install a specific version use: Vendor::Pack@x.y.z
  To install the newest version of the major version if a version greater equal x.y.z is not already installed use: Vendor::Pack@^x.y.z
  To install the newest version of the major and the minor version if a version greater or equal x.y.z is not already installed use: Vendor::Pack@~x.y.z
  To install the newest available version if a version greater or equal x.y.z is not already installed use: Vendor::Pack@>=x.y.z

  The file can be a local file or a file hosted somewhere else on the Internet.
  If it's hosted somewhere, cpackget will first download it then extract all pack files into "CMSIS_PACK_ROOT/<vendor>/<packName>/<x.y.z>/"
  If "-f" is used, cpackget will call "cpackget pack add" on each URL specified in the <packs list> file.`,
	Args: cobra.MinimumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {

		utils.SetEncodedProgress(addCmdFlags.encodedProgress)
		utils.SetSkipTouch(addCmdFlags.skipTouch)

		createPackRoot = true
		err := configureInstaller(cmd, args)
		if err != nil {
			return err
		}

		files, err := utils.GetListFiles(addCmdFlags.packsListFileName)
		if err != nil {
			return err
		}
		args = append(args, files...)

		if len(args) == 0 {
			log.Warn("Missing a pack-path or list with pack urls specified via -f/--packs-list-filename")

			if addCmdFlags.packsListFileName != "" {
				return nil
			}

			return errs.ErrIncorrectCmdArgs
		}

		var lastErr error

		log.Debugf("Specified packs %v", args)
		installer.UnlockPackRoot()
		for _, packPath := range args {
			var err error
			if filepath.Ext(packPath) == ".pdsc" {
				err = installer.AddPdsc(packPath)
			} else {
				err = installer.AddPack(packPath, !addCmdFlags.skipEula, addCmdFlags.extractEula, addCmdFlags.forceReinstall, addCmdFlags.noRequirements, false, viper.GetInt("timeout"))
			}
			if err != nil {
				lastErr = err
				if !errs.AlreadyLogged(err) {
					log.Error(err)
				}
			}
		}
		installer.LockPackRoot()
		return lastErr
	},
}

func init() {
	AddCmd.Flags().BoolVarP(&addCmdFlags.skipEula, "agree-embedded-license", "a", false, "agrees with the embedded license of the pack")
	AddCmd.Flags().BoolVarP(&addCmdFlags.extractEula, "extract-embedded-license", "x", false, "extracts the embedded license of the pack and aborts the installation")
	AddCmd.Flags().BoolVarP(&addCmdFlags.forceReinstall, "force-reinstall", "F", false, "forces installation of an already installed pack")
	AddCmd.Flags().BoolVarP(&addCmdFlags.noRequirements, "no-dependencies", "n", false, "do not install package dependencies")
	AddCmd.Flags().StringVarP(&addCmdFlags.packsListFileName, "packs-list-filename", "f", "", "specifies a file listing packs urls, one per line")
	AddCmd.Flags().BoolVar(&addCmdFlags.skipTouch, "skip-touch", false, "do not touch pack.idx")
	AddCmd.Flags().BoolVarP(&addCmdFlags.encodedProgress, "encoded-progress", "E", false, "Reports encoded progress for files and download when used by other tools")

	AddCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		// Small workaround to keep the linter happy, not
		// really necessary to test this
		err := command.Flags().MarkHidden("concurrent-downloads")
		log.Debug(err)
		command.Parent().HelpFunc()(command, strings)
	})
}
