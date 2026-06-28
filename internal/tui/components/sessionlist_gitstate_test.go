package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
)

// ---------------------------------------------------------------------------
// Git state dot rendering
// ---------------------------------------------------------------------------

func TestSessionList_GitStateDot_NilMap(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions([]data.Session{{ID: "s1", Summary: "test", LastActiveAt: "2025-01-01T00:00:00Z"}})
	sl.SetSize(120, 10)

	// With nil git state map, dot should be 2 spaces.
	dot := sl.gitStateDot("s1", false)
	if dot != "  " {
		t.Errorf("gitStateDot with nil map = %q, want %q", dot, "  ")
	}
}

func TestSessionList_GitStateDot_Unknown(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions([]data.Session{{ID: "s1", Summary: "test", LastActiveAt: "2025-01-01T00:00:00Z"}})
	sl.SetSize(120, 10)
	sl.SetGitStates(map[string]platform.GitState{"s1": platform.GitStateUnknown})

	dot := sl.gitStateDot("s1", false)
	if dot != "  " {
		t.Errorf("gitStateDot(unknown) = %q, want %q", dot, "  ")
	}
}

func TestSessionList_GitStateDot_Clean(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions([]data.Session{{ID: "s1", Summary: "test", LastActiveAt: "2025-01-01T00:00:00Z"}})
	sl.SetSize(120, 10)
	sl.SetGitStates(map[string]platform.GitState{"s1": platform.GitStateClean})

	// Clean state shows no badge (two spaces).
	dot := sl.gitStateDot("s1", false)
	if dot != "  " {
		t.Errorf("gitStateDot(clean) = %q, want %q", dot, "  ")
	}
}

func TestSessionList_GitStateDot_Dirty(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions([]data.Session{{ID: "s1", Summary: "test", LastActiveAt: "2025-01-01T00:00:00Z"}})
	sl.SetSize(120, 10)
	sl.SetGitStates(map[string]platform.GitState{"s1": platform.GitStateDirty})

	dot := sl.gitStateDot("s1", false)
	if dot == "  " {
		t.Error("gitStateDot(dirty) should not be blank")
	}
}

func TestSessionList_GitStateDot_Missing(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions([]data.Session{{ID: "s1", Summary: "test", LastActiveAt: "2025-01-01T00:00:00Z"}})
	sl.SetSize(120, 10)
	sl.SetGitStates(map[string]platform.GitState{"s1": platform.GitStateMissing})

	dot := sl.gitStateDot("s1", false)
	if dot == "  " {
		t.Error("gitStateDot(missing) should not be blank")
	}
}

func TestSessionList_GitStateDot_NotInMap(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSessions([]data.Session{{ID: "s1", Summary: "test", LastActiveAt: "2025-01-01T00:00:00Z"}})
	sl.SetSize(120, 10)
	sl.SetGitStates(map[string]platform.GitState{"other": platform.GitStateDirty})

	// Session not in the map should show blank.
	dot := sl.gitStateDot("s1", false)
	if dot != "  " {
		t.Errorf("gitStateDot(not in map) = %q, want %q", dot, "  ")
	}
}

func TestSessionList_RenderWithGitState(t *testing.T) {
	t.Parallel()
	sessions := []data.Session{
		{ID: "s1", Summary: "Clean session", Cwd: "/tmp/a", LastActiveAt: "2025-01-01T00:00:00Z", TurnCount: 3},
		{ID: "s2", Summary: "Dirty session", Cwd: "/tmp/b", LastActiveAt: "2025-01-01T00:00:00Z", TurnCount: 5},
	}

	sl := NewSessionList()
	sl.SetSessions(sessions)
	sl.SetSize(120, 10)
	sl.SetGitStates(map[string]platform.GitState{
		"s1": platform.GitStateClean,
		"s2": platform.GitStateDirty,
	})

	view := sl.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}

	lines := strings.Split(view, "\n")
	if len(lines) != 10 {
		t.Fatalf("View() has %d lines, want 10", len(lines))
	}
}
