//go:build linux

package platform

// DefaultConfigDir returns the configuration directory for hived on Linux.
func DefaultConfigDir() string {
	return "/etc/hive"
}

// DefaultDataDir returns the data directory for hived on Linux.
func DefaultDataDir() string {
	return "/var/lib/hive"
}

// DefaultLogDir returns the log directory for hived on Linux.
func DefaultLogDir() string {
	return "/var/log/hive"
}
