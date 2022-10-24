/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	"github.com/open-cmsis-pack/cpackget/cmd/cryptography"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var signatureCreateflags struct {
	// certOnly skips private key usage
	certOnly bool

	// certPath points to the signer's certificate
	certPath string

	// keyPath points to the signer's private key
	keyPath string

	// outputDir saves the signed pack to a specific path
	outputDir string

	// pgp mode embeds a PGP signature instead
	pgp bool

	// skipCertValidation skips sanity/safety checks on the provided certificate
	skipCertValidation bool

	// skipInfo skips displaying certificate info
	skipInfo bool
}

var signatureVerifyflags struct {
	// export doesn't sign but only exports the embedded certificate
	export bool

	// pgpKey loads a PGP public key to verify against the signature
	pgpKey string

	// skipCertValidation skips sanity/safety checks on the provided certificate
	skipCertValidation bool

	// skipInfo skips displaying certificate info
	skipInfo bool
}

func init() {
	SignatureCreateCmd.Flags().BoolVar(&signatureCreateflags.certOnly, "cert-only", false, "certificate-only signature mode")
	SignatureCreateCmd.Flags().StringVarP(&signatureCreateflags.certPath, "certificate", "c", "", "path of the signer's certificate")
	SignatureCreateCmd.Flags().StringVarP(&signatureCreateflags.keyPath, "key", "k", "", "path of the signer's private key")
	SignatureCreateCmd.Flags().StringVarP(&signatureCreateflags.outputDir, "output-dir", "o", "", "save the signed pack to a specific path")
	SignatureCreateCmd.Flags().BoolVar(&signatureCreateflags.pgp, "pgp", false, "PGP signature mode")
	SignatureCreateCmd.Flags().BoolVar(&signatureCreateflags.skipCertValidation, "skip-validation", false, "do not validate certificate")
	SignatureCreateCmd.Flags().BoolVar(&signatureCreateflags.skipInfo, "skip-info", false, "do not display certificate information")

	SignatureVerifyCmd.Flags().BoolVarP(&signatureVerifyflags.export, "export", "e", false, "only export embed certificate")
	SignatureVerifyCmd.Flags().StringVarP(&signatureVerifyflags.pgpKey, "pub-key", "k", "", "path of the PGP public key")
	SignatureVerifyCmd.Flags().BoolVar(&signatureVerifyflags.skipCertValidation, "skip-validation", false, "do not validate certificate")
	SignatureVerifyCmd.Flags().BoolVar(&signatureVerifyflags.skipInfo, "skip-info", false, "do not display certificate information")

	SignatureCreateCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		err := command.Flags().MarkHidden("pack-root")
		_ = command.Flags().MarkHidden("concurrent-downloads")
		_ = command.Flags().MarkHidden("timeout")
		log.Debug(err)
		command.Parent().HelpFunc()(command, strings)
	})

	SignatureVerifyCmd.SetHelpFunc(SignatureCreateCmd.HelpFunc())
}

var SignatureCreateCmd = &cobra.Command{
	Use:   "signature-create [<local .path pack>]",
	Short: "Digitally signs a pack with a X509 certificate or PGP key",
	Long: `
Signs a pack using X509 Public Key Infrastructure or PGP signatures.

Three modes are available. "full" is the default, and takes a X509 public key
certificate and its private key (currently only RSA supported), which is used
to sign the hashed (SHA256) contents of a pack.
If "--cert-only" is specified, only a X509 certificate will be embed in the pack. This
offers a lesser degree of security guarantees.
Both these options perform some basic validations on the X509 certificate, which can
be skipped.

If "--pgp" is specified, the user must provide a PGP private key (Curve25519 or RSA 2048,
3072 and 4096 bits are supported).

The signature follows a simple scheme which includes the cpackget version used to sign,
the mode, and the outputs, base64 encoded - saved to the pack's Zip comment field.
These can be viewed with any text/hex editor or dedicated zip tools like "zipinfo".

The referenced pack must be in its original/compressed form (.pack), and be present locally:

  $ cpackget signature-create Vendor.Pack.1.2.3.pack -k private.key -c certificate.pem`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: skipCertinfo and validation
		if signatureCreateflags.keyPath == "" {
			if !signatureCreateflags.certOnly || signatureCreateflags.pgp {
				log.Error("specify private key file with the -k/-key flag")
				return errs.ErrIncorrectCmdArgs
			}
		} else {
			if signatureCreateflags.certOnly {
				log.Error("-k/--key should not be provided in certificate-only mode")
				return errs.ErrIncorrectCmdArgs
			}
		}
		if signatureCreateflags.pgp {
			if signatureCreateflags.certPath != "" {
				log.Error("PGP signature scheme does not need a x509 certificate")
				return errs.ErrIncorrectCmdArgs
			}
			if signatureCreateflags.skipCertValidation {
				log.Error("PGP signature scheme does not validate certificates (--skip-validation)")
				return errs.ErrIncorrectCmdArgs
			}
			if signatureCreateflags.skipInfo {
				log.Error("PGP signature scheme does not display certificate info (--skip-info)")
				return errs.ErrIncorrectCmdArgs
			}
		}
		return cryptography.SignPack(args[0], signatureCreateflags.certPath, signatureCreateflags.keyPath, signatureCreateflags.outputDir, Version, signatureCreateflags.certOnly, signatureCreateflags.skipCertValidation, signatureCreateflags.skipInfo)
	},
}

var SignatureVerifyCmd = &cobra.Command{
	Use:   "signature-verify [<local .path pack>]",
	Short: "Verifies a signed pack",
	Long: `
Verifies the integrity and authenticity of a pack signed
with the "signature-create" command.

For more information on the signatures, use "cpackget help signature-create".

If attempting to verify a PGP signed pack, use the -k/--pub-key flag to specify
the publisher's public GPG key.

The referenced pack must be in its original/compressed form (.pack), and be present locally:

  $ cpackget signature-verify Vendor.Pack.1.2.3.pack.signed`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if signatureVerifyflags.export && (signatureVerifyflags.skipCertValidation || signatureVerifyflags.skipInfo) {
			log.Error("-e/--export does not need any other flags")
			return errs.ErrIncorrectCmdArgs
		}
		if signatureVerifyflags.pgpKey != "" {
			if signatureVerifyflags.export {
				log.Error("can't export non X509 (full, cert-only) signature scheme")
				return errs.ErrIncorrectCmdArgs
			}
			if signatureVerifyflags.skipCertValidation || signatureVerifyflags.skipInfo {
				log.Error("PGP verification does not need any flags other than -k/-pub-key")
				return errs.ErrIncorrectCmdArgs
			}
		}
		return cryptography.VerifyPackSignature(args[0], signatureVerifyflags.pgpKey, Version, signatureVerifyflags.export, signatureVerifyflags.skipCertValidation, signatureVerifyflags.skipInfo)
	},
}
