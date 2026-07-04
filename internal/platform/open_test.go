package platform

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenFile_NonExistentPath(t *testing.T) {
	t.Parallel()
	// OpenFile should not panic even for a non-existent path.
	// It will start the OS opener which may fail silently or report
	// an error through its own UI. We only verify no Go-level panic.
	err := OpenFile("/nonexistent/path/that/does/not/exist.txt")
	// On Windows cmd /c start returns immediately without error even for
	// missing paths; on Linux/macOS xdg-open/open may or may not error.
	// We just verify no panic occurred.
	_ = err
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
