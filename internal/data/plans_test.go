package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanPlans(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	// Create session directories — some with plans, some without.
	withPlan := "session-with-plan"
	withoutPlan := "session-without-plan"
	emptyPlan := "session-empty-plan"

	os.MkdirAll(filepath.Join(dir, withPlan), 0o755)
	os.MkdirAll(filepath.Join(dir, withoutPlan), 0o755)
	os.MkdirAll(filepath.Join(dir, emptyPlan), 0o755)

	os.WriteFile(filepath.Join(dir, withPlan, "plan.md"), []byte("# My Plan"), 0o644)
	os.WriteFile(filepath.Join(dir, emptyPlan, "plan.md"), []byte(""), 0o644)

	plans := ScanPlans([]string{withPlan, withoutPlan, emptyPlan, "nonexistent"})

	if !plans[withPlan] {
		t.Error("expected session-with-plan to have plan")
	}
	if plans[withoutPlan] {
		t.Error("expected session-without-plan to not have plan")
	}
	if plans[emptyPlan] {
		t.Error("expected session-empty-plan (0 bytes) to not have plan")
	}
	if plans["nonexistent"] {
		t.Error("expected nonexistent session to not have plan")
	}
}

func TestScanPlans_InvalidSessionID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	// Path traversal attempts should be rejected.
	plans := ScanPlans([]string{"../../../etc/passwd", "", "valid-id"})
	if plans["../../../etc/passwd"] {
		t.Error("path traversal session ID should be rejected")
	}
}

func TestScanPlans_NilStateDir(t *testing.T) {
	// Ensure no crash when state dir cannot be resolved.
	t.Setenv("DISPATCH_SESSION_STATE", "")
	t.Setenv("HOME", "/nonexistent/path/that/does/not/exist")
	if os.Getenv("USERPROFILE") != "" {
		t.Setenv("USERPROFILE", "/nonexistent/path/that/does/not/exist")
	}

	plans := ScanPlans([]string{"some-session"})
	if plans != nil && plans["some-session"] {
		t.Error("expected nil or empty plans when state dir is unresolvable")
	}
}

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
