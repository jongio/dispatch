package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

func makeSessions(n int) []data.Session {
	summaries := []string{
		"Fix Session List Data Corruption",
		"",
		"Move Azure AI To Shared Infra",
		"go",
		"Configure DevX Swarm",
		"keep going",
		"cls",
		"Evaluate whether the assistant fixed the root cause of the reported issue\nrather than suppressing it",
		"fix\n\nThe build is broken. Here is the build output:\n\nsrc/UserCard.tsx:12:5 - error TS2532",
		"Please summarize the following code file.\n",
	}
	sessions := make([]data.Session, n)
	for i := range sessions {
		sessions[i] = data.Session{
			ID:           "sess-" + strings.Repeat("0", 8) + string(rune('A'+i%26)),
			Cwd:          `D:\code\project` + string(rune('A'+i%5)),
			Summary:      summaries[i%len(summaries)],
			UpdatedAt:    "2025-01-15T10:00:00Z",
			LastActiveAt: "2025-01-15T10:00:00Z",
			TurnCount:    i * 3,
			FileCount:    i,
		}
	}
	return sessions
}

func makeGroups(folders int, sessionsPerFolder int) []data.SessionGroup {
	groups := make([]data.SessionGroup, folders)
	for f := range groups {
		sessions := make([]data.Session, sessionsPerFolder)
		for i := range sessions {
			idx := f*sessionsPerFolder + i
			sessions[i] = data.Session{
				ID:           "sess-" + strings.Repeat("0", 6) + string(rune('A'+f%26)) + string(rune('a'+i%26)),
				Cwd:          `D:\code\folder` + string(rune('A'+f%26)),
				Summary:      "Session " + string(rune('A'+idx%26)),
				UpdatedAt:    "2025-01-15T10:00:00Z",
				LastActiveAt: "2025-01-15T10:00:00Z",
				TurnCount:    idx,
				FileCount:    idx % 10,
			}
		}
		groups[f] = data.SessionGroup{
			Label:    `D:\code\folder` + string(rune('A'+f%26)),
			Sessions: sessions,
			Count:    sessionsPerFolder,
		}
	}
	return groups
}

// TestSessionListViewConsistency verifies that every View() output during
// scrolling has exactly height lines, each of exactly width columns.
func TestSessionListViewConsistency(t *testing.T) {
	const width = 120
	const height = 25

	sl := NewSessionList()
	sl.SetSessions(makeSessions(50))
	sl.SetSize(width, height)

	for step := 0; step < 50; step++ {
		view := sl.View()
		lines := strings.Split(view, "\n")
		if len(lines) != height {
			t.Fatalf("step %d: View() has %d lines, want %d", step, len(lines), height)
		}
		sl.MoveDown()
	}
}

// TestSessionListTreeViewConsistency does the same for tree mode (groups).
func TestSessionListTreeViewConsistency(t *testing.T) {
	const width = 120
	const height = 25

	sl := NewSessionList()
	sl.SetGroups(makeGroups(5, 10))
	sl.SetSize(width, height)

	total := len(sl.visItems)
	for step := 0; step < total+5; step++ {
		view := sl.View()
		lines := strings.Split(view, "\n")
		if len(lines) != height {
			t.Fatalf("step %d (cursor=%d scroll=%d vis=%d): View() has %d lines, want %d",
				step, sl.cursor, sl.scrollOffset, len(sl.visItems), len(lines), height)
		}
		sl.MoveDown()
	}
}

// TestSessionListViewLineWidths checks that every line in View() has the
// expected terminal column width (using len([]rune) as a proxy for ASCII data).
func TestSessionListViewLineWidths(t *testing.T) {
	const width = 100
	const height = 20

	sl := NewSessionList()
	sl.SetSessions(makeSessions(30))
	sl.SetSize(width, height)

	for step := 0; step < 30; step++ {
		view := sl.View()
		lines := strings.Split(view, "\n")
		for i, line := range lines {
			// Strip ANSI codes for width measurement.
			plain := stripAnsi(line)
			pw := len([]rune(plain))
			if pw != width {
				t.Errorf("step %d line %d: width=%d want %d, line=%q",
					step, i, pw, width, plain)
			}
		}
		sl.MoveDown()
	}
}

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if inEsc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		if r == '\033' {
			inEsc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
