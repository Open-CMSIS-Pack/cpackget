/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the vidx2pidx project. */

package utils_test

import (
	"archive/zip"
	"bufio"
	"bytes"
	"os"
	"path"
	"strings"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/stretchr/testify/assert"
)

func TestSecureCopy(t *testing.T) {
	assert := assert.New(t)

	t.Run("test fail to copy extra large files", func(t *testing.T) {
		// Reduce max copy size to 5 bytes
		currMaxDownloadSize := utils.MaxDownloadSize
		utils.MaxDownloadSize = 5
		defer func() {
			utils.MaxDownloadSize = currMaxDownloadSize
		}()

		var outBuffer bytes.Buffer
		writer := bufio.NewWriter(&outBuffer)
		reader := strings.NewReader("some content that extrapolates cpackget max copy limit")

		_, err := utils.SecureCopy(writer, reader)
		assert.NotNil(err)
		assert.True(errs.Is(err, errs.ErrFileTooBig))
	})
}

func TestSecureInflateFile(t *testing.T) {
	assert := assert.New(t)

	t.Run("test fail to inflate tainted file names", func(t *testing.T) {
		zipFile := &zip.File{}
		zipFile.Name = "../tainted-file"
		err := utils.SecureInflateFile(zipFile, "")
		assert.NotNil(err)
		assert.True(errs.Is(err, errs.ErrInsecureZipFileName))
	})

	t.Run("test inflating a directory", func(t *testing.T) {
		dirName := "test-inflate-zip-dir"
		zipFile := &zip.File{}
		zipFile.Name = dirName + "/"
		err := utils.SecureInflateFile(zipFile, "./")
		assert.Nil(err)
		defer os.Remove(dirName)

		// Make sure directory exists
		info, err := os.Stat(dirName)
		assert.Nil(err)
		assert.True(info.IsDir())
	})

	t.Run("test fail to write to inflated file", func(t *testing.T) {
		// Archive:  testdata/utils/test-secureinflatefile.zip
		//   Length      Date    Time    Name
		// ---------  ---------- -----   ----
		//        14  2021-08-20 13:44   file-to-zip
		//         0  2021-08-23 09:06   zipped-dir/file-in-folder
		zipReader, err := zip.OpenReader("../../testdata/utils/test-secureinflatefile.zip")
		assert.Nil(err)
		defer zipReader.Close()

		zipFile := zipReader.File[0]
		err = utils.SecureInflateFile(zipFile, "/")
		assert.NotNil(err)
		assert.Equal(err, errs.ErrFailedCreatingFile)

		zipFile = zipReader.File[1]
		err = utils.SecureInflateFile(zipFile, "/")
		assert.NotNil(err)
		assert.Equal(err, errs.ErrFailedCreatingDirectory)
	})

	t.Run("test inflating a file", func(t *testing.T) {
		// Archive:  testdata/utils/test-secureinflatefile.zip
		//   Length      Date    Time    Name
		// ---------  ---------- -----   ----
		//        14  2021-08-20 13:44   file-to-zip
		//         0  2021-08-23 09:06   zipped-dir/file-in-folder
		zipReader, err := zip.OpenReader("../../testdata/utils/test-secureinflatefile.zip")
		assert.Nil(err)
		defer zipReader.Close()

		outDir := "test-inflating-file"
		defer os.RemoveAll(outDir)

		// Inflate all files
		for _, file := range zipReader.File {
			assert.Nil(utils.SecureInflateFile(file, outDir))
		}

		// Make sure files are OK
		assert.True(utils.FileExists(path.Join(outDir, "file-to-zip")))
		assert.True(utils.FileExists(path.Join(outDir, "zipped-dir/file-in-folder")))
	})
}
