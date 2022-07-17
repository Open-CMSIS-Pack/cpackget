package cryptography

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
)

type checksum struct {
	hashFunction, digest, filename string
}

func isValidHash(hashFunction string) bool {
	valid := false
	for _, h := range Hashes {
		if h == hashFunction {
			valid = true
		}
	}
	return valid
}

// GetChecksumList receives a list of file paths and returns
// their hashed content using either a specified function or
// the default one
func getChecksumList(baseDir string, filePathList []string, hashFunction string) ([]checksum, error) {
	digests := make([]checksum, len(filePathList))
	for i := 0; i < len(filePathList); i++ {
		if utils.DirExists(filePathList[i]) {
			continue
		}
		f, err := os.Open(filePathList[i])
		if err != nil {
			return []checksum{}, err
		}
		defer f.Close()

		var h hash.Hash
		switch hashFunction {
		case "sha256":
			h = sha256.New()
			digests[i].hashFunction = "sha256"
		default:
			h = sha256.New()
			digests[i].hashFunction = "sha256"
		}

		if _, err := io.Copy(h, f); err != nil {
			return []checksum{}, err
		}

		digests[i].digest = fmt.Sprintf("%x", h.Sum(nil))
		digests[i].filename = strings.ReplaceAll(filePathList[i], baseDir, "")
	}
	return digests, nil
}

func GenerateChecksum(sourcePack, destinationDir, hashFunction string) error {
	// Use default Cryptographic Hash Function if none is provided
	if hashFunction == "" {
		hashFunction = Hashes[0]
	}
	if !isValidHash(hashFunction) {
		return errs.ErrInvalidHashFunction
	}

	if !utils.FileExists(sourcePack) {
		log.Errorf("\"%s\" is not a valid .pack", sourcePack)
		return errs.ErrFileNotFound
	}

	// Checksum file path defaults to the .pack's location
	base := ""
	if destinationDir == "" {
		base = filepath.Clean(strings.TrimSuffix(sourcePack, filepath.Ext(sourcePack)))
	} else {
		if !utils.DirExists(destinationDir) {
			log.Errorf("\"%s\" is not a valid directory", destinationDir)
			return errs.ErrDirectoryNotFound
		}
		base = filepath.Clean(destinationDir) + string(filepath.Separator) + strings.TrimSuffix(string(filepath.Base(sourcePack)), ".pack")
	}
	checksumFilename := base + "." + strings.Replace(hashFunction, "-", "", -1) + ".checksum"

	if utils.FileExists(checksumFilename) {
		log.Errorf("\"%s\" already exists, choose a diferent path", checksumFilename)
		return errs.ErrPathAlreadyExists
	}

	// Unpack it to the same directory as the .pack
	packDir := strings.TrimSuffix(sourcePack, filepath.Ext(sourcePack)) + string(os.PathSeparator)
	if utils.DirExists(packDir) {
		log.Errorf("pack was already extracted to \"%s\"", packDir)
		return errs.ErrPathAlreadyExists
	}

	zipReader, err := zip.OpenReader(sourcePack)
	if err != nil {
		log.Errorf("can't decompress \"%s\": %s", sourcePack, err)
		return errs.ErrFailedDecompressingFile
	}
	var packFileList []string
	for _, file := range zipReader.File {
		if err := utils.SecureInflateFile(file, packDir, ""); err != nil {
			if err == errs.ErrTerminatedByUser {
				log.Infof("aborting pack extraction. Removing \"%s\"", packDir)
				return os.RemoveAll(packDir)
			}
			return err
		}
		packFileList = append(packFileList, filepath.Join(filepath.Clean(packDir), filepath.Clean(file.Name)))
	}

	digests, err := getChecksumList(packDir, packFileList, hashFunction)
	if err != nil {
		return err
	}

	out, err := os.Create(checksumFilename)
	if err != nil {
		log.Error(err)
		return errs.ErrFailedCreatingFile
	}
	defer out.Close()
	for i := 0; i < len(digests); i++ {
		_, err := out.Write([]byte(digests[i].digest + " " + digests[i].filename + "\n"))
		if err != nil {
			return err
		}
	}
	// Cleanup extracted pack
	log.Debugf("deleting \"%s\"", packDir)
	if err := os.RemoveAll(packDir); err != nil {
		return err
	}
	return nil
}

func VerifyChecksum(sourcePack, checksum string) error {
	if !utils.FileExists(sourcePack) {
		log.Errorf("\"%s\" is not a valid .pack", sourcePack)
		return errs.ErrFileNotFound
	}
	if !utils.FileExists(checksum) {
		log.Errorf("\"%s\" does not exist", checksum)
		return errs.ErrFileNotFound
	}
	hashAlgorithm := filepath.Ext(strings.Split(checksum, ".checksum")[0])[1:]
	if !isValidHash(hashAlgorithm) {
		log.Errorf("\"%s\" is not a valid .checksum file (correct format is [<pack>].[<hash-algorithm>].checksum). Please confirm if the algorithm is supported.", checksum)
		return errs.ErrInvalidHashFunction
	}

	// Unpack it to the same directory as the .pack
	packDir := strings.TrimSuffix(sourcePack, filepath.Ext(sourcePack)) + string(os.PathSeparator)
	if utils.DirExists(packDir) {
		log.Errorf("pack was already extracted to \"%s\"", packDir)
		return errs.ErrPathAlreadyExists
	}

	zipReader, err := zip.OpenReader(sourcePack)
	if err != nil {
		log.Errorf("can't decompress \"%s\": %s", sourcePack, err)
		return errs.ErrFailedDecompressingFile
	}
	var packFileList []string
	for _, file := range zipReader.File {
		if err := utils.SecureInflateFile(file, packDir, ""); err != nil {
			if err == errs.ErrTerminatedByUser {
				log.Infof("aborting pack extraction. Removing \"%s\"", packDir)
				return os.RemoveAll(packDir)
			}
			return err
		}
		packFileList = append(packFileList, filepath.Join(filepath.Clean(packDir), filepath.Clean(file.Name)))
	}

	// Calculate pack digests
	digests, err := getChecksumList(packDir, packFileList, hashAlgorithm)
	if err != nil {
		return err
	}
	// Compare with target checksum file
	checksumFile, err := os.Open(checksum)
	if err != nil {
		return err
	}
	defer checksumFile.Close()

	scanner := bufio.NewScanner(checksumFile)
	linesRead := 0
	failure := false
	for scanner.Scan() {
		// They might not be in the same order
		if scanner.Text() == "" {
			continue
		}
		targetDigest := strings.Split(scanner.Text(), " ")[0]
		targetFile := strings.Split(scanner.Text(), " ")[1]
		// The checksum file might not have the same order
		fileMatched := false
		for i := 0; i < len(digests); i++ {
			if digests[i].filename == targetFile {
				if digests[i].digest != targetDigest {
					log.Debugf("%s != %s", digests[i].digest, targetDigest)
					log.Errorf("%s: computed checksum did NOT match", targetFile)
					failure = true
				}
				fileMatched = true
			}
		}
		// Make sure all files match
		if !fileMatched {
			log.Errorf("file \"%s\" was not found in the provided checksum file", targetFile)
			return errs.ErrCorruptPack
		}
		linesRead++
	}

	// Cleanup extracted pack
	log.Debugf("deleting \"%s\"", packDir)
	if err := os.RemoveAll(packDir); err != nil {
		return err
	}

	if linesRead != len(digests) {
		log.Errorf("provided checksum file lists %d files, but pack contains %d files", linesRead, len(digests))
		return errs.ErrCorruptPack
	}
	if failure {
		return errs.ErrCorruptPack
	}

	log.Info("pack integrity verified, all checksums match.")
	return nil
}
