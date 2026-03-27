//go:build linux

package sysinfo

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// MemInfo returns total and available memory in bytes.
func MemInfo() (total, available uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		val, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			continue
		}
		kb := val * 1024
		switch parts[0] {
		case "MemTotal:":
			total = kb
		case "MemAvailable:":
			available = kb
		}
	}
	return
}

// DiskInfo returns total and available bytes for the given path's filesystem.
func DiskInfo(path string) (total, available uint64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}
	total = stat.Blocks * uint64(stat.Bsize)
	available = stat.Bavail * uint64(stat.Bsize)
	return
}

// CPUCount returns the number of online CPUs.
func CPUCount() uint32 {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	count := uint32(0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "processor") {
			count++
		}
	}
	return count
}
