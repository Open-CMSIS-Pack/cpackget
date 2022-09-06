package cryptography

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"hash"
	"os"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
)

// Hashes is the list of supported Cryptographic Hash Functions used for the checksum feature
var Hashes = [1]string{"sha256"}

// GetDigestList computes the digests of a pack according
// to the specified hash function.
func GetDigestList(sourcePack, hashFunction string) (map[string]string, error) {
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
		defer reader.Close()
		_, err = utils.SecureCopy(h, reader)
		if err != nil {
			return nil, err
		}
		digests[file.Name] = fmt.Sprintf("%x", h.Sum(nil))
	}
	return digests, nil
}

// WriteChecksumFile takes the digests of a pack
// and writes it to a .checksum file on disk
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
