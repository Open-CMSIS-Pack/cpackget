/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands

import (
	// "path/filepath"
	// "regexp"
	"github.com/spf13/cobra"
)

var All = []*cobra.Command {
	PackCmd,
	PdscCmd,
}

// Use this to validate args

// PackPathToPdscTag takes in a path to a pack and returns a PdscTag
// struct representation
// Valid packPath's are:
// - /path/to/dev/Vendor.Pack.pdsc
// - /path/to/local/Vendor.Pack.Version.pack (or .zip)
// - https://web.com/Vendor.Pack.Version.pack (or .zip)
// func PackPathToPdscTag(packPath string) (xml.PdscTag, error) {
// 	log.Debugf("Parsing pack path \"%s\"", packPath)
// 
// 	tag := xml.PdscTag{}
// 	url, packName := path.Split(packPath)
// 	isPdsc := false
// 	validExtensions := []string{".zip", ".pack"}
// 
// 	if strings.HasSuffix(packName, ".pdsc") {
// 		isPdsc = true
// 		validExtensions = []string{".pdsc"}
// 	}
// 
// 	details := strings.SplitAfterN(packName, ".", 3)
// 	if len(details) != 3 {
// 		return tag, errs.BadPackName
// 	}
// 
// 	extension := ""
// 	for _, validExtension := range validExtensions {
// 		if strings.HasSuffix(packName, validExtension) {
// 			extension = validExtension
// 		}
// 	}
// 
// 	if extension == "" {
// 		return tag, errs.BadPackNameInvalidExtension
// 	}
// 
// 	tag.Vendor  = strings.ReplaceAll(details[0], ".", "")
// 	tag.Name    = strings.ReplaceAll(details[1], ".", "")
// 
// 	if !isPdsc {
// 		tag.Version = strings.ReplaceAll(details[2], extension, "")
// 	}
// 
// 	// nameRegex validates pack name and pack vendor name
// 	nameRegex := regexp.MustCompile(`^[0-9a-zA-Z_\-]+$`)
// 
// 	// versionRegex validates pack version, it can be in the format referenced here: https://semver.org/#is-there-a-suggested-regular-expression-regex-to-check-a-semver-string
// 	//                                   <major>         . <minor>        . <patch>        - <quality>                                                                                       + <meta info>
// 	versionRegex := regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(0|[1-9][0-9]*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*)(\.(0|[1-9][0-9]*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*))*)?(\+[0-9a-zA-Z-]+(\.[0-9a-zA-Z-]+)*)?$`)
// 
// 	var err error
// 	if !nameRegex.MatchString(tag.Vendor) {
// 		log.Errorf("Pack vendor \"%s\" does not match %s", tag.Vendor, nameRegex)
// 		err = errs.BadPackNameInvalidVendor
// 	} else if !nameRegex.MatchString(tag.Name) {
// 		log.Errorf("Pack name \"%s\" does not match %s", tag.Name, nameRegex)
// 		err = errs.BadPackNameInvalidName
// 	} else if !isPdsc && !versionRegex.MatchString(tag.Version) {
// 		log.Errorf("Pack version \"%s\" does not match %s", tag.Version, versionRegex)
// 		err = errs.BadPackNameInvalidVersion
// 	}
// 
// 	if err != nil {
// 		return tag, err
// 	}
// 
// 	// tag.URL can be either an actual URL or a path to the local
// 	// file system. If it's the latter, make sure to fill in
// 	// in case the file is coming from the current directory
// 	if !(strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "file://")) {
// 		if !filepath.IsAbs(url) {
// 			absPath, _ := os.Getwd()
// 			url = path.Join(absPath, url)
// 			url, _ = filepath.Abs(url)
// 		}
// 
// 		url = "file://" + url
// 	}
// 
// 	tag.URL = url
// 	return tag, nil
// }
