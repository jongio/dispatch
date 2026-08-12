package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDetectGitStatusUsesKnownRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	statuses := ScanGitStatusesWithRepositories(
		map[string]string{"session": dir},
		map[string]string{"session": "owner/known"},
	)
	if got := statuses["session"].Repository; got != "owner/known" {
		t.Fatalf("Repository = %q, want owner/known", got)
	}
}
