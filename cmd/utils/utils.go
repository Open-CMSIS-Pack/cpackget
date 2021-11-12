/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils

import (
	"bytes"
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
	"strings"
	"time"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html/charset"
)

// CacheDir is used for cpackget to temporarily host downloaded pack files
// before moving it to CMSIS_PACK_ROOT
var CacheDir string

var HTTPClient *http.Client

// DownloadFile downloads a file from an URL and saves it locally under destionationFilePath
func DownloadFile(URL string) (string, error) {
	parsedURL, _ := url.Parse(URL)
	fileBase := path.Base(parsedURL.Path)
	filePath := filepath.Join(CacheDir, fileBase)
	log.Debugf("Downloading %s to %s", URL, filePath)
	if FileExists(filePath) {
		log.Debugf("Download not required, using the one from cache")
		return filePath, nil
	}

	out, err := os.Create(filePath)
	if err != nil {
		log.Error(err)
		return "", errs.ErrFailedCreatingFile
	}
	defer out.Close()

	resp, err := HTTPClient.Get(URL) // #nosec
	if err != nil {
		log.Error(err)
		return "", errs.ErrFailedDownloadingFile
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Errorf("bad status: %s", resp.Status)
		return "", errs.ErrBadRequest
	}

	writers := []io.Writer{out}
	if log.GetLevel() != log.ErrorLevel {
		progressWriter := progressbar.DefaultBytes(
			resp.ContentLength,
			"Downloading "+fileBase,
		)
		writers = append(writers, progressWriter)
	}

	// Download file in smaller bits straight to a local file
	written, err := SecureCopy(io.MultiWriter(writers...), resp.Body)
	log.Debugf("Downloaded %d bytes", written)

	if err != nil {
		_ = os.Remove(filePath)
	}

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

// DirExists checks if dirPath is an actual directory in the local file system
func DirExists(dirPath string) bool {
	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
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

	if source == destination {
		return errs.ErrCopyingEqualPaths
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

	if source == destination {
		return errs.ErrCopyingEqualPaths
	}

	err := os.Rename(source, destination)
	if err != nil {
		log.Errorf("Can't move file \"%s\" to \"%s\": %s", source, destination, err)
		return err
	}

	return nil
}

// ReadXML reads in a file into an XML struct
func ReadXML(path string, targetStruct interface{}) error {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(contents)
	decoder := xml.NewDecoder(reader)
	decoder.CharsetReader = charset.NewReaderLabel
	return decoder.Decode(targetStruct)
}

// WriteXML writes an XML struct to a file
func WriteXML(path string, targetStruct interface{}) error {
	output, err := xml.MarshalIndent(targetStruct, "", " ")
	if err != nil {
		return err
	}

	xmlText := []byte(xml.Header)
	xmlText = append(xmlText, output...)

	return ioutil.WriteFile(path, xmlText, 0600)
}

// ListDir generates a list of files and directories in "dir".
// If pattern is specified, generates a list with matches only.
// It does NOT walk subdirectories
func ListDir(dir, pattern string) ([]string, error) {
	regexPattern := regexp.MustCompile(`.*`)
	if pattern != "" {
		regexPattern = regexp.MustCompile(pattern)
	}

	log.Debugf("Listing files and directories in \"%v\" that match \"%v\"", dir, regexPattern)

	files := []string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// The target dir is always passed, skip it
		if path == dir {
			return nil
		}

		if regexPattern.MatchString(path) {
			files = append(files, path)
		}

		// Avoid digging subdirs
		if info.IsDir() {
			return filepath.SkipDir
		}

		return nil
	})

	return files, err
}

// TouchFile touches the file specified by filePath.
// If the file does not exist, create it.
// Touch also updates the modified timestamp of the file.
func TouchFile(filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		log.Error(err)
		return err
	}
	defer file.Close()

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

// CountLines returns the number of lines in a string
// Ref: https://stackoverflow.com/a/24563853
func CountLines(content string) int {
	reader := strings.NewReader(content)
	buffer := make([]byte, 32*1024)
	count := 0
	lineFeed := []byte{'\n'}

	for {
		c, err := reader.Read(buffer)
		count += bytes.Count(buffer[:c], lineFeed)

		switch {
		case err == io.EOF:
			return count

		case err != nil:
			return count
		}
	}
}

// IsTerminalInteractive tells whether or not the current terminal is
// capable of complex interactions
func IsTerminalInteractive() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// URLExists checks whether a HEAD request to url is valid or not.
func URLExists(url string) bool {
	resp, err := HTTPClient.Head(url)
	if err == nil {
		exists := resp.StatusCode/100 == 2
		resp.Body.Close()
		return exists
	}
	return false
}

func init() {
	rand.Seed(time.Now().UnixNano())
	HTTPClient = &http.Client{}
}
