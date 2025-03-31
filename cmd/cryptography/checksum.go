package cryptography

import (
	"os"
	"path/filepath"
	"strings"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
)

// Hashes is the list of supported Cryptographic Hash Functions used for the checksum feature.
var Hashes = [1]string{"sha256"}

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

// WriteChecksumFile writes the digests of a pack
// and writes it to a local file
func WriteChecksumFile(digests map[string]string, filename string) error {
	out, err := os.Create(filename)
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

// GenerateChecksum creates a .checksum file for a pack.
func GenerateChecksum(sourcePack, destinationDir, hashFunction string) error {
	if !isValidHash(hashFunction) {
		return errs.ErrHashNotSupported
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
	checksumFilename := base + "." + strings.ReplaceAll(hashFunction, "-", "") + ".checksum"
	if utils.FileExists(checksumFilename) {
		log.Errorf("\"%s\" already exists, choose a diferent path", checksumFilename)
		return errs.ErrPathAlreadyExists
	}

	digests, err := getDigestList(sourcePack, hashFunction)
	if err != nil {
		return err
	}
	err = WriteChecksumFile(digests, checksumFilename)
	if err != nil {
		return err
	}
	return nil
}

// VerifyChecksum validates the contents of a pack
// according to a provided .checksum file.
func VerifyChecksum(packPath, checksumPath string) error {
	if !utils.FileExists(packPath) {
		log.Errorf("\"%s\" does not exist", packPath)
		return errs.ErrFileNotFound
	}

	// When multiple hash algos are supported,
	// some more refined logic is needed as there may
	// exist .checksums with different algos in the same dir
	if checksumPath == "" {
		for _, hash := range Hashes {
			checksumPath = strings.ReplaceAll(packPath, ".pack", "."+hash+".checksum")
			if utils.FileExists(checksumPath) {
				break
			}
		}
	}

	if !utils.FileExists(checksumPath) {
		log.Errorf("\"%s\" does not exist", checksumPath)
		return errs.ErrFileNotFound
	}
	hashFunction := filepath.Ext(strings.Split(checksumPath, ".checksum")[0])[1:]
	if !isValidHash(hashFunction) {
		return errs.ErrNotValidChecksumFile
	}

	// Compute pack's digests
	digests, err := getDigestList(packPath, hashFunction)
	if err != nil {
		return err
	}

	// Check if pack and checksum file have the same number of files listed
	b, err := os.ReadFile(checksumPath)
	checksumFile := string(b)
	if err != nil {
		return err
	}
	if strings.Count(checksumFile, "\n") != len(digests) {
		log.Errorf("provided checksum file lists %d file(s), but pack contains %d file(s)", len(digests), strings.Count(checksumFile, "\n"))
		return errs.ErrIntegrityCheckFailed
	}

	// Compare with provided checksum file
	failure := false
	lines := strings.Split(checksumFile, "\n")
	for i := 0; i < len(lines)-1; i++ {
		targetFile := strings.Split(lines[i], " ")[1]
		targetDigest := strings.Split(lines[i], " ")[0]

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
		return errs.ErrBadIntegrity
	}

	log.Info("pack integrity verified, all checksums match.")
	return nil
}
