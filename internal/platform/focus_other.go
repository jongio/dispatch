//go:build !windows

package platform

import "fmt"

// FocusSessionWindow is a stub on non-Windows platforms.
// Window focus is currently only supported on Windows where dispatch
// can locate the terminal window via the Win32 API.
func FocusSessionWindow(pid int) error {
	return fmt.Errorf("window focus not supported on this platform")
}
