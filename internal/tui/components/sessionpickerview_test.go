package components

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// makePickerRows returns n rows whose IDs are "id0".."id{n-1}" and whose other
// columns are empty, so tests can search for a row by its unique ID substring.
func makePickerRows(n int) []SessionPickerRow {
	rows := make([]SessionPickerRow, n)
	for i := range rows {
		rows[i] = SessionPickerRow{ID: "id" + strconv.Itoa(i)}
	}
	return rows
}

func firstLine(s string) string {
	return strings.SplitN(s, "\n", 2)[0]
}

// lineContaining returns the first output line that contains sub, or "" if none.
func lineContaining(s, sub string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, sub) {
			return line
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Render: header line (three distinct title forms)
// ---------------------------------------------------------------------------

func TestSessionPickerView_RenderHeader(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		view SessionPickerView
		want string
	}{
		{
			// Not hasMore: HasMore false and everything loaded is shown.
			name: "plain count",
			view: SessionPickerView{Rows: makePickerRows(3), Visible: 3, Height: 40},
			want: "Select a session (3)",
		},
		{
			// Visible < len(Rows) forces the "of" form.
			name: "visible of total",
			view: SessionPickerView{Rows: makePickerRows(5), Visible: 2, Height: 40},
			want: "Select a session (2 of 5)",
		},
		{
			// HasMore with everything loaded shown uses the "loaded" form.
			name: "loaded with more available",
			view: SessionPickerView{Rows: makePickerRows(3), Visible: 3, HasMore: true, Height: 40},
			want: "Select a session (3 loaded)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := firstLine(tc.view.Render())
			if got != tc.want {
				t.Errorf("header = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Render: cursor prefix
// ---------------------------------------------------------------------------

func TestSessionPickerView_RenderCursor(t *testing.T) {
	t.Parallel()
	v := SessionPickerView{Rows: makePickerRows(4), Visible: 4, Cursor: 2, Height: 40}
	out := v.Render()

	cursorLine := lineContaining(out, "id2")
	if cursorLine == "" {
		t.Fatal("expected a rendered row containing id2")
	}
	if !strings.HasPrefix(cursorLine, "> ") {
		t.Errorf("cursor row = %q, want prefix %q", cursorLine, "> ")
	}

	otherLine := lineContaining(out, "id0")
	if otherLine == "" {
		t.Fatal("expected a rendered row containing id0")
	}
	if !strings.HasPrefix(otherLine, "  ") {
		t.Errorf("non-cursor row = %q, want prefix %q", otherLine, "  ")
	}
	if strings.HasPrefix(otherLine, "> ") {
		t.Errorf("non-cursor row must not carry the cursor marker: %q", otherLine)
	}
}

// ---------------------------------------------------------------------------
// Render: "show more" affordance
// ---------------------------------------------------------------------------

func TestSessionPickerView_RenderShowMore(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		view        SessionPickerView
		wantContain string
		wantAbsent  string
	}{
		{
			name:        "loading",
			view:        SessionPickerView{Rows: makePickerRows(5), Visible: 2, Loading: true, Height: 40},
			wantContain: "Loading more sessions…",
		},
		{
			name:        "remaining smaller than batch",
			view:        SessionPickerView{Rows: makePickerRows(10), Visible: 3, Height: 40},
			wantContain: "Show 7 more sessions (7 remaining)",
		},
		{
			name:        "remaining larger than batch is capped",
			view:        SessionPickerView{Rows: makePickerRows(100), Visible: 10, Height: 40},
			wantContain: "Show 50 more sessions (90 remaining)",
		},
		{
			name:        "morecount fallback honored",
			view:        SessionPickerView{Rows: makePickerRows(3), Visible: 3, HasMore: true, MoreCount: 25, Height: 40},
			wantContain: "Show 25 more sessions",
			wantAbsent:  "remaining",
		},
		{
			name:        "morecount zero uses default batch",
			view:        SessionPickerView{Rows: makePickerRows(3), Visible: 3, HasMore: true, MoreCount: 0, Height: 40},
			wantContain: "Show 50 more sessions",
			wantAbsent:  "remaining",
		},
		{
			name:        "morecount negative uses default batch",
			view:        SessionPickerView{Rows: makePickerRows(3), Visible: 3, HasMore: true, MoreCount: -4, Height: 40},
			wantContain: "Show 50 more sessions",
			wantAbsent:  "remaining",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := tc.view.Render()
			if !strings.Contains(out, tc.wantContain) {
				t.Errorf("Render() missing %q\n---\n%s", tc.wantContain, out)
			}
			if tc.wantAbsent != "" && strings.Contains(out, tc.wantAbsent) {
				t.Errorf("Render() should not contain %q\n---\n%s", tc.wantAbsent, out)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Render: footer hints
// ---------------------------------------------------------------------------

func TestSessionPickerView_RenderFooter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		view        SessionPickerView
		wantMoreHnt bool
	}{
		{
			name:        "no more data",
			view:        SessionPickerView{Rows: makePickerRows(2), Visible: 2, Height: 40},
			wantMoreHnt: false,
		},
		{
			name:        "more data available",
			view:        SessionPickerView{Rows: makePickerRows(5), Visible: 2, Height: 40},
			wantMoreHnt: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := tc.view.Render()
			for _, hint := range []string{"↑/↓ move", "Enter resume", "q cancel"} {
				if !strings.Contains(out, hint) {
					t.Errorf("footer missing required hint %q\n---\n%s", hint, out)
				}
			}
			hasMoreHint := strings.Contains(out, "· m more")
			if hasMoreHint != tc.wantMoreHnt {
				t.Errorf("footer 'm more' hint present = %v, want %v", hasMoreHint, tc.wantMoreHnt)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SessionPickerVisibleRows: boundary values and the floor of 1
// ---------------------------------------------------------------------------

func TestSessionPickerVisibleRows(t *testing.T) {
	t.Parallel()

	cases := []struct {
		height int
		want   int
	}{
		{height: 100, want: 94},
		{height: 10, want: 4},
		{height: 7, want: 1},  // 7-6 = 1
		{height: 6, want: 1},  // 6-6 = 0, floored to 1
		{height: 1, want: 1},  // floored
		{height: 0, want: 1},  // floored
		{height: -5, want: 1}, // floored despite negative height
	}

	for _, tc := range cases {
		if got := SessionPickerVisibleRows(tc.height); got != tc.want {
			t.Errorf("SessionPickerVisibleRows(%d) = %d, want %d", tc.height, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// columnWidths: min widths, growth toward targets, leftover, ID sizing
// ---------------------------------------------------------------------------

func TestSessionPickerColumnWidths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                                   string
		view                                   SessionPickerView
		wantID, wantSummary, wantRepo, wantBrn int
	}{
		{
			// Tiny width: every column collapses to its minimum, and the ID
			// column collapses to the "SESSION ID" header width.
			name:        "minimum widths at tiny width",
			view:        SessionPickerView{Width: 1, IDWidth: 10},
			wantID:      10,
			wantSummary: 8,
			wantRepo:    10,
			wantBrn:     6,
		},
		{
			// Partway: summary reaches its target first, repo grows partially.
			name:        "partial growth",
			view:        SessionPickerView{Width: 60, IDWidth: 10},
			wantID:      10,
			wantSummary: 24,
			wantRepo:    12,
			wantBrn:     6,
		},
		{
			// Further: summary and repo at target, branch grows partially.
			name:        "more growth",
			view:        SessionPickerView{Width: 80, IDWidth: 10},
			wantID:      10,
			wantSummary: 24,
			wantRepo:    28,
			wantBrn:     10,
		},
		{
			// All targets met and leftover space is handed to the summary.
			name:        "leftover goes to summary",
			view:        SessionPickerView{Width: 102, IDWidth: 10},
			wantID:      10,
			wantSummary: 34, // 24 target + 10 leftover
			wantRepo:    28,
			wantBrn:     22,
		},
		{
			// An explicit IDWidth is honored and the rows are not scanned,
			// even though the row ID is far wider than IDWidth.
			name:        "explicit id width honored",
			view:        SessionPickerView{Width: 1, IDWidth: 20, Rows: []SessionPickerRow{{ID: strings.Repeat("x", 40)}}},
			wantID:      20,
			wantSummary: 8,
			wantRepo:    10,
			wantBrn:     6,
		},
		{
			// IDWidth <= 0 derives the ID column from the widest row ID.
			name:        "derive id width from widest row",
			view:        SessionPickerView{Width: 1, IDWidth: 0, Rows: []SessionPickerRow{{ID: "short"}, {ID: "a-much-longer-id"}}},
			wantID:      len("a-much-longer-id"), // 16
			wantSummary: 8,
			wantRepo:    10,
			wantBrn:     6,
		},
		{
			// IDWidth <= 0 never drops below the "SESSION ID" header width.
			name:        "derived id width floors at header width",
			view:        SessionPickerView{Width: 1, IDWidth: 0, Rows: []SessionPickerRow{{ID: "ab"}, {ID: "cd"}}},
			wantID:      10,
			wantSummary: 8,
			wantRepo:    10,
			wantBrn:     6,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			id, summary, repo, branch := tc.view.columnWidths()
			if id != tc.wantID {
				t.Errorf("id width = %d, want %d", id, tc.wantID)
			}
			if summary != tc.wantSummary {
				t.Errorf("summary width = %d, want %d", summary, tc.wantSummary)
			}
			if repo != tc.wantRepo {
				t.Errorf("repo width = %d, want %d", repo, tc.wantRepo)
			}
			if branch != tc.wantBrn {
				t.Errorf("branch width = %d, want %d", branch, tc.wantBrn)
			}
		})
	}
}

func TestSessionPickerColumnWidths_GrowWithWidth(t *testing.T) {
	t.Parallel()

	widths := []int{40, 60, 80, 100, 140}
	var prevID, prevSummary, prevRepo, prevBranch, prevTotal int
	for i, w := range widths {
		v := SessionPickerView{Width: w, IDWidth: 10}
		id, summary, repo, branch := v.columnWidths()
		total := id + summary + repo + branch
		if i > 0 {
			if id < prevID || summary < prevSummary || repo < prevRepo || branch < prevBranch {
				t.Errorf("width %d: columns shrank (id %d->%d, summary %d->%d, repo %d->%d, branch %d->%d)",
					w, prevID, id, prevSummary, summary, prevRepo, repo, prevBranch, branch)
			}
			if total <= prevTotal {
				t.Errorf("width %d: total column width %d did not grow past %d", w, total, prevTotal)
			}
		}
		prevID, prevSummary, prevRepo, prevBranch, prevTotal = id, summary, repo, branch, total
	}
}

// ---------------------------------------------------------------------------
// padSessionPickerCell: truncate + pad to an exact display width
// ---------------------------------------------------------------------------

func TestPadSessionPickerCell(t *testing.T) {
	t.Parallel()

	t.Run("short value is padded on the right", func(t *testing.T) {
		t.Parallel()
		got := padSessionPickerCell("abc", 6)
		want := "abc   "
		if got != want {
			t.Errorf("padSessionPickerCell(\"abc\", 6) = %q, want %q", got, want)
		}
		if w := ansi.StringWidth(got); w != 6 {
			t.Errorf("display width = %d, want 6", w)
		}
	})

	t.Run("exact width value is unchanged", func(t *testing.T) {
		t.Parallel()
		got := padSessionPickerCell("hello", 5)
		if got != "hello" {
			t.Errorf("padSessionPickerCell(\"hello\", 5) = %q, want %q", got, "hello")
		}
	})

	t.Run("over-long value is truncated with ellipsis", func(t *testing.T) {
		t.Parallel()
		got := padSessionPickerCell("abcdefghij", 5)
		want := "abcd…" // 4 columns of content plus the 1-column ellipsis tail
		if got != want {
			t.Errorf("padSessionPickerCell(\"abcdefghij\", 5) = %q, want %q", got, want)
		}
		if w := ansi.StringWidth(got); w != 5 {
			t.Errorf("display width = %d, want 5", w)
		}
	})

	t.Run("wide characters padded to display width", func(t *testing.T) {
		t.Parallel()
		// "日" has a display width of 2, so padding to 5 yields width 5, not
		// a byte-length-based result.
		got := padSessionPickerCell("日", 5)
		if w := ansi.StringWidth(got); w != 5 {
			t.Errorf("display width = %d, want 5 (got %q)", w, got)
		}
		if !strings.HasPrefix(got, "日") {
			t.Errorf("padded cell should start with the original wide char, got %q", got)
		}
	})

	t.Run("wide characters truncated to display width", func(t *testing.T) {
		t.Parallel()
		// "日本語" is 6 display columns; truncating to 4 must yield a cell of
		// display width 4 (not 4 bytes) and include the ellipsis.
		got := padSessionPickerCell("日本語", 4)
		if w := ansi.StringWidth(got); w != 4 {
			t.Errorf("display width = %d, want 4 (got %q)", w, got)
		}
		if !strings.Contains(got, "…") {
			t.Errorf("truncated wide cell should contain the ellipsis, got %q", got)
		}
	})
}

// ---------------------------------------------------------------------------
// Render: offset windowing
// ---------------------------------------------------------------------------

func TestSessionPickerView_RenderOffsetWindow(t *testing.T) {
	t.Parallel()

	// 10 loaded rows, a viewport that shows 4 (Height 10 -> visibleRows 4),
	// scrolled so the window starts at Offset 3.
	v := SessionPickerView{Rows: makePickerRows(10), Visible: 10, Offset: 3, Cursor: 3, Height: 10}
	if got := v.visibleRows(); got != 4 {
		t.Fatalf("precondition: visibleRows = %d, want 4", got)
	}
	out := v.Render()

	// Rows inside the window [3,7) must render.
	for _, id := range []string{"id3", "id4", "id5", "id6"} {
		if lineContaining(out, id) == "" {
			t.Errorf("row %s should be rendered within the window", id)
		}
	}
	// Rows before the offset and past the window must not render.
	for _, id := range []string{"id0", "id1", "id2", "id7", "id8", "id9"} {
		if lineContaining(out, id) != "" {
			t.Errorf("row %s should be outside the rendered window", id)
		}
	}

	// No more than visibleRows() data rows are rendered.
	rendered := 0
	for _, line := range strings.Split(out, "\n") {
		if (strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "> ")) && strings.Contains(line, "id") {
			rendered++
		}
	}
	if rendered != 4 {
		t.Errorf("rendered %d data rows, want 4 (== visibleRows)", rendered)
	}
}
