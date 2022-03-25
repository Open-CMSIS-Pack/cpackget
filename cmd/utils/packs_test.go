/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils_test

import (
	"os"
	"path/filepath"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	name     string
	path     string
	expected utils.PackInfo
	err      error
}

func absPath(filePath string) string {
	abs, _ := filepath.Abs(filePath)
	return abs
}

func TestExtractPackInfo(t *testing.T) {
	assert := assert.New(t)
	cwd, _ := os.Getwd()
	localFilePrefix := "file://localhost/"

	var tests = []testCase{
		{
			name: "test short path bad pack name",
			path: "this-is-not-a-valid-pack-name",
			err:  errs.ErrBadPackName,
		},
		{
			name: "test short path bad pack name with invalid version",
			path: "TheVendor.ThePack.not-a-valid-version",
			err:  errs.ErrBadPackName,
		},
		{
			name: "test short path bad pack name with invalid vendor name",
			path: "not-a-valid-vendor?.ThePack",
			err:  errs.ErrBadPackName,
		},
		{
			name: "test short path bad pack name with invalid pack name",
			path: "TheVendor.not-a-valid-pack-name?",
			err:  errs.ErrBadPackName,
		},
		{
			name: "test short path successfully extract pack info",
			path: "TheVendor.ThePack.0.0.1",
			expected: utils.PackInfo{
				Vendor:       "TheVendor",
				Pack:         "ThePack",
				Version:      "0.0.1",
				ExactVersion: true,
			},
		},
		{
			name: "test short path successfully extract pack info without version",
			path: "TheVendor.ThePack",
			expected: utils.PackInfo{
				Vendor: "TheVendor",
				Pack:   "ThePack",
			},
		},
		{
			name: "test extract pack info using legacy format without version",
			path: "TheVendor::ThePack",
			expected: utils.PackInfo{
				Vendor: "TheVendor",
				Pack:   "ThePack",
			},
		},
		{
			name: "test extract pack info using legacy format with exact version",
			path: "TheVendor::ThePack@1.0.0",
			expected: utils.PackInfo{
				Vendor:       "TheVendor",
				Pack:         "ThePack",
				Version:      "1.0.0",
				ExactVersion: true,
			},
		},
		{
			name: "test extract pack info using legacy format with minimum version",
			path: "TheVendor::ThePack>=1.0.0",
			expected: utils.PackInfo{
				Vendor:       "TheVendor",
				Pack:         "ThePack",
				Version:      "1.0.0",
				ExactVersion: false,
			},
		},
		{
			name: "test extract pack info using legacy format with minimum version alternative syntax",
			path: "TheVendor::ThePack@~1.0.0",
			expected: utils.PackInfo{
				Vendor:       "TheVendor",
				Pack:         "ThePack",
				Version:      "1.0.0",
				ExactVersion: false,
			},
		},
		{
			name: "test pdsc path with bad vendor name",
			path: "not-a-valid-vendor-name?.ThePack.pdsc",
			err:  errs.ErrBadPackName,
		},
		{
			name: "test pack path with bad vendor name",
			path: "not-a-valid-vendor-name?.ThePack.0.0.1.pack",
			err:  errs.ErrBadPackName,
		},
		{
			name: "test zip path with bad vendor name",
			path: "not-a-valid-vendor-name?.ThePack.0.0.1.pdsc",
			err:  errs.ErrBadPackName,
		},
		{
			name: "test pdsc path with bad pack name",
			path: "TheVendor.not-a-valid-pack-name?.pdsc",
			err:  errs.ErrBadPackName,
		},
		{
			name: "test pack path with bad pack name",
			path: "TheVendor.not-a-valid-pack-name?.0.0.1.pack",
			err:  errs.ErrBadPackName,
		},
		{
			name: "test zip path with bad pack name",
			path: "TheVendor.not-a-valid-pack-name?.0.0.1.zip",
			err:  errs.ErrBadPackName,
		},
		{
			name: "test pack path with bad version",
			path: "TheVendor.ThePack.not-a-valid-version?.pack",
			err:  errs.ErrBadPackName,
		},
		{
			name: "test zip path with bad version",
			path: "TheVendor.ThePack.not-a-valid-version?.zip",
			err:  errs.ErrBadPackName,
		},
		{
			name: "test path with with http URL",
			path: "http://vendor.com/TheVendor.ThePack.0.0.1.pack",
			expected: utils.PackInfo{
				Vendor:    "TheVendor",
				Pack:      "ThePack",
				Version:   "0.0.1",
				Extension: "pack",
				Location:  "http://vendor.com/",
			},
		},
		{
			name: "test path with with relative path",
			path: filepath.Join("relative", "path", "to", "TheVendor.ThePack.0.0.1.pack"),
			expected: utils.PackInfo{
				Vendor:    "TheVendor",
				Pack:      "ThePack",
				Version:   "0.0.1",
				Extension: "pack",
				Location:  localFilePrefix + filepath.Join(cwd, "relative", "path", "to") + string(os.PathSeparator),
			},
		},
		{
			name: "test path with with relative path and dot-dot",
			path: filepath.Join("..", "path", "to", "TheVendor.ThePack.0.0.1.pack"),
			expected: utils.PackInfo{
				Vendor:    "TheVendor",
				Pack:      "ThePack",
				Version:   "0.0.1",
				Extension: "pack",
				Location:  localFilePrefix + absPath(filepath.Join(cwd, "..", "path", "to")) + string(os.PathSeparator),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info, err := utils.ExtractPackInfo(test.path)
			if test.err != nil {
				assert.True(errs.Is(err, test.err))
			} else {
				assert.Equal(test.expected, info)
			}
		})
	}
}
