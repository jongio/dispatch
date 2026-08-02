package tui

import (
	"path/filepath"
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

// richPaletteModel builds a Model in which every command-palette action is
// enabled and effectful: a session is selected, the preview is populated, the
// detail carries an openable ref, and a store is present. This lets the test
// drive each action through the real handler and observe an effect.
func richPaletteModel(t *testing.T) Model {
	t.Helper()

	m := newTestModelWithSize(120, 40)

	sess := data.Session{ID: "s1", Cwd: "/tmp", Summary: "hello", Repository: "jongio/dispatch"}
	m.sessions = []data.Session{sess}
	m.sessionList.SetSessions(m.sessions)

	detail := &data.SessionDetail{
		Session: sess,
		Turns: []data.Turn{
			{SessionID: "s1", TurnIndex: 0, UserMessage: "hi there", AssistantResponse: "hello back"},
		},
		Refs: []data.SessionRef{{SessionID: "s1", RefType: "pr", RefValue: "42"}},
	}
	m.detail = detail
	m.showPreview = true
	m.preview.SetSize(60, 30)
	m.preview.SetDetail(detail)
	// A non-nil store makes the "export" action effectful. Its Close is only
	// reached by the "quit" action, which the test drives with a nil store.
	m.store = &data.Store{}
	m.attentionMap = map[string]data.AttentionStatus{"s1": data.AttentionWaiting}

	m.favoritedSet = map[string]struct{}{}
	m.waitingNotified = map[string]struct{}{}

	return m
}

// TestCmdPaletteActionsAllHandled asserts that every action produced by
// openCmdPalette is dispatched by handleCmdPaletteAction (never falls through
// to the silent default branch). Each action is driven through the handler and
// must produce an observable effect: a non-nil command or a change to a
// user-visible field.
func TestCmdPaletteActionsAllHandled(t *testing.T) {
	// Redirect config persistence (the "preview"/"settings" paths call
	// saveConfig) to a throwaway file.
	t.Setenv("DISPATCH_CONFIG", filepath.Join(t.TempDir(), "config.json"))

	// Enumerate the actions exactly as the palette presents them.
	enum := richPaletteModel(t)
	enum.openCmdPalette()

	var actions []string
	n := enum.cmdPalette.FilteredCount()
	for i := 0; i < n; i++ {
		if action, ok := enum.cmdPalette.Selected(); ok {
			actions = append(actions, action)
		}
		enum.cmdPalette.MoveDown()
	}

	if len(actions) == 0 {
		t.Fatal("openCmdPalette produced no enabled actions")
	}

	effectful := func(m Model, action string) bool {
		// The quit path calls store.Close, which panics on a zero-value
		// store; a nil store is closed safely and still yields tea.Quit.
		if action == "quit" {
			m.store = nil
		}
		before := m
		res, cmd := m.handleCmdPaletteAction(cmdPaletteActionMsg{action: action})
		after := res.(Model)
		return cmd != nil ||
			after.state != before.state ||
			after.showPreview != before.showPreview ||
			after.statusInfo != before.statusInfo ||
			after.statusErr != before.statusErr ||
			after.reindexing != before.reindexing
	}

	// Sanity check: an action with no case must be reported as not effectful,
	// confirming the detector distinguishes the default branch.
	if effectful(richPaletteModel(t), "totally-bogus-action") {
		t.Fatal("bogus action was treated as handled; the effect detector is unreliable")
	}

	for _, action := range actions {
		if !effectful(richPaletteModel(t), action) {
			t.Errorf("palette action %q hit the silent default branch (no state change or command)", action)
		}
	}
}
