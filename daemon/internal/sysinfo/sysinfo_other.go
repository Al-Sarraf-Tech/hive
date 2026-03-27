//go:build !linux && !windows

package sysinfo

import "runtime"

// MemInfo returns total and available memory in bytes.
// Not implemented on this platform.
func MemInfo() (total, available uint64) {
	return 0, 0
}

// DiskInfo returns total and available bytes for the given path's filesystem.
// Not implemented on this platform.
func DiskInfo(path string) (total, available uint64) {
	return 0, 0
}

// CPUCount returns the number of logical CPUs via Go runtime.
func CPUCount() uint32 {
	return uint32(runtime.NumCPU())
}
