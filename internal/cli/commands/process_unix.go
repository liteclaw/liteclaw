//go:build !windows

package commands

import (
	"syscall"
	"time"
)

// checkProcessRunning checks if a process with given PID is running
func checkProcessRunning(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// terminateProcess sends SIGTERM to the process
func terminateProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}

// waitForProcessExit waits for the process to exit
func waitForProcessExit(pid int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
}
