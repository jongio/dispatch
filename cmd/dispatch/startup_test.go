package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/update"
)

// errNotGit is a sentinel used by the detectGitRepoFn seam in tests.
var errNotGit = errors.New("not a git repository")

// ---------------------------------------------------------------------------
// startupOptions.SeedQuery
// ---------------------------------------------------------------------------

func TestSeedQuery_TokenOrderAndQuoting(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts startupOptions
		want string
	}{
		{"empty", startupOptions{}, ""},
		{"query only", startupOptions{Query: "auth bug"}, "auth bug"},
		{"repo only", startupOptions{Repository: "owner/repo"}, "repo:owner/repo"},
		{
			"all fields",
			startupOptions{Repository: "owner/repo", Branch: "main", Folder: "/code", Query: "auth"},
			"repo:owner/repo branch:main folder:/code auth",
		},
		{
			"quoted folder with space",
			startupOptions{Folder: "/my code"},
			`folder:"/my code"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.opts.SeedQuery(); got != tc.want {
				t.Errorf("SeedQuery() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStartupOptions_Active(t *testing.T) {
	if (startupOptions{Query: "auth"}).active() {
		t.Error("a free-text query alone should not count as an active filter")
	}
	if !(startupOptions{Repository: "owner/repo"}).active() {
		t.Error("a repository filter should be active")
	}
	if !(startupOptions{Folder: "/code"}).active() {
		t.Error("a folder filter should be active")
	}
}

// ---------------------------------------------------------------------------
// resolveStartupOptions
// ---------------------------------------------------------------------------

func TestResolveStartupOptions_QueryCombinesFlagAndParts(t *testing.T) {
	got, err := resolveStartupOptions(startupFlags{query: "fix", queryParts: []string{"auth", "bug"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Query != "fix auth bug" {
		t.Errorf("Query = %q, want %q", got.Query, "fix auth bug")
	}
}

func TestResolveStartupOptions_CwdValid(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveStartupOptions(startupFlags{cwd: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Folder != filepath.Clean(dir) {
		t.Errorf("Folder = %q, want %q", got.Folder, filepath.Clean(dir))
	}
}

func TestResolveStartupOptions_CwdMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	_, err := resolveStartupOptions(startupFlags{cwd: missing})
	if err == nil {
		t.Fatal("expected error for a missing --cwd path")
	}
	if !strings.Contains(err.Error(), "invalid --cwd path") {
		t.Errorf("error = %v, want it to mention an invalid path", err)
	}
}

func TestResolveStartupOptions_CwdIsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := resolveStartupOptions(startupFlags{cwd: file})
	if err == nil {
		t.Fatal("expected error when --cwd points at a file")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error = %v, want it to mention a non-directory", err)
	}
}

func TestResolveStartupOptions_CurrentDetects(t *testing.T) {
	orig := detectGitRepoFn
	t.Cleanup(func() { detectGitRepoFn = orig })
	detectGitRepoFn = func(string) (string, string, error) {
		return "jongio/dispatch", "feature/x", nil
	}

	got, err := resolveStartupOptions(startupFlags{current: true, cwd: t.TempDir()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Repository != "jongio/dispatch" {
		t.Errorf("Repository = %q, want %q", got.Repository, "jongio/dispatch")
	}
	if got.Branch != "feature/x" {
		t.Errorf("Branch = %q, want %q", got.Branch, "feature/x")
	}
}

func TestResolveStartupOptions_ExplicitFlagsWinOverCurrent(t *testing.T) {
	orig := detectGitRepoFn
	t.Cleanup(func() { detectGitRepoFn = orig })
	detectGitRepoFn = func(string) (string, string, error) {
		return "jongio/dispatch", "main", nil
	}

	got, err := resolveStartupOptions(startupFlags{
		current: true,
		cwd:     t.TempDir(),
		repo:    "other/repo",
		branch:  "release",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Repository != "other/repo" {
		t.Errorf("Repository = %q, want explicit flag to win", got.Repository)
	}
	if got.Branch != "release" {
		t.Errorf("Branch = %q, want explicit flag to win", got.Branch)
	}
}

func TestResolveStartupOptions_CurrentNonGit(t *testing.T) {
	orig := detectGitRepoFn
	t.Cleanup(func() { detectGitRepoFn = orig })
	detectGitRepoFn = func(dir string) (string, string, error) {
		return "", "", errNotGit
	}

	_, err := resolveStartupOptions(startupFlags{current: true, cwd: t.TempDir()})
	if err == nil {
		t.Fatal("expected error for a non-git directory")
	}
}

func TestResolveStartupOptions_CurrentNothingDetected(t *testing.T) {
	orig := detectGitRepoFn
	t.Cleanup(func() { detectGitRepoFn = orig })
	detectGitRepoFn = func(string) (string, string, error) {
		return "", "", nil // git repo, but no remote and detached HEAD
	}

	_, err := resolveStartupOptions(startupFlags{current: true, cwd: t.TempDir()})
	if err == nil {
		t.Fatal("expected error when neither repo nor branch can be detected")
	}
}

// ---------------------------------------------------------------------------
// handleArgs — startup filter flags
// ---------------------------------------------------------------------------

func TestHandleArgs_StartupFilters_Spaced(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)

	done, _, startup, err := handleArgs(
		[]string{"--repo", "owner/repo", "--branch", "main", "--query", "auth"},
		io.Discard, ch,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("expected done=false for startup filters")
	}
	if startup.Repository != "owner/repo" || startup.Branch != "main" || startup.Query != "auth" {
		t.Errorf("startup = %+v, want repo=owner/repo branch=main query=auth", startup)
	}
	if got := startup.SeedQuery(); got != "repo:owner/repo branch:main auth" {
		t.Errorf("SeedQuery = %q", got)
	}
}

func TestHandleArgs_StartupFilters_Inline(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)

	done, _, startup, err := handleArgs(
		[]string{"--repo=owner/repo", "--branch=feature/x"},
		io.Discard, ch,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("expected done=false")
	}
	if startup.Repository != "owner/repo" || startup.Branch != "feature/x" {
		t.Errorf("startup = %+v, want repo=owner/repo branch=feature/x", startup)
	}
}

func TestHandleArgs_StartupFilters_MissingValue(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)

	done, _, _, err := handleArgs([]string{"--repo"}, io.Discard, ch)
	if err == nil {
		t.Fatal("expected error for --repo with no value")
	}
	if !done {
		t.Error("expected done=true on the error path")
	}
}

func TestHandleArgs_Current(t *testing.T) {
	orig := detectGitRepoFn
	t.Cleanup(func() { detectGitRepoFn = orig })
	detectGitRepoFn = func(string) (string, string, error) {
		return "jongio/dispatch", "main", nil
	}

	ch := make(chan *update.UpdateInfo, 1)
	done, _, startup, err := handleArgs([]string{"--current"}, io.Discard, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("expected done=false for --current")
	}
	if startup.Repository != "jongio/dispatch" || startup.Branch != "main" {
		t.Errorf("startup = %+v, want repo=jongio/dispatch branch=main", startup)
	}
}

func TestHandleArgs_CurrentNonGit(t *testing.T) {
	orig := detectGitRepoFn
	t.Cleanup(func() { detectGitRepoFn = orig })
	detectGitRepoFn = func(string) (string, string, error) {
		return "", "", errNotGit
	}

	ch := make(chan *update.UpdateInfo, 1)
	done, _, _, err := handleArgs([]string{"--current"}, io.Discard, ch)
	if err == nil {
		t.Fatal("expected error for --current in a non-git directory")
	}
	if !done {
		t.Error("expected done=true on the error path")
	}
}

func TestHandleArgs_UnknownInlineFlag(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)

	done, _, _, err := handleArgs([]string{"--bogus=1"}, io.Discard, ch)
	if err == nil {
		t.Fatal("expected error for an unknown inline flag")
	}
	if !done {
		t.Error("expected done=true for an unknown flag")
	}
	if !strings.Contains(err.Error(), "--bogus") {
		t.Errorf("error should mention the flag, got: %v", err)
	}
}
