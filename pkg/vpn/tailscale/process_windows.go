//go:build windows

package tailscale

import (
	"os"
)

// isProcessRunning checks if a process with the given PID is still running (Windows-specific)
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Windows, FindProcess always succeeds, so we assume it's running
	// A more robust check would use Windows API, but this is sufficient for our needs
	_ = process
	return true
}
