/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"errors"
	"os"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
)

// TODO: Compare actual ErrFileNotFound output
var signatureCreateCmdTests = []TestCase{
	{
		name:        "test different number of parameters",
		args:        []string{"signature-create"},
		expectedErr: errors.New("accepts 1 arg(s), received 0"),
	},
	{
		name:        "test creating signature of unexisting pack",
		args:        []string{"signature-create", "DoesNotExist.Pack.1.2.3.pack"},
		expectedErr: errs.ErrFileNotFound,
	},
}

var signatureVerifyCmdTests = []TestCase{
	{
		name:        "test different number of parameters",
		args:        []string{"signature-verify"},
		expectedErr: errors.New("accepts 2 arg(s), received 0"),
	},
	{
		name:        "test signature of unexisting .checksum",
		args:        []string{"signature-verify", "Pack.1.2.3.sha256.checksum", "signature_curve25519.key"},
		expectedErr: errs.ErrFileNotFound,
		setUpFunc: func(t *TestCase) {
			x, _ := os.Create("signature_curve25519.key")
			y, _ := os.Create("Pack.1.2.3.sha256.signature")
			x.Close()
			y.Close()
		},
		tearDownFunc: func() {
			os.Remove("signature_curve25519.key")
			os.Remove("Pack.1.2.3.sha256.signature")
		},
	},
	{
		name:        "test verifying unexisting .signature",
		args:        []string{"signature-verify", "Pack.1.2.3.sha256.checksum", "signature_curve25519.key"},
		expectedErr: errs.ErrFileNotFound,
		setUpFunc: func(t *TestCase) {
			x, _ := os.Create("signature_curve25519.key")
			y, _ := os.Create("Pack.1.2.3.sha256.checksum")
			x.Close()
			y.Close()
		},
		tearDownFunc: func() {
			os.Remove("signature_curve25519.key")
			os.Remove("Pack.1.2.3.sha256.checksum")
		},
	},
}

func TestSignatureCreateCmd(t *testing.T) {
	runTests(t, signatureCreateCmdTests)
}

func TestSignatureVerifyCmd(t *testing.T) {
	runTests(t, signatureVerifyCmdTests)
}
