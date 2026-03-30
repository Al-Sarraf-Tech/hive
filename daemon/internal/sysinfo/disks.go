package sysinfo

import (
	"bufio"
	"os"
	"strings"
	"syscall"
)

// DiskInfo holds information about a mounted filesystem.
type DiskEntry struct {
	Path      string
	Total     uint64
	Available uint64
	FSType    string
	Device    string
}

// ListDisks returns all mounted filesystems on Linux.
func ListDisks() []DiskEntry {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil
	}
	defer f.Close()

	seen := make(map[string]bool)
	var disks []DiskEntry

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		device, mountpoint, fstype := fields[0], fields[1], fields[2]

		// Skip virtual filesystems
		if strings.HasPrefix(fstype, "sys") || strings.HasPrefix(fstype, "proc") ||
			strings.HasPrefix(fstype, "dev") || fstype == "tmpfs" || fstype == "cgroup" ||
			fstype == "cgroup2" || fstype == "securityfs" || fstype == "pstore" ||
			fstype == "debugfs" || fstype == "tracefs" || fstype == "fusectl" ||
			fstype == "configfs" || fstype == "hugetlbfs" || fstype == "mqueue" ||
			fstype == "binfmt_misc" || fstype == "autofs" || fstype == "overlay" ||
			fstype == "nsfs" || fstype == "fuse.portal" {
			continue
		}

		if seen[mountpoint] {
			continue
		}
		seen[mountpoint] = true

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mountpoint, &stat); err != nil {
			continue
		}

		total := stat.Blocks * uint64(stat.Bsize)
		avail := stat.Bavail * uint64(stat.Bsize)
		if total == 0 {
			continue
		}

		disks = append(disks, DiskEntry{
			Path:      mountpoint,
			Total:     total,
			Available: avail,
			FSType:    fstype,
			Device:    device,
		})
	}

	return disks
}
