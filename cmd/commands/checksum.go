/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/cryptography"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var checksumCreateCmdFlags struct {
	// hashAlgorithm is the cryptographic hash function to be used
	hashAlgorithm string

	// outputDir is the target directory where the checksum file is written to
	outputDir string
}

func init() {
	ChecksumCreateCmd.Flags().StringVarP(&checksumCreateCmdFlags.hashAlgorithm, "hash-algorithm", "a", "", "specifies the hash function to be used")
	ChecksumCreateCmd.Flags().StringVarP(&checksumCreateCmdFlags.outputDir, "output-dir", "o", "", "specifies output directory for the checksum file")
}

var ChecksumCreateCmd = &cobra.Command{
	Use:   "checksum-create [<local .path pack>]",
	Short: "Generates a .checksum file containing the digests of a pack",
	// TODO: Long, show valid hash algorithms
	Args:              cobra.MinimumNArgs(0),
	PersistentPreRunE: configureInstaller,
	RunE: func(cmd *cobra.Command, args []string) error {

		if len(args) == 0 {
			log.Error("missing .pack local path")
			return errs.ErrIncorrectCmdArgs
		}

		return cryptography.GenerateChecksum(args[0], checksumCreateCmdFlags.outputDir, checksumCreateCmdFlags.hashAlgorithm)
	},
}

var ChecksumVerifyCmd = &cobra.Command{
	Use:   "checksum-verify [<local .path pack>] [<local .checksum path>]",
	Short: "Verifies the integrity of a pack using its .checksum file",
	// TODO: Long
	Args:              cobra.MinimumNArgs(0),
	PersistentPreRunE: configureInstaller,
	RunE: func(cmd *cobra.Command, args []string) error {

		if len(args) != 2 {
			log.Error("Please provide path to pack and checksum file")
			return errs.ErrIncorrectCmdArgs
		}

		return cryptography.VerifyChecksum(args[0], args[1])
	},
}
