//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// redirectStderr replaces the process-level stderr handle so that child
// processes (notably the Copilot SDK subprocess) inherit the redirected
// handle instead of the real console stderr.  On Windows we use
// SetStdHandle which affects what CreateProcess gives to children.
func redirectStderr(target *os.File) {
	_ = windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(target.Fd()))
	os.Stderr = target
}
