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

// isValidHash returns whether a hash function is
// supported or not.
func isValidHash(hashFunction string) bool {
	for _, h := range Hashes {
		if h == hashFunction {
			return true
		}
	}
	return false
}

// getChecksumList computes the digests of a pack according
// to the specified hash function.
func getChecksumList(sourcePack, hashFunction string) (map[string]string, error) {
	var h hash.Hash
	switch hashFunction {
	case "sha256":
		h = sha256.New()
	} // Default will always be "sha256" if nothing is passed

	zipReader, err := zip.OpenReader(sourcePack)
	if err != nil {
		log.Errorf("can't decompress \"%s\": %s", sourcePack, err)
		return nil, errs.ErrFailedDecompressingFile
	}

	digests := make(map[string]string)
	for _, file := range zipReader.File {
		reader, err := file.Open()
		if err != nil {
			return nil, err
		}
		// Avoid Potential DoS vulnerability via decompression bomb
		for {
			_, err = io.CopyN(h, reader, 1024)
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
		}
		digests[file.Name] = fmt.Sprintf("%x", h.Sum(nil))
	}
	return digests, nil
}

// GenerateChecksum creates a .checksum file for a pack.
func GenerateChecksum(sourcePack, destinationDir, hashFunction string) error {
	if !isValidHash(hashFunction) {
		return errs.ErrInvalidHashFunction
	}
	if !utils.FileExists(sourcePack) {
		log.Errorf("\"%s\" does not exist", sourcePack)
		return errs.ErrFileNotFound
	}

	// Checksum file path defaults to the .pack's location
	base := ""
	if destinationDir == "" {
		base = filepath.Clean(strings.TrimSuffix(sourcePack, filepath.Ext(sourcePack)))
	} else {
		if !utils.DirExists(destinationDir) {
			return errs.ErrDirectoryNotFound
		}
		base = filepath.Clean(destinationDir) + string(filepath.Separator) + strings.TrimSuffix(string(filepath.Base(sourcePack)), ".pack")
	}
	checksumFilename := base + "." + strings.Replace(hashFunction, "-", "", -1) + ".checksum"
	if utils.FileExists(checksumFilename) {
		log.Errorf("\"%s\" already exists, choose a diferent path", checksumFilename)
		return errs.ErrPathAlreadyExists
	}

	digests, err := getChecksumList(sourcePack, hashFunction)
	if err != nil {
		return err
	}

	out, err := os.Create(checksumFilename)
	if err != nil {
		log.Error(err)
		return errs.ErrFailedCreatingFile
	}
	defer out.Close()
	for filename, digest := range digests {
		_, err := out.Write([]byte(digest + " " + filename + "\n"))
		if err != nil {
			return err
		}
	}
	return nil
}

// VerifyChecksum validates the contents of a pack
// according to a provided .checksum file.
func VerifyChecksum(sourcePack, sourceChecksum string) error {
	if !utils.FileExists(sourcePack) {
		log.Errorf("\"%s\" does not exist", sourcePack)
		return errs.ErrFileNotFound
	}
	if !utils.FileExists(sourceChecksum) {
		log.Errorf("\"%s\" does not exist", sourceChecksum)
		return errs.ErrFileNotFound
	}
	hashFunction := filepath.Ext(strings.Split(sourceChecksum, ".checksum")[0])[1:]
	if !isValidHash(hashFunction) {
		log.Errorf("\"%s\" is not a valid .checksum file (correct format is [<pack>].[<hash-algorithm>].checksum). Please confirm if the algorithm is supported.", sourceChecksum)
		return errs.ErrInvalidHashFunction
	}

	// Compute pack's digests
	digests, err := getChecksumList(sourcePack, hashFunction)
	if err != nil {
		return err
	}

	// Check if pack and checksum file have the same number of files listed
	b, err := os.ReadFile(sourceChecksum)
	if err != nil {
		return err
	}
	if strings.Count(string(b), "\n") != len(digests) {
		log.Errorf("provided checksum file lists %d file(s), but pack contains %d file(s)", len(digests), strings.Count(string(b), "\n"))
		return errs.ErrIntegrityCheckFailed
	}

	// Compare with target checksum file
	checksumFile, err := os.Open(sourceChecksum)
	if err != nil {
		return err
	}
	defer checksumFile.Close()

	failure := false
	scanner := bufio.NewScanner(checksumFile)
	for scanner.Scan() {
		if scanner.Text() == "" {
			continue
		}
		targetFile := strings.Split(scanner.Text(), " ")[1]
		targetDigest := strings.Split(scanner.Text(), " ")[0]
		if digests[targetFile] != targetDigest {
			if digests[targetFile] == "" {
				log.Errorf("\"%s\" does not exist in the provided pack but is listed in the checksum file", targetFile)
				return errs.ErrIntegrityCheckFailed
			}
			log.Debugf("%s != %s", digests[targetFile], targetDigest)
			log.Errorf("%s: computed checksum did NOT match", targetFile)
			failure = true
		}
	}
	if failure {
		return errs.ErrBadPackIntegrity
	}

	log.Info("pack integrity verified, all checksums match.")
	return nil
}