package cryptography

import (
	"archive/zip"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	gopgp "github.com/ProtonMail/gopenpgp/v2/crypto"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
	"golang.org/x/term"
)

const sigVersionPrefix = "cpackget-"

// validateSignatureScheme parses and identifies a packs
// signature scheme (stored in the Zip comment field).
func validateSignatureScheme(zip *zip.ReadCloser, version string, signing bool) string {
	c := zip.Comment
	s := strings.Split(c, ":")
	// avoid out of bounds errors
	if len(s) != 3 && len(s) != 4 {
		return "empty"
	}
	// Valid signature schemes are:
	// sigVersionPrefix-(cpackget version):f:cert:signedhash -> 4 fields
	// sigVersionPrefix-(cpackget version):c:cert -> 3 fields
	// sigVersionPrefix-(cpackget version):p:pgpmessage -> 3 fields
	sv := strings.TrimPrefix(s[0], sigVersionPrefix)
	if sv == s[0] || !semver.IsValid(sv) {
		log.Debugf("signature: %s", c)
		return "invalid"
	}
	// Warn the user if the tag was made by an older cpackget version
	if utils.SemverCompare(strings.Split(sv, "-")[1][1:], strings.Split(version, "-")[0][1:]) == -1 {
		log.Warnf("This pack was signed with an older version of cpackget (%s)", sv)
	}
	if s[1] == "f" && len(s) == 4 {
		if !utils.IsBase64(s[2]) && !utils.IsBase64(s[3]) {
			// If signing, just warn the user instead of failing
			if signing {
				log.Warn("Existing \"full\" signature detected, will be overwritten")
				return "full"
			} else {
				return "invalid"
			}
		} else {
			return "full"
		}
	}
	if s[1] == "c" && len(s) == 3 {
		if !utils.IsBase64(s[2]) {
			if signing {
				log.Warn("Existing \"cert-only\" signature detected, will be overwritten")
				return "cert-only"
			} else {
				return "invalid"
			}
		} else {
			return "cert-only"
		}
	}
	if s[1] == "p" && len(s) == 3 {
		if !utils.IsBase64(s[2]) {
			if signing {
				log.Warn("Existing \"pgp\" signature detected, will be overwritten")
				return "pgp"
			} else {
				return "invalid"
			}
		} else {
			return "pgp"
		}
	}
	log.Debugf("found zip comment: %s", c)
	return "invalid"
}

// getSignField reads from a specific element of a VALID pack
// signature. No other validations are performed - it's up to the caller
// to pass a valid signature (such as calling validateSignatureScheme before).
func getSignField(signature, element string) string {
	s := strings.Split(signature, ":")
	switch element {
	case "version":
		return s[0]
	case "type":
		return s[1]
	case "certificate":
		fallthrough
	case "pubsig":
		return s[2]
	case "hash":
		return s[3]
	}
	return ""
}

// sanityCheckCertificate makes some basic validations
// against the provided X.509 certificate.
func sanityCheckCertificate(cert *x509.Certificate, vendor string) error {
	log.Info("Checking certificate's integrity and parameters ")
	// Names
	if cert.Subject.CommonName == "" {
		log.Error("Certificate's Subject Common Name (CN) is missing")
		return errs.ErrUnsafeCertificate
	}
	if vendor != "" && cert.Subject.CommonName != vendor {
		log.Error("Certificate's Subject Common Name (CN) does not match vendor name")
		return errs.ErrUnsafeCertificate
	}
	if cert.Issuer.CommonName == "" {
		log.Error("Certificate's Issuer Common Name (CN) is missing")
		return errs.ErrUnsafeCertificate
	}
	// Validity
	if time.Now().Before(cert.NotBefore) {
		log.Errorf("Certificate is only valid after %s", cert.NotBefore)
		return errs.ErrUnsafeCertificate
	}
	if time.Now().After(cert.NotAfter) {
		log.Error("Certificate has expired")
		return errs.ErrUnsafeCertificate
	}
	// Key
	if cert.PublicKeyAlgorithm.String() == "DSA" {
		log.Error("DSA keys are not supported")
		return errs.ErrUnsupportedKeyAlgo
	}
	// Usage
	if cert.IsCA {
		log.Warn("Certificate should not be a CA certificate")
	}
	ku := getKeyUsage(cert.KeyUsage)
	if len(ku) == 2 {
		if ku[0] != "\"Digital Signature\"" || ku[1] != "\"Content Commitment\"" {
			log.Warn("Does not have \"Digital Signature\" and \"Content Commitment\" key usage fields")
		}
	} else {
		log.Warn("Does not have \"Digital Signature\" and \"Content Commitment\" key usage fields")
	}
	return nil
}

// loadCertificate reads, parses and validates a X.509 certificate in PEM format.
func loadCertificate(rawCert []byte, vendor string, skipCertValidation, skipInfo bool) (*x509.Certificate, error) {
	certPEM, rest := pem.Decode(rawCert)
	if len(rest) > 0 {
		log.Warn("The provided certificate included other PEM objects, only the first was read")
	}
	if certPEM == nil {
		log.Error("Could not decode signature certificate as PEM, please check for corruption")
		log.Debugf("rest: %s", string(rest))
		return &x509.Certificate{}, errs.ErrCannotVerifySignature
	}
	certificate, err := x509.ParseCertificate(certPEM.Bytes)
	if err != nil {
		return &x509.Certificate{}, err
	}
	if !skipInfo {
		displayCertificateInfo(certificate)
	}
	if !skipCertValidation {
		log.Debugf("pack vendor identified as: %s", vendor)
		if err := sanityCheckCertificate(certificate, ""); err != nil {
			return &x509.Certificate{}, err
		}
	}
	return certificate, nil
}

// exportCertificate saves a PEM encoded x509 certificate
// to a local file.
func exportCertificate(b64Cert, path string) error {
	if utils.FileExists(path) {
		log.Error("Existing certificate found")
		return errs.ErrPathAlreadyExists
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	b64, err := base64.StdEncoding.DecodeString(b64Cert)
	if err != nil {
		return err
	}
	_, err = out.WriteString(string(b64))
	if err != nil {
		return err
	}
	log.Infof("Certificate successfully exported to %s", path)
	return nil
}

// signPackHash takes a private RSA key and PKCS1v15 signs
// the hashed zip contents of a pack.
func signPackHashX509(keyPath string, cert *x509.Certificate, hash []byte) ([]byte, error) {
	k, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	block, rest := pem.Decode([]byte(k))
	if block == nil {
		log.Error("Could not decode key as PEM, please check for corruption")
		log.Debugf("rest: %s", string(rest))
		return nil, errs.ErrBadPrivateKey
	}

	keyType, err := detectKeyType(string(k))
	if err != nil {
		return nil, err
	}
	var rsaPrivateKey *rsa.PrivateKey
	var signedHash []byte
	rng := rand.Reader
	hashed := sha256.Sum256(hash)
	// written as a switch to future proof
	// for more key types (i.e PKCS8 encrypted)
	switch keyType {
	case "PKCS1":
		b, err := isPrivateKeyFromCertificate(cert, block.Bytes, "PKCS1")
		if !b {
			log.Error("Private key does not derive from provided x509 certificate")
			return nil, err
		}
		rsaPrivateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
	case "PKCS8":
		b, err := isPrivateKeyFromCertificate(cert, block.Bytes, "PKCS8")
		if !b {
			log.Error("Private key does not derive from provided x509 certificate")
			return nil, err
		}
		pk, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaPrivateKey = pk.(*rsa.PrivateKey)
	}
	signedHash, err = rsa.SignPKCS1v15(rng, rsaPrivateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return nil, err
	}
	log.Debugf("signedHash: %s", fmt.Sprintf("%x", signedHash))
	return signedHash, nil
}

func signPackHashPGP(keyring *gopgp.KeyRing, hash []byte) (string, error) {
	message := gopgp.NewPlainMessage(hash)
	signature, err := keyring.SignDetached(message)
	if err != nil {
		return "", err
	}
	return signature.GetArmored()
}

// embedPackX509 embeds a full signature (cert + signed hash)
// by creating a new copy of the pack, copying its zipped contents and
// setting its comment field with the version:type:cert/key:(signedhash) scheme.
// Currently the new pack gets its original filename and a ".signature" extension added.
func embedPack(packFilename, version string, z *zip.ReadCloser, rawCert, signedHash []byte) error {
	// Copy each of the original zipped files to a new one
	signedPack, err := os.Create(packFilename)
	if err != nil {
		return err
	}
	defer signedPack.Close()
	w := zip.NewWriter(signedPack)
	for _, file := range z.File {
		// Read old one
		reader, err := file.Open()
		if err != nil {
			return err
		}
		defer reader.Close()
		// Copy to new
		if err = w.Copy(file); err != nil {
			return err
		}
	}
	// Write tag scheme to comment field
	signature := ""
	version = sanitizeVersionForSignature(version)
	// full
	if len(signedHash) != 0 && len(rawCert) != 0 {
		signature = version + ":f:" + base64.StdEncoding.EncodeToString([]byte(rawCert)) + ":" + base64.StdEncoding.EncodeToString(signedHash)
	} else {
		// cert-only
		if len(rawCert) != 0 {
			signature = version + ":c:" + base64.StdEncoding.EncodeToString([]byte(rawCert))
		} else {
			signature = version + ":p:" + base64.StdEncoding.EncodeToString([]byte(signedHash))
		}
	}
	log.Debugf("signature: %s", signature)
	if err = w.SetComment(signature); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return nil
}

// SignPack is the command entrypoint to the signature
// specific creation functions.
func SignPack(packPath, certPath, keyPath, outputDir, version string, certOnly, skipCertValidation, skipInfo bool) error {
	if !utils.FileExists(packPath) {
		log.Errorf("\"%s\" does not exist", packPath)
		return errs.ErrFileNotFound
	}
	// Flag validation is already performed in the command package,
	// so we can assume they make sense
	if keyPath != "" && !utils.FileExists(keyPath) {
		log.Errorf("\"%s\" does not exist", keyPath)
		return errs.ErrFileNotFound
	}
	pgp := false
	if certPath != "" {
		if !utils.FileExists(certPath) {
			log.Errorf("\"%s\" does not exist", certPath)
			return errs.ErrFileNotFound
		}
	} else {
		pgp = true
	}
	// Check for previous packs/signatures
	// Default dir is where cpackget is
	packFilenameBase := filepath.Base(packPath)
	packFilenameSigned := packFilenameBase + ".signed"
	if outputDir != "" {
		if utils.FileExists(filepath.Join(outputDir, packFilenameSigned)) {
			log.Error("Destination path would overwrite an existing signed pack")
			return errs.ErrPathAlreadyExists
		} else {
			packFilenameSigned = filepath.Join(outputDir, packFilenameSigned)
		}
	} else {
		if utils.FileExists(packFilenameSigned) {
			log.Error("Destination path would overwrite an existing signed pack")
			return errs.ErrPathAlreadyExists
		}
	}

	zip, err := zip.OpenReader(packPath)
	if err != nil {
		log.Errorf("Can't decompress \"%s\": %s", packPath, err)
		return errs.ErrFailedDecompressingFile
	}
	switch validateSignatureScheme(zip, version, true) {
	case "full":
		log.Error("\"Full\" signature found in provided pack")
		return errs.ErrAlreadySigned
	case "cert-only":
		log.Error("\"cert-only\" signature found in provided pack")
		return errs.ErrAlreadySigned
	case "pgp":
		log.Error("PGP signature found in provided pack")
		return errs.ErrAlreadySigned
	case "empty":
		log.Info("Provided pack's zip comment is empty, OK to use")
	case "invalid":
		log.Info("Provided pack's zip comment already set, will overwrite")
	}

	var keyring *gopgp.KeyRing
	var rawCert []byte
	var cert *x509.Certificate
	if pgp {
		key, err := ioutil.ReadFile(keyPath)
		if err != nil {
			return err
		}
		fmt.Printf("Enter key passphrase: \n")
		passphrase, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return err
		}
		keyring, err = getUnlockedKeyring(string(key), passphrase)
		if err != nil {
			return err
		}
	} else {
		// Load & analyze certificate
		rawCert, err = ioutil.ReadFile(certPath)
		if err != nil {
			return err
		}
		vendor := strings.Split(filepath.Base(certPath), ".")[0]
		cert, err = loadCertificate(rawCert, vendor, skipCertValidation, skipInfo)
		if err != nil {
			return err
		}
	}
	var signedHash []byte
	if !certOnly {
		// Get & sign pack hash
		hash, err := calculatePackHash(zip)
		if err != nil {
			return err
		}
		if pgp {
			s := ""
			s, err = signPackHashPGP(keyring, hash)
			signedHash = []byte(s)
		} else {
			signedHash, err = signPackHashX509(keyPath, cert, hash)
		}
		if err != nil {
			return err
		}
	}
	// Finally embed the signature onto the pack
	if err = embedPack(packFilenameSigned, version, zip, rawCert, signedHash); err != nil {
		return err
	}
	log.Infof("Successfully written signed pack %s to %s", filepath.Base(packPath), filepath.Join(outputDir, packFilenameSigned))
	return nil
}

// verifyPackFullSignature validates the integrity of a pack
// by computing its digest and verifying the embedded PKCS1v15
// signature.
func verifyPackFullSignature(zip *zip.ReadCloser, vendor, b64Cert, b64Hash string, skipCertValidation, skipInfo bool) error {
	rawCert, err := base64.StdEncoding.DecodeString(b64Cert)
	if err != nil {
		return err
	}
	hashSig, err := base64.StdEncoding.DecodeString(b64Hash)
	if err != nil {
		return err
	}
	certificate, err := loadCertificate(rawCert, vendor, skipCertValidation, skipInfo)
	if err != nil {
		return err
	}
	hashPack, err := calculatePackHash(zip)
	if err != nil {
		return err
	}
	hashPack256 := sha256.Sum256(hashPack)
	return rsa.VerifyPKCS1v15(certificate.PublicKey.(*rsa.PublicKey), crypto.SHA256, hashPack256[:], hashSig)
}

// verifyPackCertOnlySignature validates the integrity of a pack
// by performing some validations on the embed certificate.
func verifyPackCertOnlySignature(zip *zip.ReadCloser, vendor, b64Cert string, skipCertValidation, skipInfo bool) error {
	rawCert, err := base64.StdEncoding.DecodeString(b64Cert)
	if err != nil {
		return err
	}
	_, err = loadCertificate(rawCert, vendor, skipCertValidation, skipInfo)
	if err != nil {
		return err
	}
	return nil
}

// verifyPackCertOnlySignature validates the integrity of a pack
// by verifying a PGP detached signature against a public key.
func verifyPackPGPSignature(zip *zip.ReadCloser, keyPath, b64Signature string) error {
	if keyPath == "" {
		log.Error("Please provide the public key to use for verification")
		return errs.ErrCannotVerifySignature
	}
	k, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return err
	}
	s, err := base64.StdEncoding.DecodeString(b64Signature)
	if err != nil {
		return err
	}
	packHash, err := calculatePackHash(zip)
	if err != nil {
		return err
	}
	message := gopgp.NewPlainMessage(packHash)
	pgpSignature, err := gopgp.NewPGPSignatureFromArmored(string(s))
	if err != nil {
		return err
	}
	publicKeyObj, err := gopgp.NewKeyFromArmored(string(k))
	if err != nil {
		return err
	}
	signingKeyRing, err := gopgp.NewKeyRing(publicKeyObj)
	if err != nil {
		return err
	}
	return signingKeyRing.VerifyDetached(message, pgpSignature, gopgp.GetUnixTime())
}

// VerifyPackSignature is the command entrypoint to the signature
// specific validation functions.
func VerifyPackSignature(packPath, pubPath, version string, export, skipCertValidation, skipInfo bool) error {
	if !utils.FileExists(packPath) {
		log.Errorf("\"%s\" does not exist", packPath)
		return errs.ErrFileNotFound
	}
	if pubPath != "" && !utils.FileExists(pubPath) {
		log.Errorf("\"%s\" does not exist", packPath)
		return errs.ErrFileNotFound
	}
	zip, err := zip.OpenReader(packPath)
	if err != nil {
		log.Errorf("Can't decompress \"%s\": %s", packPath, err)
		return errs.ErrFailedDecompressingFile
	}

	vendor := strings.Split(filepath.Base(packPath), ".")[0]
	certPath := filepath.Base(packPath) + ".pem"
	switch validateSignatureScheme(zip, version, false) {
	case "full":
		if export {
			err := exportCertificate(getSignField(zip.Comment, "certificate"), certPath)
			if err != nil {
				return err
			}
			return nil
		}
		err := verifyPackFullSignature(zip, vendor, getSignField(zip.Comment, "certificate"), getSignField(zip.Comment, "hash"), skipCertValidation, skipInfo)
		if err != nil {
			return errs.ErrPossibleMaliciousPack
		}
	case "cert-only":
		if export {
			err := exportCertificate(getSignField(zip.Comment, "certificate"), certPath)
			if err != nil {
				return err
			}
			return nil
		}
		err := verifyPackCertOnlySignature(zip, vendor, getSignField(zip.Comment, "certificate"), skipCertValidation, skipInfo)
		if err != nil {
			return errs.ErrPossibleMaliciousPack
		}
	case "pgp":
		if err = verifyPackPGPSignature(zip, pubPath, getSignField(zip.Comment, "pubsig")); err != nil {
			return err
		}
	case "empty":
		log.Error("Pack's signature field is empty, nothing to check")
		return errs.ErrBadSignatureScheme
	case "invalid":
		return errs.ErrBadSignatureScheme
	}

	log.Info("Pack signature verification success - pack is authentic")
	return nil
}
