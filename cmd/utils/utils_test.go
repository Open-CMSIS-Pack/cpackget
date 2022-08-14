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
	"path/filepath"
	"strings"
	"testing"
	"time"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// Copy of cmd/log.go
type LogFormatter struct{}

func (s *LogFormatter) Format(entry *log.Entry) ([]byte, error) {
	level := strings.ToUpper(entry.Level.String())
	msg := fmt.Sprintf("%s: %s\n", level[0:1], entry.Message)
	return []byte(msg), nil
}

func TestDownloadFile(t *testing.T) {
	assert := assert.New(t)

	t.Run("test fail to create temporary file", func(t *testing.T) {
		oldCache := utils.CacheDir
		utils.CacheDir = "non-existant-path"
		goodResponse := []byte("all good")
		goodServer := httptest.NewServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, string(goodResponse))
				},
			),
		)
		_, err := utils.DownloadFile(goodServer.URL+"/file.txt", 0)
		assert.NotNil(err)
		assert.True(errs.Is(err, errs.ErrFailedCreatingFile))
		utils.CacheDir = oldCache
	})

	t.Run("test fail with bad http location", func(t *testing.T) {
		fileName := "file.txt"
		defer os.Remove(fileName)

		_, err := utils.DownloadFile(fileName, 0)
		assert.NotNil(err)
		assert.True(errs.Is(err, errs.ErrFailedDownloadingFile))
	})

	t.Run("test fail with bad http request", func(t *testing.T) {
		fileName := "file.txt"

		notFoundServer := httptest.NewServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				},
			),
		)

		_, err := utils.DownloadFile(notFoundServer.URL+"/"+fileName, 0)
		assert.NotNil(err)
		assert.True(errs.Is(err, errs.ErrBadRequest))
		assert.False(utils.FileExists(fileName))
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

		_, err := utils.DownloadFile(bodyErrorServer.URL+"/"+fileName, 0)
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
		_, err1 := utils.DownloadFile(url, 0)
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
		_, err1 := utils.DownloadFile(url, 0)
		assert.Nil(err1)
		assert.True(utils.FileExists(fileName))
		bytes, err2 := ioutil.ReadFile(fileName)
		assert.Nil(err2)
		assert.Equal(bytes, goodResponse)
		assert.Equal(1, requestCount)

		// Download it again, this time it shouldn't trigger any HTTP request
		_, err1 = utils.DownloadFile(url, 0)
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

		assert.False(utils.DirExists(dirName))

		assert.Nil(utils.EnsureDir(dirName))

		// Make sure it really exists
		assert.True(utils.DirExists(dirName))
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
	})

	t.Run("test read malformed xml", func(t *testing.T) {
		dummy := dummyXML{}
		err := utils.ReadXML(filepath.Join("..", "..", "testdata", "MalformedPack.pidx"), &dummy)
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

	t.Run("test read encoding ascii", func(t *testing.T) {
		contents := "Dummy content"
		xmlFileName := "dummy.xml"
		xmlFileContent := []byte("<?xml version='1.0' encoding='ASCII'?><dummy><contents>" + contents + "</contents></dummy>")
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
		_, ok := err.(*fs.PathError)
		assert.True(ok)
	})

	t.Run("test fail to write to file", func(t *testing.T) {
		fileName := "~/cannot-create-this-file"
		dummy := new(dummyXML)
		err := utils.WriteXML(fileName, dummy)

		assert.NotNil(err)
		_, ok := err.(*fs.PathError)
		assert.True(ok)

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

		assert.Equal(written, []byte(`<?xml version="1.0" encoding="UTF-8"?>
<dummyXML>
 <dummy></dummy>
 <contents>dummy content</contents>
</dummyXML>`))
	})
}

func TestListDir(t *testing.T) {
	assert := assert.New(t)
	testDir := filepath.Join("..", "..", "testdata", "utils", "test-listdir")

	t.Run("test fail listing non-existing dir", func(t *testing.T) {
		_, err := utils.ListDir("dir-does-not-exist", "")
		assert.NotNil(err)
		_, ok := err.(*fs.PathError)
		assert.True(ok)
	})

	t.Run("test find no files or dirs", func(t *testing.T) {
		dir := "empty-dir"
		assert.Nil(os.MkdirAll(dir, 0600))
		defer os.Remove(dir)
		files, err := utils.ListDir(dir, "")
		assert.Nil(err)
		assert.Equal(files, []string{})
	})

	t.Run("test find everything", func(t *testing.T) {
		files, err := utils.ListDir(testDir, "")
		assert.Nil(err)
		assert.Equal(files, []string{
			filepath.Join(testDir, "dir1"),
			filepath.Join(testDir, "dir2"),
			filepath.Join(testDir, "dir3"),
			filepath.Join(testDir, "file1"),
			filepath.Join(testDir, "file2"),
		})
	})

	t.Run("test find with pattern", func(t *testing.T) {
		files, err := utils.ListDir(testDir, "file.*")
		assert.Nil(err)
		assert.Equal(files, []string{
			filepath.Join(testDir, "file1"),
			filepath.Join(testDir, "file2"),
		})
	})
}

func TestTouchFile(t *testing.T) {
	assert := assert.New(t)

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
	testDir := filepath.Join("..", "..", "testdata", "utils", "test-listdir")

	t.Run("test cannot access directory", func(t *testing.T) {
		assert.False(utils.IsEmpty("dir-does-not-exist"))
	})

	t.Run("test non empty dir", func(t *testing.T) {
		assert.False(utils.IsEmpty(testDir))
	})

	t.Run("test empty dir", func(t *testing.T) {
		dir := "empty-dir"
		assert.Nil(os.MkdirAll(dir, 0600))
		defer os.Remove(dir)
		assert.True(utils.IsEmpty(dir))
	})
}

func TestRandStringBytes(t *testing.T) {
	assert.Equal(t, 10, len(utils.RandStringBytes(10)))
}

func TestCountLines(t *testing.T) {
	assert.Equal(t, 3, utils.CountLines("this\nis\r\nacool\ncontent"))
}

func TestFilterPackId(t *testing.T) {
	assert.Equal(t, "", utils.FilterPackID("TheVendor::Pack@1.2.3", ""))
	assert.Equal(t, "", utils.FilterPackID("TheVendor::Pack@1.2.3", ":"))
	assert.Equal(t, "", utils.FilterPackID("TheVendor::Pack@1.2.3", "@"))
	assert.Equal(t, "TheVendor::Pack@1.2.3", utils.FilterPackID("TheVendor::Pack@1.2.3", "Vendor"))
}

func TestCleanPath(t *testing.T) {
	expected := fmt.Sprintf("c:%csome%cpath", os.PathSeparator, os.PathSeparator)
	result := utils.CleanPath(fmt.Sprintf("%cc:%csome%cpath", os.PathSeparator, os.PathSeparator, os.PathSeparator))
	assert.Equal(t, expected, result)
}

func TestSetReadOnly(t *testing.T) {
	fileName := "test-file-perms"
	defer os.Remove(fileName)

	assert.Nil(t, utils.TouchFile(fileName))
	utils.SetReadOnly(fileName)

	info, err := os.Stat(fileName)
	assert.Nil(t, err)
	permBits := info.Mode().Perm()
	assert.Equal(t, fs.FileMode(0444), permBits&0444)

	dirName := "test-dir-perms"
	defer os.Remove(dirName)

	assert.Nil(t, utils.EnsureDir(dirName))
	utils.SetReadOnly(dirName)

	info, err = os.Stat(dirName)
	assert.Nil(t, err)
	permBits = info.Mode().Perm()
	assert.Equal(t, fs.FileMode(0555), permBits&0555)
}

func TestSetReadOnlyR(t *testing.T) {
	// Create a directory structure like
	// test-dir-perms-r
	// +- sub-file
	// +- sub-dir
	//    +- sub-sub-file

	dir := "test-dir-perms-r"
	subDir := filepath.Join(dir, "sub-dir")
	subFile := filepath.Join(dir, "sub-file")
	subSubFile := filepath.Join(subDir, "sub-sub-file")
	defer os.RemoveAll(dir)

	// Create sub-dir, which also creates dir by automatically
	assert.Nil(t, utils.EnsureDir(subDir))
	assert.Nil(t, utils.TouchFile(subFile))
	assert.Nil(t, utils.TouchFile(subSubFile))

	utils.SetReadOnlyR(dir)

	// Check sub-sub-file
	info, err := os.Stat(subSubFile)
	assert.Nil(t, err)
	permBits := info.Mode().Perm()
	assert.Equal(t, fs.FileMode(0444), permBits&0444)

	// Check sub-file
	info, err = os.Stat(subFile)
	assert.Nil(t, err)
	permBits = info.Mode().Perm()
	assert.Equal(t, fs.FileMode(0444), permBits&0444)

	// Check sub-dir
	info, err = os.Stat(subDir)
	assert.Nil(t, err)
	permBits = info.Mode().Perm()
	assert.Equal(t, fs.FileMode(0555), permBits&0555)

	// Finally, check the root dir
	info, err = os.Stat(dir)
	assert.Nil(t, err)
	permBits = info.Mode().Perm()
	assert.Equal(t, fs.FileMode(0555), permBits&0555)

	// Unset read-only so testing code can remove it
	utils.UnsetReadOnlyR(dir)
}

func TestUnsetReadOnly(t *testing.T) {
	fileName := "test-file-perms-unset"
	defer os.Remove(fileName)

	assert.Nil(t, utils.TouchFile(fileName))
	utils.SetReadOnly(fileName)
	utils.UnsetReadOnly(fileName)

	info, err := os.Stat(fileName)
	assert.Nil(t, err)
	permBits := info.Mode().Perm()
	assert.Equal(t, fs.FileMode(0666), permBits&0666)

	dirName := "test-dir-perms-unset"
	defer os.Remove(dirName)

	assert.Nil(t, utils.EnsureDir(dirName))
	utils.SetReadOnly(dirName)
	utils.UnsetReadOnly(dirName)

	info, err = os.Stat(dirName)
	assert.Nil(t, err)
	permBits = info.Mode().Perm()
	assert.Equal(t, fs.FileMode(0777), permBits&0777)
}

func TestUnsetReadOnlyR(t *testing.T) {
	// Create a directory structure like
	// test-unset-dir-perms-r
	// +- sub-file
	// +- sub-dir
	//    +- sub-sub-file

	dir := "test-unset-dir-perms-r"
	subDir := filepath.Join(dir, "sub-dir")
	subFile := filepath.Join(dir, "sub-file")
	subSubFile := filepath.Join(subDir, "sub-sub-file")
	defer os.RemoveAll(dir)

	// Create sub-dir, which also creates dir by automatically
	assert.Nil(t, utils.EnsureDir(subDir))
	assert.Nil(t, utils.TouchFile(subFile))
	assert.Nil(t, utils.TouchFile(subSubFile))

	utils.UnsetReadOnlyR(dir)

	// Check sub-sub-file
	info, err := os.Stat(subSubFile)
	assert.Nil(t, err)
	permBits := info.Mode().Perm()
	assert.Equal(t, fs.FileMode(0666), permBits&0666)

	// Check sub-file
	info, err = os.Stat(subFile)
	assert.Nil(t, err)
	permBits = info.Mode().Perm()
	assert.Equal(t, fs.FileMode(0666), permBits&0666)

	// Check sub-dir
	info, err = os.Stat(subDir)
	assert.Nil(t, err)
	permBits = info.Mode().Perm()
	assert.Equal(t, fs.FileMode(0777), permBits&0777)

	// Finally, check the root dir
	info, err = os.Stat(dir)
	assert.Nil(t, err)
	permBits = info.Mode().Perm()
	assert.Equal(t, fs.FileMode(0777), permBits&0777)
}

func init() {
	logLevel := log.InfoLevel
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = log.DebugLevel
	}
	log.SetLevel(logLevel)
	log.SetFormatter(new(LogFormatter))
}
