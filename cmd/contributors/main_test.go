package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// runAll — error paths
// ---------------------------------------------------------------------------

func TestRunAll_NonGitDir(t *testing.T) {
	t.Parallel()
	// Use a temp directory with no git history — should fail.
	err := runAll(t.TempDir())
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
	if !strings.Contains(err.Error(), "extracting contributors") {
		t.Errorf("error should mention extracting, got: %v", err)
	}
}

func TestRunAll_NonexistentDir(t *testing.T) {
	t.Parallel()
	err := runAll("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

// ---------------------------------------------------------------------------
// runAll — success path with temp git repo
// ---------------------------------------------------------------------------

func TestRunAll_WithGitRepo(t *testing.T) {
	t.Parallel()
	repoDir := createTempGitRepo(t)

	err := runAll(repoDir)
	if err != nil {
		t.Fatalf("runAll: %v", err)
	}

	// Verify CONTRIBUTORS.md was created.
	contribPath := filepath.Join(repoDir, "CONTRIBUTORS.md")
	data, err := os.ReadFile(contribPath)
	if err != nil {
		t.Fatalf("CONTRIBUTORS.md not found: %v", err)
	}
	if len(data) == 0 {
		t.Error("CONTRIBUTORS.md should not be empty")
	}
}

// ---------------------------------------------------------------------------
// runRelease — error paths
// ---------------------------------------------------------------------------

func TestRunRelease_NonGitDir(t *testing.T) {
	t.Parallel()
	err := runRelease(t.TempDir(), "v0.1.0", "v0.2.0")
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
	if !strings.Contains(err.Error(), "extracting release contributors") {
		t.Errorf("error should mention extracting release, got: %v", err)
	}
}

func TestRunRelease_EmptyFromTag(t *testing.T) {
	t.Parallel()
	// Empty fromTag means "first release" — all contributors are first-timers.
	// But with a non-git dir, it should still fail at extraction.
	err := runRelease(t.TempDir(), "", "v1.0.0")
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
}

func TestRunRelease_NonexistentDir(t *testing.T) {
	t.Parallel()
	err := runRelease("/nonexistent/path", "v0.1.0", "v0.2.0")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

// ---------------------------------------------------------------------------
// runRelease — success path with temp git repo
// ---------------------------------------------------------------------------

func TestRunRelease_WithGitRepo(t *testing.T) {
	t.Parallel()
	repoDir := createTempGitRepo(t)

	// Tag the initial commit.
	mustGit(t, repoDir, "tag", "v0.1.0")

	// Add another commit and tag.
	writeFile(t, filepath.Join(repoDir, "feature.go"), "package main\n")
	mustGit(t, repoDir, "add", ".")
	mustGit(t, repoDir, "commit", "-m", "add feature")
	mustGit(t, repoDir, "tag", "v0.2.0")

	err := runRelease(repoDir, "v0.1.0", "v0.2.0")
	if err != nil {
		t.Fatalf("runRelease: %v", err)
	}
}

func TestRunRelease_EmptyFromTag_WithGitRepo(t *testing.T) {
	t.Parallel()
	repoDir := createTempGitRepo(t)
	mustGit(t, repoDir, "tag", "v1.0.0")

	// Empty fromTag = first release, everyone is first-timer.
	err := runRelease(repoDir, "", "v1.0.0")
	if err != nil {
		t.Fatalf("runRelease with empty fromTag: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Usage constant
// ---------------------------------------------------------------------------

func TestUsageContainsExpectedContent(t *testing.T) {
	t.Parallel()
	if !strings.Contains(usage, "--all") {
		t.Error("usage should mention --all")
	}
	if !strings.Contains(usage, "--release") {
		t.Error("usage should mention --release")
	}
	if !strings.Contains(usage, "<fromTag>") {
		t.Error("usage should mention <fromTag>")
	}
	if !strings.Contains(usage, "<toTag>") {
		t.Error("usage should mention <toTag>")
	}
}

// ---------------------------------------------------------------------------
// Main subprocess tests — test argument parsing via exit codes
// ---------------------------------------------------------------------------

func TestMain_NoArgs(t *testing.T) {
	if len(os.Args) < 1 {
		t.Skip("os.Args not available")
	}
	if !strings.Contains(usage, "Usage:") {
		t.Error("usage should contain 'Usage:'")
	}
}

// ---------------------------------------------------------------------------
// Main subprocess tests — exercise main() via subprocess
// ---------------------------------------------------------------------------

// TestMainSubprocess_NoArgs verifies that running with no arguments
// prints usage and exits with code 1.
func TestMainSubprocess_NoArgs(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_MAIN_SUBPROCESS") == "1" {
		os.Args = []string{"contributors"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestMainSubprocess_NoArgs$")
	cmd.Env = append(os.Environ(), "TEST_MAIN_SUBPROCESS=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for no arguments")
	}
	if !strings.Contains(string(out), "Usage:") {
		t.Errorf("expected usage message, got: %s", out)
	}
}

// TestMainSubprocess_UnknownArg verifies that unknown arguments print
// an error and exit with code 1.
func TestMainSubprocess_UnknownArg(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_MAIN_SUBPROCESS") == "1" {
		os.Args = []string{"contributors", "--bogus"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestMainSubprocess_UnknownArg$")
	cmd.Env = append(os.Environ(), "TEST_MAIN_SUBPROCESS=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for unknown arg")
	}
	if !strings.Contains(string(out), "unknown argument") {
		t.Errorf("expected 'unknown argument' error, got: %s", out)
	}
}

// TestMainSubprocess_ReleaseMissingArgs verifies that --release without
// enough arguments prints an error.
func TestMainSubprocess_ReleaseMissingArgs(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_MAIN_SUBPROCESS") == "1" {
		os.Args = []string{"contributors", "--release"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestMainSubprocess_ReleaseMissingArgs$")
	cmd.Env = append(os.Environ(), "TEST_MAIN_SUBPROCESS=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for --release without args")
	}
	if !strings.Contains(string(out), "requires") {
		t.Errorf("expected 'requires' error, got: %s", out)
	}
}

// TestMainSubprocess_AllFlag verifies that --all runs successfully in
// a git repository.
func TestMainSubprocess_AllFlag(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_MAIN_SUBPROCESS") == "1" {
		os.Args = []string{"contributors", "--all"}
		main()
		return
	}

	repoDir := createTempGitRepo(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestMainSubprocess_AllFlag$")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "TEST_MAIN_SUBPROCESS=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected success for --all in git repo: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "CONTRIBUTORS.md updated") {
		t.Errorf("expected success message, got: %s", out)
	}
}

// TestMainSubprocess_ReleaseFlag verifies that --release with valid tags
// runs successfully.
func TestMainSubprocess_ReleaseFlag(t *testing.T) {
	t.Parallel()
	if os.Getenv("TEST_MAIN_SUBPROCESS") == "1" {
		os.Args = []string{"contributors", "--release", "v0.1.0", "v0.2.0"}
		main()
		return
	}

	repoDir := createTempGitRepo(t)
	mustGit(t, repoDir, "tag", "v0.1.0")
	writeFile(t, filepath.Join(repoDir, "feature.go"), "package main\n")
	mustGit(t, repoDir, "add", ".")
	mustGit(t, repoDir, "commit", "-m", "add feature")
	mustGit(t, repoDir, "tag", "v0.2.0")

	cmd := exec.Command(os.Args[0], "-test.run=^TestMainSubprocess_ReleaseFlag$")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "TEST_MAIN_SUBPROCESS=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected success for --release: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Contributors") {
		t.Errorf("expected contributors output, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// createTempGitRepo creates a temporary directory with an initialized git
// repository containing one commit. Returns the repo directory path.
func createTempGitRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()

	mustGit(t, repoDir, "init")
	mustGit(t, repoDir, "config", "user.email", "test@example.com")
	mustGit(t, repoDir, "config", "user.name", "Test User")

	writeFile(t, filepath.Join(repoDir, "README.md"), "# Test\n")
	mustGit(t, repoDir, "add", ".")
	mustGit(t, repoDir, "commit", "-m", "initial commit")

	return repoDir
}

// mustGit runs a git command in the given directory and fails the test on error.
func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// writeFile writes content to a file, creating it if needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
