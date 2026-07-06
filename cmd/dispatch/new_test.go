package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/update"
)

func TestParseNewArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantDir  string
		wantMode string
		wantErr  bool
	}{
		{name: "no args", args: []string{"new"}, wantDir: ""},
		{name: "dir only", args: []string{"new", "/tmp/proj"}, wantDir: "/tmp/proj"},
		{name: "mode space", args: []string{"new", "/tmp/proj", "--mode", "window"}, wantDir: "/tmp/proj", wantMode: "window"},
		{name: "mode equals", args: []string{"new", "/tmp/proj", "--mode=pane"}, wantDir: "/tmp/proj", wantMode: "pane"},
		{name: "short mode no dir", args: []string{"new", "-m", "tab"}, wantDir: "", wantMode: "tab"},
		{name: "mode before dir", args: []string{"new", "--mode", "inplace", "/x"}, wantDir: "/x", wantMode: "inplace"},
		{name: "two dirs", args: []string{"new", "a", "b"}, wantErr: true},
		{name: "unknown flag", args: []string{"new", "--nope"}, wantErr: true},
		{name: "mode without value", args: []string{"new", "--mode"}, wantErr: true},
		{name: "invalid mode", args: []string{"new", "--mode", "sideways"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir, mode, err := parseNewArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got dir=%q mode=%q", dir, mode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dir != tc.wantDir {
				t.Errorf("dir = %q, want %q", dir, tc.wantDir)
			}
			if mode != tc.wantMode {
				t.Errorf("mode = %q, want %q", mode, tc.wantMode)
			}
		})
	}
}

func TestResolveNewDir(t *testing.T) {
	// Existing directory resolves to an absolute path.
	tmp := t.TempDir()
	got, err := resolveNewDir(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("resolveNewDir(%q) = %q, want absolute path", tmp, got)
	}

	// Missing directory is a clear error.
	if _, err := resolveNewDir(filepath.Join(tmp, "does-not-exist")); err == nil {
		t.Error("expected error for missing directory")
	}

	// A file is not a directory.
	file := filepath.Join(tmp, "afile.txt")
	if wErr := os.WriteFile(file, []byte("x"), 0o600); wErr != nil {
		t.Fatalf("setup: %v", wErr)
	}
	if _, err := resolveNewDir(file); err == nil {
		t.Error("expected error when target is a file, not a directory")
	}
}

// newCapture records what defaultNewLaunch would have received.
type newCapture struct {
	launched bool
	dir      string
	mode     string
}

func withNewStubs(t *testing.T, cfg *config.Config) *newCapture {
	t.Helper()
	capture := &newCapture{}
	origCfg, origLaunch := newLoadConfigFn, newLaunchFn
	newLoadConfigFn = func() (*config.Config, error) { return cfg, nil }
	newLaunchFn = func(_ io.Writer, _ *config.Config, dir string, mode string) error {
		capture.launched = true
		capture.dir = dir
		capture.mode = mode
		return nil
	}
	t.Cleanup(func() {
		newLoadConfigFn, newLaunchFn = origCfg, origLaunch
	})
	return capture
}

func TestRunNew_HappyPath(t *testing.T) {
	cfg := config.Default()
	cfg.LaunchMode = config.LaunchModeTab
	capture := withNewStubs(t, cfg)

	tmp := t.TempDir()
	if err := runNew(io.Discard, []string{"new", tmp, "--mode", "window"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !capture.launched {
		t.Fatal("expected launch to be invoked")
	}
	if capture.mode != config.LaunchModeWindow {
		t.Errorf("launched mode = %q, want %q", capture.mode, config.LaunchModeWindow)
	}
	if !filepath.IsAbs(capture.dir) {
		t.Errorf("launched dir = %q, want absolute path", capture.dir)
	}
}

func TestRunNew_DefaultsToCurrentDir(t *testing.T) {
	cfg := config.Default()
	capture := withNewStubs(t, cfg)

	if err := runNew(io.Discard, []string{"new"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !capture.launched || capture.dir == "" {
		t.Errorf("expected launch in current directory, got %+v", capture)
	}
	if capture.mode != cfg.EffectiveLaunchMode() {
		t.Errorf("mode = %q, want configured default %q", capture.mode, cfg.EffectiveLaunchMode())
	}
}

func TestRunNew_MissingDir(t *testing.T) {
	withNewStubs(t, config.Default())
	tmp := t.TempDir()
	if err := runNew(io.Discard, []string{"new", filepath.Join(tmp, "nope")}); err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestRunNew_BadArgs(t *testing.T) {
	withNewStubs(t, config.Default())
	if err := runNew(io.Discard, []string{"new", "a", "b"}); err == nil {
		t.Fatal("expected error for too many directories")
	}
}

func TestHandleArgs_New(t *testing.T) {
	cfg := config.Default()
	withNewStubs(t, cfg)
	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	tmp := t.TempDir()
	done, _, _, err := handleArgs([]string{"new", tmp}, io.Discard, ch)
	if !done {
		t.Error("expected done=true for new")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
