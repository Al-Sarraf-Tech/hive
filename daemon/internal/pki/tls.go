package pki

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
)

// MeshServerTLSConfig returns a TLS config for the gRPC HiveMesh server.
// Enforces mTLS: requires and verifies client certificates against the cluster CA.
func MeshServerTLSConfig(dataDir string) (*tls.Config, error) {
	nodeCert, err := LoadNodeCert(dataDir)
	if err != nil {
		return nil, fmt.Errorf("load node cert: %w", err)
	}

	caCert, err := LoadCACert(dataDir)
	if err != nil {
		return nil, fmt.Errorf("load CA cert: %w", err)
	}

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	return &tls.Config{
		Certificates: []tls.Certificate{nodeCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// MeshClientTLSConfig returns a TLS config for outbound gRPC HiveMesh connections.
// Presents the node certificate as a client cert and verifies the server against the CA.
func MeshClientTLSConfig(dataDir string) (*tls.Config, error) {
	nodeCert, err := LoadNodeCert(dataDir)
	if err != nil {
		return nil, fmt.Errorf("load node cert: %w", err)
	}

	caCert, err := LoadCACert(dataDir)
	if err != nil {
		return nil, fmt.Errorf("load CA cert: %w", err)
	}

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	return &tls.Config{
		Certificates: []tls.Certificate{nodeCert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// APIServerTLSConfig returns a TLS config for the gRPC HiveAPI server.
// Server-only TLS — does not require client certificates.
func APIServerTLSConfig(dataDir string) (*tls.Config, error) {
	nodeCert, err := LoadNodeCert(dataDir)
	if err != nil {
		return nil, fmt.Errorf("load node cert: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{nodeCert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// CACertFingerprint returns the SHA-256 fingerprint of a CA certificate as a colon-separated hex string.
func CACertFingerprint(caCert *x509.Certificate) string {
	hash := sha256.Sum256(caCert.Raw)
	fp := ""
	for i, b := range hash {
		if i > 0 {
			fp += ":"
		}
		fp += fmt.Sprintf("%02X", b)
	}
	return fp
}
