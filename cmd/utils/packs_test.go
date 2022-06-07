/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils_test

import (
	"fmt"
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
				Vendor:          "TheVendor",
				Pack:            "ThePack",
				Version:         "0.0.1",
				VersionModifier: utils.ExactVersion,
				IsPackID:        true,
			},
		},
		{
			name: "test short path successfully extract pack info without version",
			path: "TheVendor.ThePack",
			expected: utils.PackInfo{
				Vendor:          "TheVendor",
				Pack:            "ThePack",
				VersionModifier: utils.AnyVersion,
				IsPackID:        true,
			},
		},
		{
			name: "test extract pack info using legacy format without version",
			path: "TheVendor::ThePack",
			expected: utils.PackInfo{
				Vendor:          "TheVendor",
				Pack:            "ThePack",
				VersionModifier: utils.AnyVersion,
				IsPackID:        true,
			},
		},
		{
			name: "test extract pack info using legacy format with exact version",
			path: "TheVendor::ThePack@1.0.0",
			expected: utils.PackInfo{
				Vendor:          "TheVendor",
				Pack:            "ThePack",
				Version:         "1.0.0",
				VersionModifier: utils.ExactVersion,
				IsPackID:        true,
			},
		},
		{
			name: "test extract pack info using legacy format with minimum version",
			path: "TheVendor::ThePack>=1.0.0",
			expected: utils.PackInfo{
				Vendor:          "TheVendor",
				Pack:            "ThePack",
				Version:         "1.0.0",
				VersionModifier: utils.GreaterVersion,
				IsPackID:        true,
			},
		},
		{
			name: "test extract pack info using legacy format with minimum version alternative syntax",
			path: "TheVendor::ThePack@~1.0.0",
			expected: utils.PackInfo{
				Vendor:          "TheVendor",
				Pack:            "ThePack",
				Version:         "1.0.0",
				VersionModifier: utils.GreatestCompatibleVersion,
				IsPackID:        true,
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

	// Test some extra samples extracted from semver.org
	validSemanticVersions := []string{
		"0.0.4",
		"1.0.0",
		"1.2.3",
		"01.1.1",
		"1.01.1",
		"1.1.01",
		"10.20.30",
		"01.02.03",
		"1.2.3-0123",
		"1.2.3-0123.0123",
		"1.1.2-prerelease+meta",
		"1.1.2+meta",
		"1.1.2+meta-valid",
		"1.0.0-alpha",
		"1.0.0-beta",
		"1.0.0-alpha.beta",
		"1.0.0-alpha.beta.1",
		"1.0.0-alpha.1",
		"1.0.0-alpha.1.1",
		"1.0.0-alpha.1.1.1",
		"1.0.0-alpha0.valid",
		"1.0.0-alpha.0valid",
		"1.0.0-alpha-a.b-c-somethinglong+build.1-aef.1-its-okay",
		"1.0.0-rc.1+build.1",
		"2.0.0-rc.1+build.123",
		"1.2.3-beta",
		"10.2.3-DEV-SNAPSHOT",
		"1.2.3-SNAPSHOT-123",
		"2.0.0+build.1848",
		"2.0.1-alpha.1227",
		"1.0.0-alpha+beta",
		"1.2.3----RC-SNAPSHOT.12.9.1--.12+788",
		"1.2.3----R-S.12.9.1--.12+meta",
		"1.2.3----RC-SNAPSHOT.12.9.1--.12",
		"1.0.0+0.build.1-rc.10000aaa-kk-0.1",
		"99999999999999999999999.999999999999999999.99999999999999999",
		"1.0.0-0A.is.legal",
	}
	for _, version := range validSemanticVersions {
		tests = append(tests, testCase{
			name: fmt.Sprintf("test valid sem version %s", version),
			path: fmt.Sprintf("TheVendor.ThePack.%s", version),
			expected: utils.PackInfo{
				Vendor:          "TheVendor",
				Pack:            "ThePack",
				Version:         version,
				VersionModifier: utils.ExactVersion,
				IsPackID:        true,
			},
		})

		// Just for kicks, test it against ::@ as well
		tests = append(tests, testCase{
			name: fmt.Sprintf("test valid ::@ and sem version %s", version),
			path: fmt.Sprintf("TheVendor::ThePack@%s", version),
			expected: utils.PackInfo{
				Vendor:          "TheVendor",
				Pack:            "ThePack",
				Version:         version,
				VersionModifier: utils.ExactVersion,
				IsPackID:        true,
			},
		})

		// And test it as file name
		tests = append(tests, testCase{
			name: fmt.Sprintf("test valid file name with sem version %s", version),
			path: fmt.Sprintf("TheVendor.ThePack.%s.pack", version),
			expected: utils.PackInfo{
				Vendor:    "TheVendor",
				Pack:      "ThePack",
				Version:   version,
				Extension: "pack",
				Location:  localFilePrefix + cwd + string(os.PathSeparator),
			},
		})
	}

	// Also test invalid semantic versions
	invalidSemanticVersions := []string{
		"1",
		"1.2",
		"1.1.2+.123",
		"+invalid",
		"-invalid",
		"-invalid+invalid",
		"-invalid.01",
		"alpha",
		"alpha.beta",
		"alpha.beta.1",
		"alpha.1",
		"alpha+beta",
		"alpha_beta",
		"alpha.",
		"alpha..",
		"beta",
		"1.0.0-alpha_beta",
		"-alpha.",
		"1.0.0-alpha..",
		"1.0.0-alpha..1",
		"1.0.0-alpha...1",
		"1.2.3.DEV",
		"1.2-SNAPSHOT",
		"1.2.31.2.3----RC-SNAPSHOT.12.09.1--..12+788",
		"1.2-RC-SNAPSHOT",
		"-1.0.3-gamma+b7718",
		"+justmeta",
		"9.8.7+meta+meta",
		"9.8.7-whatever+meta+meta",
		"99999999999999999999999.999999999999999999.99999999999999999----RC-SNAPSHOT.12.09.1--------------------------------..12",
	}
	for _, version := range invalidSemanticVersions {
		tests = append(tests, testCase{
			name: fmt.Sprintf("test invalid sem version %s", version),
			path: fmt.Sprintf("TheVendor.ThePack.%s", version),
			err:  errs.ErrBadPackName,
		})

		// Just for kicks, test it against ::@ as well
		tests = append(tests, testCase{
			name: fmt.Sprintf("test invalid ::@ and sem version %s", version),
			path: fmt.Sprintf("TheVendor::ThePack@%s", version),
			err:  errs.ErrBadPackName,
		})

		// And test it as file name
		tests = append(tests, testCase{
			name: fmt.Sprintf("test invalid file name with sem version %s", version),
			path: fmt.Sprintf("TheVendor.ThePack.%s.pack", version),
			err:  errs.ErrBadPackName,
		})
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
