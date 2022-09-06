package cryptography

import (
	b64 "encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/ProtonMail/gopenpgp/v2/helper"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
)

// abortSignatureCreate wraps and prevents
// a .checksum file to be written in case of an
// error
func abortSignatureCreate(err error, checksumPath string) error {
	if r := os.Remove(checksumPath); r != nil {
		log.Error(r)
	}
	return err
}

// generatePrivateKey creates a new, PGP formatted
// private key, prompting the user for its details
// and key type. Similar to gpg's "--full-generate-key"
// option.
func generatePrivateKey() (string, []byte, error) {
	// Get key details
	var name, email string
	var k int
	fmt.Printf("Enter new key owner name: ")
	fmt.Scanln(&name)
	fmt.Printf("Enter new key email: ")
	fmt.Scanln(&email)
	fmt.Printf("Enter new key passphrase: \n")
	passphrase, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", nil, err
	}
	if strings.Count(string(passphrase), "")-1 < 1 {
		log.Error("passphrase cannot be empty")
		return "", nil, errs.ErrKeyGenerationFailure
	}
	log.Debugf("name: %s, email: %s, passphrase: %s", name, email, passphrase)
	fmt.Printf("Choose key type:\n1) Curve25519\n2) RSA\n")
	fmt.Scanln(&k)

	key := ""
	keyName := "signature_"
	switch k {
	case 1:
		key, err = helper.GenerateKey(name, email, passphrase, "x25519", 0)
		if err != nil {
			return "", nil, err
		}
		keyName = keyName + "curve25519.key"
		log.Debugf("new key name: %s", keyName)
	case 2:
		bits := 0
		fmt.Printf("Choose key size (3072 recommended):\n1) 2048\n2) 3072\n3) 4096\n")
		fmt.Scanln(&bits)
		switch bits {
		case 1:
			key, err = helper.GenerateKey(name, email, passphrase, "rsa", 2048)
			if err != nil {
				return "", nil, err
			}
			keyName = keyName + "rsa_2048.key"
			log.Debugf("new key name: %s", keyName)
		case 2:
			key, err = helper.GenerateKey(name, email, passphrase, "rsa", 3072)
			if err != nil {
				return "", nil, err
			}
			keyName = keyName + "rsa_3072.key"
			log.Debugf("new key name: %s", keyName)
		case 3:
			key, err = helper.GenerateKey(name, email, passphrase, "rsa", 4096)
			if err != nil {
				return "", nil, err
			}
			keyName = keyName + "rsa_4096.key"
			log.Debugf("new key name: %s", keyName)
		default:
			log.Error("invalid option, please choose between 1, 2 or 3")
			return "", nil, errs.ErrKeyGenerationFailure
		}
	default:
		log.Error("invalid option, please choose between 1 or 2")
		return "", nil, errs.ErrKeyGenerationFailure
	}

	if utils.FileExists(keyName) {
		log.Error("a key with the same name already exists in the current directory")
		return "", nil, errs.ErrKeyGenerationFailure
	}
	out, err := os.Create(keyName)
	if err != nil {
		return "", nil, err
	}
	_, err = out.Write([]byte(key))
	if err != nil {
		return "", nil, err
	}
	log.Infof("saved new private key to \"%s\"", keyName)
	return key, passphrase, err
}

// getUnlockedKeyring returns a ready to use
// KeyRing based on a private key
func getUnlockedKeyring(key string, passphrase []byte) (*crypto.KeyRing, error) {
	privateKeyObj, err := crypto.NewKeyFromArmored(key)
	if err != nil {
		return nil, err
	}
	unlockedKeyObj, err := privateKeyObj.Unlock(passphrase)
	if err != nil {
		return nil, err
	}
	signingKeyRing, err := crypto.NewKeyRing(unlockedKeyObj)
	if err != nil {
		return nil, err
	}
	return signingKeyRing, nil
}

// signData takes some data and signs it
// with an unlocked KeyRing
func signData(data []byte, keyring *crypto.KeyRing) (string, error) {
	message := crypto.NewPlainMessage(data)
	signature, err := keyring.SignDetached(message)
	if err != nil {
		return "", err
	}
	return signature.GetArmored()
}

// writeChecksumSignature processes and signs
// a .checksum file using an unlocked private key
func writeChecksumSignature(checksumFilename, signatureFilename, key string, passphrase []byte) (string, error) {
	log.Debugf("opening \"%s\" to create \"%s\"", checksumFilename, signatureFilename)
	chk, err := os.Open(checksumFilename)
	if err != nil {
		return "", err
	}
	defer chk.Close()
	contents, err := ioutil.ReadFile(checksumFilename)
	if err != nil {
		return "", err
	}
	keyring, err := getUnlockedKeyring(key, passphrase)
	if err != nil {
		return "", err
	}
	signatureFile, err := os.Create(signatureFilename)
	if err != nil {
		return "", errs.ErrFailedCreatingFile
	}
	signedData, err := signData(contents, keyring)
	if err != nil {
		return "", err
	}
	_, err = signatureFile.Write([]byte(signedData))
	if err != nil {
		return "", err
	}
	return signedData, nil
}

// GenerateSignature creates a .signature file based on a pack
func GenerateSignedChecksum(packPath, keyPath, destinationDir, passphrase string, base64 bool) error {
	if !utils.FileExists(packPath) {
		log.Errorf("\"%s\" does not exist", packPath)
		return errs.ErrFileNotFound
	}

	// Signature file path defaults to the .pack's location
	base := ""
	if destinationDir == "" {
		base = filepath.Clean(strings.TrimSuffix(packPath, filepath.Ext(packPath)))
	} else {
		if !utils.DirExists(destinationDir) {
			return errs.ErrDirectoryNotFound
		}
		base = filepath.Clean(destinationDir) + string(filepath.Separator) + strings.TrimSuffix(string(filepath.Base(packPath)), ".pack")
	}
	ext := base + "." + Hashes[0]
	checksumFilename := ext + ".checksum"
	signatureFilename := ext + ".signature"
	if utils.FileExists(checksumFilename) {
		log.Errorf("\"%s\" already exists, choose a different path or delete it", checksumFilename)
		return errs.ErrPathAlreadyExists
	}

	// First get digests/checksum file
	digests, err := GetDigestList(packPath, Hashes[0])
	if err != nil {
		return err
	}
	err = WriteChecksumFile(digests, checksumFilename)
	if err != nil {
		return abortSignatureCreate(err, checksumFilename)
	}

	// Generate/input private key and sign the new checksum file
	sig := ""
	if keyPath == "" {
		log.Info("No private key path provided. Creating a new key pair.")
		log.Warnf("This key pair will be saved to \"%s\". Ideally import it to the OS keyring after.", filepath.Dir(signatureFilename))
		// if utils.FileExists(signatureFilename)
		key, passphrase, err := generatePrivateKey()
		if err != nil {
			return abortSignatureCreate(err, checksumFilename)
		}
		sig, err = writeChecksumSignature(checksumFilename, signatureFilename, key, passphrase)
		if err != nil {
			return abortSignatureCreate(err, checksumFilename)
		}
	} else {
		if !utils.FileExists(keyPath) {
			log.Errorf("\"%s\" does not exist", keyPath)
			return abortSignatureCreate(errs.ErrFileNotFound, checksumFilename)
		}
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return abortSignatureCreate(err, checksumFilename)
		}
		if passphrase == "" {
			fmt.Printf("Enter key passphrase: \n")
			p, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return abortSignatureCreate(err, checksumFilename)
			}
			passphrase = string(p)
		}
		sig, err = writeChecksumSignature(checksumFilename, signatureFilename, string(key), []byte(passphrase))
		if err != nil {
			return abortSignatureCreate(err, checksumFilename)
		}
	}
	log.Infof("successfully written \"%s\"", signatureFilename)

	if base64 {
		fmt.Println(b64.StdEncoding.EncodeToString([]byte(sig)))
	}
	return nil
}

// VerifyChecksum validates a signed .checksum. Currently
// for demo purposes it validates the checksum file against
// its detached .signature and only accepts a private key,
// preferably created through "cpackget signature-create"
func VerifySignature(checksumPath, keyPath, signaturePath, passphrase string) error {
	if !utils.FileExists(checksumPath) {
		log.Errorf("\"%s\" does not exist", checksumPath)
		return errs.ErrFileNotFound
	}
	if !utils.FileExists(keyPath) {
		log.Errorf("\"%s\" does not exist", keyPath)
		return errs.ErrFileNotFound
	}
	// .signature path defaults to the .checksum's location
	if signaturePath == "" {
		signaturePath = strings.ReplaceAll(checksumPath, ".checksum", ".signature")
	}
	if !utils.FileExists(signaturePath) {
		log.Errorf("\"%s\" does not exist", signaturePath)
		return errs.ErrFileNotFound
	}

	log.Debugf("checksum file: %s, key file: %s, signature file: %s", checksumPath, keyPath, signaturePath)
	chkFile, err := os.ReadFile(checksumPath)
	if err != nil {
		return err
	}
	keyFile, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}
	sigFile, err := os.ReadFile(signaturePath)
	if err != nil {
		return err
	}

	log.Debug("passphrase was provided")
	if passphrase == "" {
		fmt.Printf("Enter key passphrase: \n")
		p, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return err
		}
		passphrase = string(p)
	}

	message := crypto.NewPlainMessage(chkFile)
	pgpSignature, err := crypto.NewPGPSignatureFromArmored(string(sigFile))
	log.Debug(pgpSignature.GetArmored())
	if err != nil {
		return err
	}
	signingKeyRing, err := getUnlockedKeyring(string(keyFile), []byte(passphrase))
	if err != nil {
		return err
	}
	err = signingKeyRing.VerifyDetached(message, pgpSignature, crypto.GetUnixTime())
	if err != nil {
		return err
	}
	log.Info("verification successfull, .checksum matches its signature")

	return nil
}
