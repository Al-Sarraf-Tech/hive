//go:build !linux

package sysinfo

// MemInfo returns total and available memory in bytes.
// Not implemented on non-Linux platforms.
func MemInfo() (total, available uint64) {
	return 0, 0
}

// DiskInfo returns total and available bytes for the given path's filesystem.
// Not implemented on non-Linux platforms.
func DiskInfo(path string) (total, available uint64) {
	return 0, 0
}

// CPUCount returns the number of online CPUs.
// Not implemented on non-Linux platforms.
func CPUCount() uint32 {
	return 0
}
