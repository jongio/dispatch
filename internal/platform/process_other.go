//go:build !windows

package platform

import "syscall"

// IsProcessAlive reports whether the process with the given PID is running.
// On Unix-like systems this sends signal 0, which checks for existence
// without actually delivering a signal.
//
// Non-positive PIDs are never valid process identifiers and are rejected up
// front: syscall.Kill interprets pid 0 as "the caller's process group" and
// negative pids as other process groups, so passing them through would report
// a live process for a PID we never launched.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
