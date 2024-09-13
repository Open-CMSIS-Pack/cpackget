/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"bufio"
	"os"
	"strings"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var updateCmdFlags struct {
	// noRequirements skips installing package requirements
	noRequirements bool

	// packsListFileName is the file name where a list of pack urls is present
	packsListFileName string

	// skipEula tells whether pack's license should be presented to the user or not for a yay-or-nay acceptance
	skipEula bool

	// skipTouch does not touch pack.idx after update
	skipTouch bool

	// Reports encoded progress for files and download when used by other tools
	encodedProgress bool
}

var UpdateCmd = &cobra.Command{
	Use:   "update [<pack> | -f <packs list>]",
	Short: "Update Open-CMSIS-Pack packages to latest",
	Long: `
Update a pack using the following "<pack>" specification or using packs provided by "-f <packs list>":

  $ cpackget update Vendor.Pack

  The pack will be updated to the latest version

  $ cpackget update

  Use this to update all installed packs to the latest version

  The pack can be local file or hosted somewhere else on the Internet.
  If it's hosted somewhere, cpackget will first download it then extract all pack files into "CMSIS_PACK_ROOT/<vendor>/<packName>/<x.y.z>/"
  If "-f" is used, cpackget will call "cpackget update pack" on each URL specified in the <packs list> file.`,
	Args:              cobra.MinimumNArgs(0),
	PersistentPreRunE: configureInstaller,
	RunE: func(cmd *cobra.Command, args []string) error {

		utils.SetEncodedProgress(updateCmdFlags.encodedProgress)
		utils.SetSkipTouch(updateCmdFlags.skipTouch)

		if updateCmdFlags.packsListFileName != "" {
			log.Infof("Parsing packs urls via file %v", updateCmdFlags.packsListFileName)

			file, err := os.Open(updateCmdFlags.packsListFileName)
			if err != nil {
				return err
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				tmpEntry := strings.TrimSpace(scanner.Text())
				if len(tmpEntry) == 0 {
					continue
				}
				args = append(args, tmpEntry)
			}

			if err := scanner.Err(); err != nil {
				return err
			}
		}

		var lastErr error

		if len(args) == 0 {
			if updateCmdFlags.packsListFileName != "" {
				return nil // nothing to do
			}
			installer.UnlockPackRoot()
			err := installer.UpdatePack("", !updateCmdFlags.skipEula, updateCmdFlags.noRequirements, viper.GetInt("timeout"))
			if err != nil {
				lastErr = err
				if !errs.AlreadyLogged(err) {
					log.Error(err)
				}
			}
			installer.LockPackRoot()
			return lastErr
		}

		log.Debugf("Specified packs %v", args)
		installer.UnlockPackRoot()
		for _, packPath := range args {
			err := installer.UpdatePack(packPath, !updateCmdFlags.skipEula, updateCmdFlags.noRequirements, viper.GetInt("timeout"))
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
	UpdateCmd.Flags().BoolVarP(&updateCmdFlags.skipEula, "agree-embedded-license", "a", false, "agrees with the embedded license of the pack")
	UpdateCmd.Flags().BoolVarP(&updateCmdFlags.noRequirements, "no-dependencies", "n", false, "do not install package dependencies")
	UpdateCmd.Flags().StringVarP(&updateCmdFlags.packsListFileName, "packs-list-filename", "f", "", "specifies a file listing packs urls, one per line")
	UpdateCmd.Flags().BoolVar(&updateCmdFlags.skipTouch, "skip-touch", false, "do not touch pack.idx")
	UpdateCmd.Flags().BoolVarP(&updateCmdFlags.encodedProgress, "encoded-progress", "E", false, "Reports encoded progress for files and download when used by other tools")

	UpdateCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		// Small workaround to keep the linter happy, not
		// really necessary to test this
		err := command.Flags().MarkHidden("concurrent-downloads")
		log.Debug(err)
		command.Parent().HelpFunc()(command, strings)
	})
}
