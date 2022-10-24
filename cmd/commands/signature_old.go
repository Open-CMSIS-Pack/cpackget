/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/cryptography"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var signatureCreateOldflags struct {
	// keyPath points to an existing PGP private key
	keyPath string

	// passphrase bypasses the prompt
	passphrase string

	// outputDir is the target directory where the signature file is written to
	outputDir string

	// outputB64 prints the signature in base64 encoding
	outputB64 bool
}

var signatureVerifyOldflags struct {
	// signaturePath is the path of the signature file
	signaturePath string

	// passphrase bypasses the prompt
	passphrase string
}

func init() {
	SignatureCreateOldCmd.Flags().StringVarP(&signatureCreateOldflags.keyPath, "key-path", "k", "", "provide a private key instead of generating one")
	SignatureCreateOldCmd.Flags().StringVarP(&signatureCreateOldflags.passphrase, "passphrase", "p", "", "passphrase for the provided private key")
	SignatureCreateOldCmd.Flags().StringVarP(&signatureCreateOldflags.outputDir, "output-dir", "o", "", "specifies an output directory of the signature file")
	SignatureCreateOldCmd.Flags().BoolVarP(&signatureCreateOldflags.outputB64, "output-base64", "6", false, "show signature contents as base64")

	SignatureVerifyOldCmd.Flags().StringVarP(&signatureVerifyOldflags.signaturePath, "sig-path", "s", "", "path of the .signature file")
	SignatureVerifyOldCmd.Flags().StringVarP(&signatureVerifyOldflags.passphrase, "passphrase", "p", "", "passphrase for the provided private key")

	SignatureCreateOldCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		err := command.Flags().MarkHidden("pack-root")
		_ = command.Flags().MarkHidden("concurrent-downloads")
		_ = command.Flags().MarkHidden("timeout")
		log.Debug(err)
		command.Parent().HelpFunc()(command, strings)
	})
	SignatureVerifyOldCmd.SetHelpFunc(SignatureCreateOldCmd.HelpFunc())
}

var SignatureCreateOldCmd = &cobra.Command{
	Deprecated: "Consider running `cpackget signature-create` instead",
	Use:        "signature-create-pgp [<local .path pack>]",
	Short:      "Create a digest list of a pack and signs it",
	Long: `
Generates a digest list of a pack, and signs it, creating
a detached PGP signature.

This creates a ".checksum" file, containing hashes of the contents
of the provided pack. It then gets processed and signed with a private
key, producing a PGP signature, stored in the equivalent ".signature".

If a .checksum file already exists in the target path, it will fail as to
guarantee hash freshness.

Currently Curve25519 and RSA (2048, 3072, 4096 bits) key types are supported.
If no private key (it MUST be in PGP PEM format) is provided with the -k/--key-path,
one will be created using the builtin GopenPGP module.

The contents of the generated ".checksum" file are the same as the one
created by "cpackget checksum-create":

  "6f95628e4e0824b0ff4a9f49dad1c3eb073b27c2dd84de3b985f0ef3405ca9ca Vendor.Pack.1.2.3.pdsc
  435fsdf..."

  The referenced pack must be in its original/compressed form (.pack), and be present locally:

  $ cpackget signature-create-pgp Vendor.Pack.1.2.3.pack

By default the signature file will be created in the same directory as the provided pack.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if signatureCreateOldflags.keyPath == "" && signatureCreateOldflags.passphrase != "" {
			log.Error("-p/--passphrase is only specified when providing a key")
			return errs.ErrIncorrectCmdArgs
		}
		return cryptography.GenerateSignedPGPChecksum(args[0], signatureCreateOldflags.keyPath, signatureCreateOldflags.outputDir, signatureCreateOldflags.passphrase, signatureCreateOldflags.outputB64)
	},
}

var SignatureVerifyOldCmd = &cobra.Command{
	Deprecated: "Consider running `cpackget signature-verify` instead",
	Use:        "signature-verify-pgp [<local .checksum pack>] [<local private pgp key>]",
	Short:      "Verifies the integrity of a .checksum against its signature",
	Long: `
Verifies the integrity and authenticity of a .checksum file, by
checking it against a provided .signature file (a detached PGP signature) and
a private PGP key (either RSA or Curve25519).

The .signature and key files should have been created with the "signature-create" command,
as they need to be in the PEM format.

If not specified by the -s/--sig-path flag, the .signature path will be read
from the same directory as the .checksum file:

  $ cpackget signature-verify-pgp Vendor.Pack.1.2.3.sha256.checksum signature_curve25519.key

The passphrase prompt can be skipped with -p/--passphrase, which is useful for CI and automation
but should be used carefully as it exposes the passphrase.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cryptography.VerifyPGPSignature(args[0], args[1], signatureVerifyOldflags.signaturePath, signatureVerifyOldflags.passphrase)
	},
}
