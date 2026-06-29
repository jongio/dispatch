package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

func TestCompareView_NilSessions(t *testing.T) {
	t.Parallel()
	cv := NewCompareView()
	if got := cv.View(); got != "" {
		t.Errorf("View() with nil sessions should be empty, got %q", got)
	}
	if got := cv.PlainText(); got != "" {
		t.Errorf("PlainText() with nil sessions should be empty, got %q", got)
	}
}

func TestCompareView_SetSessions(t *testing.T) {
	t.Parallel()
	cv := NewCompareView()
	a := &data.SessionDetail{
		Session: data.Session{
			ID:         "aaaa-bbbb-cccc-dddd",
			Repository: "org/repo-a",
			Branch:     "main",
			HostType:   "github",
			UpdatedAt:  "2024-06-01T10:00:00Z",
		},
		Turns: []data.Turn{{TurnIndex: 0}, {TurnIndex: 1}},
		Files: []data.SessionFile{
			{FilePath: "src/main.go"},
			{FilePath: "src/utils.go"},
		},
		Refs: []data.SessionRef{
			{RefType: "pr", RefValue: "#42"},
		},
	}
	b := &data.SessionDetail{
		Session: data.Session{
			ID:         "eeee-ffff-0000-1111",
			Repository: "org/repo-a",
			Branch:     "feature",
			HostType:   "github",
			UpdatedAt:  "2024-06-02T12:00:00Z",
		},
		Turns: []data.Turn{{TurnIndex: 0}},
		Files: []data.SessionFile{
			{FilePath: "src/main.go"},
			{FilePath: "src/new.go"},
		},
		Refs: []data.SessionRef{
			{RefType: "pr", RefValue: "#42"},
			{RefType: "commit", RefValue: "abc123"},
		},
	}

	cv.SetSessions(a, b)
	cv.SetSize(80, 40)

	view := cv.View()
	if view == "" {
		t.Fatal("View() should not be empty after SetSessions")
	}
	// Check that session IDs appear (truncated to 8 chars).
	if !strings.Contains(view, "aaaa-bbb") {
		t.Error("View() should contain truncated left session ID")
	}
	if !strings.Contains(view, "eeee-fff") {
		t.Error("View() should contain truncated right session ID")
	}
}

func TestCompareView_PlainText(t *testing.T) {
	t.Parallel()
	cv := NewCompareView()
	a := &data.SessionDetail{
		Session: data.Session{
			ID:         "left-session-id-1234",
			Repository: "org/left",
			Branch:     "main",
		},
		Files: []data.SessionFile{
			{FilePath: "common.go"},
			{FilePath: "only-left.go"},
		},
	}
	b := &data.SessionDetail{
		Session: data.Session{
			ID:         "right-session-id-5678",
			Repository: "org/right",
			Branch:     "dev",
		},
		Files: []data.SessionFile{
			{FilePath: "common.go"},
			{FilePath: "only-right.go"},
		},
	}
	cv.SetSessions(a, b)

	txt := cv.PlainText()
	if !strings.Contains(txt, "Session Compare") {
		t.Error("PlainText() should contain header")
	}
	if !strings.Contains(txt, "common.go") {
		t.Error("PlainText() should list common files")
	}
	if !strings.Contains(txt, "only-left.go") {
		t.Error("PlainText() should list left-only files")
	}
	if !strings.Contains(txt, "only-right.go") {
		t.Error("PlainText() should list right-only files")
	}
}

func TestCompareView_EmptyFields(t *testing.T) {
	t.Parallel()
	cv := NewCompareView()
	a := &data.SessionDetail{
		Session: data.Session{ID: "sess-a"},
	}
	b := &data.SessionDetail{
		Session: data.Session{ID: "sess-b"},
	}
	cv.SetSessions(a, b)
	cv.SetSize(60, 20)

	view := cv.View()
	if view == "" {
		t.Error("View() should render even with empty session fields")
	}
	txt := cv.PlainText()
	if !strings.Contains(txt, "sess-a") {
		t.Error("PlainText() should contain left session ID")
	}
}

func TestCompareView_ScrollBounds(t *testing.T) {
	t.Parallel()
	cv := NewCompareView()
	a := &data.SessionDetail{
		Session: data.Session{ID: "a-id"},
		Files:   make([]data.SessionFile, 50),
	}
	for i := range a.Files {
		a.Files[i].FilePath = strings.Repeat("x", i+1) + ".go"
	}
	b := &data.SessionDetail{
		Session: data.Session{ID: "b-id"},
	}
	cv.SetSessions(a, b)
	cv.SetSize(80, 10) // very short viewport

	// Scroll up from 0 should stay at 0.
	cv.ScrollUp()
	if cv.scroll != 0 {
		t.Errorf("scroll should be 0 after ScrollUp at top, got %d", cv.scroll)
	}

	// Scroll down multiple times.
	for i := 0; i < 100; i++ {
		cv.ScrollDown()
	}
	// Render to clamp scroll.
	cv.View()
	// scroll should be clamped (not exceed content).
	if cv.scroll < 0 {
		t.Errorf("scroll should not be negative, got %d", cv.scroll)
	}
}

func TestCompareView_KeyHelp(t *testing.T) {
	t.Parallel()
	cv := NewCompareView()
	a := &data.SessionDetail{Session: data.Session{ID: "a"}}
	b := &data.SessionDetail{Session: data.Session{ID: "b"}}
	cv.SetSessions(a, b)
	cv.SetSize(80, 30)

	view := cv.View()
	if !strings.Contains(view, "esc") {
		t.Error("View() footer should mention esc")
	}
	if !strings.Contains(view, "copy") {
		t.Error("View() footer should mention copy")
	}
}

func TestDiffSets(t *testing.T) {
	t.Parallel()
	a := map[string]struct{}{"x": {}, "y": {}, "z": {}}
	b := map[string]struct{}{"y": {}, "z": {}, "w": {}}

	common, onlyA, onlyB := diffSets(a, b)
	if len(common) != 2 || common[0] != "y" || common[1] != "z" {
		t.Errorf("common = %v, want [y z]", common)
	}
	if len(onlyA) != 1 || onlyA[0] != "x" {
		t.Errorf("onlyA = %v, want [x]", onlyA)
	}
	if len(onlyB) != 1 || onlyB[0] != "w" {
		t.Errorf("onlyB = %v, want [w]", onlyB)
	}
}

func TestDiffSets_Empty(t *testing.T) {
	t.Parallel()
	common, onlyA, onlyB := diffSets(nil, nil)
	if len(common) != 0 || len(onlyA) != 0 || len(onlyB) != 0 {
		t.Error("diffSets(nil, nil) should return empty slices")
	}
}

func TestShort(t *testing.T) {
	t.Parallel()
	if got := short("abcdefghij"); got != "abcdefgh" {
		t.Errorf("short() = %q, want %q", got, "abcdefgh")
	}
	if got := short("abc"); got != "abc" {
		t.Errorf("short() = %q, want %q", got, "abc")
	}
}
