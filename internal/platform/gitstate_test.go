package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGitState_String verifies the human-readable labels for all git states.
func TestGitState_String(t *testing.T) {
	tests := []struct {
		state GitState
		want  string
	}{
		{GitStateUnknown, "unknown"},
		{GitStateClean, "clean"},
		{GitStateDirty, "dirty"},
		{GitStateUntracked, "untracked"},
		{GitStateAhead, "ahead"},
		{GitStateBehind, "behind"},
		{GitStateMissing, "missing"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("GitState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

// TestGitStatus_State verifies the enum derivation precedence: missing and
// non-repo first, then dirty over untracked, then behind over ahead, then clean.
func TestGitStatus_State(t *testing.T) {
	tests := []struct {
		name   string
		status GitStatus
		want   GitState
	}{
		{"missing", GitStatus{Exists: false}, GitStateMissing},
		{"non-repo", GitStatus{Exists: true, IsRepo: false}, GitStateUnknown},
		{"clean synced", GitStatus{Exists: true, IsRepo: true}, GitStateClean},
		{"modified", GitStatus{Exists: true, IsRepo: true, Modified: 1}, GitStateDirty},
		{"staged", GitStatus{Exists: true, IsRepo: true, Staged: 1}, GitStateDirty},
		{"deleted", GitStatus{Exists: true, IsRepo: true, Deleted: 1}, GitStateDirty},
		{"conflicts", GitStatus{Exists: true, IsRepo: true, Conflicts: 1}, GitStateDirty},
		{"untracked only", GitStatus{Exists: true, IsRepo: true, Untracked: 1}, GitStateUntracked},
		{"dirty beats untracked", GitStatus{Exists: true, IsRepo: true, Modified: 1, Untracked: 1}, GitStateDirty},
		{"behind", GitStatus{Exists: true, IsRepo: true, Behind: 2}, GitStateBehind},
		{"ahead", GitStatus{Exists: true, IsRepo: true, Ahead: 2}, GitStateAhead},
		{"behind beats ahead", GitStatus{Exists: true, IsRepo: true, Ahead: 1, Behind: 1}, GitStateBehind},
		{"dirty beats divergence", GitStatus{Exists: true, IsRepo: true, Modified: 1, Ahead: 3}, GitStateDirty},
	}
	for _, tt := range tests {
		if got := tt.status.State(); got != tt.want {
			t.Errorf("%s: State() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestDetectGitStatus_State_MissingDir verifies a nonexistent path maps to the
// missing badge state.
func TestDetectGitStatus_State_MissingDir(t *testing.T) {
	if got := DetectGitStatus(filepath.Join(t.TempDir(), "nonexistent")).State(); got != GitStateMissing {
		t.Errorf("State() = %v, want GitStateMissing", got)
	}
}

// TestDetectGitStatus_State_NotGitRepo verifies a directory without .git maps to
// the unknown badge state.
func TestDetectGitStatus_State_NotGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if got := DetectGitStatus(t.TempDir()).State(); got != GitStateUnknown {
		t.Errorf("State() = %v, want GitStateUnknown", got)
	}
}

// TestDetectGitStatus_State_Clean verifies a fresh committed repo maps to clean.
func TestDetectGitStatus_State_Clean(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if got := DetectGitStatus(initTestRepo(t)).State(); got != GitStateClean {
		t.Errorf("State() = %v, want GitStateClean", got)
	}
}

// TestDetectGitStatus_State_Dirty verifies modified tracked files map to dirty.
func TestDetectGitStatus_State_Dirty(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectGitStatus(dir).State(); got != GitStateDirty {
		t.Errorf("State() = %v, want GitStateDirty", got)
	}
}

// TestDetectGitStatus_State_Untracked verifies untracked files map to untracked.
func TestDetectGitStatus_State_Untracked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectGitStatus(dir).State(); got != GitStateUntracked {
		t.Errorf("State() = %v, want GitStateUntracked", got)
	}
}

// TestDetectGitStatus_State_File verifies pointing at a file maps to missing.
func TestDetectGitStatus_State_File(t *testing.T) {
	f := filepath.Join(t.TempDir(), "afile.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectGitStatus(f).State(); got != GitStateMissing {
		t.Errorf("State() = %v, want GitStateMissing", got)
	}
}

// TestScanGitStatuses verifies the batch scanning function returns detailed
// statuses keyed by session ID, from which badge states can be derived.
func TestScanGitStatuses(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	cleanDir := initTestRepo(t)
	missingDir := filepath.Join(t.TempDir(), "gone")

	sessions := map[string]string{
		"sess-clean":   cleanDir,
		"sess-missing": missingDir,
	}
	results := ScanGitStatuses(sessions)
	if !results["sess-clean"].IsRepo || results["sess-clean"].State() != GitStateClean {
		t.Errorf("ScanGitStatuses[clean] = %+v, want a clean repo", results["sess-clean"])
	}
	if results["sess-missing"].Exists || results["sess-missing"].State() != GitStateMissing {
		t.Errorf("ScanGitStatuses[missing] = %+v, want missing", results["sess-missing"])
	}
}

// initTestRepo creates a temporary git repository with one committed file.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "commit.gpgsign", "false"},
	}
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %s\n%s", args, err, out)
		}
	}

	// Create and commit a file.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "file.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %s\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %s\n%s", err, out)
	}

	return dir
}
