/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/cryptography"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var signatureCreateX509flags struct {
	// certOnly skips private key usage
	certOnly bool

	// certPath points to the signer's certificate
	certPath string

	// keyPath points to the signer's private key
	keyPath string

	// outputDir saves the signed pack to a specific path
	outputDir string

	// TODO: for verify
	// export doesn't sign but only exports the embedded certificate
	// export bool

	// skipCertValidation skips sanity/safety checks on the provided certificate
	skipCertValidation bool

	// skipInfo skips displaying certificate info
	skipInfo bool
}

func init() {
	SignatureCreateX509Cmd.Flags().BoolVar(&signatureCreateX509flags.certOnly, "cert-only", false, "certificate-only signature")
	SignatureCreateX509Cmd.Flags().StringVarP(&signatureCreateX509flags.certPath, "certificate", "c", "", "path for the signer's certificate")
	_ = SignatureCreateX509Cmd.MarkFlagRequired("certificate")
	SignatureCreateX509Cmd.Flags().StringVarP(&signatureCreateX509flags.keyPath, "key", "k", "", "path for the signer's private key")
	SignatureCreateX509Cmd.Flags().StringVarP(&signatureCreateX509flags.outputDir, "output-dir", "o", "", "save the signed pack to a specific path")
	SignatureCreateX509Cmd.Flags().BoolVar(&signatureCreateX509flags.skipCertValidation, "skip-validation", false, "do not validate certificate")
	SignatureCreateX509Cmd.Flags().BoolVar(&signatureCreateX509flags.skipInfo, "skip-info", false, "do not display certificate information")

	SignatureCreateX509Cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		err := command.Flags().MarkHidden("pack-root")
		_ = command.Flags().MarkHidden("concurrent-downloads")
		_ = command.Flags().MarkHidden("timeout")
		log.Debug(err)
		command.Parent().HelpFunc()(command, strings)
	})
	// SignatureVerifyPGPCmd.SetHelpFunc(SignatureCreateX509Cmd.HelpFunc())
}

var SignatureCreateX509Cmd = &cobra.Command{
	Use:   "signature-create-x509 [<local .path pack>]",
	Short: "Digitally signs a pack with a X509 certificate",
	Long: `
Signs a pack using a x509 certificate and its private key.

  The referenced pack must be in its original/compressed form (.pack), and be present locally:

  $ cpackget signature-create-x509 Vendor.Pack.1.2.3.pack

By default the signature file will be created in the same directory as the provided pack.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if signatureCreateX509flags.certOnly && signatureCreateX509flags.keyPath != "" {
			log.Error("-k/--key should not be provided in certificate-only mode")
			return errs.ErrIncorrectCmdArgs
		}
		return cryptography.SignPackX509(args[0], signatureCreateX509flags.certPath, signatureCreateX509flags.keyPath, signatureCreateX509flags.outputDir, Version, signatureCreateX509flags.certOnly, signatureCreateX509flags.skipCertValidation, signatureCreateX509flags.skipInfo)
	},
}

// var SignatureVerifyPGPCmd = &cobra.Command{
// 	Use:   "signature-verify-pgp [<local .checksum pack>] [<local private pgp key>]",
// 	Short: "Verifies the integrity of a .checksum against its signature",
// 	Long: `
// Verifies the integrity and authenticity of a .checksum file, by
// checking it against a provided .signature file (a detached PGP signature) and
// a private PGP key (either RSA or Curve25519).

// The .signature and key files should have been created with the "signature-create" command,
// as they need to be in the PEM format.

// If not specified by the -s/--sig-path flag, the .signature path will be read
// from the same directory as the .checksum file:

//   $ cpackget signature-verify-pgp Vendor.Pack.1.2.3.sha256.checksum signature_curve25519.key

// The passphrase prompt can be skipped with -p/--passphrase, which is useful for CI and automation
// but should be used carefully as it exposes the passphrase.`,
// 	Args: cobra.ExactArgs(2),
// 	RunE: func(cmd *cobra.Command, args []string) error {
// 		return cryptography.VerifyPGPSignature(args[0], args[1], signatureVerifyPGPflags.signaturePath, signatureVerifyPGPflags.passphrase)
// 	},
// }
