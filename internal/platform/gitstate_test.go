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

// TestDetectGitState_MissingDir verifies that a nonexistent path returns GitStateMissing.
func TestDetectGitState_MissingDir(t *testing.T) {
	state := DetectGitState(filepath.Join(t.TempDir(), "nonexistent"))
	if state != GitStateMissing {
		t.Errorf("DetectGitState(nonexistent) = %v, want GitStateMissing", state)
	}
}

// TestDetectGitState_NotGitRepo verifies that a directory without .git returns GitStateUnknown.
func TestDetectGitState_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	state := DetectGitState(dir)
	if state != GitStateUnknown {
		t.Errorf("DetectGitState(non-git) = %v, want GitStateUnknown", state)
	}
}

// TestDetectGitState_Clean verifies that a fresh git repo with a commit returns GitStateClean.
func TestDetectGitState_Clean(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)
	state := DetectGitState(dir)
	if state != GitStateClean {
		t.Errorf("DetectGitState(clean) = %v, want GitStateClean", state)
	}
}

// TestDetectGitState_Dirty verifies that modified tracked files produce GitStateDirty.
func TestDetectGitState_Dirty(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	// Modify the tracked file.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	state := DetectGitState(dir)
	if state != GitStateDirty {
		t.Errorf("DetectGitState(dirty) = %v, want GitStateDirty", state)
	}
}

// TestDetectGitState_Untracked verifies that untracked files produce GitStateUntracked.
func TestDetectGitState_Untracked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)

	// Add an untracked file.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	state := DetectGitState(dir)
	if state != GitStateUntracked {
		t.Errorf("DetectGitState(untracked) = %v, want GitStateUntracked", state)
	}
}

// TestDetectGitState_File verifies that pointing at a file returns GitStateMissing.
func TestDetectGitState_File(t *testing.T) {
	f := filepath.Join(t.TempDir(), "afile.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := DetectGitState(f)
	if state != GitStateMissing {
		t.Errorf("DetectGitState(file) = %v, want GitStateMissing", state)
	}
}

// TestScanGitStates verifies the batch scanning function.
func TestScanGitStates(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	cleanDir := initTestRepo(t)
	missingDir := filepath.Join(t.TempDir(), "gone")

	sessions := map[string]string{
		"sess-clean":   cleanDir,
		"sess-missing": missingDir,
	}
	results := ScanGitStates(sessions)
	if results["sess-clean"] != GitStateClean {
		t.Errorf("ScanGitStates[clean] = %v, want GitStateClean", results["sess-clean"])
	}
	if results["sess-missing"] != GitStateMissing {
		t.Errorf("ScanGitStates[missing] = %v, want GitStateMissing", results["sess-missing"])
	}
}

// TestGitStatusPorcelain_EmptyOutput verifies clean state from no output.
func TestGitStatusPorcelain_EmptyOutput(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)
	state := gitStatusPorcelain(dir)
	if state != GitStateClean {
		t.Errorf("gitStatusPorcelain(clean) = %v, want GitStateClean", state)
	}
}

// TestGitAheadBehind_NoUpstream verifies zero counts when no upstream is set.
func TestGitAheadBehind_NoUpstream(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initTestRepo(t)
	ahead, behind := gitAheadBehind(dir)
	if ahead != 0 || behind != 0 {
		t.Errorf("gitAheadBehind(no upstream) = (%d, %d), want (0, 0)", ahead, behind)
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
