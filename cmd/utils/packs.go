/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	log "github.com/sirupsen/logrus"
)

// namePattern specifies a regular expression that matches Pack and Vendor names.
// Ref: https://github.com/Open-CMSIS-Pack/Open-CMSIS-Pack-Spec/blob/4e2ef7dddc4bcd2a43b530d79908720c9c52da9e/schema/PACK.xsd#L1659
var namePattern = `[\-_A-Za-z0-9]+`

// nameRegex has a pre-compiled namePattern ready for use
var nameRegex = regexp.MustCompile(fmt.Sprintf("^%s$", namePattern))

// versionPattern validates pack version.
// Ref: https://github.com/Open-CMSIS-Pack/Open-CMSIS-Pack-Spec/blob/4e2ef7dddc4bcd2a43b530d79908720c9c52da9e/schema/PACK.xsd#L1672
// With little adjustments to reduce the number of capturing groups to a single one
//
//	<major> .<minor> .<patch>   - <quality>                                                                         + <meta info>
var versionPattern = `(?:\d+)\.(?:\d+)\.(?:\d+)(?:-(?:(?:\d+|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:\d+|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?:[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?`

// versionRegex pre-compiles versionPattern.
var versionRegex = regexp.MustCompile(fmt.Sprintf("^%s$", versionPattern))

// packFileNamePattern formats all possible pack files
// - Vendor.Pack.x.y.z.pack
// - Vendor.Pack.x.y.z.zip
// - Vendor.Pack.pdsc
var packFileNamePattern = fmt.Sprintf(`^(?P<vendor>%s)\.(?P<pack>%s)\.(?:(%s)\.(pack|zip)|(pdsc))$`, namePattern, namePattern, versionPattern)

// packFileNameRegex pre-compiles packFileNamePattern
var packFileNameRegex = regexp.MustCompile(packFileNamePattern)

// packIDPattern is one of the following:
// - Vendor.Pack
// - Vendor.Pack.x.y.z
// - Vendor::Pack
// - Vendor::Pack@x.y.z
// - Vendor::Pack@^x.y.z
// - Vendor::Pack@~x.y.z
// - Vendor::Pack@>=x.y.z
// - Vendor::Pack>=x.y.z
var dottedPackIDPattern = fmt.Sprintf(`^(?P<vendor>%s)\.(?P<pack>%s)(?:\.(?P<version>%s))?$`, namePattern, namePattern, versionPattern)
var legacyPackIDPattern = fmt.Sprintf(`^(?P<vendor>%s)::(?P<pack>%s)(?:(@|@\^|@~|@>=|>=)(?P<version>%s|latest))?$`, namePattern, namePattern, versionPattern)
var packIDPattern = fmt.Sprintf(`(?:%s|%s)`, dottedPackIDPattern, legacyPackIDPattern)

// packIDRegex pre-compiles packIdPattern
var packIDRegex = regexp.MustCompile(packIDPattern)

// IsVendorNameValid checks whether a pack vendor name string matches specified
// regular expression.
func IsPackVendorNameValid(vendorName string) bool {
	return nameRegex.MatchString(vendorName)
}

// IsPackNameValid checks whether a pack name string matches specified
// regular expression.
func IsPackNameValid(packName string) bool {
	return nameRegex.MatchString(packName)
}

// matchPackFileName checks whether packFileName matches packFileNamePattern.
// If so, return a list of strings matched, otherwise returns an empty list
// The matches string list should contain 4 or 5 items 0-indexed:
// - 0: entire matched string
// - 1: vendor match
// - 2: pack name match
// - 3: version match (if it's a pdsc file, version won't be present)
// - 4: extension match
func matchPackFileName(packFileName string) []string {
	matches := packFileNameRegex.FindStringSubmatch(packFileName)

	// Golang's optional regex groups generate empty group matches, need to filter them out
	nonEmpty := []string{}
	for _, group := range matches {
		if group != "" {
			nonEmpty = append(nonEmpty, group)
		}
	}

	return nonEmpty
}

// matchPackID checks whether a given string matches packIdPattern.
// The matches string list should contain 3 or 4 items 0-indexes:
// - 0: entire matched string
// - 1: vendor match
// - 2: pack name match
// - 3: pack version match (optional)
func matchPackID(packID string) []string {
	matches := packIDRegex.FindStringSubmatch(packID)

	// Golang's optional regex groups generate empty group matches, need to filter them out
	nonEmpty := []string{}
	for _, group := range matches {
		if group != "" {
			nonEmpty = append(nonEmpty, group)
		}
	}

	return nonEmpty
}

// IsPackVersion checks whether a pack version string matches specified
// regular expression
func IsPackVersionValid(packVersion string) bool {
	return versionRegex.MatchString(packVersion)
}

// The version modifiers below are helpers to determine how to
// interpret the version specified by the packID.
const (
	// Examples: Vendor::PackName@x.y.z, Vendor.PackName.x.y.z
	ExactVersion int = 0

	// Example: Vendor::PackName@latest
	LatestVersion = 1

	// Examples: Vendor::PackName, Vendor.PackName
	AnyVersion = 2

	// Example: Vendor::PackName@>=x.y.z
	GreaterVersion = 3

	// Example: Vendor::PackName@^x.y.z (the greatest version of the pack keeping the same major number)
	GreatestCompatibleVersion = 4

	// Example: Vendor::PackName@~x.y.z (the greatest version of the pack keeping the same major and minor number)
	PatchVersion = 5

	// For the <package/requirements/packages> spec only. Example: Vendor.PackName.a.b.c:x.y.z
	RangeVersion = 6
)

var versionModMap = map[string]int{
	"@":   ExactVersion,
	"@^":  GreatestCompatibleVersion,
	"@~":  PatchVersion,
	"@>=": GreaterVersion,
	">=":  GreaterVersion,
}

// PackInfo defines a basic pack information set
type PackInfo struct {
	Location, Vendor, Pack, Version, Extension string
	IsPackID                                   bool
	VersionModifier                            int
}

// ExtractPackInfo takes in a path to a pack and extracts the needed information.
// It returns an error if any information is wrong
// Valid packPath's are:
// - /path/to/dev/Vendor.Pack.pdsc
// - /path/to/local/Vendor.Pack.Version.pack (or .zip)
// - https://web.com/Vendor.Pack.Version.pack (or .zip)
// If short is true, then prepare it considering that path is in the simpler
// form of Vendor.Pack[.x.y.z], used when removing packs/pdscs.
// NOTE: a malformed packPath e.g. "my.pack" DOES look like a valid
//
//	pack name, with "my" for vendor and "pack" for pack name.
//
// The pack path can be in various formats, such as:
// - Vendor.Pack.a.b.c:x.y.z
// - Vendor.Pack.latest
// - Vendor.Pack.x.y.z.pack
// - Vendor.Pack
//
// It returns a PackInfo struct containing the extracted information and an error if any.
// The function handles both local file paths and URLs.
//
// Parameters:
// - packPath: The path to the pack, which can be a file path or a URL.
//
// Returns:
// - PackInfo: A struct containing the extracted pack information.
// - error: An error if the pack path is invalid or if there is an issue during extraction.
func ExtractPackInfo(packPath string) (PackInfo, error) {
	log.Debugf("Extracting pack info from \"%s\"", FormatVersions(packPath))

	info := PackInfo{}
	maxVersion := ""

	// Matches Vendor.Pack.a.b.c:x.y.z
	r, err := regexp.Compile(`([\-_A-Za-z0-9]+\.){4}[\-_A-Za-z0-9]+\:`)
	if err != nil {
		return info, err
	}
	if r.MatchString(packPath) {
		maxVersion = strings.Split(packPath, ":")[1]
		packPath = strings.Split(packPath, ":")[0]
	}
	// Matches Vendor.Pack.latest - small workaround for requirement packages with no version (latest is used)
	r, err = regexp.Compile(`([\-_A-Za-z0-9]+\.){2}latest`)
	if err != nil {
		return info, err
	}
	if r.MatchString(packPath) {
		packPath = strings.TrimSuffix(packPath, ".latest")
	}
	// packPath is normally either a file (Vendor.Pack.x.y.z.pack) or simply just the packID (Vendor.Pack)
	location, packName := filepath.Split(packPath)
	// Most common scenario should be the use of packId
	matches := matchPackFileName(packName)
	if len(matches) > 0 {
		info.Vendor = matches[1]
		info.Pack = matches[2]
		info.Extension = filepath.Ext(packPath)[1:]

		if len(matches) > 3 {
			info.Version = matches[3]
		}

		// location can be either a URL or a path to the local
		// file system. If it's the latter, make sure to fill in
		// in case the file is coming from the current directory
		//nolint:staticcheck // intentional logic for clarity
		if !(strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") || strings.HasPrefix(location, "file://")) {
			if !filepath.IsAbs(location) {
				if len(filepath.VolumeName(location)) == 0 && len(location) > 0 && (location[0] == '/' || location[0] == '\\') {
					location, _ = filepath.Abs(location) // relative from root, only in windows
				} else {
					absPath, _ := os.Getwd()
					location = filepath.Join(absPath, location)
					location, _ = filepath.Abs(location)
				}
			} else {
				location = filepath.Clean(location)
			}

			location = "file://localhost/" + location + string(os.PathSeparator)
		}

		// As per the specification, no path backslashes allowed
		// (found in Windows)
		location = strings.ReplaceAll(location, "\\", "/")
		info.Location = location
		log.Debugf("\"%s\" is a file name or a URL with Vendor=\"%s\", Pack=\"%s\", Version=\"%s\", Extension=\"%v\", Location=\"%s\"", packPath, info.Vendor, info.Pack, info.Version, info.Extension, location)
		return info, nil
	}

	// It's known that packPath is either a file or a url
	matches = matchPackID(packName)
	if len(matches) == 0 {
		// packPath is neither packId nor a valid pack file name
		return info, errs.ErrBadPackName
	}

	info.IsPackID = true
	info.Vendor = matches[1]
	info.Pack = matches[2]
	info.VersionModifier = AnyVersion

	if len(matches) == 4 {
		// 4 matches: [Vendor.Pack.x.y.z, Vendor, Pack, x.y.z] (dotted version)
		info.Version = matches[3]
		if maxVersion != "" {
			info.Version = info.Version + ":" + maxVersion
			info.VersionModifier = RangeVersion
		} else {
			info.VersionModifier = ExactVersion
		}
	} else if len(matches) == 5 {
		// 5 matches: [Vendor::Pack(@|@^|@~|@>=|>=)x.y.z, Vendor, Pack, (@|@^|@~|@>=|>=), x.y.z] (legacy version)
		versionModifier := matches[3]
		version := matches[4]

		info.VersionModifier = versionModMap[versionModifier]
		if version == "latest" {
			info.VersionModifier = LatestVersion
		}

		info.Version = version
	}

	log.Debugf("\"%s\" is a packID with Vendor=\"%s\", Pack=\"%s\", Version=\"%s\", VersionModifier=\"%v\"", packPath, info.Vendor, info.Pack, FormatVersions(info.Version), info.VersionModifier)
	return info, nil
}

// FormatPackVersion returns a modern representation
// of an internally versioned pack (for dependencies).
// Example: CMSIS,ARM,5.6.0:_ -> ARM::CMSIS@>=5.6.0
// Ref: https://github.com/Open-CMSIS-Pack/devtools/blob/main/tools/projmgr/docs/Manual/YML-Input-Format.md#pack-name-conventions
func FormatPackVersion(pack []string) string {
	name, vendor, version := pack[0], pack[1], pack[2]
	if version == "latest" {
		return vendor + "::" + name + "@latest"
	} else {
		if string(version[len(version)-1]) == "_" {
			// @>=<version>
			return vendor + "::" + name + "@>=" + strings.Split(version, ":")[0]
		} else {
			minVersion, maxVersion := strings.Split(version, ":")[0], strings.Split(version, ":")[1]
			if minVersion == maxVersion {
				// @<version>
				return vendor + "::" + name + "@" + strings.Split(version, ":")[0]
			} else {
				// @<minVersion>:<maxVersion> - unspecified yet, it should not cross major version boundaries
				return vendor + "::" + name + "@" + strings.Split(version, ":")[0] + ":" + strings.Split(version, ":")[1]
			}
		}
	}
}

func FormatVersions(version string) string {
	s := strings.Split(version, ":")
	if len(s) > 1 && s[1] == "_" {
		return ">=" + s[0]
	}
	return version
}
