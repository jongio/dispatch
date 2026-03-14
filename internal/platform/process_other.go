//go:build !windows

package platform

import "syscall"

// IsProcessAlive reports whether the process with the given PID is running.
// On Unix-like systems this sends signal 0, which checks for existence
// without actually delivering a signal.
func IsProcessAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
