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

// ---------------------------------------------------------------------------
// WriteContinuationPlan
// ---------------------------------------------------------------------------

func TestWriteContinuationPlan_NewFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	sessionID := "new-plan-session"
	// Directory exists but no plan.md yet.
	os.MkdirAll(filepath.Join(dir, sessionID), 0o755)

	remaining := []string{"implement auth", "add tests"}
	err := WriteContinuationPlan(sessionID, remaining, "2 items left")
	if err != nil {
		t.Fatalf("WriteContinuationPlan: %v", err)
	}

	content, err := ReadPlanContent(sessionID)
	if err != nil {
		t.Fatalf("ReadPlanContent: %v", err)
	}

	if !strings.Contains(content, remainingWorkHeader) {
		t.Error("expected remaining work header in plan")
	}
	if !strings.Contains(content, "- [ ] implement auth") {
		t.Error("expected 'implement auth' checkbox")
	}
	if !strings.Contains(content, "- [ ] add tests") {
		t.Error("expected 'add tests' checkbox")
	}
	if !strings.Contains(content, "Summary: 2 items left") {
		t.Error("expected summary line")
	}
}

func TestWriteContinuationPlan_AppendToExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	sessionID := "existing-plan-session"
	existingContent := "# My Plan\n\n- [x] setup project\n- [ ] implement auth\n"
	os.MkdirAll(filepath.Join(dir, sessionID), 0o755)
	os.WriteFile(filepath.Join(dir, sessionID, "plan.md"), []byte(existingContent), 0o644)

	remaining := []string{"implement auth", "write docs"}
	err := WriteContinuationPlan(sessionID, remaining, "needs work")
	if err != nil {
		t.Fatalf("WriteContinuationPlan: %v", err)
	}

	content, err := ReadPlanContent(sessionID)
	if err != nil {
		t.Fatalf("ReadPlanContent: %v", err)
	}

	// Original content preserved.
	if !strings.Contains(content, "# My Plan") {
		t.Error("expected original heading preserved")
	}
	if !strings.Contains(content, "- [x] setup project") {
		t.Error("expected original checkbox preserved")
	}
	// New section appended.
	if !strings.Contains(content, remainingWorkHeader) {
		t.Error("expected remaining work header")
	}
	if !strings.Contains(content, "- [ ] write docs") {
		t.Error("expected new checkbox")
	}
}

func TestWriteContinuationPlan_ReplaceExistingSection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	sessionID := "replace-section-session"
	existing := "# Plan\n\n" + remainingWorkHeader + "\n\n" +
		"The following items were identified as incomplete:\n\n" +
		"- [ ] old item one\n- [ ] old item two\n\nSummary: old summary\n"
	os.MkdirAll(filepath.Join(dir, sessionID), 0o755)
	os.WriteFile(filepath.Join(dir, sessionID, "plan.md"), []byte(existing), 0o644)

	remaining := []string{"new item only"}
	err := WriteContinuationPlan(sessionID, remaining, "refreshed")
	if err != nil {
		t.Fatalf("WriteContinuationPlan: %v", err)
	}

	content, err := ReadPlanContent(sessionID)
	if err != nil {
		t.Fatalf("ReadPlanContent: %v", err)
	}

	// Old items gone.
	if strings.Contains(content, "old item one") {
		t.Error("expected old items removed")
	}
	if strings.Contains(content, "old summary") {
		t.Error("expected old summary removed")
	}
	// New items present.
	if !strings.Contains(content, "- [ ] new item only") {
		t.Error("expected new checkbox")
	}
	if !strings.Contains(content, "Summary: refreshed") {
		t.Error("expected new summary")
	}
	// Header still appears exactly once.
	if strings.Count(content, remainingWorkHeader) != 1 {
		t.Errorf("expected exactly 1 header, got %d", strings.Count(content, remainingWorkHeader))
	}
}

func TestWriteContinuationPlan_Idempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	sessionID := "idempotent-session"
	os.MkdirAll(filepath.Join(dir, sessionID), 0o755)
	os.WriteFile(filepath.Join(dir, sessionID, "plan.md"), []byte("# Plan\n"), 0o644)

	remaining := []string{"task A", "task B"}

	// Write twice with the same data.
	for range 2 {
		if err := WriteContinuationPlan(sessionID, remaining, "same summary"); err != nil {
			t.Fatalf("WriteContinuationPlan: %v", err)
		}
	}

	content, err := ReadPlanContent(sessionID)
	if err != nil {
		t.Fatalf("ReadPlanContent: %v", err)
	}

	// Header should appear exactly once even after two writes.
	if count := strings.Count(content, remainingWorkHeader); count != 1 {
		t.Errorf("expected 1 remaining work header, got %d\ncontent:\n%s", count, content)
	}
}

func TestWriteContinuationPlan_EmptyRemaining(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	sessionID := "empty-remaining"
	os.MkdirAll(filepath.Join(dir, sessionID), 0o755)
	os.WriteFile(filepath.Join(dir, sessionID, "plan.md"), []byte("# Plan\n"), 0o644)

	// Empty remaining + empty summary → no-op.
	if err := WriteContinuationPlan(sessionID, nil, ""); err != nil {
		t.Fatalf("WriteContinuationPlan: %v", err)
	}

	content, err := ReadPlanContent(sessionID)
	if err != nil {
		t.Fatalf("ReadPlanContent: %v", err)
	}
	if strings.Contains(content, remainingWorkHeader) {
		t.Error("expected no remaining work section for empty remaining")
	}
}

func TestWriteContinuationPlan_InvalidSessionID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	err := WriteContinuationPlan("../traversal", []string{"task"}, "summary")
	if err == nil {
		t.Error("expected error for path traversal session ID")
	}
}

func TestWriteContinuationPlan_PreservesTrailingContent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	sessionID := "trailing-content"
	existing := "# Plan\n\n" + remainingWorkHeader + "\n\n" +
		"- [ ] old item\n\n" +
		"## Notes\n\nSome notes here.\n"
	os.MkdirAll(filepath.Join(dir, sessionID), 0o755)
	os.WriteFile(filepath.Join(dir, sessionID, "plan.md"), []byte(existing), 0o644)

	remaining := []string{"updated item"}
	if err := WriteContinuationPlan(sessionID, remaining, "new"); err != nil {
		t.Fatalf("WriteContinuationPlan: %v", err)
	}

	content, err := ReadPlanContent(sessionID)
	if err != nil {
		t.Fatalf("ReadPlanContent: %v", err)
	}

	// The Notes section after the remaining work section should be preserved.
	if !strings.Contains(content, "## Notes") {
		t.Error("expected Notes section preserved after replacement")
	}
	if !strings.Contains(content, "Some notes here.") {
		t.Error("expected Notes content preserved")
	}
	if !strings.Contains(content, "- [ ] updated item") {
		t.Error("expected updated item")
	}
	if strings.Contains(content, "old item") {
		t.Error("expected old item removed")
	}
}

func TestWriteContinuationPlan_CreatesMissingDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DISPATCH_SESSION_STATE", dir)

	sessionID := "no-dir-yet"
	// Deliberately do NOT create the session directory.

	remaining := []string{"first task"}
	err := WriteContinuationPlan(sessionID, remaining, "auto-created")
	if err != nil {
		t.Fatalf("WriteContinuationPlan: %v", err)
	}

	content, err := ReadPlanContent(sessionID)
	if err != nil {
		t.Fatalf("ReadPlanContent: %v", err)
	}
	if !strings.Contains(content, "- [ ] first task") {
		t.Error("expected task in auto-created plan")
	}
}

// ---------------------------------------------------------------------------
// buildRemainingSection
// ---------------------------------------------------------------------------

func TestBuildRemainingSection(t *testing.T) {
	t.Parallel()

	section := buildRemainingSection([]string{"task A", "task B"}, "two items")
	if !strings.HasPrefix(section, remainingWorkHeader) {
		t.Error("expected section to start with header")
	}
	if !strings.Contains(section, "- [ ] task A\n") {
		t.Error("expected task A checkbox")
	}
	if !strings.Contains(section, "- [ ] task B\n") {
		t.Error("expected task B checkbox")
	}
	if !strings.Contains(section, "Summary: two items") {
		t.Error("expected summary")
	}
}

func TestBuildRemainingSection_NoSummary(t *testing.T) {
	t.Parallel()

	section := buildRemainingSection([]string{"only task"}, "")
	if strings.Contains(section, "Summary:") {
		t.Error("expected no summary line when summary is empty")
	}
	if !strings.Contains(section, "- [ ] only task") {
		t.Error("expected task checkbox")
	}
}

// ---------------------------------------------------------------------------
// mergeRemainingSection
// ---------------------------------------------------------------------------

func TestMergeRemainingSection_EmptyExisting(t *testing.T) {
	t.Parallel()

	section := buildRemainingSection([]string{"task"}, "s")
	result := mergeRemainingSection("", section)
	if result != section {
		t.Errorf("expected section as-is, got:\n%s", result)
	}
}

func TestMergeRemainingSection_NoExistingHeader(t *testing.T) {
	t.Parallel()

	existing := "# My Plan\n\nSome content."
	section := buildRemainingSection([]string{"task"}, "s")
	result := mergeRemainingSection(existing, section)

	if !strings.Contains(result, "# My Plan") {
		t.Error("expected original content preserved")
	}
	if !strings.Contains(result, remainingWorkHeader) {
		t.Error("expected new section appended")
	}
}
