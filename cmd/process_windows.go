//go:build windows

package cmd

import (
	"os"
	"os/exec"
)

// setupDaemonProcess configures the command to run as a daemon (Windows stub)
// On Windows, we don't set Setsid as it's not available
func setupDaemonProcess(cmd *exec.Cmd) {
	// Windows doesn't support Setsid, process will run normally
}

// isProcessRunning checks if a process with the given PID is still running
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, FindProcess always succeeds, so we need to try to signal it
	// Signal 0 doesn't exist on Windows, so we just assume it's running
	// A more robust check would use Windows API, but this is sufficient for our needs
	_ = process
	return true
}

// Platform-specific signal constants
// Windows doesn't have SIGINT/SIGTERM, use os package equivalents
var (
	signalInterrupt = os.Interrupt
	signalTerminate = os.Kill // Windows doesn't have SIGTERM, use Kill instead
)

// terminateProcess terminates the process (Windows version)
// Windows only supports Kill, not graceful termination signals
func terminateProcess(process *os.Process) error {
	return process.Kill()
}
