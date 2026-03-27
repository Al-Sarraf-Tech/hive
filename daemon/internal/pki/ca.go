// Package pki provides self-contained PKI operations for Hive cluster mTLS.
// Uses Go stdlib crypto only — no external dependencies.
package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const (
	pkiDir     = "pki"
	caCertFile = "ca.crt"
	caKeyFile  = "ca.key"
	caValidity = 10 * 365 * 24 * time.Hour // 10 years
)

// GenerateCA creates a new self-signed ECDSA P-256 CA keypair.
// Returns the private key, parsed certificate, and PEM-encoded forms.
func GenerateCA() (*ecdsa.PrivateKey, *x509.Certificate, []byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("generate CA key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "Hive Cluster CA",
			Organization: []string{"Hive"},
		},
		NotBefore:             now,
		NotAfter:              now.Add(caValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create CA certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("parse CA certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("marshal CA key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return key, cert, certPEM, keyPEM, nil
}

// LoadCA loads the CA private key and certificate from the data directory.
// Returns os.ErrNotExist if the CA key file does not exist (normal for non-init nodes).
// If decryptFn is non-nil and the key is not valid PEM, it will be decrypted first.
// This provides backward compatibility: plaintext PEM files work without decryptFn.
func LoadCA(dataDir string, decryptFn func([]byte) ([]byte, error)) (*ecdsa.PrivateKey, *x509.Certificate, error) {
	dir := filepath.Join(dataDir, pkiDir)

	keyData, err := os.ReadFile(filepath.Join(dir, caKeyFile))
	if err != nil {
		return nil, nil, err
	}

	// Try parsing as plaintext PEM first (backward compatibility)
	block, _ := pem.Decode(keyData)
	if block == nil && decryptFn != nil {
		// Not valid PEM — try decrypting
		decrypted, err := decryptFn(keyData)
		if err != nil {
			return nil, nil, fmt.Errorf("decrypt CA key: %w", err)
		}
		keyData = decrypted
	}

	certPEM, err := os.ReadFile(filepath.Join(dir, caCertFile))
	if err != nil {
		return nil, nil, fmt.Errorf("read CA cert: %w", err)
	}

	key, err := parseECKey(keyData)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA key: %w", err)
	}

	cert, err := parseCert(certPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA cert: %w", err)
	}

	return key, cert, nil
}

// HasCAKey checks if the CA private key exists in the data directory.
func HasCAKey(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, pkiDir, caKeyFile))
	return err == nil
}

// LoadCACert loads only the CA certificate (all nodes have this, not all have ca.key).
func LoadCACert(dataDir string) (*x509.Certificate, error) {
	certPEM, err := os.ReadFile(filepath.Join(dataDir, pkiDir, caCertFile))
	if err != nil {
		return nil, err
	}
	return parseCert(certPEM)
}

// LoadCACertPEM returns the raw PEM bytes of the CA certificate.
func LoadCACertPEM(dataDir string) ([]byte, error) {
	return os.ReadFile(filepath.Join(dataDir, pkiDir, caCertFile))
}

// SaveCA writes the CA certificate and private key to the data directory.
// If encryptFn is non-nil, the CA key is encrypted before writing (defense-in-depth).
// Pass nil for encryptFn to write plaintext PEM (backward compatible).
func SaveCA(dataDir string, certPEM, keyPEM []byte, encryptFn func([]byte) ([]byte, error)) error {
	dir := filepath.Join(dataDir, pkiDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create pki directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, caCertFile), certPEM, 0o644); err != nil {
		return fmt.Errorf("write CA cert: %w", err)
	}

	keyData := keyPEM
	if encryptFn != nil {
		encrypted, err := encryptFn(keyPEM)
		if err != nil {
			return fmt.Errorf("encrypt CA key: %w", err)
		}
		keyData = encrypted
	}
	if err := os.WriteFile(filepath.Join(dir, caKeyFile), keyData, 0o600); err != nil {
		return fmt.Errorf("write CA key: %w", err)
	}
	return nil
}

// SaveCACert writes only the CA certificate (used by joining nodes).
func SaveCACert(dataDir string, certPEM []byte) error {
	dir := filepath.Join(dataDir, pkiDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create pki directory: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, caCertFile), certPEM, 0o644)
}

// randomSerial generates a random 128-bit serial number for X.509 certificates.
func randomSerial() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}
	return serial, nil
}

// parseECKey decodes a PEM-encoded EC private key.
func parseECKey(pemData []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in key data")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

// parseCert decodes a PEM-encoded X.509 certificate.
func parseCert(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in cert data")
	}
	return x509.ParseCertificate(block.Bytes)
}
