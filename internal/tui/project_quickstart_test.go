package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/components"
)

func TestDiscoverGitRepos_FindsReposWithinDepthAndSkipsNoise(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	repoA := filepath.Join(root, "repo-a")
	repoB := filepath.Join(root, "team", "repo-b")
	deepRepo := filepath.Join(root, "a", "b", "c", "d", "too-deep")
	nodeRepo := filepath.Join(root, "node_modules", "dep")
	for _, dir := range []string{
		filepath.Join(repoA, ".git"),
		filepath.Join(repoB, ".git"),
		filepath.Join(deepRepo, ".git"),
		filepath.Join(nodeRepo, ".git"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", dir, err)
		}
	}

	repos, err := discoverGitRepos([]string{root}, 3)
	if err != nil {
		t.Fatalf("discoverGitRepos: %v", err)
	}
	got := pathsOfQuickStarts(repos)
	want := map[string]bool{repoA: true, repoB: true}
	if len(got) != len(want) {
		t.Fatalf("repos = %v, want %v", got, want)
	}
	for _, path := range got {
		if !want[path] {
			t.Fatalf("unexpected repo %q in %v", path, got)
		}
	}
}

func TestFilterQuickStartRepos_ExcludesReposWithRecentSession(t *testing.T) {
	t.Parallel()
	root := filepath.Clean(filepath.Join(string(filepath.Separator), "code"))
	repos := []components.QuickStart{
		{Name: "has-session", Path: filepath.Join(root, "has-session")},
		{Name: "empty", Path: filepath.Join(root, "empty")},
	}
	sessions := []data.Session{{ID: "s1", Cwd: filepath.Join(root, "has-session", "subdir")}}

	got := filterQuickStartRepos(repos, sessions)
	if len(got) != 1 || got[0].Name != "empty" {
		t.Fatalf("filterQuickStartRepos = %#v, want only empty repo", got)
	}
}

func pathsOfQuickStarts(qs []components.QuickStart) []string {
	paths := make([]string, 0, len(qs))
	for _, q := range qs {
		paths = append(paths, q.Path)
	}
	return paths
}
