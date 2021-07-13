/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package main

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"os"



	log "github.com/sirupsen/logrus"
)

func ExitOnError(err error) {
	if err != nil {
		log.Error(err.Error())
		os.Exit(-1)
	}
}

// PackPathToPdscTag takes in a path to a pack and returns a PdscTag
// struct representation
// Valid packPath's are:
// - /path/to/dev/Vendor.Pack.pdsc
// - /path/to/local/Vendor.Pack.Version.pack (or .zip)
// - https://web.com/Vendor.Pack.Version.pack (or .zip)
func PackPathToPdscTag(packPath string) (PdscTag, error) {
	log.Debugf("Parsing pack path \"%s\"", packPath)

	tag := PdscTag{}
	url, packName := path.Split(packPath)
	isPdsc := false
	validExtensions := []string{".zip", ".pack"}

	if strings.HasSuffix(packName, ".pdsc") {
		isPdsc = true
		validExtensions = []string{".pdsc"}
	}

	details := strings.SplitAfterN(packName, ".", 3)
	if len(details) != 3 {
		return tag, ErrBadPackName
	}

	extension := ""
	for _, validExtension := range validExtensions {
		if strings.HasSuffix(packName, validExtension) {
			extension = validExtension
		}
	}

	if extension == "" {
		return tag, ErrBadPackNameInvalidExtension
	}

	tag.Vendor  = strings.ReplaceAll(details[0], ".", "")
	tag.Name    = strings.ReplaceAll(details[1], ".", "")

	if !isPdsc {
		tag.Version = strings.ReplaceAll(details[2], extension, "")
	}

	// nameRegex validates pack name and pack vendor name
	nameRegex := regexp.MustCompile(`^[0-9a-zA-Z_\-]+$`)

	// versionRegex validates pack version, it can be in the format referenced here: https://semver.org/#is-there-a-suggested-regular-expression-regex-to-check-a-semver-string
	//                                   <major>         . <minor>        . <patch>        - <quality>                                                                                       + <meta info>
	versionRegex := regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(0|[1-9][0-9]*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*)(\.(0|[1-9][0-9]*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*))*)?(\+[0-9a-zA-Z-]+(\.[0-9a-zA-Z-]+)*)?$`)

	var err error
	if !nameRegex.MatchString(tag.Vendor) {
		log.Errorf("Pack vendor \"%s\" does not match %s", tag.Vendor, nameRegex)
		err = ErrBadPackNameInvalidVendor
	} else if !nameRegex.MatchString(tag.Name) {
		log.Errorf("Pack name \"%s\" does not match %s", tag.Name, nameRegex)
		err = ErrBadPackNameInvalidName
	} else if !isPdsc && !versionRegex.MatchString(tag.Version) {
		log.Errorf("Pack version \"%s\" does not match %s", tag.Version, versionRegex)
		err = ErrBadPackNameInvalidVersion
	}

	if err != nil {
		return tag, err
	}

	// tag.URL can be either an actual URL or a path to the local
	// file system. If it's the latter, make sure to fill in
	// in case the file is coming from the current directory
	if !(strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "file://")) {
		if !filepath.IsAbs(url) {
			absPath, _ := os.Getwd()
			url = path.Join(absPath, url)
			url, _ = filepath.Abs(url)
		}

		url = "file://" + url
	}

	tag.URL = url
	return tag, nil
}

// CacheDir is used for cpackget to temporarily host downloaded pack files
// before moving it to CMSIS_PACK_ROOT
var CacheDir string

// DownloadFile downloads a file from an URL and saves it locally under destionationFilePath
func DownloadFile(url string) (string, error) {
	filePath := path.Join(CacheDir, path.Base(url))
	log.Debugf("Downloading %s to %s", url, filePath)

	out, err := os.Create(filePath)
	if err != nil {
		log.Error(err)
		return "", ErrFailedCreatingFile
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		log.Error(err)
		return "", ErrFailedDownloadingFile
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Errorf("bad status: %s", resp.Status)
		return "", ErrBadRequest
	}

	// Download file in smaller bits straight to a local file
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		log.Error(err)
		return "", ErrFailedWrittingToLocalFile
	}

	log.Debugf("Downloaded %d bytes", written)

	return filePath, nil
}

// FileExists checks if filePath is an actual file in the local file system
func FileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// EnsureDir recursevily creates a directory tree if it doesn't exist already
func EnsureDir(dirName string) error {
	log.Debugf("Ensuring \"%s\" directory exists", dirName)
	err := os.MkdirAll(dirName, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func InflateFile(file *zip.File, destinationDir string) error {
	log.Debugf("Inflating \"%s\"", file.Name)

	if strings.HasSuffix(file.Name, "/") {
		return EnsureDir(path.Join(destinationDir, file.Name))
	}

	reader, err := file.Open()
	if err != nil {
		log.Errorf("Can't inflate file \"%s\": %s", file.Name, err)
		return ErrFailedInflatingFile
	}
	defer reader.Close()

	filePath := path.Join(destinationDir, file.Name)
	out, err := os.Create(filePath)
	if err != nil {
		log.Error(err)
		return ErrFailedCreatingFile
	}
	defer out.Close()

	_, err = io.Copy(out, reader)
	if err != nil {
		log.Error(err)
		return ErrFailedWrittingToLocalFile
	}

	return nil
}

// CopyFile copies the contents of source into a new file in destination
func CopyFile(source, destination string) error {
	log.Debugf("Copying file from \"%s\" to \"%s\"", source, destination)

	_, err := os.Stat(source)
	if err != nil {
		return err
	}

	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	return err
}

// MoveFile moves a file from one source to destination
func MoveFile(source, destination string) error {
	log.Debugf("Moving file from \"%s\" to \"%s\"", source, destination)

	err := os.Rename(source, destination)
	if err != nil {
		log.Errorf("Can't move file \"%s\" to \"%s\": %s", source, destination, err)
		return err
	}

	return nil
}

// ReadXML reads in a file into an XML struct
func ReadXML(path string, targetStruct interface{}) error {
	xmlFile, err := os.Open(path)
	if err != nil {
		return err
	}

	contents, err := ioutil.ReadAll(xmlFile)
	if err != nil {
		return err
	}

	if err = xml.Unmarshal(contents, targetStruct); err != nil {
		return err
	}

	return nil
}

// WriteXML writes an XML struct to a file
func WriteXML(path string, targetStruct interface{}) error {
	output, err := xml.MarshalIndent(targetStruct, "", " ")
	if err != nil {
		return err
	}

	if path == "" || path == "-" {
		os.Stdout.Write(output)
		return nil
	}

	err = ioutil.WriteFile(path, output, 0600)
	if err != nil {
		return err
	}

	return nil
}
