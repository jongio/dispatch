package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanAllPlans(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	// Create session directories — some with plans, some without.
	os.MkdirAll(filepath.Join(dir, "session-a"), 0o755)
	os.MkdirAll(filepath.Join(dir, "session-b"), 0o755)
	os.MkdirAll(filepath.Join(dir, "session-c"), 0o755)

	os.WriteFile(filepath.Join(dir, "session-a", "plan.md"), []byte("# Plan A"), 0o644)
	os.WriteFile(filepath.Join(dir, "session-c", "plan.md"), []byte("# Plan C"), 0o644)

	plans := ScanAllPlans()

	if !plans["session-a"] {
		t.Error("expected session-a to have plan")
	}
	if plans["session-b"] {
		t.Error("expected session-b to not have plan")
	}
	if !plans["session-c"] {
		t.Error("expected session-c to have plan")
	}
}

func TestReadPlanContent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	sessionID := "test-session-read"
	os.MkdirAll(filepath.Join(dir, sessionID), 0o755)

	planText := "# Test Plan\n\nThis is a test plan with some content."
	os.WriteFile(filepath.Join(dir, sessionID, "plan.md"), []byte(planText), 0o644)

	content, err := ReadPlanContent(sessionID)
	if err != nil {
		t.Fatalf("ReadPlanContent: %v", err)
	}
	if content != planText {
		t.Errorf("content = %q, want %q", content, planText)
	}
}

func TestReadPlanContent_Missing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	os.MkdirAll(filepath.Join(dir, "no-plan-session"), 0o755)

	_, err := ReadPlanContent("no-plan-session")
	if err == nil {
		t.Error("expected error for missing plan.md")
	}
}

func TestReadPlanContent_InvalidSessionID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	_, err := ReadPlanContent("../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal session ID")
	}
}

func TestReadPlanContent_LargeFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	sessionID := "large-plan-session"
	os.MkdirAll(filepath.Join(dir, sessionID), 0o755)

	// Create a plan larger than maxPlanFileSize.
	large := strings.Repeat("x", maxPlanFileSize+1000)
	os.WriteFile(filepath.Join(dir, sessionID, "plan.md"), []byte(large), 0o644)

	content, err := ReadPlanContent(sessionID)
	if err != nil {
		t.Fatalf("ReadPlanContent: %v", err)
	}
	if len(content) > maxPlanFileSize {
		t.Errorf("content length = %d, want <= %d", len(content), maxPlanFileSize)
	}
}

func TestPlanFilePath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	t.Run("valid session ID", func(t *testing.T) {
		path, err := PlanFilePath("abc-123")
		if err != nil {
			t.Fatalf("PlanFilePath: %v", err)
		}
		want := filepath.Join(dir, "abc-123", "plan.md")
		if path != want {
			t.Errorf("path = %q, want %q", path, want)
		}
	})

	t.Run("invalid session ID", func(t *testing.T) {
		_, err := PlanFilePath("../traversal")
		if err == nil {
			t.Error("expected error for traversal session ID")
		}
	})

	t.Run("empty session ID", func(t *testing.T) {
		_, err := PlanFilePath("")
		if err == nil {
			t.Error("expected error for empty session ID")
		}
	})
}

// ---------------------------------------------------------------------------
// sessionStatePath — additional coverage
// ---------------------------------------------------------------------------

func TestSessionStatePath_WithOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	got := sessionStatePath()
	if got == "" {
		t.Error("sessionStatePath should return the override path")
	}
}

func TestSessionStatePath_EmptyOverride(t *testing.T) {
	t.Setenv("DISPATCH_SESSION_STATE", "")

	got := sessionStatePath()
	if got == "" {
		// When no override is set, sessionStatePath uses os.UserHomeDir + sessionStateRel.
		// It should only be empty if UserHomeDir fails.
		t.Log("sessionStatePath returned empty — UserHomeDir may have failed")
	}
}

func TestSessionStatePath_UNCPath(t *testing.T) {
	// UNC paths (\\server\share) should be rejected on Windows
	t.Setenv("DISPATCH_SESSION_STATE", `\\server\share\path`)

	got := sessionStatePath()
	// On Windows: should return "" (UNC paths rejected)
	// On non-Windows: UNC path is just a regular path
	_ = got
}

// ---------------------------------------------------------------------------
// PlanFilePath — additional branches
// ---------------------------------------------------------------------------

func TestPlanFilePath_NoStateDir(t *testing.T) {
	// Set DISPATCH_SESSION_STATE to empty to force sessionStatePath to use home dir
	// but set HOME to non-existent to make it fail
	t.Setenv("DISPATCH_SESSION_STATE", "")
	t.Setenv("HOME", "/nonexistent-path-12345")
	t.Setenv("USERPROFILE", "/nonexistent-path-12345")

	_, err := PlanFilePath("valid-session-id")
	// May or may not error depending on platform
	_ = err
}

func TestReadPlanContent_NoStateDir(t *testing.T) {
	t.Setenv("DISPATCH_SESSION_STATE", "")
	t.Setenv("HOME", "/nonexistent-path-12345")
	t.Setenv("USERPROFILE", "/nonexistent-path-12345")

	_, err := ReadPlanContent("valid-session-id")
	if err == nil {
		t.Error("expected error when state dir unavailable")
	}
}

func TestScanAllPlans_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	plans := ScanAllPlans()
	if len(plans) != 0 {
		t.Errorf("expected 0 plans in empty dir, got %d", len(plans))
	}
}

func TestScanAllPlans_NoStateDir(t *testing.T) {
	t.Setenv("DISPATCH_SESSION_STATE", "/nonexistent-path-12345")

	plans := ScanAllPlans()
	if len(plans) != 0 {
		t.Errorf("expected 0 plans when state dir missing, got %d", len(plans))
	}
}
