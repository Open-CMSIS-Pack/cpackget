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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
)

// TODO: send to utils
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
	// cpackget-vX.Y.Z:f:cert:signedhash -> 4 fields
	// cpackget-vX.Y.Z:c:cert -> 3 fields
	// cpackget-vX.Y.Z:p:pgpmessage -> 3 fields
	// TODO: check for version tag
	// Warn the user if the tag was made by an older cpackget version
	if utils.SemverCompare(strings.Split(s[0], "-")[0][1:], strings.Split(version, "-")[0][1:]) == -1 {
		log.Warnf("this pack was signed with an older version of cpackget (%s)", s[0])
	}
	if s[1] == "f" && len(s) == 4 {
		if !utils.IsBase64(s[2]) && !utils.IsBase64(s[3]) {
			// If signing, just warn the user instead of failing
			if signing {
				log.Warn("existing \"full\" signature detected, will be overwritten")
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
				log.Warn("existing \"cert-only\" signature detected, will be overwritten")
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
				log.Warn("existing \"pgp\" signature detected, will be overwritten")
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
		return s[2]
	case "hash":
		return s[3]
	}
	return ""
}

// getKeyUsage prints the RFC/human friendly version
// of possible X509 key usages (https://www.rfc-editor.org/rfc/rfc5280#section-4.2.1.3).
func getKeyUsage(k x509.KeyUsage) []string {
	getSingleUsage := func(k x509.KeyUsage) string {
		switch k {
		case x509.KeyUsageDigitalSignature:
			return "\"Digital Signature\""
		case x509.KeyUsageContentCommitment:
			return "\"Content Commitment\""
		case x509.KeyUsageKeyEncipherment:
			return "\"Key Encipherment\""
		case x509.KeyUsageDataEncipherment:
			return "\"Data Encipherment\""
		case x509.KeyUsageKeyAgreement:
			return "\"Key Agreement\""
		case x509.KeyUsageCertSign:
			return "\"Certificate Signing\""
		case x509.KeyUsageCRLSign:
			return "\"CRL Signing\""
		case x509.KeyUsageEncipherOnly:
			return "\"Encipher Only\""
		case x509.KeyUsageDecipherOnly:
			return "\"Decipher Only\""
		}
		return ""
	}

	// Simple bitmask "decoding"
	var uses []string
	for key := x509.KeyUsageDigitalSignature; key < 9; key <<= 1 {
		if k&key != 0 {
			uses = append(uses, getSingleUsage(key))
		}
	}
	return uses
}

// displayCertInfo prints the relevant fields of a certificate.
func displayCertificateInfo(cert *x509.Certificate) {
	log.Info("Loading relevant info from provided certificate")
	log.Info("To manually inspect it, use the --export/-e flag to export a copy")
	// This representation is loosely based on how Mozilla Firefox
	// represents certificate info (about:certificate?cert=...)
	log.Info("Subject:")
	log.Infof("	Country (C): %s", cert.Subject.Country)
	log.Infof("	Organization (O): %s", cert.Subject.Organization)
	log.Infof("	Common Name (CN): %s", cert.Subject.CommonName)
	log.Infof("	Alt Names: %s", cert.Subject.ExtraNames)
	log.Info("Issuer:")
	log.Infof("	Country (C): %s", cert.Issuer.Country)
	log.Infof("	Organization (O): %s", cert.Issuer.Organization)
	log.Infof("	Common Name (CN): %s", cert.Issuer.CommonName)
	log.Info("Validity:")
	log.Infof("	Not Valid Before: %s", cert.NotBefore)
	log.Infof("	Not Valid After: %s", cert.NotAfter)
	log.Info("Public key Info:")
	log.Infof("	Algorithm: %s", cert.PublicKeyAlgorithm.String())
	// Modulus size in bits, not bytes
	log.Infof("	Key Size: %d", cert.PublicKey.(*rsa.PublicKey).Size()*8)
	log.Infof("	Exponent: %d", cert.PublicKey.(*rsa.PublicKey).E)
	log.Info("Miscellaneous")
	log.Infof("	Signature Algorithm: %s", cert.SignatureAlgorithm.String())
	log.Infof("	Version: %d", cert.Version)
	log.Info("Basic Constraints")
	log.Infof("	Certificate Authority: %t", cert.IsCA)
	log.Info("Key Usages:")
	log.Infof("	Purposes: %s", getKeyUsage(cert.KeyUsage))
}

// sanityCheckCertificate makes some basic validations
// against the provided X509 certificate.
func sanityCheckCertificate(cert *x509.Certificate, vendor string) error {
	log.Info("Checking certificate's integrity and parameters ")
	// Names
	if cert.Subject.CommonName == "" {
		log.Error("certificate's Subject Common Name (CN) is missing")
		return errs.ErrSignUnsafeCertificate
	}
	// TODO: Uncomment
	if vendor != "" && cert.Subject.CommonName != vendor {
		log.Error("certificate's Subject Common Name (CN) does not match vendor name")
		return errs.ErrSignUnsafeCertificate
	}
	if cert.Issuer.CommonName == "" {
		log.Error("certificate's Issuer Common Name (CN) is missing")
		return errs.ErrSignUnsafeCertificate
	}
	// Validity
	if time.Now().Before(cert.NotBefore) {
		log.Errorf("certificate is only valid after %s", cert.NotBefore)
		return errs.ErrSignUnsafeCertificate
	}
	if time.Now().After(cert.NotAfter) {
		log.Error("certificate has expired")
		return errs.ErrSignUnsafeCertificate
	}
	// Key
	if cert.PublicKeyAlgorithm.String() == "DSA" {
		log.Error("DSA keys are not supported")
		return errs.ErrSignUnsupportedKeyAlgo
	}
	// Usage
	if cert.IsCA {
		log.Warn("certificate should not be a CA certificate")
	}
	ku := getKeyUsage(cert.KeyUsage)
	if len(ku) == 2 {
		if ku[0] != "\"Digital Signature\"" || ku[1] != "\"Content Commitment\"" {
			log.Warn("Certificate should preferably only have \"Digital Signature\" and \"Content Commitment\" key usage fields")
		}
	} else {
		log.Warn("certificate should preferably only have \"Digital Signature\" and \"Content Commitment\" key usage fields")
	}
	return nil
}

// isPrivateKeyFromCertificate tells whether a DER encoded key
// is the private counterpart to a X509 certificate.
func isPrivateKeyFromCertificate(cert *x509.Certificate, keyDER []byte, keyType string) (bool, error) {
	if keyType == "PKCS1" {
		pv, err := x509.ParsePKCS1PrivateKey(keyDER)
		if err != nil {
			return false, err
		}
		pubCert := cert.PublicKey
		return pv.PublicKey.Equal(pubCert), nil
	} else {
		if keyType == "PKCS8" {
			pv, err := x509.ParsePKCS8PrivateKey(keyDER)
			if err != nil {
				return false, err
			}
			pubCert := cert.PublicKey
			return pv.(*rsa.PrivateKey).PublicKey.Equal(pubCert), nil
		}
	}
	return false, errs.ErrSignBadPrivateKey
}

// loadCertificate reads, parses and validates a X509 certificate in PEM format.
func loadCertificate(rawCert []byte, vendor string, skipInfo, skipCertValidation bool) (*x509.Certificate, error) {
	certPEM, rest := pem.Decode(rawCert)
	if len(rest) > 0 {
		log.Warn("the provided certificate included other PEM objects, only the first was read")
	}
	if certPEM == nil {
		log.Error("could not decode signature certificate as PEM, please check for corruption")
		log.Debugf("rest: %s", string(rest))
		return &x509.Certificate{}, errs.ErrSignCannotVerify
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

// calculatePackHash hashes the contents of a zip file using the
// SHA256 algorithm and returns the underlying hash object.
func calculatePackHash(zip *zip.ReadCloser) ([]byte, error) {
	hashes := make([]byte, 0)
	h := sha256.New()
	for _, file := range zip.File {
		reader, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		_, err = utils.SecureCopy(h, reader)
		if err != nil {
			return nil, err
		}
		hashes = h.Sum(hashes)
	}
	log.Debugf("Calculated hash: %s", fmt.Sprintf("%x", hashes))
	return hashes, nil
}

// detectKeyType identifies a PEM encoded RSA private key. It can be
// either PKCS1 or PKCS8, the latter not password-protected (std crypto
// does not support it currently).
func detectKeyType(key string) (string, error) {
	switch strings.Split(key, "-----")[1] {
	case "BEGIN RSA PRIVATE KEY":
		return "PKCS1", nil
	case "BEGIN PRIVATE KEY":
		return "PKCS8", nil
	case "BEGIN ENCRYPTED PRIVATE KEY":
		log.Error("encrypted PKCSC8 private keys aren't currently supported")
		return "", errs.ErrSignUnsupportedKeyAlgo
	default:
		log.Error("could not decode private key as PEM, please check for corruption")
		return "", errs.ErrSignBadPrivateKey
	}
}

// signPackHash takes a private RSA key and PKCS1v15 signs
// the hashed zip contents of a pack.
func signPackHash(keyPath string, cert *x509.Certificate, hash []byte) ([]byte, error) {
	k, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	block, rest := pem.Decode([]byte(k))
	if block == nil {
		log.Error("could not decode key as PEM, please check for corruption")
		log.Debugf("rest: %s", string(rest))
		return nil, errs.ErrSignBadPrivateKey
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
			log.Error("private key does not derive from provided x509 certificate")
			return nil, err
		}
		rsaPrivateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
	case "PKCS8":
		b, err := isPrivateKeyFromCertificate(cert, block.Bytes, "PKCS8")
		if !b {
			log.Error("private key does not derive from provided x509 certificate")
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

// embedPackX509 embeds a full signature (cert + signed hash)
// by creating a new copy of the pack, copying its zipped contents and
// setting its comment field with the version:type:cert/key:(signedhash) scheme.
// Currently the new pack gets its original filename and a ".signature" extension added.
func embedPackX509(packFilename, version string, z *zip.ReadCloser, rawCert, signedHash []byte) error {
	// TODO: func and utils this (more general for PGP)
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
	//TODO: if signedHash != nil {
	signature = version + ":f:" + base64.StdEncoding.EncodeToString([]byte(rawCert)) + ":" + base64.StdEncoding.EncodeToString(signedHash)
	log.Debugf("signature: %s", signature)
	// }

	if err = w.SetComment(signature); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return nil
}

// X509SignPack signs a pack
// TODO: may be split in 2 (cert-only and not), also in /command
func SignPackX509(packPath, certPath, keyPath, outputDir, version string, certOnly, skipCertValidation, skipInfo bool) error {
	if !utils.FileExists(packPath) {
		log.Errorf("\"%s\" does not exist", packPath)
		return errs.ErrFileNotFound
	}
	if !utils.FileExists(keyPath) {
		log.Errorf("\"%s\" does not exist", keyPath)
		return errs.ErrFileNotFound
	}
	if !utils.FileExists(certPath) {
		log.Errorf("\"%s\" does not exist", certPath)
		return errs.ErrFileNotFound
	}
	// Check for previous packs/signatures
	// Default dir is where cpackget is
	packFilenameBase := filepath.Base(packPath)
	packFilenameSigned := packFilenameBase + ".signed"
	if outputDir != "" {
		if utils.FileExists(filepath.Join(outputDir, packFilenameSigned)) {
			return errors.New("destination path would overwrite an existing signed pack")
		} else {
			packFilenameSigned = filepath.Join(outputDir, packFilenameSigned)
		}
	} else {
		if utils.FileExists(packFilenameSigned) {
			return errors.New("destination path would overwrite an existing signed pack")
		}
	}

	zip, err := zip.OpenReader(packPath)
	if err != nil {
		log.Errorf("can't decompress \"%s\": %s", packPath, err)
		return errs.ErrFailedDecompressingFile
	}
	// TODO: split this in smaller funcs
	switch validateSignatureScheme(zip, version, true) {
	case "full":
		log.Error("full X509 signature found in provided pack")
		return errs.ErrSignAlreadySigned
	case "cert-only":
		log.Error("cert-only X509 signature found in provided pack")
		return errs.ErrSignAlreadySigned
	case "pgp":
		log.Error("PGP signature found in provided pack")
		return errs.ErrSignAlreadySigned
	case "empty":
		log.Info("provided pack's zip comment is empty, OK to use")
	case "invalid":
		log.Info("provided pack's zip comment already set, will overwrite")
	}
	// Load & analyze certificate
	rawCert, err := ioutil.ReadFile(certPath)
	if err != nil {
		return err
	}
	vendor := strings.Split(filepath.Base(certPath), ".")[0]
	cert, err := loadCertificate(rawCert, vendor, skipCertValidation, skipInfo)
	if err != nil {
		return err
	}
	// Get & sign pack hash
	hash, err := calculatePackHash(zip)
	if err != nil {
		return err
	}
	signedHash, err := signPackHash(keyPath, cert, hash)
	if err != nil {
		return err
	}
	// Finally embed the full signature onto the pack
	if err = embedPackX509(packFilenameSigned, version, zip, rawCert, signedHash); err != nil {
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

// func verifyPackCertOnlySignature() error {
// 	return nil
// }

// func verifyPackPGPSignature() error {
// 	return nil
// }

// VerifyPackSignature is the  command entrypoint to the signature
// specific validation functions.
func VerifyPackSignature(packPath, version string, export, skipCertValidation, skipInfo bool) error {
	if !utils.FileExists(packPath) {
		log.Errorf("\"%s\" does not exist", packPath)
		return errs.ErrFileNotFound
	}
	zip, err := zip.OpenReader(packPath)
	if err != nil {
		log.Errorf("can't decompress \"%s\": %s", packPath, err)
		return errs.ErrFailedDecompressingFile
	}

	vendor := strings.Split(filepath.Base(packPath), ".")[0]
	switch validateSignatureScheme(zip, version, false) {
	case "full":
		if export {
			path := filepath.Base(packPath) + ".pem"
			if utils.FileExists(path) {
				log.Error("existing certificate found")
				return errs.ErrPathAlreadyExists
			}
			err := exportCertificate(getSignField(zip.Comment, "certificate"), path)
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
		log.Info("cert-only")
	case "pgp":
		log.Error("pgp signing not yet implemented")
		return errs.ErrUnknownBehavior
	case "empty":
		log.Error("pack's signature field is empty, nothing to check")
		return errs.ErrBadSignatureScheme
	case "invalid":
		return errs.ErrBadSignatureScheme
	}

	log.Info("pack signature verification success - pack is authentic")
	return nil
}
