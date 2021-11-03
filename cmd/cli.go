/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/open-cmsis-pack/cpackget/cmd/commands"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var flags struct {
	version bool
}

func printVersionAndLicense(file io.Writer) {
	fmt.Fprintf(file, "cpackget version %v\n", Version)
	fmt.Fprintf(file, "%v\n", License)
}

func NewCli() *cobra.Command {
	cobra.OnInitialize(initCobra)

	rootCmd := &cobra.Command{
		Use:           "cpackget",
		Short:         "This utility adds/removes CMSIS-Packs",
		Long:          "Please refer to the upstream repository for further information: https://github.com/Open-CMSIS-Pack/cpackget.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.version {
				printVersionAndLicense(cmd.OutOrStdout())
				return nil
			}

			return cmd.Help()
		},
	}

	defaultPackRoot := os.Getenv("CMSIS_PACK_ROOT")

	rootCmd.Flags().BoolVarP(&flags.version, "version", "V", false, "Prints the version number of cpackget and exit")
	rootCmd.PersistentFlags().CountP("verbosiness", "v", "Sets verbosiness level: None (Errors), -v (Info), -vv (Debugging)")
	rootCmd.PersistentFlags().StringP("pack-root", "R", defaultPackRoot, "Specifies pack root folder. Defaults to CMSIS_PACK_ROOT environment variable")
	_ = viper.BindPFlag("pack-root", rootCmd.PersistentFlags().Lookup("pack-root"))
	_ = viper.BindPFlag("verbosiness", rootCmd.PersistentFlags().Lookup("verbosiness"))

	for _, cmd := range commands.All {
		rootCmd.AddCommand(cmd)
	}

	return rootCmd
}

func initCobra() {
	viper.AutomaticEnv()
}
