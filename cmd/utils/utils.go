/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils

import (
	"bufio"
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
	"runtime"
	"strings"
	"time"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html/charset"
)

// FileURLToPath converts a file:// URL to a local file system path.
// It handles various formats including:
//   - file:///path/to/file (Unix)
//   - file://localhost/path/to/file
//   - file:///C:/path/to/file (Windows)
//   - file://localhost/C:/path/to/file (Windows)
//
// The function performs URL decoding and removes leading slashes on Windows
// for absolute paths with drive letters.
func FileURLToPath(fileURL string) (string, error) {
	if !strings.HasPrefix(fileURL, "file://") {
		// Not a file URL, return as is
		return fileURL, nil
	}

	// Remove file:// prefix
	path := strings.TrimPrefix(fileURL, "file://")

	// Remove localhost if present
	path = strings.TrimPrefix(path, "localhost/")
	path = strings.TrimPrefix(path, "localhost\\")

	// URL decode (handles %20 and other encoded characters)
	decodedPath, err := url.PathUnescape(path)
	if err != nil {
		return "", fmt.Errorf("failed to decode file URL %q: %w", fileURL, err)
	}

	// On Windows, remove leading slash before drive letter (e.g., /C:/ -> C:/)
	if runtime.GOOS == "windows" {
		// Match patterns like /C:/ or /c:/
		if len(decodedPath) >= 3 && decodedPath[0] == '/' &&
			((decodedPath[1] >= 'A' && decodedPath[1] <= 'Z') || (decodedPath[1] >= 'a' && decodedPath[1] <= 'z')) &&
			decodedPath[2] == ':' {
			decodedPath = decodedPath[1:]
		}
	}

	// Convert forward slashes to backslashes on Windows
	decodedPath = filepath.FromSlash(decodedPath)

	return decodedPath, nil
}

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
		//nolint:staticcheck
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

// DownloadFile downloads a file from the specified URL and saves it to a local cache directory.
// It supports optional caching, progress bar display, and configurable timeout.
//
// Parameters:
//   - URL: The URL of the file to download.
//   - useCache: If true, uses the cached file if it exists instead of downloading again.
//   - showInfo: If true, logs informational messages about the download.
//   - showProgressBar: If true, shows the progress bar during download.
//   - insecureSkipVerify: If true, skips TLS certificate verification for HTTPS downloads.
//   - timeout: The download timeout in seconds. If 0, no timeout is set.
//
// Returns:
//   - The local file path where the downloaded file is saved.
//   - An error if the download fails or the file cannot be saved.
//
// The function handles special cases for localhost HTTPS downloads by skipping TLS verification,
// retries the request without a user agent if a 404 is received, and handles cookies if a 403 is returned.
// It also supports progress reporting and secure file writing.
func DownloadFile(URL string, useCache, showInfo, showProgressBar, insecureSkipVerify bool, timeout int) (string, error) {
	parsedURL, _ := url.Parse(URL)
	fileBase := path.Base(parsedURL.Path)
	filePath := filepath.Join(CacheDir, fileBase)
	log.Debugf("Downloading %s to %s", URL, filePath)
	if useCache && FileExists(filePath) {
		log.Debugf("Download not required, using the one from cache")
		return filePath, nil
	}

	// For now, skip insecure HTTPS downloads verification only for localhost
	var tls tls.Config
	if strings.Contains(URL, "https://127.0.0.1") {
		// #nosec G402
		tls.InsecureSkipVerify = true //nolint:gosec
	} else {
		// #nosec G402
		tls.InsecureSkipVerify = insecureSkipVerify //nolint:gosec
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
	//nolint:gosec // G704: URL is provided as function parameter and validated by caller
	resp, err := client.Do(req)
	if err != nil {
		log.Error(err)
		return "", fmt.Errorf("%q: %w", URL, errs.ErrFailedDownloadingFile)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// resend GET request without user agent header
		req.Header.Del("User-Agent")
		//nolint:gosec // G704: URL is provided as function parameter and validated by caller
		resp, err = client.Do(req)
		if err != nil {
			log.Error(err)
			return "", fmt.Errorf("%q: %w", URL, errs.ErrFailedDownloadingFile)
		}
	}

	if resp.StatusCode == http.StatusForbidden {
		cookie := resp.Header.Get("Set-Cookie")
		if len(cookie) > 0 {
			// add cookie and resend GET request
			log.Debugf("Cookie: %s", cookie)
			req.Header.Add("Cookie", cookie)
			//nolint:gosec // G704: URL is provided as function parameter and validated by caller
			resp, err = client.Do(req)
			if err != nil {
				log.Error(err)
				return "", fmt.Errorf("%q: %w", URL, errs.ErrFailedDownloadingFile)
			}
		}
	}

	if resp.StatusCode != http.StatusOK {
		log.Debugf("bad status: %s", resp.Status)
		return "", fmt.Errorf("%q: %w", URL, errs.ErrBadRequest)
	}

	//nolint:gosec // G703: filePath is safely constructed using path.Base() which prevents directory traversal
	out, err := os.Create(filePath)
	if err != nil {
		log.Error(err)
		return "", errs.ErrFailedCreatingFile
	}
	defer out.Close()

	if showInfo {
		log.Infof("Downloading %s...", fileBase)
	}
	writers := []io.Writer{out}
	if log.GetLevel() != log.ErrorLevel {
		length := resp.ContentLength
		if GetEncodedProgress() {
			progressWriter := NewEncodedProgress(length, instCnt, fileBase)
			writers = append(writers, progressWriter)
			instCnt++
		} else {
			if showProgressBar && IsTerminalInteractive() {
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
		//nolint:gosec // G703: filePath is safely constructed using path.Base() which prevents directory traversal
		_ = os.Remove(filePath)
	}

	return filePath, err
}

var onlineInfo struct {
	url        string
	connStatus string
}

func CheckConnection(url string, timeOut int) error {
	if onlineInfo.url == url && onlineInfo.connStatus == "online" { // already checked
		return nil
	}
	onlineInfo.url = url
	timeout := time.Duration(timeOut) * time.Second
	client := http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get(url)
	onlineInfo.connStatus = "offline"
	if err == nil {
		onlineInfo.connStatus = "online"
		if !GetEncodedProgress() {
			log.Debugf("Respond: %v (%v)", resp.Status, onlineInfo.connStatus)
		}
	}

	if GetEncodedProgress() {
		log.Infof("[O:%v]", onlineInfo.connStatus)
	}

	if onlineInfo.connStatus == "offline" {
		return fmt.Errorf("%q: %w", url, errs.ErrOffline)
	}

	return nil
}

// FileExists checks if filePath is an actual file in the local file system
func FileExists(filePath string) bool {
	//nolint:gosec // G703: filePath parameter is from trusted callers, path validation done at entry points
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
	log.Debugf("Ensuring %q directory exists", dirName)
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
	log.Debugf("Copying file from %q to %q", source, destination)

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
	log.Debugf("Moving file from %q to %q", source, destination)

	if SameFile(source, destination) {
		return nil
	}

	UnsetReadOnly(source)

	err := os.Rename(source, destination)
	if err != nil {
		log.Errorf("Can't move file %q to %q: %s", source, destination, err)
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
	log.Debugf("Filtering by words %q", filter)

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
// to read-write mode. Should work on both Windows and Linux.
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
	// rand.Seed(time.Now().UnixNano())	// rand.Seed deprecated, not necessary anymore
	HTTPClient = &http.Client{}
}

func GetListFiles(fileName string) (args []string, err error) {
	args = []string{}
	if fileName != "" {
		log.Infof("Parsing packs urls via file %v", fileName)

		var file *os.File
		file, err = os.Open(fileName)
		if err != nil {
			if os.IsNotExist(err) {
				err = errs.ErrFileNotFound
			}
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			tmpEntry := strings.TrimSpace(scanner.Text())
			if len(tmpEntry) == 0 {
				continue
			}
			args = append(args, tmpEntry)
		}

		err = scanner.Err()
	}
	return
}
