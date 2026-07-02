package main

import (
	"errors"
	"io"
	"testing"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/update"
)

func TestParseOpenArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantID   string
		wantMode string
		wantErr  bool
	}{
		{name: "id only", args: []string{"open", "abc123"}, wantID: "abc123"},
		{name: "mode space", args: []string{"open", "abc", "--mode", "window"}, wantID: "abc", wantMode: "window"},
		{name: "mode equals", args: []string{"open", "abc", "--mode=pane"}, wantID: "abc", wantMode: "pane"},
		{name: "short mode", args: []string{"open", "-m", "tab", "abc"}, wantID: "abc", wantMode: "tab"},
		{name: "mode before id", args: []string{"open", "--mode", "inplace", "xyz"}, wantID: "xyz", wantMode: "inplace"},
		{name: "missing id", args: []string{"open"}, wantErr: true},
		{name: "missing id with mode", args: []string{"open", "--mode", "tab"}, wantErr: true},
		{name: "two ids", args: []string{"open", "a", "b"}, wantErr: true},
		{name: "unknown flag", args: []string{"open", "--nope", "a"}, wantErr: true},
		{name: "mode without value", args: []string{"open", "a", "--mode"}, wantErr: true},
		{name: "invalid mode", args: []string{"open", "a", "--mode", "sideways"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, mode, err := parseOpenArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got id=%q mode=%q", id, mode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tc.wantID {
				t.Errorf("id = %q, want %q", id, tc.wantID)
			}
			if mode != tc.wantMode {
				t.Errorf("mode = %q, want %q", mode, tc.wantMode)
			}
		})
	}
}

func TestNormalizeLaunchMode(t *testing.T) {
	valid := map[string]string{
		"inplace":  config.LaunchModeInPlace,
		"in-place": config.LaunchModeInPlace,
		"TAB":      config.LaunchModeTab,
		"Window":   config.LaunchModeWindow,
		"pane":     config.LaunchModePane,
	}
	for in, want := range valid {
		got, err := normalizeLaunchMode(in)
		if err != nil {
			t.Errorf("normalizeLaunchMode(%q) unexpected error: %v", in, err)
		}
		if got != want {
			t.Errorf("normalizeLaunchMode(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := normalizeLaunchMode("bogus"); err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestResolveOpenMode(t *testing.T) {
	cfg := config.Default()
	cfg.LaunchMode = config.LaunchModeWindow

	if got := resolveOpenMode("pane", cfg); got != config.LaunchModePane {
		t.Errorf("flag override = %q, want %q", got, config.LaunchModePane)
	}
	if got := resolveOpenMode("", cfg); got != config.LaunchModeWindow {
		t.Errorf("default = %q, want %q", got, config.LaunchModeWindow)
	}
}

func TestLaunchStyleForOpenMode(t *testing.T) {
	cases := map[string]string{
		config.LaunchModeWindow:  "window",
		config.LaunchModePane:    "pane",
		config.LaunchModeTab:     "",
		config.LaunchModeInPlace: "",
	}
	for mode, want := range cases {
		if got := launchStyleForOpenMode(mode); got != want {
			t.Errorf("launchStyleForOpenMode(%q) = %q, want %q", mode, got, want)
		}
	}
}

// withOpenStubs swaps the package test seams and returns a restore func.
func withOpenStubs(t *testing.T, cfg *config.Config, sess *data.Session, getErr error) *openCapture {
	t.Helper()
	capture := &openCapture{}
	origCfg, origGet, origLaunch := openLoadConfigFn, openGetSessionFn, openLaunchFn
	openLoadConfigFn = func() (*config.Config, error) { return cfg, nil }
	openGetSessionFn = func(id string) (*data.Session, error) {
		capture.gotID = id
		return sess, getErr
	}
	openLaunchFn = func(_ io.Writer, c *config.Config, s *data.Session, mode string) error {
		capture.launched = true
		capture.mode = mode
		capture.session = s
		return nil
	}
	t.Cleanup(func() {
		openLoadConfigFn, openGetSessionFn, openLaunchFn = origCfg, origGet, origLaunch
	})
	return capture
}

type openCapture struct {
	gotID    string
	launched bool
	mode     string
	session  *data.Session
}

func TestRunOpen_HappyPath(t *testing.T) {
	cfg := config.Default()
	cfg.LaunchMode = config.LaunchModeTab
	sess := &data.Session{ID: "sess-1", Cwd: "/tmp/project"}
	capture := withOpenStubs(t, cfg, sess, nil)

	if err := runOpen(io.Discard, []string{"open", "sess-1", "--mode", "window"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capture.gotID != "sess-1" {
		t.Errorf("looked up id %q, want sess-1", capture.gotID)
	}
	if !capture.launched {
		t.Fatal("expected launch to be invoked")
	}
	if capture.mode != config.LaunchModeWindow {
		t.Errorf("launched mode = %q, want %q", capture.mode, config.LaunchModeWindow)
	}
}

func TestRunOpen_NotFound(t *testing.T) {
	withOpenStubs(t, config.Default(), nil, nil)
	err := runOpen(io.Discard, []string{"open", "missing"})
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}

func TestRunOpen_LookupError(t *testing.T) {
	withOpenStubs(t, config.Default(), nil, errors.New("boom"))
	if err := runOpen(io.Discard, []string{"open", "x"}); err == nil {
		t.Fatal("expected lookup error to propagate")
	}
}

func TestRunOpen_BadArgs(t *testing.T) {
	if err := runOpen(io.Discard, []string{"open"}); err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestHandleArgs_OpenMissingID(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil
	// parseOpenArgs fails before touching config or the store, so this is safe.
	done, _, _, err := handleArgs([]string{"open"}, io.Discard, ch)
	if !done {
		t.Error("expected done=true for open")
	}
	if err == nil {
		t.Error("expected error for open with no session ID")
	}
}
