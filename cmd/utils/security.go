/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils

import (
	"archive/zip"
	"io"
	"os"
	"path"
	"strings"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	log "github.com/sirupsen/logrus"
)

// MaxDownloadSize determines that the max file to be downloaded. Defaults to 20G
// It prevents malicious requests from providing infinite or very long files
var MaxDownloadSize = int64(20 * 1024 * 1024 * 1024)

// DownloadBufferSize is the number of bytes to transfer from the stream to the downloaded
// file per iteration. It is 4kb
const DownloadBufferSize = 4096

// SecureCopy avoids potential DoS vulnerabilities when
// downloading a stream from a remote origin or decompressing
// a file.
// Ref: G110: Potential DoS vulnerability via decompression bomb (https://cwe.mitre.org/data/definitions/409.html)
func SecureCopy(dst io.Writer, src io.Reader) (int64, error) {
	bytesRead := int64(0)
	for {
		partialRead, err := io.CopyN(dst, src, DownloadBufferSize)

		// Check if copy limit has explode before checking for errors
		bytesRead += int64(partialRead)
		if bytesRead > MaxDownloadSize {
			log.Errorf("Attempted to copy a file over %v bytes", MaxDownloadSize)
			return bytesRead, errs.ErrFileTooBig
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			log.Error(err)
			return bytesRead, errs.ErrFailedWrittingToLocalFile
		}
	}

	return bytesRead, nil
}

// SecureInflateFile avoids potentions file traversal vulnerabilities when inflating
// compressed files. It avoids extracting files with "../"
func SecureInflateFile(file *zip.File, destinationDir string) error {
	log.Debugf("Inflating \"%s\"", file.Name)

	if strings.Contains(file.Name, ".."+string(os.PathSeparator)) {
		return errs.ErrInsecureZipFileName
	}

	if strings.HasSuffix(file.Name, "/") {
		return EnsureDir(path.Join(destinationDir, file.Name)) // #nosec
	}

	reader, _ := file.Open()
	defer reader.Close()

	filePath := path.Join(destinationDir, file.Name) // #nosec
	out, err := os.Create(filePath)
	if err != nil {
		log.Error(err)
		return errs.ErrFailedCreatingFile
	}
	defer out.Close()

	written, err := SecureCopy(out, reader)
	log.Debugf("Inflated %d bytes", written)

	return err
}
