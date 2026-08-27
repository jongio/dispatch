package platform

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenFile_EmptyPath(t *testing.T) {
	t.Parallel()
	if err := OpenFile(""); err == nil {
		t.Error("OpenFile(\"\") = nil, want an error for an empty path")
	}
}

func TestOpenFile_MissingPath(t *testing.T) {
	t.Parallel()
	// The OS openers detach and report a missing path through their own UI,
	// not through the exit status, so OpenFile must reject it up front or a
	// bad path is indistinguishable from a successful open.
	missing := filepath.Join(t.TempDir(), "does-not-exist.txt")
	if err := OpenFile(missing); err == nil {
		t.Error("OpenFile() = nil, want an error for a path that does not exist")
	}
}

func TestOpenFile_ErrorWhenOpenerUnavailable(t *testing.T) {
	// Past validation, OpenFile launches the platform opener
	// (explorer/open/xdg-open) and returns that process's start error. Use a
	// file that really exists so validation passes, then point PATH at an
	// empty dir so the opener cannot be resolved, and confirm the launch
	// failure is surfaced rather than swallowed.
	// (Cannot use t.Parallel with t.Setenv.)
	existing := filepath.Join(t.TempDir(), "real.txt")
	if err := os.WriteFile(existing, []byte("x"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	t.Setenv("PATH", t.TempDir())
	if err := OpenFile(existing); err == nil {
		t.Error("OpenFile() = nil, want an error when the platform opener cannot be started")
	}
}

func TestOpenCommand_PerOS(t *testing.T) {
	t.Parallel()
	cmd := openCommand(context.Background(), "/some/path")
	if len(cmd.Args) == 0 {
		t.Fatal("expected command args")
	}
	var want string
	switch runtime.GOOS {
	case "windows":
		want = "explorer"
	case "darwin":
		want = "open"
	default:
		want = "xdg-open"
	}
	if got := filepath.Base(cmd.Args[0]); got != want {
		t.Errorf("openCommand on %s = %q, want %q", runtime.GOOS, got, want)
	}
	last := cmd.Args[len(cmd.Args)-1]
	if last != "/some/path" {
		t.Errorf("openCommand path arg = %q, want %q", last, "/some/path")
	}
}

func TestOpenDir_EmptyPath(t *testing.T) {
	t.Parallel()
	if err := OpenDir(""); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestOpenDir_MissingPath(t *testing.T) {
	t.Parallel()
	if err := OpenDir(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Error("expected error for missing path")
	}
}

func TestOpenDir_FileNotDir(t *testing.T) {
	t.Parallel()
	f := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := OpenDir(f); err == nil {
		t.Error("expected error when path is a file, not a directory")
	}
}

func TestOpenURL_RejectsInvalid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"no scheme", "github.com/owner/repo"},
		{"file scheme", "file:///etc/passwd"},
		{"javascript scheme", "javascript:alert(1)"},
		{"no host", "https://"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := OpenURL(tt.url); err == nil {
				t.Errorf("OpenURL(%q) = nil, want error", tt.url)
			}
		})
	}
}
