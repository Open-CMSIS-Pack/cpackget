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
	SignatureCreateCmd.Flags().BoolVar(&signatureCreateflags.certOnly, "cert-only", false, "certificate-only signature")
	SignatureCreateCmd.Flags().StringVarP(&signatureCreateflags.certPath, "certificate", "c", "", "path for the signer's certificate")
	_ = SignatureCreateCmd.MarkFlagRequired("certificate")
	SignatureCreateCmd.Flags().StringVarP(&signatureCreateflags.keyPath, "key", "k", "", "path for the signer's private key")
	SignatureCreateCmd.Flags().StringVarP(&signatureCreateflags.outputDir, "output-dir", "o", "", "save the signed pack to a specific path")
	SignatureCreateCmd.Flags().BoolVar(&signatureCreateflags.skipCertValidation, "skip-validation", false, "do not validate certificate")
	SignatureCreateCmd.Flags().BoolVar(&signatureCreateflags.skipInfo, "skip-info", false, "do not display certificate information")

	SignatureVerifyCmd.Flags().BoolVarP(&signatureVerifyflags.export, "export", "e", false, "only export embed certificate")
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
	// TODO: refactor to generic SignatureCreateCmd (default full, --cert-only or --pgp to specify)
	Use:   "signature-create [<local .path pack>]",
	Short: "Digitally signs a pack with a X509 certificate",
	Long: `
Signs a pack using a x509 certificate and its private key.

The referenced pack must be in its original/compressed form (.pack), and be present locally:

  $ cpackget signature-create Vendor.Pack.1.2.3.pack`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if signatureCreateflags.keyPath == "" {
			if !signatureCreateflags.certOnly {
				log.Error("specify private key file with the --key/-k flag")
				return errs.ErrIncorrectCmdArgs
			}
		} else {
			if signatureCreateflags.certOnly {
				log.Error("-k/--key should not be provided in certificate-only mode")
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

The referenced pack must be in its original/compressed form (.pack), and be present locally:

  $ cpackget signature-verify Vendor.Pack.1.2.3.pack.signed`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if signatureVerifyflags.export && (signatureVerifyflags.skipCertValidation || signatureVerifyflags.skipInfo) {
			log.Error("-e/--export does not need any other flags")
			return errs.ErrIncorrectCmdArgs
		}
		return cryptography.VerifyPackSignature(args[0], Version, signatureVerifyflags.export, signatureVerifyflags.skipCertValidation, signatureVerifyflags.skipInfo)
	},
}
