//go:build darwin

package main

import (
	"os"
	"syscall"
)

// redirectStderr replaces file descriptor 2 so that child processes
// (notably the Copilot SDK subprocess) inherit the redirected fd
// instead of the real console stderr.
func redirectStderr(target *os.File) {
	_ = syscall.Dup2(int(target.Fd()), 2) //nolint:errcheck
	os.Stderr = target
}
