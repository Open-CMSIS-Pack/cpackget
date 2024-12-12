/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"math"
	"math/rand"
	"net"
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

var gEncodedProgress = false
var gSkipTouch = false
var gUserAgent string

func SetEncodedProgress(encodedProgress bool) {
	gEncodedProgress = encodedProgress
}

func GetEncodedProgress() bool {
	return gEncodedProgress
}

func SetSkipTouch(skipTouch bool) {
	gSkipTouch = skipTouch
}

func GetSkipTouch() bool {
	return gSkipTouch
}

func SetUserAgent(userAgent string) {
	gUserAgent = userAgent
}

// CacheDir is used for cpackget to temporarily host downloaded pack files
// before moving it to CMSIS_PACK_ROOT
var CacheDir string

var instCnt = 0

var HTTPClient *http.Client

type TimeoutTransport struct {
	http.Transport
	RoundTripTimeout time.Duration
}

// Helper function to set timeouts on HTTP connections
// that use keep-alive connections (the most common one)
func (t *TimeoutTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	type respAndErr struct {
		resp *http.Response
		err  error
	}

	timeout := time.After(t.RoundTripTimeout)
	resp := make(chan respAndErr, 1)

	go func() {
		r, e := t.Transport.RoundTrip(req)
		resp <- respAndErr{
			resp: r,
			err:  e,
		}
	}()

	select {
	case <-timeout:
		t.Transport.CancelRequest(req)
		return nil, errs.ErrHTTPtimeout
	case r := <-resp:
		return r.resp, r.err
	}
}

var (
	// File RO (ReadOnly) and RW (Read + Write) modes
	FileModeRO = fs.FileMode(0444)
	FileModeRW = fs.FileMode(0666)

	// Directory RO (ReadOnly + Traverse) and RW (Read + Write + Traverse) modes
	DirModeRO = fs.FileMode(0555)
	DirModeRW = fs.FileMode(0777)
)

// DownloadFile downloads a file from an URL and saves it locally under destionationFilePath
func DownloadFile(URL string, timeout int) (string, error) {
	parsedURL, _ := url.Parse(URL)
	fileBase := path.Base(parsedURL.Path)
	filePath := filepath.Join(CacheDir, fileBase)
	log.Debugf("Downloading %s to %s", URL, filePath)
	if FileExists(filePath) {
		log.Debugf("Download not required, using the one from cache")
		return filePath, nil
	}

	// For now, skip insecure HTTPS downloads verification only for localhost
	var tls tls.Config
	if strings.Contains(URL, "https://127.0.0.1") {
		tls.InsecureSkipVerify = true //nolint:gosec
	} else {
		tls.InsecureSkipVerify = false
	}

	var rtt time.Duration
	if timeout == 0 {
		rtt = time.Duration(math.MaxInt64)
	} else {
		rtt = time.Second * time.Duration(timeout)
	}

	client := &http.Client{
		Transport: &TimeoutTransport{
			Transport: http.Transport{
				Dial: func(netw, addr string) (net.Conn, error) {
					return net.Dial(netw, addr)
				},
				TLSClientConfig: &tls,
				Proxy:           http.ProxyFromEnvironment,
			},
			RoundTripTimeout: rtt,
		},
	}

	req, _ := http.NewRequest("GET", URL, nil)
	req.Header.Add("User-Agent", gUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		log.Error(err)
		return "", fmt.Errorf("\"%s\": %w", URL, errs.ErrFailedDownloadingFile)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debugf("bad status: %s", resp.Status)
		return "", fmt.Errorf("\"%s\": %w", URL, errs.ErrBadRequest)
	}

	out, err := os.Create(filePath)
	if err != nil {
		log.Error(err)
		return "", errs.ErrFailedCreatingFile
	}
	defer out.Close()

	log.Infof("Downloading %s...", fileBase)
	writers := []io.Writer{out}
	if log.GetLevel() != log.ErrorLevel {
		length := resp.ContentLength
		if GetEncodedProgress() {
			progressWriter := NewEncodedProgress(length, instCnt, fileBase)
			writers = append(writers, progressWriter)
			instCnt++
		} else {
			if IsTerminalInteractive() {
				progressWriter := progressbar.DefaultBytes(length, "I:")
				writers = append(writers, progressWriter)
			}
		}
	}

	// Download file in smaller bits straight to a local file
	written, err := SecureCopy(io.MultiWriter(writers...), resp.Body)
	//	fmt.Printf("\n")
	log.Debugf("Downloaded %d bytes", written)

	if err != nil {
		out.Close()
		_ = os.Remove(filePath)
	}

	return filePath, err
}

func CheckConnection(url string, timeOut int) error {
	timeout := time.Duration(timeOut) * time.Second
	client := http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get(url)
	connStatus := "offline"
	if err != nil {
		if !GetEncodedProgress() {
			log.Info(err)
		}
	} else {
		connStatus = "online"
		if !GetEncodedProgress() {
			log.Debugf("Respond: %v (%v)", resp.Status, connStatus)
		}
	}

	if GetEncodedProgress() {
		log.Infof("[O:%v]", connStatus)
	}

	if connStatus == "offline" {
		return fmt.Errorf("\"%s\": %w", url, errs.ErrOffline)
	}

	return nil
}

// FileExists checks if filePath is an actual file in the local file system
func FileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	if info == nil || os.IsNotExist(err) {
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

func SameFile(source, destination string) bool {
	if source == destination {
		return true
	}
	srcInfo, err := os.Stat(source)
	if err != nil {
		return false
	}
	dstInfo, err := os.Stat(destination)
	if err != nil {
		return false
	}
	return os.SameFile(srcInfo, dstInfo)
}

// CopyFile copies the contents of source into a new file in destination
func CopyFile(source, destination string) error {
	log.Debugf("Copying file from \"%s\" to \"%s\"", source, destination)

	if SameFile(source, destination) {
		return nil
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

	_, err = SecureCopy(destinationFile, sourceFile)
	return err
}

// MoveFile moves a file from one source to destination
func MoveFile(source, destination string) error {
	log.Debugf("Moving file from \"%s\" to \"%s\"", source, destination)

	if SameFile(source, destination) {
		return nil
	}

	UnsetReadOnly(source)

	err := os.Rename(source, destination)
	if err != nil {
		log.Errorf("Can't move file \"%s\" to \"%s\": %s", source, destination, err)
		return err
	}

	return nil
}

// ReadXML reads in a file into an XML struct
func ReadXML(path string, targetStruct interface{}) error {
	contents, err := os.ReadFile(path)
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

	return os.WriteFile(path, xmlText, FileModeRW)
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

// IsBase64 tells whether a string is correctly b64 encoded.
func IsBase64(s string) bool {
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
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

// FilterPackId returns the original string if any of the
// received filter words are present - designed specifically to filter pack IDs
func FilterPackID(content string, filter string) string {
	log.Debugf("Filtering by words \"%s\"", filter)

	// Don't accept the separator or version char
	if filter == "" || strings.ContainsAny(filter, ":") || strings.ContainsAny(filter, "@") {
		return ""
	}

	words := strings.Split(filter, " ")
	// We're only interested in the first "word" (pack id)
	target := strings.Split(content, " ")[0]

	for w := 0; w < len(words); w++ {
		if strings.Contains(target, words[w]) {
			return target
		}
	}
	return ""
}

// IsTerminalInteractive tells whether or not the current terminal is
// capable of complex interactions
func IsTerminalInteractive() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

func CleanPath(path string) string {
	cleanPath := filepath.Clean(path)
	windowsLeadingSlashRegex := regexp.MustCompile(`^[\\/][a-zA-Z]:[\\/]`)
	if windowsLeadingSlashRegex.MatchString(cleanPath) {
		cleanPath = cleanPath[1:]
	}
	return cleanPath
}

// SetReadOnly takes in a file or directory and set it
// to read-only mode. Should work on both Windows and Linux.
func SetReadOnly(path string) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return
	}

	if !info.IsDir() {
		_ = os.Chmod(path, FileModeRO)
		return
	}

	_ = os.Chmod(path, DirModeRO)
}

// SetReadOnlyR works the same as SetReadOnly, except that it is recursive
func SetReadOnlyR(path string) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) || !info.IsDir() {
		return
	}

	// At this point all files and subdirs should be set to read-only recurisively
	// there's only one catch that files and subdirs need to be set to read-only before
	// its parent directory. This is why dirsByLevel exist. It'll help set read-only
	// permissions in leaf directories before setting it root ones
	dirsByLevel := make(map[int][]string)
	maxLevel := -1

	_ = filepath.WalkDir(path, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			_ = os.Chmod(path, FileModeRO)
		} else {
			levelCount := strings.Count(path, "/") + strings.Count(path, "\\")
			dirsByLevel[levelCount] = append(dirsByLevel[levelCount], path)
			if levelCount > maxLevel {
				maxLevel = levelCount
			}
		}

		return nil
	})

	// Now set all directories to read-only, from bottom-up
	for level := maxLevel; level >= 0; level-- {
		if dirs, ok := dirsByLevel[level]; ok {
			for _, dir := range dirs {
				_ = os.Chmod(dir, DirModeRO)
			}
		}
	}
}

// UnsetReadOnly takes in a file or directory and set it
// to read-only mode. Should work on both Windows and Linux.
func UnsetReadOnly(path string) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return
	}

	mode := FileModeRW
	if info.IsDir() {
		mode = DirModeRW
	}
	_ = os.Chmod(path, mode)
}

// UnsetReadOnlyR works the same as UnsetReadOnly, but recursive
func UnsetReadOnlyR(path string) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) || !info.IsDir() {
		return
	}

	_ = filepath.WalkDir(path, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		mode := FileModeRW
		if info.IsDir() {
			mode = DirModeRW
		}
		_ = os.Chmod(path, mode)

		return nil
	})
}

func init() {
	rand.Seed(time.Now().UnixNano())
	HTTPClient = &http.Client{}
}
