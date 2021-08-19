/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils

import (
	"encoding/xml"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	log "github.com/sirupsen/logrus"
)

// CacheDir is used for cpackget to temporarily host downloaded pack files
// before moving it to CMSIS_PACK_ROOT
var CacheDir string

// DownloadFile downloads a file from an URL and saves it locally under destionationFilePath
func DownloadFile(URL string) (string, error) {
	parsedURL, _ := url.Parse(URL)
	filePath := path.Join(CacheDir, path.Base(parsedURL.Path))
	log.Debugf("Downloading %s to %s", URL, filePath)

	out, err := os.Create(filePath)
	if err != nil {
		log.Error(err)
		return "", errs.ErrFailedCreatingFile
	}
	defer out.Close()

	resp, err := http.Get(URL) // #nosec
	if err != nil {
		log.Error(err)
		return "", errs.ErrFailedDownloadingFile
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Errorf("bad status: %s", resp.Status)
		return "", errs.ErrBadRequest
	}

	// Download file in smaller bits straight to a local file
	written, err := SecureCopy(out, resp.Body)
	log.Debugf("Downloaded %d bytes", written)

	return filePath, err
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
		log.Error(err)
		return errs.ErrFailedCreatingDirectory
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

// ListDir generates a list of files and directories in "dir".
// If pattern is specified, generates a list with matches only
func ListDir(dir, pattern string) ([]string, error) {
	regexPattern := regexp.MustCompile(`.*`)
	if pattern != "" {
		regexPattern = regexp.MustCompile(pattern)
	}

	log.Debugf("Listing files and directories in \"%v\" that match \"%v\"", dir, regexPattern)

	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if regexPattern.MatchString(path) {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// TouchFile touches the file specified by filePath.
// If the file does not exist, create it.
func TouchFile(filePath string) error {
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		file, err := os.Create(filePath)
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
	}

	currentTime := time.Now().Local()
	return os.Chtimes(filePath, currentTime, currentTime)
}

// IsEmpty tells whether a directory specified by "dir" is empty or not
func IsEmpty(dir string) bool {
	file, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer file.Close()

	_, err = file.Readdirnames(1)
	return err == io.EOF
}

// RandStringBytes returns a random string with n bytes long
// Ref: https://stackoverflow.com/a/31832326/3908350
func RandStringBytes(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))] // #nosec
	}
	return string(b)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
