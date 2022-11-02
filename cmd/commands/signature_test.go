/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"errors"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
)

var signatureCreateCmdTests = []TestCase{
	{
		name:        "test help command",
		args:        []string{"help", "signature-create"},
		expectedErr: nil,
	},
	{
		name:        "test different number of parameters",
		args:        []string{"signature-create", "Vendor.Pack.1.2.3.pack", "foo"},
		expectedErr: errors.New("accepts 1 arg(s), received 2"),
	},
	{
		name:        "test missing certificate path",
		args:        []string{"signature-create", "Vendor.Pack.1.2.3.pack", "--cert-only"},
		expectedErr: errs.ErrIncorrectCmdArgs,
	},
	{
		name:        "test passing cert-only and key flag",
		args:        []string{"signature-create", "Vendor.Pack.1.2.3.pack", "--cert-only", "--private-key", "foo"},
		expectedErr: errs.ErrIncorrectCmdArgs,
	},
	{
		name:        "test passing pgp and cert-only flag",
		args:        []string{"signature-create", "Vendor.Pack.1.2.3.pack", "--pgp", "--cert-only"},
		expectedErr: errs.ErrIncorrectCmdArgs,
	},
	{
		name:        "test passing pgp flag and missing key",
		args:        []string{"signature-create", "Vendor.Pack.1.2.3.pack", "--pgp"},
		expectedErr: errs.ErrIncorrectCmdArgs,
	},
	{
		name:        "test passing pgp flag and certificate",
		args:        []string{"signature-create", "Vendor.Pack.1.2.3.pack", "--pgp", "--private-key", "foo", "--certificate", "bar"},
		expectedErr: errs.ErrIncorrectCmdArgs,
	},
	{
		name:        "test passing pgp flag and skip-validation",
		args:        []string{"signature-create", "Vendor.Pack.1.2.3.pack", "--pgp", "--private-key", "foo", "--skip-validation"},
		expectedErr: errs.ErrIncorrectCmdArgs,
	},
	{
		name:        "test passing pgp flag and skip-info",
		args:        []string{"signature-create", "Vendor.Pack.1.2.3.pack", "--pgp", "--private-key", "foo", "--skip-info"},
		expectedErr: errs.ErrIncorrectCmdArgs,
	},
}

var signatureVerifyCmdTests = []TestCase{
	{
		name:        "test help command",
		args:        []string{"help", "signature-verify"},
		expectedErr: nil,
	},
	{
		name:        "test different number of parameters",
		args:        []string{"signature-verify", "Vendor.Pack.1.2.3.pack", "foo"},
		expectedErr: errors.New("accepts 1 arg(s), received 2"),
	},
	{
		name:        "test passing export and skip-validation",
		args:        []string{"signature-verify", "Vendor.Pack.1.2.3.pack", "--export", "--skip-validation"},
		expectedErr: errs.ErrIncorrectCmdArgs,
	},
	{
		name:        "test passing export and skip-info",
		args:        []string{"signature-verify", "Vendor.Pack.1.2.3.pack", "--export", "--skip-info"},
		expectedErr: errs.ErrIncorrectCmdArgs,
	},
	{
		name:        "test passing key and export",
		args:        []string{"signature-verify", "Vendor.Pack.1.2.3.pack", "--pub-key", "foo", "--export"},
		expectedErr: errs.ErrIncorrectCmdArgs,
	},
	{
		name:        "test passing key and skip-validation",
		args:        []string{"signature-verify", "Vendor.Pack.1.2.3.pack", "--pub-key", "foo", "--skip-validation"},
		expectedErr: errs.ErrIncorrectCmdArgs,
	},
	{
		name:        "test passing key and skip-info",
		args:        []string{"signature-verify", "Vendor.Pack.1.2.3.pack", "--pub-key", "foo", "--skip-info"},
		expectedErr: errs.ErrIncorrectCmdArgs,
	},
}

func TestSignatureCreateCmd(t *testing.T) {
	runTests(t, signatureCreateCmdTests)
}

func TestSignatureVerifyCmd(t *testing.T) {
	runTests(t, signatureVerifyCmdTests)
}
