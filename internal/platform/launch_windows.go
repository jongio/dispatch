//go:build windows

package platform

import "errors"

// launchInPlaceUnix is not used on Windows but is present to satisfy the
// compiler. On Windows, LaunchSessionInPlace takes a different code path
// (launchInPlaceWindows) before this can be reached.
func launchInPlaceUnix(_ ShellInfo, _ string, _ string) error {
	return errors.New("launchInPlaceUnix: not supported on Windows")
}
