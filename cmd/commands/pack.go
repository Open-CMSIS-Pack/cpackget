/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

var PackCmd := &cobra.Command{
	Use:   "pack",
	Short: "Download and install Open-CMSIS-Pack packages",
	Long: `<pack-path> can be a local file or a file hosted somewhere else on the Internet.
cpack will extract information from it and install the files in specific directories inside this machine.`,
	Args: cobra.MinimumNArgs(1),
	Run: runPackAdd,
}

var packAddCmd := &cobra.Command{
	Use:   "add <pack-path>",
	Short: "Download and install Open-CMSIS-Pack packages",
	Long: `<pack-path> can be a local file or a file hosted somewhere else on the Internet.
cpack will extract information from it and install the files in specific directories inside this machine.`,
	Args: cobra.MinimumNArgs(1),
	Run: runPackAdd,
}

var packRmCmd := &cobra.Command{
	Use:   "rm <pack-name>",
	Short: "Uninstall Open-CMSIS-Pack packages",
	Long: `<pack-name> should be in the format of "PackVendor.PackName.PackVersion".
This will remove the pack from the reference index files. If files need to be actually removed,
please use "cpackget purge <pack-name>"`,
	Args: cobra.MinimumNArgs(1),
	Run: runPackRm,
}

func init() {
	PackCmd.AddCommand(packRmCmd, packRmCmd)
}

func runPackAdd(cmd *cobra.Command, args []string) {
	log.Info("")
	packRoot := viper.GetString("pack-root")
	manager, err := NewPacksManager(packRoot)
	if err != nil {
		log.Errorf("Could not initialize pack manager: %s", err)
		return
	}

	for _, packPath := range args {
		err = manager.Install(packPath)
		if err != nil {
			if errors.Is(err, ErrPdscEntryExists) {
				log.Infof("%s is already installed", packPath)
			} else {
				log.Error(err.Error())
			}
		}
	}

	manager.Save()
}

func runPackRm(cmd *cobra.Command, args []string) {
	log.SetLevel(log.DebugLevel)
	manager, err := NewPacksManager(flags.packRoot)
	if err != nil {
		log.Errorf("Could not initialize pack manager: %s", err)
		return
	}

	for _, packName := range args {
		err = manager.Uninstall(packName)
		if err != nil {
			if errors.Is(err, ErrPdscNotFound) {
				log.Infof("Pack \"%s\" is not installed", packName)
			} else {
				log.Error(err.Error())
			}
		}
	}

	manager.Save()
}
