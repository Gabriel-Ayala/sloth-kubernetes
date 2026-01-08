//go:build unix

package cmd

import (
	"os"
	"os/exec"
	"syscall"
)

// setupDaemonProcess configures the command to run as a daemon (Unix-specific)
func setupDaemonProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}

// isProcessRunning checks if a process with the given PID is still running
func isProcessRunning(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// Platform-specific signal constants
var (
	signalInterrupt = syscall.SIGINT
	signalTerminate = syscall.SIGTERM
)

// terminateProcess sends a termination signal to the process
func terminateProcess(process *os.Process) error {
	return process.Signal(syscall.SIGTERM)
}
