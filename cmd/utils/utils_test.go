/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the vidx2pidx project. */

package utils_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/stretchr/testify/assert"
)

func TestDownloadFile(t *testing.T) {
	assert := assert.New(t)

	t.Run("test fail to create temporary file", func(t *testing.T) {
		oldCache := utils.CacheDir
		utils.CacheDir = "non-existant-path"
		_, err := utils.DownloadFile("http://fake.com/file.txt")
		assert.NotNil(err)
		assert.True(errs.Is(err, errs.FailedCreatingFile))
		utils.CacheDir = oldCache
	})

	t.Run("test fail with bad http location", func(t *testing.T) {
		fileName := "file.txt"
		defer os.Remove(fileName)

		_, err := utils.DownloadFile(fileName)
		assert.NotNil(err)
		assert.True(errs.Is(err, errs.FailedDownloadingFile))
	})

	t.Run("test fail with bad http request", func(t *testing.T) {
		fileName := "file.txt"
		defer os.Remove(fileName)

		notFoundServer := httptest.NewServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				},
			),
		)

		_, err := utils.DownloadFile(notFoundServer.URL + "/" + fileName)
		assert.NotNil(err)
		assert.True(errs.Is(err, errs.BadRequest))
	})

	t.Run("test fail to read data stream", func(t *testing.T) {
		fileName := "file.txt"
		defer os.Remove(fileName)

		bodyErrorServer := httptest.NewServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Length", "1")
				},
			),
		)

		_, err := utils.DownloadFile(bodyErrorServer.URL + "/" + fileName)
		assert.NotNil(err)
		assert.True(errs.Is(err, errs.FailedWrittingToLocalFile))
	})

	t.Run("test download is OK", func(t *testing.T) {
		fileName := "file.txt"
		defer os.Remove(fileName)
		goodResponse := []byte("all good")
		goodServer := httptest.NewServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, string(goodResponse))
				},
			),
		)
		url := goodServer.URL + "/" + fileName
		_, err1 := utils.DownloadFile(url)
		assert.Nil(err1)
		assert.True(utils.FileExists(fileName))
		bytes, err2 := ioutil.ReadFile(fileName)
		assert.Nil(err2)
		assert.Equal(bytes, goodResponse)
	})
}

func TestFileExists(t *testing.T) {
	assert := assert.New(t)

	t.Run("test a file that does not exist", func(t *testing.T) {
		assert.False(utils.FileExists("this-file-does-not-exist"))
	})
}

func TestEnsureDir(t *testing.T) {
	assert := assert.New(t)
	t.Run("test if directory gets created", func(t *testing.T) {
		dirName := "tmp/ensure-dir-test"
		defer func() {
			err := os.RemoveAll(dirName)
			assert.Nil(err)
		}()

		assert.Nil(utils.EnsureDir(dirName))

		// Make sure it really exists
		stat, err := os.Stat(dirName)
		assert.Nil(err)
		assert.True(stat.IsDir())
	})

	t.Run("test catch errors", func(t *testing.T) {
		dirName := "/cannot-create-this-dir"
		err := utils.EnsureDir(dirName)
		assert.NotNil(err)
		assert.True(errs.Is(err, errs.FailedCreatingDirectory))
	})
}

func TestInflateFile(t *testing.T) {
	t.Run("test inflating a directory", func(t *testing.T) {})
	t.Run("test inflating a corrupt file", func(t *testing.T) {})
	t.Run("test fail to create inflated file", func(t *testing.T) {})
	t.Run("test fail to write to inflated file", func(t *testing.T) {})
}
