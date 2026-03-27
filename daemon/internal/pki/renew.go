package pki

import (
	"context"
	"crypto/x509"
	"log/slog"
	"math"
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
	remaining := time.Until(notAfter)
	if remaining <= 0 {
		daysLeft = 0
	} else {
		daysLeft = int(math.Ceil(remaining.Hours() / 24))
	}
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
	for {
		rc.check()

		// Adaptive interval: check more frequently when cert is close to expiry
		interval := rc.checkInterval
		if _, daysLeft, err := CertExpiryInfo(rc.dataDir); err == nil {
			switch {
			case daysLeft <= 1:
				interval = 5 * time.Minute
			case daysLeft <= 7:
				interval = 1 * time.Hour
			case daysLeft <= 30:
				interval = 3 * time.Hour
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
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

	renewDays := int(rc.renewThreshold.Hours() / 24)
	warnDays := renewDays * 2 // warn at 2x the renewal threshold

	switch {
	case daysLeft <= 0:
		slog.Error("node certificate has EXPIRED",
			"expired_at", notAfter.Format(time.RFC3339),
		)
		rc.tryRenew()
	case daysLeft <= 7:
		slog.Error("node certificate expires in less than 7 days",
			"expires_at", notAfter.Format(time.RFC3339),
			"days_left", daysLeft,
		)
		rc.tryRenew()
	case daysLeft <= renewDays:
		slog.Warn("node certificate expires soon — renewal threshold reached",
			"expires_at", notAfter.Format(time.RFC3339),
			"days_left", daysLeft,
			"renew_threshold_days", renewDays,
		)
		rc.tryRenew()
	case daysLeft <= warnDays:
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
