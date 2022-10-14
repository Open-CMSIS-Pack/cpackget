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

	//TODO
	//pgp bool

	// certPath points to the signer's certificate
	certPath string

	// keyPath points to the signer's private key
	keyPath string

	// outputDir saves the signed pack to a specific path
	outputDir string

	// skipCertValidation skips sanity/safety checks on the provided certificate
	skipCertValidation bool

	// skipInfo skips displaying certificate info
	skipInfo bool
}

var signatureVerifyflags struct {
	// export doesn't sign but only exports the embedded certificate
	export bool

	// pgpKey loads a PGP public key to verify against the signature
	// pgpKey string

	// skipCertValidation skips sanity/safety checks on the provided certificate
	skipCertValidation bool

	// skipInfo skips displaying certificate info
	skipInfo bool

	// TODO:
	//skipAddKeychain bool
}

func init() {
	SignatureCreateX509Cmd.Flags().BoolVar(&signatureCreateX509flags.certOnly, "cert-only", false, "certificate-only signature")
	SignatureCreateX509Cmd.Flags().StringVarP(&signatureCreateX509flags.certPath, "certificate", "c", "", "path for the signer's certificate")
	_ = SignatureCreateX509Cmd.MarkFlagRequired("certificate")
	SignatureCreateX509Cmd.Flags().StringVarP(&signatureCreateX509flags.keyPath, "key", "k", "", "path for the signer's private key")
	SignatureCreateX509Cmd.Flags().StringVarP(&signatureCreateX509flags.outputDir, "output-dir", "o", "", "save the signed pack to a specific path")
	SignatureCreateX509Cmd.Flags().BoolVar(&signatureCreateX509flags.skipCertValidation, "skip-validation", false, "do not validate certificate")
	SignatureCreateX509Cmd.Flags().BoolVar(&signatureCreateX509flags.skipInfo, "skip-info", false, "do not display certificate information")

	SignatureVerifyCmd.Flags().BoolVarP(&signatureVerifyflags.export, "export", "e", false, "only export embed certificate")
	SignatureVerifyCmd.Flags().BoolVar(&signatureVerifyflags.skipCertValidation, "skip-validation", false, "do not validate certificate")
	SignatureVerifyCmd.Flags().BoolVar(&signatureVerifyflags.skipInfo, "skip-info", false, "do not display certificate information")

	SignatureCreateX509Cmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		err := command.Flags().MarkHidden("pack-root")
		_ = command.Flags().MarkHidden("concurrent-downloads")
		_ = command.Flags().MarkHidden("timeout")
		log.Debug(err)
		command.Parent().HelpFunc()(command, strings)
	})

	SignatureVerifyCmd.SetHelpFunc(SignatureCreateX509Cmd.HelpFunc())
}

var SignatureCreateX509Cmd = &cobra.Command{
	// TODO: refactor to generic SignatureCreateCmd (default full, --cert-only or --pgp to specify)
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

var SignatureVerifyCmd = &cobra.Command{
	Use:   "signature-verify [<local .path pack>]",
	Short: "Verifies a signed pack",
	Long: `
Verifies the integrity and authenticity of a .checksum file, by
checking it against a provided .signature file (a detached PGP signature) and
a private PGP key (either RSA or Curve25519).

The .signature and key files should have been created with the "signature-create" command,
as they need to be in the PEM format.

If not specified by the -s/--sig-path flag, the .signature path will be read
from the same directory as the .checksum file:

  $ cpackget signature-verify-x509 Vendor.Pack.1.2.3.pack.signed

The passphrase prompt can be skipped with -p/--passphrase, which is useful for CI and automation
but should be used carefully as it exposes the passphrase.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: add flag checking
		if signatureVerifyflags.export && (signatureVerifyflags.skipCertValidation || signatureVerifyflags.skipInfo) {
			log.Error("-e/--export does not need any other flags")
			return errs.ErrIncorrectCmdArgs
		}
		return cryptography.VerifyPackSignature(args[0], Version, signatureVerifyflags.export, signatureVerifyflags.skipCertValidation, signatureVerifyflags.skipInfo)
	},
}
