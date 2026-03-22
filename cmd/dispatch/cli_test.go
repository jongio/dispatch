package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/update"
)

// ---------------------------------------------------------------------------
// handleArgs
// ---------------------------------------------------------------------------

func TestHandleArgs_Help(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil // no update available

	done, cleanup, err := handleArgs([]string{"--help"}, io.Discard, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true for --help")
	}
	if cleanup != nil {
		t.Error("expected cleanup=nil for --help")
	}
}

func TestHandleArgs_HelpShort(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	done, _, err := handleArgs([]string{"-h"}, io.Discard, ch)
	if err != nil || !done {
		t.Errorf("expected done=true, no error for -h; got done=%v, err=%v", done, err)
	}
}

func TestHandleArgs_HelpCommand(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	done, _, err := handleArgs([]string{"help"}, io.Discard, ch)
	if err != nil || !done {
		t.Errorf("expected done=true, no error for help; got done=%v, err=%v", done, err)
	}
}

func TestHandleArgs_Version(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	done, _, err := handleArgs([]string{"--version"}, io.Discard, ch)
	if err != nil || !done {
		t.Errorf("expected done=true, no error for --version; got done=%v, err=%v", done, err)
	}
}

func TestHandleArgs_VersionShort(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	done, _, err := handleArgs([]string{"-v"}, io.Discard, ch)
	if err != nil || !done {
		t.Errorf("expected done=true, no error for -v; got done=%v, err=%v", done, err)
	}
}

func TestHandleArgs_VersionCommand(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	done, _, err := handleArgs([]string{"version"}, io.Discard, ch)
	if err != nil || !done {
		t.Errorf("expected done=true, no error for version; got done=%v, err=%v", done, err)
	}
}

func TestHandleArgs_UnknownFlag(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)

	done, _, err := handleArgs([]string{"--unknown"}, io.Discard, ch)
	if err == nil {
		t.Error("expected error for unknown flag")
	}
	if !done {
		t.Error("expected done=true for unknown flag")
	}
	if !strings.Contains(err.Error(), "--unknown") {
		t.Errorf("error should mention the flag, got: %v", err)
	}
}

func TestHandleArgs_NoArgs(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)

	done, cleanup, err := handleArgs(nil, io.Discard, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("expected done=false for no args")
	}
	if cleanup != nil {
		t.Error("expected cleanup=nil for no args")
	}
}

func TestHandleArgs_ClearCache(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)

	done, _, err := handleArgs([]string{"--clear-cache"}, io.Discard, ch)
	// config.Reset may succeed or fail depending on environment.
	// Either way, done should be true.
	if !done {
		t.Error("expected done=true for --clear-cache")
	}
	_ = err // error is acceptable if config dir doesn't exist
}

func TestHandleArgs_DemoFromRepoRoot(t *testing.T) {
	// Preserve environment vars that setupDemo modifies.
	t.Setenv("DISPATCH_DB", os.Getenv("DISPATCH_DB"))
	t.Setenv("DISPATCH_SESSION_STATE", os.Getenv("DISPATCH_SESSION_STATE"))

	// Find the module root.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("could not find module root")
		}
		dir = parent
	}
	t.Chdir(dir)

	ch := make(chan *update.UpdateInfo, 1)

	done, cleanup, err := handleArgs([]string{"--demo"}, io.Discard, ch)
	if err != nil {
		t.Fatalf("--demo from repo root should succeed: %v", err)
	}
	if done {
		t.Error("--demo should not set done (TUI continues after demo setup)")
	}
	if cleanup == nil {
		t.Error("--demo should return non-nil cleanup function")
	}
	if cleanup != nil {
		cleanup()
	}
}

func TestHandleArgs_DemoNotFound(t *testing.T) {
	t.Setenv("DISPATCH_DB", "")
	t.Setenv("DISPATCH_SESSION_STATE", "")
	t.Chdir(t.TempDir())

	ch := make(chan *update.UpdateInfo, 1)

	done, _, err := handleArgs([]string{"--demo"}, io.Discard, ch)
	if err == nil {
		t.Error("expected error when demo DB not found")
	}
	if !done {
		t.Error("expected done=true on demo error")
	}
}

func TestHandleArgs_HelpWithUpdate(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)
	ch <- &update.UpdateInfo{
		CurrentVersion: "1.0.0",
		LatestVersion:  "2.0.0",
	}

	done, _, err := handleArgs([]string{"--help"}, io.Discard, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true for --help")
	}
}

func TestHandleArgs_VersionWithUpdate(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)
	ch <- &update.UpdateInfo{
		CurrentVersion: "1.0.0",
		LatestVersion:  "2.0.0",
	}

	done, _, err := handleArgs([]string{"--version"}, io.Discard, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true for --version")
	}
}

func TestHandleArgs_Reindex_CopilotNotFound(t *testing.T) {
	oldReindex := chronicleReindexFn
	oldMaintain := maintainFn
	defer func() {
		chronicleReindexFn = oldReindex
		maintainFn = oldMaintain
	}()

	chronicleReindexFn = func(_ context.Context, _ func(string)) error {
		return data.ErrCopilotNotFound
	}
	maintainFn = func() error { return nil }

	ch := make(chan *update.UpdateInfo, 1)

	done, _, err := handleArgs([]string{"--reindex"}, io.Discard, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true for --reindex")
	}
}

func TestHandleArgs_Reindex_OtherError(t *testing.T) {
	oldReindex := chronicleReindexFn
	defer func() { chronicleReindexFn = oldReindex }()

	chronicleReindexFn = func(_ context.Context, _ func(string)) error {
		return fmt.Errorf("connection refused")
	}

	ch := make(chan *update.UpdateInfo, 1)

	done, _, err := handleArgs([]string{"--reindex"}, io.Discard, ch)
	if err == nil {
		t.Error("expected error for non-CopilotNotFound reindex failure")
	}
	if !done {
		t.Error("expected done=true")
	}
}

func TestHandleArgs_Reindex_MaintainError(t *testing.T) {
	oldReindex := chronicleReindexFn
	oldMaintain := maintainFn
	defer func() {
		chronicleReindexFn = oldReindex
		maintainFn = oldMaintain
	}()

	chronicleReindexFn = func(_ context.Context, _ func(string)) error {
		return data.ErrCopilotNotFound
	}
	maintainFn = func() error { return fmt.Errorf("db locked") }

	ch := make(chan *update.UpdateInfo, 1)

	done, _, err := handleArgs([]string{"--reindex"}, io.Discard, ch)
	if err == nil {
		t.Error("expected error when maintain fails")
	}
	if !done {
		t.Error("expected done=true")
	}
}

func TestHandleArgs_Reindex_Success(t *testing.T) {
	oldReindex := chronicleReindexFn
	oldMaintain := maintainFn
	defer func() {
		chronicleReindexFn = oldReindex
		maintainFn = oldMaintain
	}()

	chronicleReindexFn = func(_ context.Context, cb func(string)) error {
		cb("Processing 10 sessions...")
		return nil
	}
	maintainFn = func() error { return nil }

	ch := make(chan *update.UpdateInfo, 1)

	done, _, err := handleArgs([]string{"--reindex"}, io.Discard, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true")
	}
}

func TestHandleArgs_UpdateSuccess(t *testing.T) {
	old := runUpdateFn
	defer func() { runUpdateFn = old }()
	runUpdateFn = func(_ string) error { return nil }

	ch := make(chan *update.UpdateInfo, 1)

	done, _, err := handleArgs([]string{"update"}, io.Discard, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true")
	}
}

func TestHandleArgs_UpdateError(t *testing.T) {
	old := runUpdateFn
	defer func() { runUpdateFn = old }()
	runUpdateFn = func(_ string) error { return fmt.Errorf("network unreachable") }

	ch := make(chan *update.UpdateInfo, 1)

	done, _, err := handleArgs([]string{"update"}, io.Discard, ch)
	if err == nil {
		t.Error("expected error for failed update")
	}
	if !done {
		t.Error("expected done=true")
	}
}

func TestHandleArgs_ClearCacheError(t *testing.T) {
	old := configResetFn
	defer func() { configResetFn = old }()
	configResetFn = func() error { return fmt.Errorf("permission denied") }

	ch := make(chan *update.UpdateInfo, 1)

	done, _, err := handleArgs([]string{"--clear-cache"}, io.Discard, ch)
	if err == nil {
		t.Error("expected error for failed config reset")
	}
	if !done {
		t.Error("expected done=true")
	}
}

func TestHandleArgs_Reindex_PostMaintainWarning(t *testing.T) {
	oldReindex := chronicleReindexFn
	oldMaintain := maintainFn
	defer func() {
		chronicleReindexFn = oldReindex
		maintainFn = oldMaintain
	}()

	// ChronicleReindex succeeds, but post-reindex Maintain returns an error
	// (non-fatal warning path).
	chronicleReindexFn = func(_ context.Context, cb func(string)) error {
		cb("Done processing.")
		return nil
	}
	calls := 0
	maintainFn = func() error {
		calls++
		if calls == 1 {
			return fmt.Errorf("db busy")
		}
		return nil
	}

	ch := make(chan *update.UpdateInfo, 1)

	done, _, err := handleArgs([]string{"--reindex"}, io.Discard, ch)
	// The post-reindex maintain warning is non-fatal, so handleArgs should
	// still succeed.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true")
	}
}

// ---------------------------------------------------------------------------
// openLogFile
// ---------------------------------------------------------------------------

func TestOpenLogFile_Empty(t *testing.T) {
	t.Parallel()
	if f := openLogFile(""); f != nil {
		f.Close()
		t.Error("expected nil for empty path")
	}
}

func TestOpenLogFile_RelativePath(t *testing.T) {
	t.Parallel()
	if f := openLogFile("relative/path.log"); f != nil {
		f.Close()
		t.Error("expected nil for relative path")
	}
}

func TestOpenLogFile_UNCPath(t *testing.T) {
	t.Parallel()
	if f := openLogFile(`\\server\share\log.txt`); f != nil {
		f.Close()
		t.Error("expected nil for UNC path")
	}
}

func TestOpenLogFile_ValidAbsolutePath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	f := openLogFile(logPath)
	if f == nil {
		t.Fatal("expected non-nil file for valid absolute path")
	}
	defer f.Close()

	// Verify we can write to it.
	msg := "test log message\n"
	if _, err := f.WriteString(msg); err != nil {
		t.Fatalf("writing to log file: %v", err)
	}

	// Close and verify content.
	f.Close()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != msg {
		t.Errorf("log content = %q, want %q", string(data), msg)
	}
}

func TestOpenLogFile_NonexistentDir(t *testing.T) {
	t.Parallel()
	badPath := filepath.Join(t.TempDir(), "no", "such", "dir", "log.txt")
	if f := openLogFile(badPath); f != nil {
		f.Close()
		t.Error("expected nil for path in nonexistent directory")
	}
}

func TestOpenLogFile_AppendMode(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "append.log")

	// Write first entry.
	f1 := openLogFile(logPath)
	if f1 == nil {
		t.Fatal("first open returned nil")
	}
	_, _ = f1.WriteString("line1\n")
	f1.Close()

	// Write second entry — should append.
	f2 := openLogFile(logPath)
	if f2 == nil {
		t.Fatal("second open returned nil")
	}
	_, _ = f2.WriteString("line2\n")
	f2.Close()

	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "line1") || !strings.Contains(string(data), "line2") {
		t.Errorf("expected both lines in file, got: %s", string(data))
	}
}

// ---------------------------------------------------------------------------
// setupLogRedirect
// ---------------------------------------------------------------------------

func TestSetupLogRedirect_NoLogFile(t *testing.T) {
	// Save and restore stderr since setupLogRedirect calls redirectStderr.
	origStderr := os.Stderr
	origFile := captureOriginalStderr()
	defer func() {
		if origFile != nil && origFile != origStderr {
			redirectStderr(origFile)
			origFile.Close()
		}
		os.Stderr = origStderr
	}()

	t.Setenv("DISPATCH_LOG", "")

	w, cleanup := setupLogRedirect()
	defer cleanup()

	if w == nil {
		t.Error("writer should not be nil")
	}
}

func TestSetupLogRedirect_WithLogFile(t *testing.T) {
	// Save and restore stderr since setupLogRedirect calls redirectStderr.
	origStderr := os.Stderr
	origFile := captureOriginalStderr()
	defer func() {
		if origFile != nil && origFile != origStderr {
			redirectStderr(origFile)
			origFile.Close()
		}
		os.Stderr = origStderr
	}()

	logPath := filepath.Join(t.TempDir(), "test.log")
	t.Setenv("DISPATCH_LOG", logPath)

	w, cleanup := setupLogRedirect()
	defer cleanup()

	if w == nil {
		t.Error("writer should not be nil when log file is configured")
	}

	// The writer should be the log file — verify it's writable.
	if f, ok := w.(*os.File); ok {
		if _, err := f.WriteString("test entry\n"); err != nil {
			t.Errorf("writing to log writer: %v", err)
		}
	}
}
