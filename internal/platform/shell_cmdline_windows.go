//go:build windows

package platform

import (
	"os/exec"
	"syscall"
)

// setCmdLine sets the raw command line on the process, bypassing Go's
// syscall.EscapeArg quoting which breaks cmd.exe argument handling.
func setCmdLine(cmd *exec.Cmd, cmdLine string) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: cmdLine}
}
