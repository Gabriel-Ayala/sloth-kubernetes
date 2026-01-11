//go:build unix

package tailscale

import (
	"os"
	"syscall"
)

// isProcessRunning checks if a process with the given PID is still running (Unix-specific)
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0 to check
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
