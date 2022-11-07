package cryptography

import (
	"archive/zip"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"hash"
	"strings"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	errs "github.com/open-cmsis-pack/cpackget/cmd/errors"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
)

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
		return "", errs.ErrUnsupportedKeyAlgo
	default:
		log.Error("could not decode private key as PEM, please check for corruption")
		return "", errs.ErrBadPrivateKey
	}
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

// getDigestList computes the digests of a pack according
// to the specified hash function.
func getDigestList(sourcePack, hashFunction string) (map[string]string, error) {
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

// getKeyUsage prints the RFC/human friendly version
// of possible X.509 key usages (https://www.rfc-editor.org/rfc/rfc5280#section-4.2.1.3).
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

// getUnlockedKeyring returns a ready to use
// KeyRing based on a private key.
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

// isPrivateKeyFromCertificate tells whether a DER encoded key
// is the private counterpart to a X.509 certificate.
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
	return false, errs.ErrBadPrivateKey
}

// sanitizeVersionForSignature cleans the version string
// for the sig scheme as it may not be the same in builds
// from source vs an automated release.
func sanitizeVersionForSignature(version string) string {
	v := sigVersionPrefix
	if string(version[0]) != "v" {
		return v + "v" + version
	}
	return v + version
}
