/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package cryptography

import (
	"archive/zip"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/stretchr/testify/assert"
)

// Helper function to create a test zip file
func createTestZipFile(t *testing.T, dir string, files map[string]string) string {
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

	// Ensure zip is fully written before returning
	zipWriter.Close()
	zipFile.Close()

	return zipPath
}

// Helper function to generate a test RSA key pair and certificate
func generateTestCertificate(t *testing.T) (*x509.Certificate, *rsa.PrivateKey, string, string) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.Nil(t, err)

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:      []string{"US"},
			Organization: []string{"Test Org"},
			CommonName:   "test.example.com",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	assert.Nil(t, err)

	cert, err := x509.ParseCertificate(certDER)
	assert.Nil(t, err)

	// Encode private key as PEM (PKCS1)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// Encode private key as PEM (PKCS8)
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	assert.Nil(t, err)
	privateKeyPEMPKCS8 := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyDER,
	})

	return cert, privateKey, string(privateKeyPEM), string(privateKeyPEMPKCS8)
}

// TestCalculatePackHash tests the calculatePackHash function
func TestCalculatePackHash(t *testing.T) {
	assert := assert.New(t)

	t.Run("calculate hash of simple zip", func(t *testing.T) {
		localTestingDir := t.TempDir()
		// Create test zip file
		files := map[string]string{
			"file1.txt": "content1",
			"file2.txt": "content2",
		}
		zipPath := createTestZipFile(t, localTestingDir, files)

		// Open zip and calculate hash
		zipReader, err := zip.OpenReader(zipPath)
		assert.Nil(err)
		if zipReader != nil {
			defer zipReader.Close()
		}

		hash, err := calculatePackHash(zipReader)
		assert.Nil(err)
		assert.NotEmpty(hash)
	})

	t.Run("calculate hash of empty zip", func(t *testing.T) {
		localTestingDir := t.TempDir()
		// Create empty zip file
		files := map[string]string{}
		zipPath := createTestZipFile(t, localTestingDir, files)

		zipReader, err := zip.OpenReader(zipPath)
		assert.Nil(err)
		if zipReader != nil {
			defer zipReader.Close()
		}

		hash, err := calculatePackHash(zipReader)
		assert.Nil(err)
		assert.NotNil(hash)
	})

	t.Run("calculate hash consistency", func(t *testing.T) {
		localTestingDir := t.TempDir()
		// Create two identical zip files
		files := map[string]string{
			"test.txt": "test content",
		}
		zipPath1 := createTestZipFile(t, localTestingDir, files)
		zipPath2 := createTestZipFile(t, localTestingDir, files)

		zipReader1, err := zip.OpenReader(zipPath1)
		assert.Nil(err)
		if zipReader1 != nil {
			defer zipReader1.Close()
		}

		hash1, err := calculatePackHash(zipReader1)
		assert.Nil(err)

		zipReader2, err := zip.OpenReader(zipPath2)
		assert.Nil(err)
		if zipReader2 != nil {
			defer zipReader2.Close()
		}

		hash2, err := calculatePackHash(zipReader2)
		assert.Nil(err)

		// Hashes should be identical for identical content
		assert.Equal(hash1, hash2)
	})
}

// TestDetectKeyType tests the detectKeyType function
func TestDetectKeyType(t *testing.T) {
	assert := assert.New(t)

	t.Run("detect PKCS1 key", func(t *testing.T) {
		key := "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----"
		keyType, err := detectKeyType(key)
		assert.Nil(err)
		assert.Equal("PKCS1", keyType)
	})

	t.Run("detect PKCS8 key", func(t *testing.T) {
		key := "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w...\n-----END PRIVATE KEY-----"
		keyType, err := detectKeyType(key)
		assert.Nil(err)
		assert.Equal("PKCS8", keyType)
	})

	t.Run("detect encrypted PKCS8 key (unsupported)", func(t *testing.T) {
		key := "-----BEGIN ENCRYPTED PRIVATE KEY-----\nMIIFHDBOBgkqhkiG9w...\n-----END ENCRYPTED PRIVATE KEY-----"
		keyType, err := detectKeyType(key)
		assert.NotNil(err)
		assert.Equal("", keyType)
	})

	t.Run("detect invalid key format", func(t *testing.T) {
		key := "-----BEGIN SOMETHING ELSE-----\ndata\n-----END SOMETHING ELSE-----"
		keyType, err := detectKeyType(key)
		assert.NotNil(err)
		assert.Equal("", keyType)
	})

	t.Run("detect malformed key", func(t *testing.T) {
		key := "not a valid PEM key"
		// This will panic with index out of range due to missing PEM markers
		// The function should be improved to handle this case gracefully
		assert.Panics(func() {
			detectKeyType(key)
		})
	})
}

// TestDisplayCertificateInfo tests the displayCertificateInfo function
func TestDisplayCertificateInfo(t *testing.T) {
	assert := assert.New(t)

	// Generate test certificate
	cert, _, _, _ := generateTestCertificate(t)
	assert.NotNil(cert)

	// This function logs information, we just test it doesn't panic
	assert.NotPanics(func() {
		displayCertificateInfo(cert)
	})
}

// TestGetDigestList tests the getDigestList function
func TestGetDigestList(t *testing.T) {
	assert := assert.New(t)

	t.Run("get digest list with sha256", func(t *testing.T) {
		localTestingDir := t.TempDir()
		files := map[string]string{
			"file1.txt": "content1",
			"file2.txt": "content2",
			"file3.txt": "content3",
		}
		zipPath := createTestZipFile(t, localTestingDir, files)

		digests, err := getDigestList(zipPath, "sha256")
		assert.Nil(err)
		assert.NotNil(digests)
		assert.Equal(3, len(digests))

		// Check that all files have digests
		assert.Contains(digests, "file1.txt")
		assert.Contains(digests, "file2.txt")
		assert.Contains(digests, "file3.txt")

		// Check digest format (should be hex string)
		for _, digest := range digests {
			assert.NotEmpty(digest)
			assert.Len(digest, 64) // SHA256 produces 64 hex characters
		}
	})

	t.Run("get digest list of empty zip", func(t *testing.T) {
		localTestingDir := t.TempDir()
		files := map[string]string{}
		zipPath := createTestZipFile(t, localTestingDir, files)

		digests, err := getDigestList(zipPath, "sha256")
		assert.Nil(err)
		assert.NotNil(digests)
		assert.Equal(0, len(digests))
	})

	t.Run("get digest list with non-existent file", func(t *testing.T) {
		_, err := getDigestList("C:\\nonexistent\\path\\file.zip", "sha256")
		assert.NotNil(err)
	})
}

// TestGetKeyUsage tests the getKeyUsage function
func TestGetKeyUsage(t *testing.T) {
	assert := assert.New(t)

	t.Run("single key usage", func(t *testing.T) {
		usage := x509.KeyUsageDigitalSignature
		uses := getKeyUsage(usage)
		assert.Equal(1, len(uses))
		assert.Contains(uses, "\"Digital Signature\"")
	})

	t.Run("multiple key usages", func(t *testing.T) {
		usage := x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
		uses := getKeyUsage(usage)
		assert.Equal(2, len(uses))
		assert.Contains(uses, "\"Digital Signature\"")
		assert.Contains(uses, "\"Key Encipherment\"")
	})

	t.Run("all key usages", func(t *testing.T) {
		usage := x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment |
			x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment |
			x509.KeyUsageKeyAgreement | x509.KeyUsageCertSign |
			x509.KeyUsageCRLSign | x509.KeyUsageEncipherOnly |
			x509.KeyUsageDecipherOnly
		uses := getKeyUsage(usage)
		// All 9 X.509 KeyUsage flags should be detected
		assert.Equal(9, len(uses))
	})

	t.Run("no key usage", func(t *testing.T) {
		usage := x509.KeyUsage(0)
		uses := getKeyUsage(usage)
		assert.Equal(0, len(uses))
	})
}

// TestGetUnlockedKeyring tests the getUnlockedKeyring function
func TestGetUnlockedKeyring(t *testing.T) {
	assert := assert.New(t)

	t.Run("unlock keyring with valid key and passphrase", func(t *testing.T) {
		// Generate a PGP key for testing
		passphrase := []byte("test-passphrase")
		name := "Test User"
		email := "test@example.com"

		key, err := crypto.GenerateKey(name, email, "rsa", 2048)
		assert.Nil(err)

		lockedKey, err := key.Lock(passphrase)
		assert.Nil(err)

		armoredKey, err := lockedKey.Armor()
		assert.Nil(err)

		// Test unlocking
		keyring, err := getUnlockedKeyring(armoredKey, passphrase)
		assert.Nil(err)
		assert.NotNil(keyring)
	})

	t.Run("unlock keyring with wrong passphrase", func(t *testing.T) {
		// Generate a PGP key
		passphrase := []byte("correct-passphrase")
		wrongPassphrase := []byte("wrong-passphrase")
		name := "Test User"
		email := "test@example.com"

		key, err := crypto.GenerateKey(name, email, "rsa", 2048)
		assert.Nil(err)

		lockedKey, err := key.Lock(passphrase)
		assert.Nil(err)

		armoredKey, err := lockedKey.Armor()
		assert.Nil(err)

		// Test unlocking with wrong passphrase
		_, err = getUnlockedKeyring(armoredKey, wrongPassphrase)
		assert.NotNil(err)
	})

	t.Run("unlock keyring with invalid key", func(t *testing.T) {
		invalidKey := "-----BEGIN PGP PRIVATE KEY BLOCK-----\ninvalid\n-----END PGP PRIVATE KEY BLOCK-----"
		passphrase := []byte("test")

		_, err := getUnlockedKeyring(invalidKey, passphrase)
		assert.NotNil(err)
	})
}

// TestIsPrivateKeyFromCertificate tests the isPrivateKeyFromCertificate function
func TestIsPrivateKeyFromCertificate(t *testing.T) {
	assert := assert.New(t)

	t.Run("verify PKCS1 private key matches certificate", func(t *testing.T) {
		cert, privateKey, _, _ := generateTestCertificate(t)

		// Encode as PKCS1
		privateKeyDER := x509.MarshalPKCS1PrivateKey(privateKey)

		matches, err := isPrivateKeyFromCertificate(cert, privateKeyDER, "PKCS1")
		assert.Nil(err)
		assert.True(matches)
	})

	t.Run("verify PKCS8 private key matches certificate", func(t *testing.T) {
		cert, privateKey, _, _ := generateTestCertificate(t)

		// Encode as PKCS8
		privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
		assert.Nil(err)

		matches, err := isPrivateKeyFromCertificate(cert, privateKeyDER, "PKCS8")
		assert.Nil(err)
		assert.True(matches)
	})

	t.Run("verify mismatched private key and certificate", func(t *testing.T) {
		cert1, _, _, _ := generateTestCertificate(t)
		_, privateKey2, _, _ := generateTestCertificate(t)

		// Use key from cert2 with cert1
		privateKeyDER := x509.MarshalPKCS1PrivateKey(privateKey2)

		matches, err := isPrivateKeyFromCertificate(cert1, privateKeyDER, "PKCS1")
		assert.Nil(err)
		assert.False(matches)
	})

	t.Run("verify with invalid key type", func(t *testing.T) {
		cert, privateKey, _, _ := generateTestCertificate(t)
		privateKeyDER := x509.MarshalPKCS1PrivateKey(privateKey)

		matches, err := isPrivateKeyFromCertificate(cert, privateKeyDER, "INVALID")
		assert.NotNil(err)
		assert.False(matches)
	})

	t.Run("verify with corrupted key data", func(t *testing.T) {
		cert, _, _, _ := generateTestCertificate(t)
		corruptedKeyDER := []byte("corrupted data")

		_, err := isPrivateKeyFromCertificate(cert, corruptedKeyDER, "PKCS1")
		assert.NotNil(err)
	})
}

// TestSanitizeVersionForSignature tests the sanitizeVersionForSignature function
func TestSanitizeVersionForSignature(t *testing.T) {
	assert := assert.New(t)

	t.Run("sanitize version with v prefix", func(t *testing.T) {
		version := "v1.2.3"
		sanitized := sanitizeVersionForSignature(version)
		assert.True(strings.HasPrefix(sanitized, "cpackget-"))
		assert.Contains(sanitized, "v1.2.3")
	})

	t.Run("sanitize version without v prefix", func(t *testing.T) {
		version := "1.2.3"
		sanitized := sanitizeVersionForSignature(version)
		assert.True(strings.HasPrefix(sanitized, "cpackget-"))
		assert.Contains(sanitized, "v1.2.3")
	})

	t.Run("sanitize empty version", func(t *testing.T) {
		version := ""
		sanitized := sanitizeVersionForSignature(version)
		assert.Equal("cpackget-", sanitized)
	})

	t.Run("sanitize version with build metadata", func(t *testing.T) {
		version := "v1.2.3+build.123"
		sanitized := sanitizeVersionForSignature(version)
		assert.True(strings.HasPrefix(sanitized, "cpackget-"))
		assert.Contains(sanitized, version)
	})

	t.Run("sanitize version with prerelease", func(t *testing.T) {
		version := "v1.2.3-alpha.1"
		sanitized := sanitizeVersionForSignature(version)
		assert.True(strings.HasPrefix(sanitized, "cpackget-"))
		assert.Contains(sanitized, version)
	})

	t.Run("sanitize version starting with non-v character", func(t *testing.T) {
		version := "1.0.0"
		sanitized := sanitizeVersionForSignature(version)
		// Should add both prefix and 'v'
		assert.True(strings.HasPrefix(sanitized, "cpackget-v"))
	})
}

// TestGetDigestListConsistency tests that digest calculation is consistent
func TestGetDigestListConsistency(t *testing.T) {
	assert := assert.New(t)

	localTestingDir := t.TempDir()

	// Create identical files
	files := map[string]string{
		"file1.txt": "identical content",
		"file2.txt": "different content",
	}

	zipPath1 := createTestZipFile(t, localTestingDir, files)
	zipPath2 := createTestZipFile(t, localTestingDir, files)

	digests1, err := getDigestList(zipPath1, "sha256")
	assert.Nil(err)

	digests2, err := getDigestList(zipPath2, "sha256")
	assert.Nil(err)

	// Digests for same files should be identical
	assert.Equal(digests1["file1.txt"], digests2["file1.txt"])
	assert.Equal(digests1["file2.txt"], digests2["file2.txt"])

	// Different content should have different digests
	assert.NotEqual(digests1["file1.txt"], digests1["file2.txt"])
}

// TestKeyUsageAllCombinations tests various combinations of key usages
func TestKeyUsageAllCombinations(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		name     string
		usage    x509.KeyUsage
		expected []string
	}{
		{
			name:     "CertSign only",
			usage:    x509.KeyUsageCertSign,
			expected: []string{"\"Certificate Signing\""},
		},
		{
			name:     "CRLSign only",
			usage:    x509.KeyUsageCRLSign,
			expected: []string{"\"CRL Signing\""},
		},
		{
			name:     "ContentCommitment only",
			usage:    x509.KeyUsageContentCommitment,
			expected: []string{"\"Content Commitment\""},
		},
		{
			name:     "DataEncipherment only",
			usage:    x509.KeyUsageDataEncipherment,
			expected: []string{"\"Data Encipherment\""},
		},
		{
			name:     "KeyAgreement only",
			usage:    x509.KeyUsageKeyAgreement,
			expected: []string{"\"Key Agreement\""},
		},
		{
			name:  "Mixed usage",
			usage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
			expected: []string{
				"\"Digital Signature\"",
				"\"Certificate Signing\"",
				"\"CRL Signing\"",
			},
		},
		{
			name:  "EncipherOnly and DecipherOnly",
			usage: x509.KeyUsageEncipherOnly | x509.KeyUsageDecipherOnly,
			expected: []string{
				"\"Encipher Only\"",
				"\"Decipher Only\"",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uses := getKeyUsage(tc.usage)
			assert.Equal(len(tc.expected), len(uses))
			for _, expectedUse := range tc.expected {
				assert.Contains(uses, expectedUse)
			}
		})
	}
}
