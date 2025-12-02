/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package cryptography

import (
	"archive/zip"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Helper function to create a zip file with a comment
func createTestZipWithComment(t *testing.T, dir, comment string, files map[string]string) string {
	zipPath := filepath.Join(dir, "test.zip")
	zipFile, err := os.Create(zipPath)
	assert.Nil(t, err)
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for name, content := range files {
		writer, err := zipWriter.Create(name)
		assert.Nil(t, err)
		_, err = writer.Write([]byte(content))
		assert.Nil(t, err)
	}

	err = zipWriter.SetComment(comment)
	assert.Nil(t, err)

	zipWriter.Close()
	zipFile.Close()

	return zipPath
}

// Helper to create a certificate with specific properties
func createCertificateWithProperties(t *testing.T, cn, issuerCN string, notBefore, notAfter time.Time, keyUsage x509.KeyUsage, isCA bool) (*x509.Certificate, *rsa.PrivateKey) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.Nil(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: cn,
		},
		Issuer: pkix.Name{
			CommonName: issuerCN,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	assert.Nil(t, err)

	cert, err := x509.ParseCertificate(certDER)
	assert.Nil(t, err)

	return cert, privateKey
}

// TestValidateSignatureScheme tests the validateSignatureScheme function
func TestValidateSignatureScheme(t *testing.T) {
	assert := assert.New(t)
	localTestingDir := t.TempDir()

	t.Run("empty comment", func(t *testing.T) {
		zipPath := createTestZipWithComment(t, localTestingDir, "", map[string]string{"test.txt": "content"})
		zipReader, err := zip.OpenReader(zipPath)
		assert.Nil(err)
		defer zipReader.Close()

		result := validateSignatureScheme(zipReader, "v1.0.0-test", false)
		assert.Equal("empty", result)
	})

	t.Run("valid full signature", func(t *testing.T) {
		comment := "cpackget-v1.0.0-test:f:" + base64.StdEncoding.EncodeToString([]byte("cert")) + ":" + base64.StdEncoding.EncodeToString([]byte("hash"))
		zipPath := createTestZipWithComment(t, localTestingDir, comment, map[string]string{"test.txt": "content"})
		zipReader, err := zip.OpenReader(zipPath)
		assert.Nil(err)
		defer zipReader.Close()

		result := validateSignatureScheme(zipReader, "v1.0.0-test", false)
		assert.Equal("full", result)
	})

	t.Run("valid cert-only signature", func(t *testing.T) {
		comment := "cpackget-v1.0.0-test:c:" + base64.StdEncoding.EncodeToString([]byte("cert"))
		zipPath := createTestZipWithComment(t, localTestingDir, comment, map[string]string{"test.txt": "content"})
		zipReader, err := zip.OpenReader(zipPath)
		assert.Nil(err)
		defer zipReader.Close()

		result := validateSignatureScheme(zipReader, "v1.0.0-test", false)
		assert.Equal("cert-only", result)
	})

	t.Run("valid pgp signature", func(t *testing.T) {
		comment := "cpackget-v1.0.0-test:p:" + base64.StdEncoding.EncodeToString([]byte("pgpmessage"))
		zipPath := createTestZipWithComment(t, localTestingDir, comment, map[string]string{"test.txt": "content"})
		zipReader, err := zip.OpenReader(zipPath)
		assert.Nil(err)
		defer zipReader.Close()

		result := validateSignatureScheme(zipReader, "v1.0.0-test", false)
		assert.Equal("pgp", result)
	})

	t.Run("invalid signature - no prefix", func(t *testing.T) {
		comment := "v1.0.0:f:cert:hash"
		zipPath := createTestZipWithComment(t, localTestingDir, comment, map[string]string{"test.txt": "content"})
		zipReader, err := zip.OpenReader(zipPath)
		assert.Nil(err)
		defer zipReader.Close()

		result := validateSignatureScheme(zipReader, "v1.0.0-test", false)
		assert.Equal("invalid", result)
	})

	t.Run("invalid signature - wrong field count", func(t *testing.T) {
		comment := "cpackget-v1.0.0-test:f"
		zipPath := createTestZipWithComment(t, localTestingDir, comment, map[string]string{"test.txt": "content"})
		zipReader, err := zip.OpenReader(zipPath)
		assert.Nil(err)
		defer zipReader.Close()

		result := validateSignatureScheme(zipReader, "v1.0.0-test", false)
		assert.Equal("empty", result)
	})

	t.Run("invalid signature - not base64", func(t *testing.T) {
		comment := "cpackget-v1.0.0-test:f:notbase64!!!:notbase64!!!"
		zipPath := createTestZipWithComment(t, localTestingDir, comment, map[string]string{"test.txt": "content"})
		zipReader, err := zip.OpenReader(zipPath)
		assert.Nil(err)
		defer zipReader.Close()

		result := validateSignatureScheme(zipReader, "v1.0.0-test", false)
		assert.Equal("invalid", result)
	})

	t.Run("signing mode with existing signature", func(t *testing.T) {
		comment := "cpackget-v1.0.0-test:f:" + base64.StdEncoding.EncodeToString([]byte("cert")) + ":" + base64.StdEncoding.EncodeToString([]byte("hash"))
		zipPath := createTestZipWithComment(t, localTestingDir, comment, map[string]string{"test.txt": "content"})
		zipReader, err := zip.OpenReader(zipPath)
		assert.Nil(err)
		defer zipReader.Close()

		result := validateSignatureScheme(zipReader, "v1.0.0-test", true)
		assert.Equal("full", result)
	})
}

// TestGetSignField tests the getSignField function
func TestGetSignField(t *testing.T) {
	assert := assert.New(t)

	signature := "cpackget-v1.0.0:f:certdata:hashdata"

	t.Run("get version field", func(t *testing.T) {
		result := getSignField(signature, "version")
		assert.Equal("cpackget-v1.0.0", result)
	})

	t.Run("get type field", func(t *testing.T) {
		result := getSignField(signature, "type")
		assert.Equal("f", result)
	})

	t.Run("get certificate field", func(t *testing.T) {
		result := getSignField(signature, "certificate")
		assert.Equal("certdata", result)
	})

	t.Run("get pubsig field", func(t *testing.T) {
		result := getSignField(signature, "pubsig")
		assert.Equal("certdata", result)
	})

	t.Run("get hash field", func(t *testing.T) {
		result := getSignField(signature, "hash")
		assert.Equal("hashdata", result)
	})

	t.Run("get unknown field", func(t *testing.T) {
		result := getSignField(signature, "unknown")
		assert.Equal("", result)
	})
}

// TestSanityCheckCertificate tests the sanityCheckCertificate function
func TestSanityCheckCertificate(t *testing.T) {
	t.Run("valid certificate", func(t *testing.T) {
		assert := assert.New(t)
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature|x509.KeyUsageContentCommitment,
			false,
		)

		err := sanityCheckCertificate(cert, "")
		assert.Nil(err)
	})

	t.Run("missing subject CN", func(t *testing.T) {
		assert := assert.New(t)
		cert, _ := createCertificateWithProperties(t,
			"",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature,
			false,
		)

		err := sanityCheckCertificate(cert, "")
		assert.NotNil(err)
	})

	// NOTE: "missing issuer CN" test is not possible with x509.CreateCertificate
	// because it automatically copies Subject to Issuer for self-signed certificates.
	// This means we cannot test the case where Issuer.CommonName is empty.

	t.Run("vendor name mismatch", func(t *testing.T) {
		assert := assert.New(t)
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature,
			false,
		)

		err := sanityCheckCertificate(cert, "different.vendor.com")
		assert.NotNil(err)
	})

	t.Run("certificate not yet valid", func(t *testing.T) {
		assert := assert.New(t)
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature,
			false,
		)

		err := sanityCheckCertificate(cert, "")
		assert.NotNil(err)
	})

	t.Run("certificate expired", func(t *testing.T) {
		assert := assert.New(t)
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-365*24*time.Hour),
			time.Now().Add(-24*time.Hour),
			x509.KeyUsageDigitalSignature,
			false,
		)

		err := sanityCheckCertificate(cert, "")
		assert.NotNil(err)
	})

	t.Run("certificate with only DigitalSignature key usage", func(t *testing.T) {
		assert := assert.New(t)
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature,
			false,
		)

		// Should pass but log warning (we can't test log output easily)
		err := sanityCheckCertificate(cert, "")
		assert.Nil(err)
	})

	t.Run("certificate with DigitalSignature and ContentCommitment", func(t *testing.T) {
		assert := assert.New(t)
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature|x509.KeyUsageContentCommitment,
			false,
		)

		err := sanityCheckCertificate(cert, "")
		assert.Nil(err)
	})

	t.Run("certificate with additional key usages", func(t *testing.T) {
		assert := assert.New(t)
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature|x509.KeyUsageContentCommitment|x509.KeyUsageKeyEncipherment,
			false,
		)

		// Should pass - now correctly handles multiple key usages
		err := sanityCheckCertificate(cert, "")
		assert.Nil(err)
	})

	t.Run("CA certificate warning", func(t *testing.T) {
		assert := assert.New(t)
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature|x509.KeyUsageContentCommitment,
			true,
		)

		// Should pass but log warning
		err := sanityCheckCertificate(cert, "")
		assert.Nil(err)
	})
}

// TestLoadCertificate tests the loadCertificate function
func TestLoadCertificate(t *testing.T) {
	assert := assert.New(t)

	t.Run("valid certificate PEM", func(t *testing.T) {
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature|x509.KeyUsageContentCommitment,
			false,
		)

		certPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})

		loadedCert, err := loadCertificate(certPEM, "", true, true)
		assert.Nil(err)
		assert.NotNil(loadedCert)
		assert.Equal(cert.Subject.CommonName, loadedCert.Subject.CommonName)
	})

	t.Run("invalid PEM encoding", func(t *testing.T) {
		invalidPEM := []byte("not a valid PEM")

		_, err := loadCertificate(invalidPEM, "", true, true)
		assert.NotNil(err)
	})

	t.Run("multiple PEM blocks", func(t *testing.T) {
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature|x509.KeyUsageContentCommitment,
			false,
		)

		certPEM1 := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})
		certPEM2 := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})

		multiplePEM := append(certPEM1, certPEM2...)

		// Should load first cert and warn about rest
		loadedCert, err := loadCertificate(multiplePEM, "", true, true)
		assert.Nil(err)
		assert.NotNil(loadedCert)
	})

	t.Run("certificate with validation enabled", func(t *testing.T) {
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature|x509.KeyUsageContentCommitment,
			false,
		)

		certPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})

		loadedCert, err := loadCertificate(certPEM, "", false, true)
		assert.Nil(err)
		assert.NotNil(loadedCert)
	})
}

// TestExportCertificate tests the exportCertificate function
func TestExportCertificate(t *testing.T) {
	assert := assert.New(t)
	localTestingDir := t.TempDir()

	t.Run("export valid certificate", func(t *testing.T) {
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature,
			false,
		)

		certPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})

		b64Cert := base64.StdEncoding.EncodeToString(certPEM)
		exportPath := filepath.Join(localTestingDir, "exported.pem")

		err := exportCertificate(b64Cert, exportPath)
		assert.Nil(err)

		// Verify file was created
		_, err = os.Stat(exportPath)
		assert.Nil(err)

		// Verify content
		content, err := os.ReadFile(exportPath)
		assert.Nil(err)
		assert.Equal(certPEM, content)
	})

	t.Run("export to existing file", func(t *testing.T) {
		exportPath := filepath.Join(localTestingDir, "existing.pem")
		err := os.WriteFile(exportPath, []byte("existing"), 0644)
		assert.Nil(err)

		err = exportCertificate("test", exportPath)
		assert.NotNil(err)
	})

	t.Run("export invalid base64", func(t *testing.T) {
		exportPath := filepath.Join(localTestingDir, "invalid.pem")
		err := exportCertificate("not-valid-base64!!!", exportPath)
		assert.NotNil(err)
	})
}

// TestSignPackHashX509 tests the signPackHashX509 function
func TestSignPackHashX509(t *testing.T) {
	localTestingDir := t.TempDir()

	t.Run("sign with PKCS1 key", func(t *testing.T) {
		assert := assert.New(t)
		cert, privateKey := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature,
			false,
		)

		// Write private key to file
		keyPath := filepath.Join(localTestingDir, "pkcs1.key")
		privateKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		})
		err := os.WriteFile(keyPath, privateKeyPEM, 0600)
		assert.Nil(err)

		hash := []byte("test hash data")
		signature, err := signPackHashX509(keyPath, cert, hash)
		assert.Nil(err)
		assert.NotNil(signature)
		assert.Greater(len(signature), 0)
	})

	t.Run("sign with PKCS8 key", func(t *testing.T) {
		assert := assert.New(t)
		cert, privateKey := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature,
			false,
		)

		// Write private key as PKCS8 to file
		keyPath := filepath.Join(localTestingDir, "pkcs8.key")
		privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
		assert.Nil(err)
		privateKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privateKeyDER,
		})
		err = os.WriteFile(keyPath, privateKeyPEM, 0600)
		assert.Nil(err)

		hash := []byte("test hash data")
		signature, err := signPackHashX509(keyPath, cert, hash)
		assert.Nil(err)
		assert.NotNil(signature)
		assert.Greater(len(signature), 0)
	})

	t.Run("sign with mismatched key and certificate", func(t *testing.T) {
		assert := assert.New(t)
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature,
			false,
		)

		// Create different key
		_, differentKey := createCertificateWithProperties(t,
			"other.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature,
			false,
		)

		keyPath := filepath.Join(localTestingDir, "mismatch.key")
		privateKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(differentKey),
		})
		err := os.WriteFile(keyPath, privateKeyPEM, 0600)
		assert.Nil(err)

		hash := []byte("test hash data")
		_, err = signPackHashX509(keyPath, cert, hash)
		assert.NotNil(err)
	})

	t.Run("sign with non-existent key file", func(t *testing.T) {
		assert := assert.New(t)
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature,
			false,
		)

		hash := []byte("test hash data")
		_, err := signPackHashX509("/nonexistent/key.pem", cert, hash)
		assert.NotNil(err)
	})

	t.Run("sign with invalid key format", func(t *testing.T) {
		assert := assert.New(t)
		cert, _ := createCertificateWithProperties(t,
			"test.example.com",
			"CA",
			time.Now().Add(-24*time.Hour),
			time.Now().Add(365*24*time.Hour),
			x509.KeyUsageDigitalSignature,
			false,
		)

		keyPath := filepath.Join(localTestingDir, "invalid.key")
		err := os.WriteFile(keyPath, []byte("not a valid key"), 0600)
		assert.Nil(err)

		hash := []byte("test hash data")
		_, err = signPackHashX509(keyPath, cert, hash)
		assert.NotNil(err)
	})
}

// TestEmbedPack tests the embedPack function
func TestEmbedPack(t *testing.T) {
	assert := assert.New(t)
	localTestingDir := t.TempDir()

	t.Run("embed full signature", func(t *testing.T) {
		// Create original zip
		files := map[string]string{
			"file1.txt": "content1",
			"file2.txt": "content2",
		}
		originalZipPath := createTestZipWithComment(t, localTestingDir, "", files)
		zipReader, err := zip.OpenReader(originalZipPath)
		assert.Nil(err)
		defer zipReader.Close()

		// Create signed pack
		signedPackPath := filepath.Join(localTestingDir, "signed.pack")
		cert := []byte("certificate data")
		hash := []byte("signature hash")

		err = embedPack(signedPackPath, "v1.0.0", zipReader, cert, hash)
		assert.Nil(err)

		// Verify signed pack
		signedZip, err := zip.OpenReader(signedPackPath)
		assert.Nil(err)
		defer signedZip.Close()

		assert.Contains(signedZip.Comment, "cpackget-v1.0.0:f:")
		assert.Equal(len(zipReader.File), len(signedZip.File))
	})

	t.Run("embed cert-only signature", func(t *testing.T) {
		files := map[string]string{"test.txt": "content"}
		originalZipPath := createTestZipWithComment(t, localTestingDir, "", files)
		zipReader, err := zip.OpenReader(originalZipPath)
		assert.Nil(err)
		defer zipReader.Close()

		signedPackPath := filepath.Join(localTestingDir, "signed-certonly.pack")
		cert := []byte("certificate data")

		err = embedPack(signedPackPath, "v1.0.0", zipReader, cert, nil)
		assert.Nil(err)

		signedZip, err := zip.OpenReader(signedPackPath)
		assert.Nil(err)
		defer signedZip.Close()

		assert.Contains(signedZip.Comment, "cpackget-v1.0.0:c:")
	})

	t.Run("embed pgp signature", func(t *testing.T) {
		files := map[string]string{"test.txt": "content"}
		originalZipPath := createTestZipWithComment(t, localTestingDir, "", files)
		zipReader, err := zip.OpenReader(originalZipPath)
		assert.Nil(err)
		defer zipReader.Close()

		signedPackPath := filepath.Join(localTestingDir, "signed-pgp.pack")
		pgpSig := []byte("pgp signature data")

		err = embedPack(signedPackPath, "v1.0.0", zipReader, nil, pgpSig)
		assert.Nil(err)

		signedZip, err := zip.OpenReader(signedPackPath)
		assert.Nil(err)
		defer signedZip.Close()

		assert.Contains(signedZip.Comment, "cpackget-v1.0.0:p:")
	})
}
