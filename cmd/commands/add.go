/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"bufio"
	"os"
	"path/filepath"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var addCmdFlags struct {
	// skipEula tells whether pack's license should be presented to the user or not for a yay-or-nay acceptance
	skipEula bool

	// extractEula forces extraction of the embedded license only, not installing the pack
	extractEula bool

	// packsListFileName is the file name where a list of pack urls is present
	packsListFileName string
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
  been released yet. This will install it as a local pack and
  keep a reference in ".Local/local_repository.pidx".

The file can be a local file or a file hosted somewhere else on the Internet.
If it's hosted somewhere, cpackget will first download it then extract all pack files into "CMSIS_PACK_ROOT/<vendor>/<packName>/<x.y.z>/"
If "-f" is used, cpackget will call "cpackget pack add" on each URL specified in the <packs list> file.`,
	Args:              cobra.MinimumNArgs(0),
	PersistentPreRunE: configureInstaller,
	RunE: func(cmd *cobra.Command, args []string) error {

		if addCmdFlags.packsListFileName != "" {
			log.Infof("Parsing packs urls via file %v", addCmdFlags.packsListFileName)

			file, err := os.Open(addCmdFlags.packsListFileName)
			if err != nil {
				return err
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				args = append(args, scanner.Text())
			}

			if err := scanner.Err(); err != nil {
				return err
			}
		}

		if len(args) == 0 {
			log.Error("Missing a pack-path or list with pack urls specified via -f/--packs-list-filename")
			return errs.ErrIncorrectCmdArgs
		}

		log.Debugf("Specified packs %v", args)
		var lastErr error
		for _, packPath := range args {
			var err error
			if filepath.Ext(packPath) == ".pdsc" {
				err = installer.AddPdsc(packPath)
			} else {
				err = installer.AddPack(packPath, !addCmdFlags.skipEula, addCmdFlags.extractEula)

			}

			if err != nil {
				lastErr = err
				if !errs.AlreadyLogged(err) {
					log.Error(err)
				}
			}
		}

		return lastErr
	},
}

func init() {
	AddCmd.Flags().BoolVarP(&addCmdFlags.skipEula, "agree-embedded-license", "a", false, "agrees with the embedded license of the pack")
	AddCmd.Flags().BoolVarP(&addCmdFlags.extractEula, "extract-embedded-license", "x", false, "extracts the embedded license of the pack and aborts the installation")
	AddCmd.Flags().StringVarP(&addCmdFlags.packsListFileName, "packs-list-filename", "f", "", "specifies a file listing packs urls, one per line")
}
