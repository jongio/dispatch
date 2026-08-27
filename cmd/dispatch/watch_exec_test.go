package main

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jongio/dispatch/internal/data"
)

func envToMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			m[k] = v
		}
	}
	return m
}

func TestParseWatchArgs_Exec(t *testing.T) {
	opts, err := parseWatchArgs([]string{"watch", "--exec", "notify-send hi"})
	if err != nil {
		t.Fatalf("parseWatchArgs: %v", err)
	}
	if opts.exec != "notify-send hi" {
		t.Errorf("exec = %q, want %q", opts.exec, "notify-send hi")
	}
}

func TestParseWatchArgs_ExecMissingValue(t *testing.T) {
	_, err := parseWatchArgs([]string{"watch", "--exec"})
	if err == nil {
		t.Fatal("expected error for --exec with no value")
	}
}

func TestParseWatchArgs_ExecWithOnceRejected(t *testing.T) {
	_, err := parseWatchArgs([]string{"watch", "--exec", "echo hi", "--once"})
	if err == nil {
		t.Fatal("expected error when --exec is combined with --once")
	}
}

func TestHookEnv(t *testing.T) {
	meta := data.Session{
		Repository: "jongio/dispatch",
		Branch:     "main",
		Cwd:        "/home/u/dispatch",
		Summary:    "fixing watch",
	}
	env := hookEnv("abc123", "waiting", "working", meta)
	got := envToMap(env)

	want := map[string]string{
		"DISPATCH_SESSION_ID":         "abc123",
		"DISPATCH_SESSION_STATE":      "waiting",
		"DISPATCH_SESSION_PREV_STATE": "working",
		"DISPATCH_SESSION_REPO":       "jongio/dispatch",
		"DISPATCH_SESSION_BRANCH":     "main",
		"DISPATCH_SESSION_FOLDER":     "/home/u/dispatch",
		"DISPATCH_SESSION_SUMMARY":    "fixing watch",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("env[%s] = %q, want %q", k, got[k], v)
		}
	}
}

func TestFireWatchHook_InvokesExec(t *testing.T) {
	var gotCmd string
	var gotEnv []string
	prev := watchExecFn
	watchExecFn = func(_ context.Context, command string, env []string) error {
		gotCmd = command
		gotEnv = env
		return nil
	}
	t.Cleanup(func() { watchExecFn = prev })

	meta := data.Session{Repository: "o/r", Branch: "b", Cwd: "/w", Summary: "s"}
	fireWatchHook(context.Background(), "do-thing", "sess1", "waiting", "none", meta)

	if gotCmd != "do-thing" {
		t.Errorf("command = %q, want %q", gotCmd, "do-thing")
	}
	got := envToMap(gotEnv)
	if got["DISPATCH_SESSION_ID"] != "sess1" {
		t.Errorf("DISPATCH_SESSION_ID = %q, want sess1", got["DISPATCH_SESSION_ID"])
	}
	if got["DISPATCH_SESSION_PREV_STATE"] != "none" {
		t.Errorf("DISPATCH_SESSION_PREV_STATE = %q, want none", got["DISPATCH_SESSION_PREV_STATE"])
	}
}

func TestFireWatchHook_ErrorDoesNotBlock(t *testing.T) {
	// The name promises that a hook returning an error does not block the
	// watch loop and does not panic; the error must instead be surfaced to
	// stderr. Assert all three: the hook is invoked, fireWatchHook returns
	// promptly (checked via a timeout), and the error reaches stderr.
	called := false
	prev := watchExecFn
	watchExecFn = func(context.Context, string, []string) error {
		called = true
		return errors.New("boom")
	}
	t.Cleanup(func() { watchExecFn = prev })

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	done := make(chan struct{})
	go func() {
		fireWatchHook(context.Background(), "bad", "session-xyz", "waiting", "none", data.Session{})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		os.Stderr = origStderr
		t.Fatal("fireWatchHook blocked when the hook returned an error")
	}

	_ = w.Close()
	os.Stderr = origStderr

	out, _ := io.ReadAll(r)

	if !called {
		t.Error("fireWatchHook did not invoke the hook command")
	}
	if !strings.Contains(string(out), "boom") {
		t.Errorf("fireWatchHook should report the hook error to stderr, got: %q", string(out))
	}
}

func TestHookShell(t *testing.T) {
	shellPath, flag := hookShell()
	if shellPath == "" {
		t.Fatal("shell path is empty")
	}
	if runtime.GOOS == "windows" {
		if flag != "/c" {
			t.Errorf("flag = %q, want /c", flag)
		}
	} else if flag != "-c" {
		t.Errorf("flag = %q, want -c", flag)
	}
}

func TestRunWatchHook_Success(t *testing.T) {
	if err := runWatchHook(context.Background(), "exit 0", nil); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestRunWatchHook_NonZeroExit(t *testing.T) {
	if err := runWatchHook(context.Background(), "exit 1", nil); err == nil {
		t.Fatal("expected error for non-zero exit")
	}
}

func TestWatchSessionMeta(t *testing.T) {
	sessions := []data.Session{
		{ID: "a", Repository: "o/r", Branch: "main", Cwd: "/w/a", Summary: "sa"},
		{ID: "b", Repository: "o/r2", Branch: "dev", Cwd: "/w/b", Summary: "sb"},
	}
	withWatchSeams(t, map[string]data.AttentionStatus{}, sessions)

	meta := watchSessionMeta(watchOptions{})
	if len(meta) != 2 {
		t.Fatalf("meta len = %d, want 2", len(meta))
	}
	if meta["a"].Cwd != "/w/a" {
		t.Errorf("meta[a].Cwd = %q, want /w/a", meta["a"].Cwd)
	}
	if meta["b"].Branch != "dev" {
		t.Errorf("meta[b].Branch = %q, want dev", meta["b"].Branch)
	}
}
