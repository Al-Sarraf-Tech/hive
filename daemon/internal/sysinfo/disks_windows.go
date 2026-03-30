//go:build windows

package sysinfo

// DiskEntry holds information about a mounted filesystem.
type DiskEntry struct {
	Path      string
	Total     uint64
	Available uint64
	FSType    string
	Device    string
}

// ListDisks returns mounted filesystems. Windows implementation is a stub.
func ListDisks() []DiskEntry {
	return nil
}
