package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
)

// renderPreviewWithGit builds a preview for a session and returns its rendered
// view with the given git status applied.
func renderPreviewWithGit(t *testing.T, st platform.GitStatus) string {
	t.Helper()
	p := NewPreviewPanel()
	p.SetSize(80, 40)
	p.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "sess-1", Branch: "main", Cwd: "/home/me/proj"},
	})
	p.SetGitStatus(st)
	return p.View()
}

// TestPreviewGitSection_NonRepoOmitted verifies no git section renders when the
// folder is not a git repository.
func TestPreviewGitSection_NonRepoOmitted(t *testing.T) {
	t.Parallel()
	out := renderPreviewWithGit(t, platform.GitStatus{Exists: true, IsRepo: false})
	if strings.Contains(out, "Push/Pull") {
		t.Errorf("non-repo preview should not render a git section, got:\n%s", out)
	}
}

// TestPreviewGitSection_PushPull verifies ahead/behind stats render in the pane.
func TestPreviewGitSection_PushPull(t *testing.T) {
	t.Parallel()
	out := renderPreviewWithGit(t, platform.GitStatus{
		Exists: true, IsRepo: true, Branch: "main",
		Upstream: "origin/main", HasUpstream: true, Ahead: 3, Behind: 2,
	})
	for _, want := range []string{"Upstream", "origin/main", "Push/Pull", "to push", "to pull", "3", "2"} {
		if !strings.Contains(out, want) {
			t.Errorf("preview git section missing %q, got:\n%s", want, out)
		}
	}
}

// TestPreviewGitSection_Clean verifies a clean, in-sync repo is labelled clearly.
func TestPreviewGitSection_Clean(t *testing.T) {
	t.Parallel()
	out := renderPreviewWithGit(t, platform.GitStatus{
		Exists: true, IsRepo: true, Branch: "main",
		Upstream: "origin/main", HasUpstream: true,
	})
	if !strings.Contains(out, "up to date") {
		t.Errorf("clean synced preview should show 'up to date', got:\n%s", out)
	}
	if !strings.Contains(out, "clean") {
		t.Errorf("clean preview should label changes 'clean', got:\n%s", out)
	}
}

// TestPreviewGitSection_NoUpstream verifies the no-upstream case is handled.
func TestPreviewGitSection_NoUpstream(t *testing.T) {
	t.Parallel()
	out := renderPreviewWithGit(t, platform.GitStatus{
		Exists: true, IsRepo: true, Branch: "local", Modified: 1,
	})
	if !strings.Contains(out, "no upstream") {
		t.Errorf("no-upstream preview should note it, got:\n%s", out)
	}
	if !strings.Contains(out, "modified") {
		t.Errorf("dirty preview should list modified count, got:\n%s", out)
	}
}

// TestGitPushPullText verifies the push/pull helper's branches directly.
func TestGitPushPullText(t *testing.T) {
	t.Parallel()
	if got := gitPushPullText(platform.GitStatus{HasUpstream: false}); !strings.Contains(got, "no upstream") {
		t.Errorf("no upstream = %q", got)
	}
	if got := gitPushPullText(platform.GitStatus{HasUpstream: true}); !strings.Contains(got, "up to date") {
		t.Errorf("synced = %q", got)
	}
	got := gitPushPullText(platform.GitStatus{HasUpstream: true, Ahead: 5})
	if !strings.Contains(got, "5") || !strings.Contains(got, "to push") {
		t.Errorf("ahead = %q", got)
	}
}

// TestGitChangesText verifies the change-counts helper's branches directly.
func TestGitChangesText(t *testing.T) {
	t.Parallel()
	if got := gitChangesText(platform.GitStatus{}); !strings.Contains(got, "clean") {
		t.Errorf("clean = %q", got)
	}
	got := gitChangesText(platform.GitStatus{Staged: 1, Modified: 2, Untracked: 3, Deleted: 4, Conflicts: 5})
	for _, want := range []string{"staged", "modified", "untracked", "deleted", "conflicts"} {
		if !strings.Contains(got, want) {
			t.Errorf("changes text missing %q, got %q", want, got)
		}
	}
}
