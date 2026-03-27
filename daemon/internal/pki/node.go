package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	nodeCertFile = "node.crt"
	nodeKeyFile  = "node.key"
	nodeValidity = 365 * 24 * time.Hour // 1 year
)

// GenerateNodeCert creates a node certificate signed by the cluster CA.
// The cert includes SANs for the advertise address, localhost, and the node name.
func GenerateNodeCert(caKey *ecdsa.PrivateKey, caCert *x509.Certificate, nodeName, advertiseAddr string) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate node key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   nodeName,
			Organization: []string{"Hive"},
		},
		NotBefore: now,
		NotAfter:  now.Add(nodeValidity),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		DNSNames: []string{nodeName, "localhost"},
	}

	// Add IP SANs
	if ip := net.ParseIP(advertiseAddr); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	}
	template.IPAddresses = append(template.IPAddresses,
		net.ParseIP("127.0.0.1"),
		net.ParseIP("::1"),
	)

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("create node certificate: %w", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal node key: %w", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM, nil
}

// GenerateCSR creates a node private key and a Certificate Signing Request.
// Used by joining nodes that do not have the CA key.
func GenerateCSR(nodeName, advertiseAddr string) (csrPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate node key: %w", err)
	}

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   nodeName,
			Organization: []string{"Hive"},
		},
		DNSNames: []string{nodeName, "localhost"},
	}
	if ip := net.ParseIP(advertiseAddr); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	}
	template.IPAddresses = append(template.IPAddresses,
		net.ParseIP("127.0.0.1"),
		net.ParseIP("::1"),
	)

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create CSR: %w", err)
	}

	csrPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal node key: %w", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return csrPEM, keyPEM, nil
}

// SignCSR signs a PEM-encoded CSR with the CA key, returning the signed cert PEM.
func SignCSR(caKey *ecdsa.PrivateKey, caCert *x509.Certificate, csrPEM []byte) ([]byte, error) {
	block, _ := pem.Decode(csrPEM)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in CSR")
	}

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse CSR: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("invalid CSR signature: %w", err)
	}

	// Validate the CSR's CommonName is not empty
	if csr.Subject.CommonName == "" {
		return nil, fmt.Errorf("CSR has empty CommonName")
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      csr.Subject,
		NotBefore:    now,
		NotAfter:     now.Add(nodeValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		// Restrict SANs to the CSR's CommonName + localhost (defense against arbitrary SAN injection)
		DNSNames:    []string{csr.Subject.CommonName, "localhost"},
		IPAddresses: filterLocalIPs(csr.IPAddresses),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, csr.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("sign certificate: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), nil
}

// filterLocalIPs restricts IP SANs to loopback addresses and the first non-loopback IP from the CSR.
// Prevents a malicious CSR from including arbitrary IP addresses.
func filterLocalIPs(requested []net.IP) []net.IP {
	ips := []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("::1"),
	}
	// Allow at most one non-loopback IP (the node's advertise address)
	for _, ip := range requested {
		if !ip.IsLoopback() {
			ips = append(ips, ip)
			break
		}
	}
	return ips
}

// LoadNodeCert loads the node certificate and private key as a tls.Certificate.
func LoadNodeCert(dataDir string) (tls.Certificate, error) {
	dir := filepath.Join(dataDir, pkiDir)
	certPath := filepath.Join(dir, nodeCertFile)
	keyPath := filepath.Join(dir, nodeKeyFile)
	return tls.LoadX509KeyPair(certPath, keyPath)
}

// SaveNodeCert writes the node certificate and private key to the data directory.
func SaveNodeCert(dataDir string, certPEM, keyPEM []byte) error {
	dir := filepath.Join(dataDir, pkiDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create pki directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, nodeCertFile), certPEM, 0o644); err != nil {
		return fmt.Errorf("write node cert: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, nodeKeyFile), keyPEM, 0o600); err != nil {
		return fmt.Errorf("write node key: %w", err)
	}
	return nil
}

// HasNodeCert checks if the node certificate exists in the data directory.
func HasNodeCert(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, pkiDir, nodeCertFile))
	return err == nil
}

// HasCACert checks if the CA certificate exists in the data directory.
func HasCACert(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, pkiDir, caCertFile))
	return err == nil
}
