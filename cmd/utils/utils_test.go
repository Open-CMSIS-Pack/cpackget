/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils_test

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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
		assert.True(errs.Is(err, errs.ErrFailedCreatingFile))
		utils.CacheDir = oldCache
	})

	t.Run("test fail with bad http location", func(t *testing.T) {
		fileName := "file.txt"
		defer os.Remove(fileName)

		_, err := utils.DownloadFile(fileName)
		assert.NotNil(err)
		assert.True(errs.Is(err, errs.ErrFailedDownloadingFile))
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
		assert.True(errs.Is(err, errs.ErrBadRequest))
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
		assert.True(errs.Is(err, errs.ErrFailedWrittingToLocalFile))
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

	t.Run("test download uses cache", func(t *testing.T) {
		fileName := "file.txt"
		defer os.Remove(fileName)
		requestCount := 0
		goodResponse := []byte("all good")
		goodServer := httptest.NewServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, string(goodResponse))
					requestCount += 1
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
		assert.Equal(1, requestCount)

		// Download it again, this time it shouldn't trigger any HTTP request
		_, err1 = utils.DownloadFile(url)
		assert.Nil(err1)
		assert.Equal(1, requestCount)
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
			err := os.RemoveAll("tmp/")
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
		assert.True(errs.Is(err, errs.ErrFailedCreatingDirectory))
	})
}

func TestCopyFile(t *testing.T) {
	assert := assert.New(t)

	t.Run("test fail if copying to same file", func(t *testing.T) {
		fileName := "dummy-file"
		err := utils.CopyFile(fileName, fileName)
		assert.NotNil(err)
		errs.Is(err, errs.ErrCopyingEqualPaths)
	})

	t.Run("test failed to open source", func(t *testing.T) {
		err := utils.CopyFile("file-that-does-not-exist", "")
		assert.NotNil(err)
		_, ok := err.(*fs.PathError)
		assert.True(ok)
		assert.Equal(err.Error(), "open file-that-does-not-exist: no such file or directory")
	})

	t.Run("test cannot create destination file", func(t *testing.T) {
		sourceFileName := "dummy-file-for-copy-test.txt"
		destinationFileName := "~/dummy-file-for-copy-test-destionation.txt"

		assert.Nil(ioutil.WriteFile(sourceFileName, []byte("hello"), 0600))
		defer os.Remove(sourceFileName)

		err := utils.CopyFile(sourceFileName, destinationFileName)
		assert.NotNil(err)
		_, ok := err.(*fs.PathError)
		assert.True(ok)
		assert.Equal(err.Error(), "open ~/dummy-file-for-copy-test-destionation.txt: no such file or directory")
	})

	t.Run("test overwrite destination file", func(t *testing.T) {
		sourceFileName := "dummy-file-for-copy-test.txt"
		sourceFileContent := []byte("hello from source")
		assert.Nil(ioutil.WriteFile(sourceFileName, sourceFileContent, 0600))
		defer os.Remove(sourceFileName)

		destinationFileName := "dummy-file-for-copy-test-destination.txt"
		destinationFileContent := []byte("hello from destination")
		assert.Nil(ioutil.WriteFile(destinationFileName, destinationFileContent, 0600))
		defer os.Remove(destinationFileName)

		err := utils.CopyFile(sourceFileName, destinationFileName)
		assert.Nil(err)

		newDestinationFileContent, err := ioutil.ReadFile(destinationFileName)
		assert.Nil(err)

		assert.Equal(sourceFileContent, newDestinationFileContent)
	})

	t.Run("test really copy one file to a new one", func(t *testing.T) {
		sourceFileName := "dummy-file-for-copy-test.txt"
		sourceFileContent := []byte("hello from source")
		assert.Nil(ioutil.WriteFile(sourceFileName, sourceFileContent, 0600))
		defer os.Remove(sourceFileName)

		destinationFileName := "dummy-file-for-copy-test-destination.txt"
		defer os.Remove(destinationFileName)

		err := utils.CopyFile(sourceFileName, destinationFileName)
		assert.Nil(err)

		destinationFileContent, err := ioutil.ReadFile(destinationFileName)
		assert.Nil(err)

		assert.Equal(sourceFileContent, destinationFileContent)
	})
}

func TestMoveFile(t *testing.T) {
	assert := assert.New(t)

	t.Run("test fail if moving to same file", func(t *testing.T) {
		fileName := "dummy-file"
		err := utils.MoveFile(fileName, fileName)
		assert.NotNil(err)
		errs.Is(err, errs.ErrMovingEqualPaths)
	})

	t.Run("test fail moving files", func(t *testing.T) {
		fileName := "dummy-file"
		err := utils.MoveFile(fileName, "new-file")
		assert.NotNil(err)
		_, ok := err.(*os.LinkError)
		assert.True(ok)
		assert.Equal(err.Error(), "rename dummy-file new-file: no such file or directory")
	})

	t.Run("test really moving files", func(t *testing.T) {
		sourceFileName := "dummy-file-for-copy-test.txt"
		sourceFileContent := []byte("hello from source")
		assert.Nil(ioutil.WriteFile(sourceFileName, sourceFileContent, 0600))
		defer os.Remove(sourceFileName)

		destinationFileName := "dummy-file-for-copy-test-destination.txt"
		defer os.Remove(destinationFileName)

		err := utils.MoveFile(sourceFileName, destinationFileName)
		assert.Nil(err)

		destinationFileContent, err := ioutil.ReadFile(destinationFileName)
		assert.Nil(err)

		assert.Equal(sourceFileContent, destinationFileContent)
	})
}

func TestReadXML(t *testing.T) {
	assert := assert.New(t)

	type dummyXML struct {
		Dummy    xml.Name `xml:"dummy"`
		Contents string   `xml:"contents"`
	}

	t.Run("test local xml file not found or fail to open", func(t *testing.T) {
		dummy := dummyXML{}
		err := utils.ReadXML("file-that-does-not-exist", &dummy)
		assert.NotNil(err)
		_, ok := err.(*fs.PathError)
		assert.True(ok)
		assert.Equal(err.Error(), "open file-that-does-not-exist: no such file or directory")
	})

	t.Run("test read malformed xml", func(t *testing.T) {
		dummy := dummyXML{}
		err := utils.ReadXML("../../testdata/MalformedPack.pidx", &dummy)
		assert.NotNil(err)
		assert.Equal(err.Error(), "XML syntax error on line 3: unexpected EOF")
	})

	t.Run("test read file", func(t *testing.T) {
		contents := "Dummy content"
		xmlFileName := "dummy.xml"
		xmlFileContent := []byte("<dummy><contents>" + contents + "</contents></dummy>")
		assert.Nil(ioutil.WriteFile(xmlFileName, xmlFileContent, 0600))
		defer os.Remove(xmlFileName)

		dummy := dummyXML{}
		assert.Nil(utils.ReadXML(xmlFileName, &dummy))

		assert.Equal(dummy.Contents, contents)
	})
}

func TestWriteXML(t *testing.T) {
	type dummyXML struct {
		Dummy    xml.Name `xml:"dummy"`
		Contents string   `xml:"contents"`
	}

	assert := assert.New(t)

	t.Run("test fail to parse xml to bytes", func(t *testing.T) {
		// Creates an unmarshable type
		unmarshable := map[string]interface{}{
			"foo": make(chan int),
		}

		err := utils.WriteXML("", unmarshable)
		assert.NotNil(err)
		assert.Equal(err.Error(), "xml: unsupported type: map[string]interface {}")
	})

	t.Run("test fail to write to empty path", func(t *testing.T) {
		err := utils.WriteXML("", new(dummyXML))
		assert.NotNil(err)
		assert.Equal(err.Error(), "open : no such file or directory")
	})

	t.Run("test fail to write to file", func(t *testing.T) {
		fileName := "~/cannot-create-this-file"
		dummy := new(dummyXML)
		err := utils.WriteXML(fileName, dummy)

		assert.Equal(err.Error(), "open ~/cannot-create-this-file: no such file or directory")

	})

	t.Run("test write to file", func(t *testing.T) {
		fileName := "test-write-xml-ok.xml"

		dummy := new(dummyXML)
		dummy.Contents = "dummy content"
		err := utils.WriteXML(fileName, dummy)
		assert.Nil(err)
		defer os.Remove(fileName)

		// Make sure content actually got written
		written, err2 := ioutil.ReadFile(fileName)
		assert.Nil(err2)

		assert.Equal(written, []byte(`<dummyXML>
 <dummy></dummy>
 <contents>dummy content</contents>
</dummyXML>`))
	})
}

func TestListDir(t *testing.T) {
	assert := assert.New(t)
	testDir := "../../testdata/utils/test-listdir/"

	t.Run("test fail listing non-existing dir", func(t *testing.T) {
		_, err := utils.ListDir("dir-does-not-exist", "")
		assert.NotNil(err)
		_, ok := err.(*fs.PathError)
		assert.True(ok)
	})

	t.Run("test find no files or dirs", func(t *testing.T) {
		files, err := utils.ListDir(testDir+"dir2/", "")
		assert.Nil(err)
		assert.Equal(files, []string{})
	})

	t.Run("test find everything", func(t *testing.T) {
		files, err := utils.ListDir(testDir, "")
		assert.Nil(err)
		assert.Equal(files, []string{
			testDir + "dir1",
			testDir + "dir2",
			testDir + "dir3",
			testDir + "file1",
			testDir + "file2",
		})
	})

	t.Run("test find with pattern", func(t *testing.T) {
		files, err := utils.ListDir(testDir, "file.*")
		assert.Nil(err)
		assert.Equal(files, []string{
			testDir + "file1",
			testDir + "file2",
		})
	})
}

func TestTouchFile(t *testing.T) {
	assert := assert.New(t)

	t.Run("test fail to access file", func(t *testing.T) {
		err := utils.TouchFile("~/cannot-access-this-file")
		assert.NotNil(err)
		_, ok := err.(*fs.PathError)
		assert.True(ok)
	})

	t.Run("test create file", func(t *testing.T) {
		fileName := "touchfile-test-file-create"
		err := utils.TouchFile(fileName)
		assert.Nil(err)
		defer os.Remove(fileName)
	})

	t.Run("test change file time", func(t *testing.T) {
		fileName := "touchfile-test-change-time"
		fileContent := []byte("")
		assert.Nil(ioutil.WriteFile(fileName, fileContent, 0600))
		defer os.Remove(fileName)

		// Set time for yesterday
		yesterday := time.Now().Add(-24 * time.Hour)
		assert.Nil(os.Chtimes(fileName, yesterday, yesterday))

		assert.Nil(utils.TouchFile(fileName))

		file, err := os.Stat(fileName)
		assert.Nil(err)
		assert.True(yesterday != file.ModTime())
	})
}

func TestIsEmpty(t *testing.T) {
	assert := assert.New(t)
	testDir := "../../testdata/utils/test-listdir/"

	t.Run("test cannot access directory", func(t *testing.T) {
		assert.False(utils.IsEmpty("dir-does-not-exist"))
	})

	t.Run("test non empty dir", func(t *testing.T) {
		assert.False(utils.IsEmpty(testDir))
	})

	t.Run("test empty dir", func(t *testing.T) {
		assert.True(utils.IsEmpty(testDir + "dir2"))
	})
}

func TestRandStringBytes(t *testing.T) {
	assert.Equal(t, 10, len(utils.RandStringBytes(10)))
}
