//go:build windows

package platform

import "golang.org/x/sys/windows"

// IsProcessAlive reports whether the process with the given PID is running.
// On Windows this opens the process with PROCESS_QUERY_LIMITED_INFORMATION
// and closes the handle immediately.
//
// Non-positive PIDs are never valid process identifiers and are rejected up
// front so behavior matches the Unix implementation.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	_ = windows.CloseHandle(h)
	return true
}
