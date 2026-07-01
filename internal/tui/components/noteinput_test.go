package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/styles"
)

func TestNoteInput_FocusAndBlur(t *testing.T) {
	t.Parallel()
	ni := NewNoteInput()

	if ni.Focused() {
		t.Fatal("NoteInput should not be focused on creation")
	}

	_ = ni.Focus("session-123", "existing note")
	if !ni.Focused() {
		t.Fatal("NoteInput should be focused after Focus()")
	}
	if ni.SessionID() != "session-123" {
		t.Errorf("SessionID = %q, want %q", ni.SessionID(), "session-123")
	}
	if ni.Value() != "existing note" {
		t.Errorf("Value = %q, want %q", ni.Value(), "existing note")
	}

	ni.Blur()
	if ni.Focused() {
		t.Fatal("NoteInput should not be focused after Blur()")
	}
	if ni.SessionID() != "" {
		t.Errorf("SessionID should be empty after Blur, got %q", ni.SessionID())
	}
}

func TestNoteInput_SetWidth(t *testing.T) {
	t.Parallel()
	ni := NewNoteInput()
	// Should not panic.
	ni.SetWidth(80)
	ni.SetWidth(10)
	ni.SetWidth(0)
}

func TestNoteInput_View(t *testing.T) {
	t.Parallel()
	ni := NewNoteInput()
	ni.SetWidth(60)
	v := ni.View()
	if v == "" {
		t.Fatal("View() should not be empty")
	}
}

func TestSessionList_NoteDot(t *testing.T) {
	t.Parallel()
	sl := NewSessionList()
	sl.SetSize(120, 10)
	sl.SetSessions([]data.Session{
		{ID: "s1", Summary: "First session"},
		{ID: "s2", Summary: "Second session"},
	})

	// No notes set: no dot should appear.
	view := sl.View()
	noteIcon := styles.IconNote()
	if strings.Contains(view, noteIcon) {
		t.Error("Note icon should not appear when no notes are set")
	}

	// Set notes for s1.
	notesSet := map[string]struct{}{"s1": {}}
	sl.SetNoteSessions(notesSet)
	view = sl.View()
	if !strings.Contains(view, noteIcon) {
		t.Error("Note icon should appear for session with a note")
	}
}

func TestPreviewPanel_NoteRendering(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 30)

	detail := &data.SessionDetail{
		Session: data.Session{
			ID:      "sess-1",
			Summary: "Test session",
			Cwd:     "/home/user/project",
		},
	}
	p.SetDetail(detail)

	// No note: "Note:" should not appear.
	content := p.Content()
	if strings.Contains(content, "Note:") {
		t.Error("Note label should not appear when no note is set")
	}

	// Set a note.
	p.SetNote("Follow up on auth refactor")
	content = p.Content()
	if !strings.Contains(content, "Note:") {
		t.Error("Note label should appear when a note is set")
	}
	if !strings.Contains(content, "Follow up on auth refactor") {
		t.Error("Note text should appear in the preview content")
	}

	// Clear the note.
	p.SetNote("")
	content = p.Content()
	if strings.Contains(content, "Note:") {
		t.Error("Note label should not appear after clearing the note")
	}
}
