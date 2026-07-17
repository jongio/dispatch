package components

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jongio/dispatch/internal/platform"
)

func TestGitStatusView_NotSet(t *testing.T) {
	t.Parallel()
	g := NewGitStatusView()
	if got := g.View(); got != "" {
		t.Errorf("View() before SetStatus should be empty, got %q", got)
	}
	if got := g.PlainText(); got != "" {
		t.Errorf("PlainText() before SetStatus should be empty, got %q", got)
	}
}

func TestGitStatusView_ViewDirty(t *testing.T) {
	t.Parallel()
	g := NewGitStatusView()
	g.SetStatus(platform.GitStatus{
		Dir:         "/home/me/proj",
		Exists:      true,
		IsRepo:      true,
		Branch:      "feature/login",
		Upstream:    "origin/feature/login",
		HasUpstream: true,
		Ahead:       2,
		Behind:      1,
		Modified:    3,
		Untracked:   1,
		Files: []platform.GitFileStatus{
			{Code: " M", Path: "internal/auth/session.go"},
			{Code: "??", Path: "notes.txt"},
		},
	})
	g.SetSize(80, 30)

	view := g.View()
	if view == "" {
		t.Fatal("View() should not be empty for a dirty status")
	}
	for _, want := range []string{"Git Status", "feature/login", "to push", "to pull", "modified"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() missing %q\n%s", want, view)
		}
	}
}

func TestGitStatusView_ViewClean(t *testing.T) {
	t.Parallel()
	g := NewGitStatusView()
	g.SetStatus(platform.GitStatus{
		Dir:         "/home/me/proj",
		Exists:      true,
		IsRepo:      true,
		Branch:      "main",
		Upstream:    "origin/main",
		HasUpstream: true,
	})
	g.SetSize(80, 30)

	view := g.View()
	if !strings.Contains(view, "clean") {
		t.Errorf("View() should mark a clean tree, got:\n%s", view)
	}
	if !strings.Contains(view, "up to date") {
		t.Errorf("View() should show up to date when synced, got:\n%s", view)
	}
}

func TestGitStatusView_ViewNonRepo(t *testing.T) {
	t.Parallel()

	missing := NewGitStatusView()
	missing.SetStatus(platform.GitStatus{Dir: "/gone", Exists: false})
	missing.SetSize(80, 20)
	if v := missing.View(); !strings.Contains(v, "Directory not found") {
		t.Errorf("View() should report missing dir, got:\n%s", v)
	}

	nonRepo := NewGitStatusView()
	nonRepo.SetStatus(platform.GitStatus{Dir: "/tmp/x", Exists: true, IsRepo: false})
	nonRepo.SetSize(80, 20)
	if v := nonRepo.View(); !strings.Contains(v, "Not a git repository") {
		t.Errorf("View() should report non-repo, got:\n%s", v)
	}
}

func TestGitStatusView_ViewNoUpstream(t *testing.T) {
	t.Parallel()
	g := NewGitStatusView()
	g.SetStatus(platform.GitStatus{
		Dir:      "/home/me/proj",
		Exists:   true,
		IsRepo:   true,
		Branch:   "local-only",
		Modified: 1,
	})
	g.SetSize(80, 30)

	view := g.View()
	if !strings.Contains(view, "no upstream") {
		t.Errorf("View() should note no upstream, got:\n%s", view)
	}
}

func TestGitStatusView_PlainText(t *testing.T) {
	t.Parallel()
	g := NewGitStatusView()
	g.SetStatus(platform.GitStatus{
		Dir:         "/home/me/proj",
		Exists:      true,
		IsRepo:      true,
		Branch:      "main",
		Upstream:    "origin/main",
		HasUpstream: true,
		Ahead:       4,
		Behind:      0,
		Staged:      2,
		Files: []platform.GitFileStatus{
			{Code: "M ", Path: "a.go"},
		},
	})

	txt := g.PlainText()
	for _, want := range []string{"Git Status", "/home/me/proj", "Push/Pull", "4 ahead", "a.go"} {
		if !strings.Contains(txt, want) {
			t.Errorf("PlainText() missing %q\n%s", want, txt)
		}
	}
}

func TestGitStatusView_PlainTextNonRepo(t *testing.T) {
	t.Parallel()
	g := NewGitStatusView()
	g.SetStatus(platform.GitStatus{Dir: "/x", Exists: true, IsRepo: false})
	if txt := g.PlainText(); !strings.Contains(txt, "Not a git repository") {
		t.Errorf("PlainText() should report non-repo, got %q", txt)
	}
}

func TestGitStatusView_Scroll(t *testing.T) {
	t.Parallel()
	g := NewGitStatusView()
	files := make([]platform.GitFileStatus, 50)
	for i := range files {
		files[i] = platform.GitFileStatus{Code: " M", Path: strings.Repeat("x", i+1) + ".go"}
	}
	g.SetStatus(platform.GitStatus{
		Dir: "/p", Exists: true, IsRepo: true, Branch: "main", Modified: 50, Files: files,
	})
	g.SetSize(80, 12) // short viewport forces scrolling

	// ScrollUp at top stays at 0.
	g.ScrollUp()
	if g.scroll != 0 {
		t.Errorf("scroll = %d after ScrollUp at top, want 0", g.scroll)
	}

	for i := 0; i < 200; i++ {
		g.ScrollDown()
	}
	g.View() // clamps
	if g.scroll < 0 {
		t.Errorf("scroll = %d, want >= 0", g.scroll)
	}
	if g.scroll > len(files) {
		t.Errorf("scroll = %d, want <= file count %d", g.scroll, len(files))
	}
}

func TestGitStatusView_Footer(t *testing.T) {
	t.Parallel()
	g := NewGitStatusView()
	g.SetStatus(platform.GitStatus{Dir: "/p", Exists: true, IsRepo: true, Branch: "main"})
	g.SetSize(80, 20)
	view := g.View()
	if !strings.Contains(view, "esc") || !strings.Contains(view, "copy") {
		t.Errorf("View() footer should mention esc and copy, got:\n%s", view)
	}
}

func TestTruncPath(t *testing.T) {
	t.Parallel()

	// Short path is returned unchanged.
	if got := truncPath("a/b.go", 20); got != "a/b.go" {
		t.Errorf("truncPath short = %q, want unchanged", got)
	}

	// Long ASCII path is left-truncated with an ellipsis and never longer
	// than the requested width.
	long := "internal/tui/components/deeply/nested/file.go"
	got := truncPath(long, 15)
	if !strings.HasPrefix(got, "…") {
		t.Errorf("truncPath long = %q, want leading ellipsis", got)
	}
	if n := len([]rune(got)); n > 15 {
		t.Errorf("truncPath rune length = %d, want <= 15", n)
	}

	// Multi-byte UTF-8 path must be sliced on rune boundaries, never emitting
	// invalid UTF-8 (regression: byte slicing could split a code point).
	multi := strings.Repeat("λ", 40) + "/файл.go"
	out := truncPath(multi, 12)
	if !utf8.ValidString(out) {
		t.Errorf("truncPath multibyte produced invalid UTF-8: %q", out)
	}
	if n := len([]rune(out)); n > 12 {
		t.Errorf("truncPath multibyte rune length = %d, want <= 12", n)
	}
}
