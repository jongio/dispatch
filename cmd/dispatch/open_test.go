package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/update"
)

func TestParseOpenArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantID    string
		wantMode  string
		wantLast  bool
		wantPrint bool
		wantStdin bool
		wantErr   bool
	}{
		{name: "id only", args: []string{"open", "abc123"}, wantID: "abc123"},
		{name: "mode space", args: []string{"open", "abc", "--mode", "window"}, wantID: "abc", wantMode: "window"},
		{name: "mode equals", args: []string{"open", "abc", "--mode=pane"}, wantID: "abc", wantMode: "pane"},
		{name: "short mode", args: []string{"open", "-m", "tab", "abc"}, wantID: "abc", wantMode: "tab"},
		{name: "mode before id", args: []string{"open", "--mode", "inplace", "xyz"}, wantID: "xyz", wantMode: "inplace"},
		{name: "last", args: []string{"open", "--last"}, wantLast: true},
		{name: "last short", args: []string{"open", "-l"}, wantLast: true},
		{name: "last with mode", args: []string{"open", "--last", "--mode", "window"}, wantLast: true, wantMode: "window"},
		{name: "print flag", args: []string{"open", "abc", "--print"}, wantID: "abc", wantPrint: true},
		{name: "print before id", args: []string{"open", "--print", "abc"}, wantID: "abc", wantPrint: true},
		{name: "print with mode", args: []string{"open", "abc", "--print", "--mode", "tab"}, wantID: "abc", wantMode: "tab", wantPrint: true},
		{name: "stdin", args: []string{"open", "--stdin"}, wantStdin: true},
		{name: "stdin with mode", args: []string{"open", "--stdin", "--mode", "tab"}, wantStdin: true, wantMode: "tab"},
		{name: "stdin with print", args: []string{"open", "--stdin", "--print"}, wantStdin: true, wantPrint: true},
		{name: "missing id", args: []string{"open"}, wantErr: true},
		{name: "missing id with mode", args: []string{"open", "--mode", "tab"}, wantErr: true},
		{name: "two ids", args: []string{"open", "a", "b"}, wantErr: true},
		{name: "last with id", args: []string{"open", "--last", "abc"}, wantErr: true},
		{name: "stdin with id", args: []string{"open", "--stdin", "abc"}, wantErr: true},
		{name: "stdin with last", args: []string{"open", "--stdin", "--last"}, wantErr: true},
		{name: "unknown flag", args: []string{"open", "--nope", "a"}, wantErr: true},
		{name: "mode without value", args: []string{"open", "a", "--mode"}, wantErr: true},
		{name: "invalid mode", args: []string{"open", "a", "--mode", "sideways"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, mode, last, printCmd, stdin, _, err := parseOpenArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got id=%q mode=%q last=%v", id, mode, last)
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
			if last != tc.wantLast {
				t.Errorf("last = %v, want %v", last, tc.wantLast)
			}
			if printCmd != tc.wantPrint {
				t.Errorf("print = %v, want %v", printCmd, tc.wantPrint)
			}
			if stdin != tc.wantStdin {
				t.Errorf("stdin = %v, want %v", stdin, tc.wantStdin)
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

func TestRunOpen_ResolvesAlias(t *testing.T) {
	cfg := config.Default()
	cfg.SessionAliases = map[string]string{"sess-1": "authfix"}
	sess := &data.Session{ID: "sess-1", Cwd: "/tmp/project"}
	capture := withOpenStubs(t, cfg, sess, nil)

	if err := runOpen(io.Discard, []string{"open", "authfix"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capture.gotID != "sess-1" {
		t.Errorf("resolved id = %q, want sess-1", capture.gotID)
	}
	if !capture.launched {
		t.Fatal("expected launch to be invoked")
	}
}

func TestRunOpen_UnknownAliasFallsBackToID(t *testing.T) {
	cfg := config.Default()
	cfg.SessionAliases = map[string]string{"sess-1": "authfix"}
	sess := &data.Session{ID: "raw-id", Cwd: "/tmp/p"}
	capture := withOpenStubs(t, cfg, sess, nil)

	if err := runOpen(io.Discard, []string{"open", "raw-id"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capture.gotID != "raw-id" {
		t.Errorf("id = %q, want raw-id (fallback)", capture.gotID)
	}
}

func TestRunOpen_Print(t *testing.T) {
	cfg := config.Default()
	sess := &data.Session{ID: "sess-1", Cwd: "/tmp/project"}
	capture := withOpenStubs(t, cfg, sess, nil)

	origResume := openResumeCmdFn
	openResumeCmdFn = func(id string, _ platform.ResumeConfig) (string, error) {
		return "copilot --resume " + id, nil
	}
	t.Cleanup(func() { openResumeCmdFn = origResume })

	var buf bytes.Buffer
	if err := runOpen(&buf, []string{"open", "sess-1", "--print"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capture.launched {
		t.Error("expected --print to skip launching")
	}
	if got := strings.TrimSpace(buf.String()); got != "copilot --resume sess-1" {
		t.Errorf("output = %q, want %q", got, "copilot --resume sess-1")
	}
}

func TestRunOpen_PrintError(t *testing.T) {
	withOpenStubs(t, config.Default(), &data.Session{ID: "s"}, nil)

	origResume := openResumeCmdFn
	openResumeCmdFn = func(string, platform.ResumeConfig) (string, error) {
		return "", errors.New("no cli")
	}
	t.Cleanup(func() { openResumeCmdFn = origResume })

	if err := runOpen(io.Discard, []string{"open", "s", "--print"}); err == nil {
		t.Fatal("expected resume command error to propagate")
	}
}

func TestRunOpen_OverridesFlowToResumeConfig(t *testing.T) {
	cfg := config.Default()
	cfg.Agent = "base-agent"
	cfg.Model = "base-model"
	cfg.YoloMode = false
	sess := &data.Session{ID: "sess-1", Cwd: "/tmp/project"}
	withOpenStubs(t, cfg, sess, nil)

	var gotRC platform.ResumeConfig
	origResume := openResumeCmdFn
	openResumeCmdFn = func(id string, rc platform.ResumeConfig) (string, error) {
		gotRC = rc
		return "copilot --resume " + id, nil
	}
	t.Cleanup(func() { openResumeCmdFn = origResume })

	args := []string{"open", "sess-1", "--print", "--agent", "coder", "--model", "gpt-5", "--yolo"}
	if err := runOpen(io.Discard, args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotRC.Agent != "coder" {
		t.Errorf("agent = %q, want %q", gotRC.Agent, "coder")
	}
	if gotRC.Model != "gpt-5" {
		t.Errorf("model = %q, want %q", gotRC.Model, "gpt-5")
	}
	if !gotRC.YoloMode {
		t.Error("yolo = false, want true")
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

// withOpenLastStub swaps openGetLastSessionFn (and the config/launch seams via
// withOpenStubs) so --last paths can be exercised without a real store.
func withOpenLastStub(t *testing.T, cfg *config.Config, sess *data.Session, getErr error) *openCapture {
	t.Helper()
	capture := withOpenStubs(t, cfg, sess, nil)
	origLast := openGetLastSessionFn
	openGetLastSessionFn = func() (*data.Session, error) { return sess, getErr }
	t.Cleanup(func() { openGetLastSessionFn = origLast })
	return capture
}

func TestRunOpen_Last(t *testing.T) {
	cfg := config.Default()
	cfg.LaunchMode = config.LaunchModeTab
	sess := &data.Session{ID: "recent-1", Cwd: "/tmp/project"}
	capture := withOpenLastStub(t, cfg, sess, nil)

	if err := runOpen(io.Discard, []string{"open", "--last"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !capture.launched {
		t.Fatal("expected launch to be invoked")
	}
	if capture.session == nil || capture.session.ID != "recent-1" {
		t.Errorf("launched session = %+v, want recent-1", capture.session)
	}
}

func TestRunOpen_LastEmptyStore(t *testing.T) {
	withOpenLastStub(t, config.Default(), nil, nil)
	if err := runOpen(io.Discard, []string{"open", "--last"}); err == nil {
		t.Fatal("expected error when no sessions to resume")
	}
}

func TestRunOpen_LastLookupError(t *testing.T) {
	withOpenLastStub(t, config.Default(), nil, errors.New("boom"))
	if err := runOpen(io.Discard, []string{"open", "--last"}); err == nil {
		t.Fatal("expected lookup error to propagate")
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

// withOpenBatchStubs installs config, per-ID session lookup, and launch seams
// for --stdin batch tests. sessions maps a session ID to the session returned
// for it; an ID absent from the map resolves to "not found" (nil, nil). The
// returned slice records the IDs launched, in order.
func withOpenBatchStubs(t *testing.T, cfg *config.Config, sessions map[string]*data.Session) *[]string {
	t.Helper()
	launched := &[]string{}
	origCfg, origGet, origLaunch := openLoadConfigFn, openGetSessionFn, openLaunchFn
	openLoadConfigFn = func() (*config.Config, error) { return cfg, nil }
	openGetSessionFn = func(id string) (*data.Session, error) {
		return sessions[id], nil
	}
	openLaunchFn = func(_ io.Writer, _ *config.Config, s *data.Session, _ string) error {
		*launched = append(*launched, s.ID)
		return nil
	}
	t.Cleanup(func() {
		openLoadConfigFn, openGetSessionFn, openLaunchFn = origCfg, origGet, origLaunch
	})
	return launched
}

// withStdin points the openStdin seam at s for the duration of the test.
func withStdin(t *testing.T, s string) {
	t.Helper()
	orig := openStdin
	openStdin = strings.NewReader(s)
	t.Cleanup(func() { openStdin = orig })
}

func TestReadSessionIDs(t *testing.T) {
	in := strings.Join([]string{
		"sess-1",
		"  sess-2  ",   // trimmed
		"",             // blank skipped
		"# a comment",  // comment skipped
		"sess-1",       // duplicate skipped
		"sess-3 extra", // first field only
		"\tsess-4\t",   // tabs trimmed
	}, "\n")

	got, err := readSessionIDs(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"sess-1", "sess-2", "sess-3", "sess-4"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("id[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestReadSessionIDs_NilReader(t *testing.T) {
	got, err := readSessionIDs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestRunOpen_StdinBatch(t *testing.T) {
	cfg := config.Default()
	cfg.LaunchMode = config.LaunchModeInPlace // default is inplace; --mode overrides
	sessions := map[string]*data.Session{
		"sess-1": {ID: "sess-1", Cwd: "/tmp/a"},
		"sess-2": {ID: "sess-2", Cwd: "/tmp/b"},
	}
	launched := withOpenBatchStubs(t, cfg, sessions)
	withStdin(t, "sess-1\nsess-2\n")

	if err := runOpen(io.Discard, []string{"open", "--stdin", "--mode", "tab"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*launched) != 2 || (*launched)[0] != "sess-1" || (*launched)[1] != "sess-2" {
		t.Errorf("launched = %v, want [sess-1 sess-2]", *launched)
	}
}

func TestRunOpen_StdinResolvesAlias(t *testing.T) {
	cfg := config.Default()
	cfg.LaunchMode = config.LaunchModeTab
	cfg.SessionAliases = map[string]string{"sess-1": "authfix"}
	sessions := map[string]*data.Session{"sess-1": {ID: "sess-1", Cwd: "/tmp/a"}}
	launched := withOpenBatchStubs(t, cfg, sessions)
	withStdin(t, "authfix\n")

	if err := runOpen(io.Discard, []string{"open", "--stdin"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*launched) != 1 || (*launched)[0] != "sess-1" {
		t.Errorf("launched = %v, want [sess-1]", *launched)
	}
}

func TestRunOpen_StdinInplaceRejected(t *testing.T) {
	cfg := config.Default()
	cfg.LaunchMode = config.LaunchModeInPlace
	withOpenBatchStubs(t, cfg, map[string]*data.Session{"s": {ID: "s"}})
	withStdin(t, "s\n")

	// No explicit mode, so the inplace default applies and batch must reject it.
	if err := runOpen(io.Discard, []string{"open", "--stdin"}); err == nil {
		t.Fatal("expected error rejecting inplace mode for batch resume")
	}
}

func TestRunOpen_StdinEmpty(t *testing.T) {
	cfg := config.Default()
	cfg.LaunchMode = config.LaunchModeTab
	withOpenBatchStubs(t, cfg, nil)
	withStdin(t, "\n#only a comment\n")

	if err := runOpen(io.Discard, []string{"open", "--stdin"}); err == nil {
		t.Fatal("expected error when standard input has no session IDs")
	}
}

func TestRunOpen_StdinPartialFailure(t *testing.T) {
	cfg := config.Default()
	cfg.LaunchMode = config.LaunchModeTab
	sessions := map[string]*data.Session{"sess-1": {ID: "sess-1", Cwd: "/tmp/a"}}
	launched := withOpenBatchStubs(t, cfg, sessions)
	withStdin(t, "sess-1\nmissing\n")

	var buf bytes.Buffer
	err := runOpen(&buf, []string{"open", "--stdin"})
	if err == nil {
		t.Fatal("expected an aggregated error for the missing session")
	}
	if len(*launched) != 1 || (*launched)[0] != "sess-1" {
		t.Errorf("launched = %v, want [sess-1] (found session still resumes)", *launched)
	}
	if !strings.Contains(buf.String(), "resumed 1 of 2") {
		t.Errorf("summary = %q, want it to mention resumed 1 of 2", buf.String())
	}
}

func TestRunOpen_StdinPrint(t *testing.T) {
	cfg := config.Default()
	cfg.LaunchMode = config.LaunchModeInPlace // print ignores mode entirely
	sessions := map[string]*data.Session{
		"sess-1": {ID: "sess-1", Cwd: "/tmp/a"},
		"sess-2": {ID: "sess-2", Cwd: "/tmp/b"},
	}
	launched := withOpenBatchStubs(t, cfg, sessions)
	withStdin(t, "sess-1\nsess-2\n")

	origResume := openResumeCmdFn
	openResumeCmdFn = func(id string, _ platform.ResumeConfig) (string, error) {
		return "copilot --resume " + id, nil
	}
	t.Cleanup(func() { openResumeCmdFn = origResume })

	var buf bytes.Buffer
	if err := runOpen(&buf, []string{"open", "--stdin", "--print"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*launched) != 0 {
		t.Errorf("expected --print to skip launching, launched = %v", *launched)
	}
	out := buf.String()
	if !strings.Contains(out, "copilot --resume sess-1") || !strings.Contains(out, "copilot --resume sess-2") {
		t.Errorf("output = %q, want resume commands for both sessions", out)
	}
}
