/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"os"

	log "github.com/sirupsen/logrus"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
)

func ExitOnError(err error) {
	if err != nil {
		log.Error(err.Error())
		os.Exit(-1)
	}
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
		return "", errs.FailedCreatingFile
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		log.Error(err)
		return "", errs.FailedDownloadingFile
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Errorf("bad status: %s", resp.Status)
		return "", errs.BadRequest
	}

	// Download file in smaller bits straight to a local file
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		log.Error(err)
		return "", errs.FailedWrittingToLocalFile
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
		return errs.FailedInflatingFile
	}
	defer reader.Close()

	filePath := path.Join(destinationDir, file.Name)
	out, err := os.Create(filePath)
	if err != nil {
		log.Error(err)
		return errs.FailedCreatingFile
	}
	defer out.Close()

	_, err = io.Copy(out, reader)
	if err != nil {
		log.Error(err)
		return errs.FailedWrittingToLocalFile
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
