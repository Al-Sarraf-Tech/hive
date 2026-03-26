// Package platform provides OS-specific abstractions for hived.
// Build tags select the correct implementation at compile time.
package platform

import "runtime"

// OS returns the current operating system.
func OS() string {
	return runtime.GOOS
}

// Arch returns the current architecture.
func Arch() string {
	return runtime.GOARCH
}
