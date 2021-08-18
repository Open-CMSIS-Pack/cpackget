/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/open-cmsis-pack/cpackget/cmd/commands"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var flags struct {
	version     bool
	verbosiness int
}

func printVersionAndLicense(file io.Writer) {
	fmt.Fprintf(file, "cpackget version %v\n", Version)
	fmt.Fprintf(file, "%v\n", License)
}

func NewCli() *cobra.Command {
	cobra.OnInitialize(initCobra)

	rootCmd := &cobra.Command{
		Use:          "cpackget",
		Short:        "This utility installs/removes CMSIS-Packs",
		SilenceUsage: true,
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

	rootCmd.Flags().BoolVarP(&flags.version, "version", "V", false, "Output the version number of cpackget and exit.")
	rootCmd.PersistentFlags().IntVarP(&flags.verbosiness, "verbosiness", "v", 1, "Set verbosiness: 0 (Errors), 1 (Info messages), 2 (Warnings), 3 (Debugging).")
	rootCmd.PersistentFlags().StringP("pack-root", "R", defaultPackRoot, "Specify pack root folder. Defaults to CMSIS_PACK_ROOT environment variable or current directory.")
	_ = viper.BindPFlag("pack-root", rootCmd.PersistentFlags().Lookup("pack-root"))

	for _, cmd := range commands.All {
		rootCmd.AddCommand(cmd)
	}

	return rootCmd
}

func initCobra() {
	viper.AutomaticEnv()
}
