package tui

import (
	"errors"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
)

// TestHandleGitStatus_NoPath verifies pressing the git-status key with no
// resolvable folder shows a hint and stays in the session list.
func TestHandleGitStatus_NoPath(t *testing.T) {
	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "abc-123", Cwd: ""}})

	result, _ := m.Update(runeKeyMsg('i'))
	rm := result.(Model)

	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList", rm.state)
	}
	if rm.statusInfo != "No folder to inspect" {
		t.Errorf("statusInfo = %q, want %q", rm.statusInfo, "No folder to inspect")
	}
}

// TestHandleGitStatus_ValidPathReturnsCmd verifies pressing the git-status key
// with a real cwd returns a command (the async status gather) without yet
// switching state — the overlay opens when the resulting message arrives.
func TestHandleGitStatus_ValidPathReturnsCmd(t *testing.T) {
	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "abc-123", Cwd: "/home/me/proj"}})

	result, cmd := m.Update(runeKeyMsg('i'))
	rm := result.(Model)

	if cmd == nil {
		t.Error("expected a command to gather git status, got nil")
	}
	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList before message arrives", rm.state)
	}
}

// TestHandleGitStatusMsg_OpensOverlay verifies a gitStatusMsg opens the overlay.
func TestHandleGitStatusMsg_OpensOverlay(t *testing.T) {
	m := newTestModel()
	status := platform.GitStatus{
		Dir: "/home/me/proj", Exists: true, IsRepo: true,
		Branch: "main", Upstream: "origin/main", HasUpstream: true, Ahead: 1,
	}

	result, _ := m.Update(gitStatusMsg{status: status})
	rm := result.(Model)

	if rm.state != stateGitStatusView {
		t.Errorf("state = %v, want stateGitStatusView", rm.state)
	}
	if view := rm.gitStatusView.View(); view == "" {
		t.Error("overlay View() should render after opening")
	}
}

// TestGitStatusOverlay_EscCloses verifies esc returns to the session list.
func TestGitStatusOverlay_EscCloses(t *testing.T) {
	m := newTestModel()
	m.state = stateGitStatusView
	m.gitStatusView.SetStatus(platform.GitStatus{Dir: "/p", Exists: true, IsRepo: true, Branch: "main"})

	result, _ := m.Update(escKeyMsg())
	rm := result.(Model)

	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList after esc", rm.state)
	}
}

// TestGitStatusOverlay_Copy verifies the copy key writes the plain-text summary.
func TestGitStatusOverlay_Copy(t *testing.T) {
	var copied string
	orig := clipboardWrite
	clipboardWrite = func(text string) error {
		copied = text
		return nil
	}
	t.Cleanup(func() { clipboardWrite = orig })

	m := newTestModel()
	m.state = stateGitStatusView
	m.gitStatusView.SetStatus(platform.GitStatus{
		Dir: "/home/me/proj", Exists: true, IsRepo: true,
		Branch: "main", Upstream: "origin/main", HasUpstream: true, Ahead: 3,
	})

	result, _ := m.Update(runeKeyMsg('c'))
	rm := result.(Model)

	if copied == "" {
		t.Fatal("clipboard was not written")
	}
	if want := "Git Status"; !strings.Contains(copied, want) {
		t.Errorf("clipboard text %q missing %q", copied, want)
	}
	if rm.statusInfo == "" {
		t.Error("statusInfo should confirm the copy")
	}
}

// TestGitStatusOverlay_CopyError verifies a clipboard failure surfaces an error.
func TestGitStatusOverlay_CopyError(t *testing.T) {
	orig := clipboardWrite
	clipboardWrite = func(string) error { return errors.New("no clipboard") }
	t.Cleanup(func() { clipboardWrite = orig })

	m := newTestModel()
	m.state = stateGitStatusView
	m.gitStatusView.SetStatus(platform.GitStatus{Dir: "/p", Exists: true, IsRepo: true, Branch: "main"})

	result, _ := m.Update(runeKeyMsg('c'))
	rm := result.(Model)
	if rm.statusErr == "" {
		t.Error("statusErr should be set on clipboard failure")
	}
}

// TestGitStatusKeybinding verifies the git-status action is bound to 'i' and is
// registered as a remappable action.
func TestGitStatusKeybinding(t *testing.T) {
	km := defaultKeyMap()
	if !key.Matches(tea.KeyPressMsg{Code: 'i', Text: "i"}, km.GitStatus) {
		t.Error("'i' should match the GitStatus binding")
	}

	found := false
	for _, e := range keybindingEntries(&km) {
		if e.name == "git_status" {
			found = true
			break
		}
	}
	if !found {
		t.Error("git_status should be a registered remappable action")
	}
}

// TestDemoGitStatus verifies the demo-mode synthetic status is a populated repo.
func TestDemoGitStatus(t *testing.T) {
	s := demoGitStatus("/demo/dir")
	if !s.Exists || !s.IsRepo {
		t.Error("demo status should be an existing repo")
	}
	if s.Dir != "/demo/dir" {
		t.Errorf("Dir = %q, want /demo/dir", s.Dir)
	}
	if s.Ahead == 0 && s.Behind == 0 {
		t.Error("demo status should show push/pull activity")
	}
	if len(s.Files) == 0 {
		t.Error("demo status should list changed files")
	}
}

// TestCmdPaletteAction_GitStatus verifies the palette "git-status" action opens
// the status flow for the selected session's folder.
func TestCmdPaletteAction_GitStatus(t *testing.T) {
	m := newTestModel()
	m.sessionList.SetSessions([]data.Session{{ID: "abc", Cwd: "/home/me/proj"}})

	result, cmd := m.Update(cmdPaletteActionMsg{action: "git-status"})
	rm := result.(Model)
	if cmd == nil {
		t.Error("git-status palette action should return a command")
	}
	if rm.state != stateSessionList {
		t.Errorf("state = %v, want stateSessionList before the status message arrives", rm.state)
	}
}

// TestGitStatusOverlay_ScrollKeys verifies up/down keys scroll the overlay's
// file list without leaving the overlay.
func TestGitStatusOverlay_ScrollKeys(t *testing.T) {
	m := newTestModel()
	m.state = stateGitStatusView
	files := make([]platform.GitFileStatus, 30)
	for i := range files {
		files[i] = platform.GitFileStatus{Code: " M", Path: "f.go"}
	}
	m.gitStatusView.SetStatus(platform.GitStatus{
		Dir: "/p", Exists: true, IsRepo: true, Branch: "main", Modified: 30, Files: files,
	})
	m.gitStatusView.SetSize(80, 10)

	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	rm := result.(Model)
	if rm.state != stateGitStatusView {
		t.Errorf("state = %v, want stateGitStatusView after scroll down", rm.state)
	}
	result2, _ := rm.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if result2.(Model).state != stateGitStatusView {
		t.Error("should remain in overlay after scroll up")
	}
}
