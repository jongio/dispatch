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
func captureOriginalStderr() *os.File {
	proc := windows.CurrentProcess()

	var dup windows.Handle
	if err := windows.DuplicateHandle(proc, windows.Handle(os.Stderr.Fd()), proc, &dup, 0, true, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return os.Stderr
	}
	return os.NewFile(uintptr(dup), "stderr")
}

func redirectStderr(target *os.File) {
	_ = windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(target.Fd()))
	os.Stderr = target
}
