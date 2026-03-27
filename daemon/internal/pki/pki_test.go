package pki

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateCA(t *testing.T) {
	key, cert, certPEM, keyPEM, err := GenerateCA()
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	if key == nil || cert == nil {
		t.Fatal("nil key or cert")
	}
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		t.Fatal("empty PEM output")
	}
	if !cert.IsCA {
		t.Error("certificate is not a CA")
	}
	if cert.Subject.CommonName != "Hive Cluster CA" {
		t.Errorf("unexpected CN: %s", cert.Subject.CommonName)
	}
}

func TestSaveLoadCA(t *testing.T) {
	dir := t.TempDir()
	_, _, certPEM, keyPEM, err := GenerateCA()
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	if err := SaveCA(dir, certPEM, keyPEM, nil); err != nil {
		t.Fatalf("SaveCA: %v", err)
	}

	// Verify key file permissions
	info, err := os.Stat(filepath.Join(dir, pkiDir, caKeyFile))
	if err != nil {
		t.Fatalf("stat ca.key: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("ca.key permissions: %04o, want 0600", perm)
	}

	key, cert, err := LoadCA(dir, nil)
	if err != nil {
		t.Fatalf("LoadCA: %v", err)
	}
	if key == nil || cert == nil {
		t.Fatal("nil key or cert after load")
	}
}

func TestGenerateNodeCert(t *testing.T) {
	caKey, caCert, _, _, err := GenerateCA()
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	certPEM, keyPEM, err := GenerateNodeCert(caKey, caCert, "node-1", "192.168.1.10")
	if err != nil {
		t.Fatalf("GenerateNodeCert: %v", err)
	}
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		t.Fatal("empty PEM output")
	}

	// Parse and verify the node cert
	cert, err := parseCert(certPEM)
	if err != nil {
		t.Fatalf("parse node cert: %v", err)
	}
	if cert.Subject.CommonName != "node-1" {
		t.Errorf("unexpected CN: %s", cert.Subject.CommonName)
	}

	// Verify against CA
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Errorf("node cert does not verify against CA: %v", err)
	}

	// Check SANs
	foundAdvertise := false
	for _, ip := range cert.IPAddresses {
		if ip.Equal(net.ParseIP("192.168.1.10")) {
			foundAdvertise = true
		}
	}
	if !foundAdvertise {
		t.Error("advertise address not in SANs")
	}
}

func TestCSRSignFlow(t *testing.T) {
	caKey, caCert, _, _, err := GenerateCA()
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	csrPEM, keyPEM, err := GenerateCSR("node-2", "10.0.0.5")
	if err != nil {
		t.Fatalf("GenerateCSR: %v", err)
	}
	if len(csrPEM) == 0 || len(keyPEM) == 0 {
		t.Fatal("empty CSR/key PEM")
	}

	signedCertPEM, err := SignCSR(caKey, caCert, csrPEM)
	if err != nil {
		t.Fatalf("SignCSR: %v", err)
	}

	cert, err := parseCert(signedCertPEM)
	if err != nil {
		t.Fatalf("parse signed cert: %v", err)
	}
	if cert.Subject.CommonName != "node-2" {
		t.Errorf("unexpected CN: %s", cert.Subject.CommonName)
	}

	// Verify against CA
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Errorf("signed cert does not verify against CA: %v", err)
	}
}

func TestSaveLoadNodeCert(t *testing.T) {
	dir := t.TempDir()
	caKey, caCert, _, _, err := GenerateCA()
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	certPEM, keyPEM, err := GenerateNodeCert(caKey, caCert, "node-1", "127.0.0.1")
	if err != nil {
		t.Fatalf("GenerateNodeCert: %v", err)
	}

	if err := SaveNodeCert(dir, certPEM, keyPEM); err != nil {
		t.Fatalf("SaveNodeCert: %v", err)
	}

	tlsCert, err := LoadNodeCert(dir)
	if err != nil {
		t.Fatalf("LoadNodeCert: %v", err)
	}
	if len(tlsCert.Certificate) == 0 {
		t.Error("loaded cert has no certificate data")
	}
}

func TestTLSConfigs(t *testing.T) {
	dir := t.TempDir()

	// Generate and save CA + node cert
	caKey, caCert, caCertPEM, caKeyPEM, err := GenerateCA()
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	if err := SaveCA(dir, caCertPEM, caKeyPEM, nil); err != nil {
		t.Fatalf("SaveCA: %v", err)
	}

	certPEM, keyPEM, err := GenerateNodeCert(caKey, caCert, "test-node", "127.0.0.1")
	if err != nil {
		t.Fatalf("GenerateNodeCert: %v", err)
	}
	if err := SaveNodeCert(dir, certPEM, keyPEM); err != nil {
		t.Fatalf("SaveNodeCert: %v", err)
	}

	// Test mesh server config
	serverCfg, err := MeshServerTLSConfig(dir)
	if err != nil {
		t.Fatalf("MeshServerTLSConfig: %v", err)
	}
	if serverCfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Error("mesh server should require client certs")
	}
	if serverCfg.MinVersion != tls.VersionTLS13 {
		t.Error("mesh server should require TLS 1.3")
	}

	// Test mesh client config
	clientCfg, err := MeshClientTLSConfig(dir)
	if err != nil {
		t.Fatalf("MeshClientTLSConfig: %v", err)
	}
	// Dynamic cert loading: GetClientCertificate callback is used instead of static Certificates
	if clientCfg.GetClientCertificate == nil {
		t.Error("mesh client config should have GetClientCertificate callback")
	}

	// Test API server config
	apiCfg, err := APIServerTLSConfig(dir)
	if err != nil {
		t.Fatalf("APIServerTLSConfig: %v", err)
	}
	if apiCfg.ClientAuth != tls.NoClientCert {
		t.Error("API server should not require client certs")
	}

	// Verify TLS handshake works with an in-process pipe
	t.Run("mTLS handshake", func(t *testing.T) {
		serverConn, clientConn := net.Pipe()
		defer serverConn.Close()
		defer clientConn.Close()

		errCh := make(chan error, 2)

		go func() {
			tlsServer := tls.Server(serverConn, serverCfg)
			errCh <- tlsServer.Handshake()
		}()

		go func() {
			// In a real gRPC dial, the ServerName is set from the target address.
			// For in-process pipe test, set it explicitly.
			testCfg := clientCfg.Clone()
			testCfg.ServerName = "test-node"
			tlsClient := tls.Client(clientConn, testCfg)
			errCh <- tlsClient.Handshake()
		}()

		for i := 0; i < 2; i++ {
			if err := <-errCh; err != nil {
				t.Errorf("TLS handshake failed: %v", err)
			}
		}
	})
}

func TestCACertFingerprint(t *testing.T) {
	_, cert, _, _, err := GenerateCA()
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	fp := CACertFingerprint(cert)
	if len(fp) == 0 {
		t.Error("empty fingerprint")
	}
	// SHA-256 fingerprint is 32 bytes = 95 chars (32*2 + 31 colons)
	if len(fp) != 95 {
		t.Errorf("fingerprint length: %d, want 95", len(fp))
	}
}

func TestCertExpiryInfo(t *testing.T) {
	dir := t.TempDir()
	caKey, caCert, caCertPEM, caKeyPEM, err := GenerateCA()
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	_ = SaveCA(dir, caCertPEM, caKeyPEM, nil)

	certPEM, keyPEM, err := GenerateNodeCert(caKey, caCert, "test", "127.0.0.1")
	if err != nil {
		t.Fatalf("GenerateNodeCert: %v", err)
	}
	_ = SaveNodeCert(dir, certPEM, keyPEM)

	notAfter, daysLeft, err := CertExpiryInfo(dir)
	if err != nil {
		t.Fatalf("CertExpiryInfo: %v", err)
	}
	if notAfter.IsZero() {
		t.Error("notAfter is zero")
	}
	// Freshly generated cert should have ~365 days left
	if daysLeft < 360 || daysLeft > 366 {
		t.Errorf("expected ~365 days left, got %d", daysLeft)
	}
}

func TestHasNodeCert(t *testing.T) {
	dir := t.TempDir()
	if HasNodeCert(dir) {
		t.Error("should not have node cert in empty dir")
	}
	if HasCACert(dir) {
		t.Error("should not have CA cert in empty dir")
	}

	caKey, caCert, caCertPEM, caKeyPEM, _ := GenerateCA()
	_ = SaveCA(dir, caCertPEM, caKeyPEM, nil)
	certPEM, keyPEM, _ := GenerateNodeCert(caKey, caCert, "n", "127.0.0.1")
	_ = SaveNodeCert(dir, certPEM, keyPEM)

	if !HasNodeCert(dir) {
		t.Error("should have node cert after save")
	}
	if !HasCACert(dir) {
		t.Error("should have CA cert after save")
	}
}
