/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the vidx2pidx project. */

package installer_test

import (
	"fmt"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	name  string
	path  string
	purge bool
	err   error
	check func(testCase) bool
}

func runTests(t *testing.T, tests []testCase, testFunction func(testCase) error) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := testFunction(test)
			fmt.Println(err.Error())
			if test.err != nil {
				assert.True(t, errs.Is(err, test.err))
			} else {
				assert.True(t, test.check(test))
			}
		})
	}
}

func TestAddPack(t *testing.T) {
	var tests = []testCase{
		testCase{
			name: "test fail if path is invalid",
			path: "not-really-valid",
			err:  errs.ErrBadPackNameInvalidExtension,
		},
	}

	runTests(t, tests, func(test testCase) error {
		return installer.AddPdsc(test.path)
	})
}

func TestRemovePack(t *testing.T) {
	var tests = []testCase{
		testCase{
			name: "test fail if path is invalid",
			path: "not-really-valid",
			err:  errs.ErrBadPackNameInvalidExtension,
		},
		testCase{
			name:  "test fail if path is invalid with purge",
			path:  "not-really-valid",
			purge: true,
			err:   errs.ErrBadPackNameInvalidExtension,
		},
	}

	runTests(t, tests, func(test testCase) error {
		return installer.AddPdsc(test.path)
	})
}

// Add tests to the following
// - .pack
//   - locally
//     - is public
//     - is no public
//   - from remote server
//     - is public
//     - is not public
// - .zip
//   - locally
//     - is public
//     - is no public
//   - from remote server
//     - is public
//     - is not public

// Remove packs with the following
// - Vendor.Pack
//   - existing pack installation
//   - non existing pack installation
// - Vendor.Pack.x.y.z
//   - existing pack installation
//   - non existing pack installation
