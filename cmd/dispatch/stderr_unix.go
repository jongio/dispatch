//go:build linux

package main

import (
	"os"
	"syscall"
)

// redirectStderr replaces file descriptor 2 so that child processes
// (notably the Copilot SDK subprocess) inherit the redirected fd
// instead of the real console stderr.
func captureOriginalStderr() *os.File {
	dupFD, err := syscall.Dup(2)
	if err != nil {
		return os.Stderr
	}
	return os.NewFile(uintptr(dupFD), "/dev/stderr")
}

func redirectStderr(target *os.File) {
	// Dup3 with flags=0 is equivalent to Dup2, which is unavailable
	// on linux/arm64.
	_ = syscall.Dup3(int(target.Fd()), 2, 0) //nolint:errcheck
	os.Stderr = target
}
