package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
)

func columnTestSession() data.Session {
	return data.Session{
		ID:           "sess-colcheck",
		Cwd:          `D:\code\projectZ`,
		Repository:   "octo/repo-xyz",
		Summary:      "ColumnVisibilityFixture",
		UpdatedAt:    "2025-01-15T10:00:00Z",
		LastActiveAt: "2025-01-15T10:00:00Z",
		TurnCount:    7,
	}
}

func renderColumnRow(t *testing.T, hidden []string) string {
	t.Helper()
	sl := NewSessionList()
	sl.SetSessions([]data.Session{columnTestSession()})
	sl.SetSize(120, 10)
	sl.SetHiddenColumns(hidden)
	return sl.View()
}

func TestSessionList_Columns_DefaultShowsAll(t *testing.T) {
	out := renderColumnRow(t, nil)
	if !strings.Contains(out, "ColumnVisibilityFixture") {
		t.Fatal("summary (session name) should always be shown")
	}
	if !strings.Contains(out, "octo/repo-xyz") {
		t.Error("repo column should be shown by default")
	}
	if !strings.Contains(out, "7t") {
		t.Error("turns column should be shown by default")
	}
	if !strings.Contains(out, "projectZ") {
		t.Error("folder column should be shown by default")
	}
}

func TestSessionList_Columns_HideRepo(t *testing.T) {
	out := renderColumnRow(t, []string{config.ColumnRepo})
	if strings.Contains(out, "octo/repo-xyz") {
		t.Error("repo column should be hidden")
	}
	if !strings.Contains(out, "ColumnVisibilityFixture") {
		t.Error("summary should remain visible after hiding repo")
	}
	if !strings.Contains(out, "7t") {
		t.Error("turns column should remain visible after hiding repo")
	}
}

func TestSessionList_Columns_HideTurns(t *testing.T) {
	out := renderColumnRow(t, []string{config.ColumnTurns})
	if strings.Contains(out, "7t") {
		t.Error("turns column should be hidden")
	}
	if !strings.Contains(out, "ColumnVisibilityFixture") {
		t.Error("summary should remain visible after hiding turns")
	}
	if !strings.Contains(out, "octo/repo-xyz") {
		t.Error("repo column should remain visible after hiding turns")
	}
}

func TestSessionList_Columns_HideFolder(t *testing.T) {
	out := renderColumnRow(t, []string{config.ColumnFolder})
	if strings.Contains(out, "projectZ") {
		t.Error("folder column should be hidden")
	}
	if !strings.Contains(out, "ColumnVisibilityFixture") {
		t.Error("summary should remain visible after hiding folder")
	}
}
