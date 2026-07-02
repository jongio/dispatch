package tui

import (
	"errors"
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

func TestHandleKey_CopyPath_Success(t *testing.T) {
	var copied string
	orig := clipboardWrite
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	t.Cleanup(func() { clipboardWrite = orig })

	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "abc-123", Cwd: "/home/me/proj"}})
	result, cmd := m.Update(runeKeyMsg('C'))
	rm := result.(Model)
	if copied != "/home/me/proj" {
		t.Errorf("clipboard text = %q, want %q", copied, "/home/me/proj")
	}
	if rm.statusInfo != statusCopiedPath {
		t.Errorf("statusInfo = %q, want %q", rm.statusInfo, statusCopiedPath)
	}
	if rm.statusErr != "" {
		t.Errorf("statusErr = %q, want empty", rm.statusErr)
	}
	if cmd == nil {
		t.Error("CopyPath success should return clearStatusAfter cmd")
	}
}

func TestHandleKey_CopyPath_NoPath(t *testing.T) {
	var called bool
	orig := clipboardWrite
	clipboardWrite = func(string) error {
		called = true
		return nil
	}
	t.Cleanup(func() { clipboardWrite = orig })

	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "abc-123", Cwd: ""}})
	result, _ := m.Update(runeKeyMsg('C'))
	rm := result.(Model)
	if called {
		t.Error("clipboard should not be written when there is no path")
	}
	if rm.statusInfo != "No path to copy" {
		t.Errorf("statusInfo = %q, want %q", rm.statusInfo, "No path to copy")
	}
}

func TestHandleKey_CopyPath_Error(t *testing.T) {
	orig := clipboardWrite
	clipboardWrite = func(string) error {
		return errors.New("no display")
	}
	t.Cleanup(func() { clipboardWrite = orig })

	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "abc-123", Cwd: "/x"}})
	result, _ := m.Update(runeKeyMsg('C'))
	rm := result.(Model)
	if rm.statusErr == "" {
		t.Error("CopyPath clipboard error should set statusErr")
	}
}
