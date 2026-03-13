//go:build !windows

package platform

import "os/exec"

// setCmdLine is a no-op stub for non-Windows platforms. The cmd.exe launch
// path that requires raw CmdLine control is only reachable on Windows.
func setCmdLine(_ *exec.Cmd, _ string) {}
