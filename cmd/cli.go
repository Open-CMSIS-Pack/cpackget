/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var flags struct {
	version  bool
	verbosiness int
	packRoot string
}

func printVersionAndLicense(file io.Writer) {
	fmt.Fprintf(file, "cpackget version %v\n", Version)
	fmt.Fprintf(file, "%v\n", License)
}

func installCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <pack-path>",
		Short: "Download and install Open-CMSIS-Pack packages",
		Long: `<pack-path> can be a local file or a file hosted somewhere else on the Internet.
cpack will extract information from it and install the files in specific directories inside this machine.`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			log.SetLevel(log.DebugLevel)
			manager, err := NewPacksManager(flags.packRoot)
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
		},
	}

	return cmd
}

func uninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall <pack-name>",
		Short: "Uninstall Open-CMSIS-Pack packages",
		Long: `<pack-name> should be in the format of "PackVendor.PackName.PackVersion".
This will remove the pack from the reference index files. If files need to be actually removed,
please use "cpackget purge <pack-name>"`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
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
		},
	}

	return cmd
}

func NewCli() *cobra.Command {

	rootCmd := &cobra.Command{
		Use:   "cpackget",
		Short: "This utility installs/removes CMSIS-Packs",
		Run: func(cmd *cobra.Command, args []string) {
			if flags.version {
				printVersionAndLicense(cmd.OutOrStdout())
				return
			}

			log.Error("Please choose a command. See --help")
		},
	}

	defaultPackRoot := os.Getenv("CMSIS_PACK_ROOT")
	if defaultPackRoot == "" {
		defaultPackRoot = ".cpackget/"
	}

	rootCmd.Flags().BoolVarP(&flags.version, "version", "V", false, "Output the version number of cpackget and exit")
	rootCmd.PersistentFlags().StringVarP(&flags.packRoot, "pack-root", "R", defaultPackRoot, "Specify pack-root folder. Defaults to CMSIS_PACK_ROOT environment var or current directory")
	rootCmd.PersistentFlags().IntVarP(&flags.verbosiness, "verbosiness", "v", 1, "Set verbosiness: 0 (Errors), 1 (Info messages), 2 (Warnings), 3 (Debugging)")
	rootCmd.AddCommand(installCmd(), uninstallCmd())

	return rootCmd
}
