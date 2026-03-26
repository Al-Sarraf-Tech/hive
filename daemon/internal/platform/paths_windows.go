//go:build windows

package platform

import (
	"os"
	"path/filepath"
)

func programDataDir() string {
	if d := os.Getenv("ProgramData"); d != "" {
		return d
	}
	return `C:\ProgramData`
}

// DefaultConfigDir returns the configuration directory for hived on Windows.
func DefaultConfigDir() string {
	return filepath.Join(programDataDir(), "Hive")
}

// DefaultDataDir returns the data directory for hived on Windows.
func DefaultDataDir() string {
	return filepath.Join(programDataDir(), "Hive", "data")
}

// DefaultLogDir returns the log directory for hived on Windows.
func DefaultLogDir() string {
	return filepath.Join(programDataDir(), "Hive", "logs")
}
