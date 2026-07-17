package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// joinLines builds porcelain-v2 style input from individual lines.
func joinLines(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

// TestParseGitStatusV2_Clean verifies a synced, clean repo parses to all-zero
// counts with Clean() true.
func TestParseGitStatusV2_Clean(t *testing.T) {
	in := joinLines(
		"# branch.oid abc123",
		"# branch.head main",
		"# branch.upstream origin/main",
		"# branch.ab +0 -0",
	)
	s := parseGitStatusV2(in)
	if !s.Clean() {
		t.Errorf("Clean() = false, want true for %+v", s)
	}
	if s.Ahead != 0 || s.Behind != 0 {
		t.Errorf("ahead/behind = %d/%d, want 0/0", s.Ahead, s.Behind)
	}
	if !s.HasUpstream {
		t.Error("HasUpstream = false, want true")
	}
}

// TestParseGitStatusV2_AheadBehind verifies push/pull counts come from branch.ab.
func TestParseGitStatusV2_AheadBehind(t *testing.T) {
	in := joinLines(
		"# branch.head main",
		"# branch.upstream origin/main",
		"# branch.ab +2 -3",
	)
	s := parseGitStatusV2(in)
	if s.Ahead != 2 {
		t.Errorf("Ahead = %d, want 2", s.Ahead)
	}
	if s.Behind != 3 {
		t.Errorf("Behind = %d, want 3", s.Behind)
	}
}

// TestParseGitStatusV2_BranchUpstream verifies branch and upstream names parse.
func TestParseGitStatusV2_BranchUpstream(t *testing.T) {
	in := joinLines(
		"# branch.head feature/login",
		"# branch.upstream origin/feature/login",
		"# branch.ab +0 -0",
	)
	s := parseGitStatusV2(in)
	if s.Branch != "feature/login" {
		t.Errorf("Branch = %q, want feature/login", s.Branch)
	}
	if s.Upstream != "origin/feature/login" {
		t.Errorf("Upstream = %q, want origin/feature/login", s.Upstream)
	}
	if s.Detached {
		t.Error("Detached = true, want false")
	}
}

// TestParseGitStatusV2_TrackedCounts verifies staged/modified/deleted counts
// from ordinary '1' records and that a space-containing path is preserved.
func TestParseGitStatusV2_TrackedCounts(t *testing.T) {
	in := joinLines(
		"# branch.head main",
		"1 M. N... 100644 100644 100644 1111 2222 staged.go",
		"1 .M N... 100644 100644 100644 1111 2222 my file.go",
		"1 .D N... 100644 100644 000000 1111 2222 gone.go",
	)
	s := parseGitStatusV2(in)
	if s.Staged != 1 {
		t.Errorf("Staged = %d, want 1", s.Staged)
	}
	if s.Modified != 1 {
		t.Errorf("Modified = %d, want 1", s.Modified)
	}
	if s.Deleted != 1 {
		t.Errorf("Deleted = %d, want 1", s.Deleted)
	}
	if len(s.Files) != 3 {
		t.Fatalf("Files len = %d, want 3", len(s.Files))
	}
	// The second record's path contains a space and must be intact.
	if s.Files[1].Path != "my file.go" {
		t.Errorf("Files[1].Path = %q, want %q", s.Files[1].Path, "my file.go")
	}
	if s.Files[1].Code != " M" {
		t.Errorf("Files[1].Code = %q, want %q", s.Files[1].Code, " M")
	}
}

// TestParseGitStatusV2_Untracked verifies '?' records are counted and listed.
func TestParseGitStatusV2_Untracked(t *testing.T) {
	in := joinLines(
		"# branch.head main",
		"? new.txt",
		"? docs/readme.md",
	)
	s := parseGitStatusV2(in)
	if s.Untracked != 2 {
		t.Errorf("Untracked = %d, want 2", s.Untracked)
	}
	if len(s.Files) != 2 || s.Files[0].Code != "??" {
		t.Errorf("Files = %+v, want two ?? entries", s.Files)
	}
	if s.Files[1].Path != "docs/readme.md" {
		t.Errorf("Files[1].Path = %q, want docs/readme.md", s.Files[1].Path)
	}
}

// TestParseGitStatusV2_Conflicts verifies unmerged 'u' records count as conflicts.
func TestParseGitStatusV2_Conflicts(t *testing.T) {
	in := joinLines(
		"# branch.head main",
		"u UU N... 100644 100644 100644 100644 h1 h2 h3 conflict.go",
	)
	s := parseGitStatusV2(in)
	if s.Conflicts != 1 {
		t.Errorf("Conflicts = %d, want 1", s.Conflicts)
	}
	if len(s.Files) != 1 || s.Files[0].Path != "conflict.go" {
		t.Errorf("Files = %+v, want conflict.go", s.Files)
	}
}

// TestParseGitStatusV2_NoUpstream verifies missing upstream headers leave
// HasUpstream false and counts zero.
func TestParseGitStatusV2_NoUpstream(t *testing.T) {
	in := joinLines(
		"# branch.oid abc",
		"# branch.head main",
	)
	s := parseGitStatusV2(in)
	if s.HasUpstream {
		t.Error("HasUpstream = true, want false")
	}
	if s.Upstream != "" {
		t.Errorf("Upstream = %q, want empty", s.Upstream)
	}
	if s.Ahead != 0 || s.Behind != 0 {
		t.Errorf("ahead/behind = %d/%d, want 0/0", s.Ahead, s.Behind)
	}
}

// TestParseGitStatusV2_Detached verifies a detached HEAD is reported.
func TestParseGitStatusV2_Detached(t *testing.T) {
	in := joinLines(
		"# branch.oid abc",
		"# branch.head (detached)",
	)
	s := parseGitStatusV2(in)
	if !s.Detached {
		t.Error("Detached = false, want true")
	}
	if s.Branch != "(detached)" {
		t.Errorf("Branch = %q, want (detached)", s.Branch)
	}
}

// TestParseGitStatusV2_Renamed verifies a rename '2' record parses the new path
// (before the tab) and counts as staged.
func TestParseGitStatusV2_Renamed(t *testing.T) {
	in := joinLines(
		"# branch.head main",
		"2 R. N... 100644 100644 100644 h1 h2 R100 new.go\told.go",
	)
	s := parseGitStatusV2(in)
	if s.Staged != 1 {
		t.Errorf("Staged = %d, want 1", s.Staged)
	}
	if len(s.Files) != 1 || s.Files[0].Path != "new.go" {
		t.Errorf("Files = %+v, want new.go", s.Files)
	}
	if s.Files[0].Code != "R " {
		t.Errorf("Files[0].Code = %q, want %q", s.Files[0].Code, "R ")
	}
}

// TestParseGitStatusV2_FileCap verifies the file list is capped while counts
// remain complete and Truncated is flagged.
func TestParseGitStatusV2_FileCap(t *testing.T) {
	var b strings.Builder
	b.WriteString("# branch.head main\n")
	total := maxGitStatusFiles + 100
	for i := 0; i < total; i++ {
		b.WriteString("? file")
		b.WriteByte(byte('0' + i%10))
		b.WriteString(".txt\n")
	}
	s := parseGitStatusV2(b.String())
	if s.Untracked != total {
		t.Errorf("Untracked = %d, want %d", s.Untracked, total)
	}
	if len(s.Files) != maxGitStatusFiles {
		t.Errorf("Files len = %d, want %d", len(s.Files), maxGitStatusFiles)
	}
	if !s.Truncated {
		t.Error("Truncated = false, want true")
	}
}

// TestDetectGitStatus_Missing verifies a nonexistent path reports Exists=false.
func TestDetectGitStatus_Missing(t *testing.T) {
	s := DetectGitStatus(filepath.Join(t.TempDir(), "nope"))
	if s.Exists {
		t.Errorf("Exists = true, want false for missing dir")
	}
	if s.IsRepo {
		t.Errorf("IsRepo = true, want false for missing dir")
	}
}

// TestDetectGitStatus_NonRepo verifies a plain directory reports IsRepo=false.
func TestDetectGitStatus_NonRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	s := DetectGitStatus(t.TempDir())
	if !s.Exists {
		t.Error("Exists = false, want true for existing dir")
	}
	if s.IsRepo {
		t.Error("IsRepo = true, want false for non-repo dir")
	}
}

// TestDetectGitStatus_RealRepo verifies detection against a live repo with a
// staged file and an untracked file.
func TestDetectGitStatus_RealRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	// Stage a new file.
	if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "add", "staged.txt")
	add.Dir = dir
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add: %s\n%s", err, out)
	}
	// Leave an untracked file.
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("u"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := DetectGitStatus(dir)
	if !s.Exists || !s.IsRepo {
		t.Fatalf("Exists/IsRepo = %v/%v, want true/true", s.Exists, s.IsRepo)
	}
	if s.Branch == "" {
		t.Error("Branch is empty, want a branch name")
	}
	if s.Staged < 1 {
		t.Errorf("Staged = %d, want >= 1", s.Staged)
	}
	if s.Untracked < 1 {
		t.Errorf("Untracked = %d, want >= 1", s.Untracked)
	}
	if s.Clean() {
		t.Error("Clean() = true, want false for a dirty tree")
	}
}

// TestGitSafeArgs_Hardens verifies the hardening flags are prepended before the
// git subcommand so an untrusted repository's core.fsmonitor cannot execute.
func TestGitSafeArgs_Hardens(t *testing.T) {
	got := gitSafeArgs("status", "--porcelain")
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "core.fsmonitor=false") {
		t.Errorf("gitSafeArgs missing core.fsmonitor hardening: %v", got)
	}
	// The subcommand and its args must be preserved at the end, after the flags.
	if len(got) < 2 || got[len(got)-2] != "status" || got[len(got)-1] != "--porcelain" {
		t.Errorf("subcommand args not preserved at end: %v", got)
	}
}

// TestDetectGitStatus_FsmonitorNotExecuted proves DetectGitStatus neutralizes a
// malicious core.fsmonitor hook (CWE-829 arbitrary code execution). A positive
// control confirms the environment actually executes fsmonitor; otherwise the
// test cannot prove the hardening and is skipped.
func TestDetectGitStatus_FsmonitorNotExecuted(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	marker := filepath.Join(dir, "PWNED.txt")
	hookPath := filepath.Join(dir, "evil.sh")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho pwned > PWNED.txt\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	setCfg := exec.Command("git", "config", "core.fsmonitor", "./evil.sh")
	setCfg.Dir = dir
	if out, err := setCfg.CombinedOutput(); err != nil {
		t.Fatalf("git config: %s\n%s", err, out)
	}

	// Positive control: an unhardened status must trigger the hook here,
	// otherwise this environment cannot demonstrate the vulnerability — skip.
	ctrl := exec.Command("git", "status", "--porcelain=v2", "--branch")
	ctrl.Dir = dir
	_, _ = ctrl.CombinedOutput()
	if _, err := os.Stat(marker); err != nil {
		t.Skip("environment does not execute core.fsmonitor; cannot verify hardening")
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}

	// DetectGitStatus is hardened, so the malicious hook must NOT run.
	_ = DetectGitStatus(dir)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("fsmonitor hook executed via DetectGitStatus — core.fsmonitor not neutralized (CWE-829)")
	}
}
