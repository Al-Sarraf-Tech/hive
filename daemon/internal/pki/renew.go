package pki

import (
	"context"
	"crypto/x509"
	"log/slog"
	"time"
)

// CertExpiryInfo returns the expiry time and days remaining for the node certificate.
func CertExpiryInfo(dataDir string) (notAfter time.Time, daysLeft int, err error) {
	cert, err := LoadNodeCert(dataDir)
	if err != nil {
		return time.Time{}, 0, err
	}
	if len(cert.Certificate) == 0 {
		return time.Time{}, 0, nil
	}
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return time.Time{}, 0, err
	}
	notAfter = x509Cert.NotAfter
	daysLeft = int(time.Until(notAfter).Hours() / 24)
	return notAfter, daysLeft, nil
}

// RenewalChecker periodically checks node certificate expiry and logs warnings.
// Call renewFn when the cert is within renewThreshold of expiry.
type RenewalChecker struct {
	dataDir        string
	checkInterval  time.Duration
	renewThreshold time.Duration // renew when less than this time remains
	renewFn        func() error  // called to perform renewal (e.g., CSR signing)
}

// NewRenewalChecker creates a certificate renewal checker.
// renewFn is called when the certificate needs renewal — it should generate a CSR,
// get it signed, and save the new cert. Pass nil to only log warnings without auto-renewal.
func NewRenewalChecker(dataDir string, renewFn func() error) *RenewalChecker {
	return &RenewalChecker{
		dataDir:        dataDir,
		checkInterval:  6 * time.Hour,
		renewThreshold: 30 * 24 * time.Hour, // 30 days
		renewFn:        renewFn,
	}
}

// Start runs the renewal checker until ctx is cancelled.
func (rc *RenewalChecker) Start(ctx context.Context) {
	// Initial check on startup
	rc.check()

	ticker := time.NewTicker(rc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rc.check()
		}
	}
}

func (rc *RenewalChecker) check() {
	if !HasNodeCert(rc.dataDir) {
		return
	}

	notAfter, daysLeft, err := CertExpiryInfo(rc.dataDir)
	if err != nil {
		slog.Warn("failed to check certificate expiry", "error", err)
		return
	}

	switch {
	case daysLeft <= 0:
		slog.Error("node certificate has EXPIRED",
			"expired_at", notAfter.Format(time.RFC3339),
		)
	case daysLeft <= 7:
		slog.Error("node certificate expires in less than 7 days",
			"expires_at", notAfter.Format(time.RFC3339),
			"days_left", daysLeft,
		)
		rc.tryRenew()
	case daysLeft <= 30:
		slog.Warn("node certificate expires soon",
			"expires_at", notAfter.Format(time.RFC3339),
			"days_left", daysLeft,
		)
		rc.tryRenew()
	case daysLeft <= 60:
		slog.Info("node certificate status",
			"expires_at", notAfter.Format(time.RFC3339),
			"days_left", daysLeft,
		)
	default:
		slog.Debug("node certificate valid",
			"expires_at", notAfter.Format(time.RFC3339),
			"days_left", daysLeft,
		)
	}
}

func (rc *RenewalChecker) tryRenew() {
	if rc.renewFn == nil {
		slog.Warn("certificate renewal not configured — manual renewal required")
		return
	}

	slog.Info("attempting automatic certificate renewal...")
	if err := rc.renewFn(); err != nil {
		slog.Error("automatic certificate renewal failed", "error", err)
	} else {
		slog.Info("certificate renewed successfully")
	}
}
