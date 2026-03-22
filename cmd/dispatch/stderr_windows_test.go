//go:build windows

package main

import (
	"os"
	"testing"
)

func TestCaptureOriginalStderr_ReturnsValidFile(t *testing.T) {
	f := captureOriginalStderr()
	if f == nil {
		t.Fatal("captureOriginalStderr returned nil")
	}
	// The file should be writable (it's a dup of the stderr handle).
	if _, err := f.WriteString("test stderr capture\n"); err != nil {
		t.Errorf("writing to captured stderr: %v", err)
	}
	if f != os.Stderr {
		f.Close()
	}
}

func TestRedirectStderr_ChangesTarget(t *testing.T) {
	// Capture the original stderr handle so we can restore it.
	origFile := captureOriginalStderr()
	origStderr := os.Stderr
	defer func() {
		// Restore the Windows stderr handle and os.Stderr.
		if origFile != nil && origFile != origStderr {
			redirectStderr(origFile)
			origFile.Close()
		}
		os.Stderr = origStderr
	}()

	// Create a temp file as the redirect target.
	tmp, err := os.CreateTemp(t.TempDir(), "stderr-redirect-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer tmp.Close()

	redirectStderr(tmp)

	// os.Stderr should now point to the temp file.
	if os.Stderr != tmp {
		t.Error("os.Stderr should point to the target file after redirect")
	}
}
