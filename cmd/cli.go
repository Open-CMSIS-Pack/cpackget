/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	log "github.com/sirupsen/logrus"
)

var flags struct {
	outputFileName string
	force          bool
	cacheDir       string
	version        bool
}

func printVersionAndLicense(file io.Writer) {
	fmt.Fprintf(file, "cpackget version %v\n", Version)
	fmt.Fprintf(file, "%v\n", License)
}

func NewCli() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cpackget",
		Short: "This utility installs/removes CMSIS-Packs",
		Run: func(cmd *cobra.Command, args []string) {
			if flags.version {
				printVersionAndLicense(cmd.OutOrStdout())
				return
			}

			if len(args) == 0 {
				log.Error("Empty arguments list. See --help for more information.")
				return
			}
		},
	}

	cmd.Flags().BoolVarP(&flags.version, "version", "V", false, "Output the version number of cpackget and exit")

	return cmd
}
