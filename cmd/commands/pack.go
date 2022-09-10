/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"bufio"
	"os"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var PackCmd = &cobra.Command{
	Deprecated:        "Consider running `cpackget add|rm|list` instead",
	Use:               "pack",
	Short:             "Adds/Removes Open-CMSIS-Pack packages",
	Long:              "Adds/Removes Open-CMSIS-Pack packages from a local file or a file hosted somewhere else on the Internet.",
	PersistentPreRunE: configureInstaller,
}

// skipEula tells whether pack's license should be presented to the user or not for a yay-or-nay acceptance
var skipEula bool

// extractEula forces extraction of the embedded license only, not installing the pack
var extractEula bool

// forceReinstall forces installation of an already installed pack
var forceReinstall bool

// packsListFileName is the file name where a list of pack urls is present
var packsListFileName string

// listPublic tells whether listing all packs in the public index
var listPublic bool

// listCached tells whether listing all cached packs
var listCached bool

// listFilter is a set of words by which to filter listed packs
var listFilter string

var packAddCmd = &cobra.Command{
	Use:   "add [<pack path> | -f <packs list>]",
	Short: "Adds Open-CMSIS-Pack packages",
	Long: `Adds a pack using the file specified in "<pack path>" or using packs URLs provided by "-f <packs list>".
The file can be a local file or a file hosted somewhere else on the Internet.
If it's hosted somewhere, cpackget will first download it then extract all pack files into "CMSIS_PACK_ROOT/<vendor>/<packName>/<x.y.z>/"
If "-f" is used, cpackget will call "cpackget pack add" on each URL specified in the <packs list> file.`,
	Args: cobra.MinimumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {

		if packsListFileName != "" {
			log.Infof("Parsing packs urls via file %v", packsListFileName)

			file, err := os.Open(packsListFileName)
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
		var firstError error
		installer.UnlockPackRoot()
		for _, packPath := range args {
			if err := installer.AddPack(packPath, !skipEula, extractEula, forceReinstall, viper.GetInt("timeout")); err != nil {
				if firstError == nil {
					firstError = err
				}

				if !errs.AlreadyLogged(err) {
					log.Error(err)
				}
			}
		}
		installer.LockPackRoot()

		if firstError == nil {
			return nil
		}
		return errs.ErrAlreadyLogged
	},
}

// purge stores the value of "--purge" flag for the "pack rm" command
var purge bool

var packRmCmd = &cobra.Command{
	Use:   "rm <pack reference>",
	Short: "Removes Open-CMSIS-Pack packages",
	Long: `Removes a pack using the reference "PackVendor.PackName[.x.y.z]".
The version "x.y.z" is optional. This will remove the pack from the reference index files.
Cache files (i.e. under CMSIS_PACK_ROOT/.Download/) are *NOT* removed. If cache files need to be actually removed, please use "--purge".`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Infof("Removing %v", args)
		installer.UnlockPackRoot()
		defer installer.LockPackRoot()
		for _, packPath := range args {
			if err := installer.RemovePack(packPath, purge, viper.GetInt("timeout")); err != nil {
				return err
			}
		}

		return nil
	},
}

var packListCmd = &cobra.Command{
	Use:   "list [--cached|--public]",
	Short: "Lists installed packs",
	Long:  `Lists all installed packs and optionally cached pack files`,
	Args:  cobra.MaximumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		return installer.ListInstalledPacks(listCached, listPublic, listFilter)
	},
}

func init() {
	packRmCmd.Flags().BoolVarP(&purge, "purge", "p", false, "forces deletion of cached pack files")
	packAddCmd.Flags().BoolVarP(&skipEula, "agree-embedded-license", "a", false, "agrees with the embedded license of the pack")
	packAddCmd.Flags().BoolVarP(&extractEula, "extract-embedded-license", "x", false, "extracts the embedded license of the pack and aborts the installation")
	packAddCmd.Flags().BoolVarP(&forceReinstall, "force-reinstall", "F", false, "forces installation of an already installed pack")
	packAddCmd.Flags().StringVarP(&packsListFileName, "packs-list-filename", "f", "", "specifies a file listing packs urls, one per line")
	packListCmd.Flags().BoolVarP(&listCached, "cached", "c", false, "lists only cached packs")
	packListCmd.Flags().BoolVarP(&listPublic, "public", "p", false, "lists packs in the public index")
	PackCmd.AddCommand(packAddCmd, packRmCmd, packListCmd)

	packAddCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		err := command.Flags().MarkHidden("concurrent-downloads")
		log.Debug(err)
		command.Parent().HelpFunc()(command, strings)
	})
	packRmCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		err := command.Flags().MarkHidden("concurrent-downloads")
		_ = command.Flags().MarkHidden("timeout")
		log.Debug(err)
		command.Parent().HelpFunc()(command, strings)
	})
	packListCmd.SetHelpFunc(packRmCmd.HelpFunc())
}
