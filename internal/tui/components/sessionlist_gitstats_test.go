package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// TestGitStatsSegment_NilMap verifies no segment renders before a scan.
func TestGitStatsSegment_NilMap(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	seg, w := sl.gitStatsSegment("s1", false)
	if seg != "" || w != 0 {
		t.Errorf("gitStatsSegment nil map = (%q, %d), want empty", seg, w)
	}
}

// TestGitStatsSegment_NonRepo verifies non-repo sessions render nothing.
func TestGitStatsSegment_NonRepo(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGitStatuses(map[string]platform.GitStatus{"s1": {Exists: true, IsRepo: false}})
	seg, w := sl.gitStatsSegment("s1", false)
	if seg != "" || w != 0 {
		t.Errorf("gitStatsSegment non-repo = (%q, %d), want empty", seg, w)
	}
}

// TestGitStatsSegment_CleanSynced verifies a clean, in-sync repo renders nothing.
func TestGitStatsSegment_CleanSynced(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGitStatuses(map[string]platform.GitStatus{
		"s1": {Exists: true, IsRepo: true, HasUpstream: true},
	})
	seg, w := sl.gitStatsSegment("s1", false)
	if seg != "" || w != 0 {
		t.Errorf("gitStatsSegment clean = (%q, %d), want empty", seg, w)
	}
}

// TestGitStatsSegment_AheadBehindDirty verifies the push/pull counts and dirty
// marker appear with a correct visual width.
func TestGitStatsSegment_AheadBehindDirty(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGitStatuses(map[string]platform.GitStatus{
		"s1": {Exists: true, IsRepo: true, HasUpstream: true, Ahead: 2, Behind: 1, Modified: 3},
	})
	seg, w := sl.gitStatsSegment("s1", false)
	if seg == "" {
		t.Fatal("expected a non-empty segment")
	}
	if w <= 0 || w > gitStatsColW {
		t.Errorf("segment width = %d, want 1..%d", w, gitStatsColW)
	}
	// The digits for ahead/behind must be present in the rendered output.
	if !strings.Contains(seg, "2") || !strings.Contains(seg, "1") {
		t.Errorf("segment %q should contain ahead=2 and behind=1", seg)
	}
	if !strings.Contains(seg, gitDirtyMarker) {
		t.Errorf("segment %q should contain the dirty marker", seg)
	}
}

// TestGitStatsSegment_NoUpstreamHidesCounts verifies ahead/behind are suppressed
// without an upstream, but a dirty marker still shows.
func TestGitStatsSegment_NoUpstreamHidesCounts(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGitStatuses(map[string]platform.GitStatus{
		"s1": {Exists: true, IsRepo: true, HasUpstream: false, Ahead: 9, Modified: 1},
	})
	seg, _ := sl.gitStatsSegment("s1", false)
	if strings.Contains(seg, "9") {
		t.Errorf("segment %q should not show ahead count without upstream", seg)
	}
	if !strings.Contains(seg, gitDirtyMarker) {
		t.Errorf("segment %q should still show the dirty marker", seg)
	}
}

// TestGitStatsSegment_Selected verifies the segment is rendered unstyled when the
// row is selected (so the row highlight spans cleanly, matching renderDot).
func TestGitStatsSegment_Selected(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGitStatuses(map[string]platform.GitStatus{
		"s1": {Exists: true, IsRepo: true, HasUpstream: true, Ahead: 1},
	})
	seg, _ := sl.gitStatsSegment("s1", true)
	if strings.Contains(seg, "\x1b[") {
		t.Errorf("selected segment %q should contain no ANSI escapes", seg)
	}
}

// TestClampCount verifies two-digit capping keeps the column width bounded.
func TestClampCount(t *testing.T) {
	t.Parallel()
	cases := map[int]string{0: "0", 5: "5", 99: "99", 100: "99", 1000: "99"}
	for in, want := range cases {
		if got := clampCount(in); got != want {
			t.Errorf("clampCount(%d) = %q, want %q", in, got, want)
		}
	}
}

// TestGitStatsSegment_Missing verifies a missing workspace renders a marker
// inline (so the inline column can replace the badge at wider widths).
func TestGitStatsSegment_Missing(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetGitStatuses(map[string]platform.GitStatus{"s1": {Exists: false}})
	seg, w := sl.gitStatsSegment("s1", false)
	if seg == "" || w == 0 {
		t.Errorf("missing workspace should render a marker, got (%q, %d)", seg, w)
	}
}

// TestGitStatsSegment_BadgeSuppressed verifies the single-glyph badge is dropped
// when the inline stats column is shown, so a behind row shows the arrow once
// (inline), not twice (badge + inline).
func TestGitStatsSegment_BadgeSuppressed(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions([]data.Session{{ID: "s1", Summary: "x", LastActiveAt: "2025-01-01T00:00:00Z"}})
	sl.SetSize(140, 10)
	sl.SetGitStates(map[string]platform.GitState{"s1": platform.GitStateBehind})
	sl.SetGitStatuses(map[string]platform.GitStatus{
		"s1": {Exists: true, IsRepo: true, HasUpstream: true, Behind: 1},
	})
	out := sl.View()
	if n := strings.Count(out, styles.IconGitBehind()); n != 1 {
		t.Errorf("behind arrow appears %d times, want 1 (badge suppressed):\n%s", n, out)
	}
}

// TestGitStatsSegment_BadgeShownWhenNoInline verifies the badge still renders at
// narrower widths where the inline stats column is not shown (fallback).
func TestGitStatsSegment_BadgeShownWhenNoInline(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions([]data.Session{{ID: "s1", Summary: "x", LastActiveAt: "2025-01-01T00:00:00Z"}})
	sl.SetSize(60, 10) // below the w>=70 inline threshold
	sl.SetGitStates(map[string]platform.GitState{"s1": platform.GitStateBehind})
	sl.SetGitStatuses(map[string]platform.GitStatus{
		"s1": {Exists: true, IsRepo: true, HasUpstream: true, Behind: 1},
	})
	out := sl.View()
	if !strings.Contains(out, styles.IconGitBehind()) {
		t.Errorf("badge should show at narrow width when inline is hidden:\n%s", out)
	}
}

func TestGitStatsSegment_InRenderedRow(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions([]data.Session{{ID: "s1", Summary: "hello world", LastActiveAt: "2025-01-01T00:00:00Z"}})
	sl.SetSize(140, 10)
	sl.SetGitStatuses(map[string]platform.GitStatus{
		"s1": {Exists: true, IsRepo: true, HasUpstream: true, Ahead: 4, Behind: 2},
	})
	out := sl.View()
	if !strings.Contains(out, "4") || !strings.Contains(out, "2") {
		t.Errorf("rendered list should include inline ahead/behind counts, got:\n%s", out)
	}
}
