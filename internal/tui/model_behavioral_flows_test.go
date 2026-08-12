package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
)

func TestEditHandlersLoadCurrentValues(t *testing.T) {
	t.Run("note", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.cfg.SessionNotes = map[string]string{"s1": "keep the migration note"}
		m.sessionList.SetSessions([]data.Session{{ID: "s1"}})

		result, _ := m.handleEditNote()
		got := result.(Model)

		if !got.noteInput.Focused() {
			t.Fatal("note input should be focused")
		}
		if got.noteInput.SessionID() != "s1" {
			t.Fatalf("note session = %q, want s1", got.noteInput.SessionID())
		}
		if got.noteInput.Value() != "keep the migration note" {
			t.Fatalf("note value = %q", got.noteInput.Value())
		}
	})

	t.Run("tags", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.cfg.SetTags("s1", []string{"backend", "urgent"})
		m.sessionList.SetSessions([]data.Session{{ID: "s1"}})

		result, _ := m.handleEditTags()
		got := result.(Model)

		if !got.tagInput.Focused() {
			t.Fatal("tag input should be focused")
		}
		if got.tagInput.SessionID() != "s1" {
			t.Fatalf("tag session = %q, want s1", got.tagInput.SessionID())
		}
		if got.tagInput.Value() != "backend, urgent" {
			t.Fatalf("tag value = %q, want %q", got.tagInput.Value(), "backend, urgent")
		}
	})

	t.Run("alias", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		if err := m.cfg.SetAlias("s1", "Checkout investigation"); err != nil {
			t.Fatalf("setting alias: %v", err)
		}
		m.sessionList.SetSessions([]data.Session{{ID: "s1"}})

		result, _ := m.handleEditAlias()
		got := result.(Model)

		if !got.aliasInput.Focused() {
			t.Fatal("alias input should be focused")
		}
		if got.aliasInput.SessionID() != "s1" {
			t.Fatalf("alias session = %q, want s1", got.aliasInput.SessionID())
		}
		if got.aliasInput.Value() != "checkout" {
			t.Fatalf("alias value = %q", got.aliasInput.Value())
		}
	})

	t.Run("no selection", func(t *testing.T) {
		m := newBehavioralFlowModel(t)

		noteModel, noteCmd := m.handleEditNote()
		tagModel, tagCmd := m.handleEditTags()
		aliasModel, aliasCmd := m.handleEditAlias()
		note := noteModel.(Model)
		tags := tagModel.(Model)
		alias := aliasModel.(Model)

		if noteCmd != nil || tagCmd != nil || aliasCmd != nil {
			t.Fatal("edit handlers should not start commands without a selected session")
		}
		if note.noteInput.Focused() || tags.tagInput.Focused() || alias.aliasInput.Focused() {
			t.Fatal("edit inputs should remain unfocused without a selection")
		}
	})
}

func TestEditInputsPersistThroughKeyHandling(t *testing.T) {
	t.Run("save note", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		_ = m.noteInput.Focus("s1", "document the rollback")

		result, cmd := m.handleKey(enterKeyMsg())
		got := result.(Model)

		if got.cfg.SessionNotes["s1"] != "document the rollback" {
			t.Fatalf("saved note = %q", got.cfg.SessionNotes["s1"])
		}
		if _, ok := got.notesSet["s1"]; !ok {
			t.Fatal("note indicator should be set")
		}
		if got.noteInput.Focused() {
			t.Fatal("note input should blur after saving")
		}
		if got.statusInfo != "Note saved" || cmd == nil {
			t.Fatalf("note save status = %q, cmd nil = %v", got.statusInfo, cmd == nil)
		}
	})

	t.Run("delete note", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.cfg.SessionNotes = map[string]string{"s1": "obsolete"}
		m.notesSet["s1"] = struct{}{}
		_ = m.noteInput.Focus("s1", "")

		result, _ := m.handleKey(enterKeyMsg())
		got := result.(Model)

		if _, ok := got.cfg.SessionNotes["s1"]; ok {
			t.Fatal("empty note should remove the saved note")
		}
		if _, ok := got.notesSet["s1"]; ok {
			t.Fatal("empty note should remove the note indicator")
		}
	})

	t.Run("save normalized tags", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		_ = m.tagInput.Focus("s1", "backend, urgent, backend")

		result, cmd := m.handleKey(enterKeyMsg())
		got := result.(Model)
		tags := got.cfg.TagsFor("s1")

		if len(tags) != 2 || tags[0] != "backend" || tags[1] != "urgent" {
			t.Fatalf("saved tags = %v, want [backend urgent]", tags)
		}
		if _, ok := got.tagsSet["s1"]; !ok {
			t.Fatal("tag indicator should be set")
		}
		if got.statusInfo != "Tags saved" || cmd == nil {
			t.Fatalf("tag save status = %q, cmd nil = %v", got.statusInfo, cmd == nil)
		}
	})

	t.Run("save alias", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		_ = m.aliasInput.Focus("s1", "Auth follow-up")

		result, cmd := m.handleKey(enterKeyMsg())
		got := result.(Model)

		if got.cfg.AliasFor("s1") != "auth" {
			t.Fatalf("saved alias = %q", got.cfg.AliasFor("s1"))
		}
		if got.statusInfo != "Alias saved" || cmd == nil {
			t.Fatalf("alias save status = %q, cmd nil = %v", got.statusInfo, cmd == nil)
		}
	})
}

func TestExportFlowWritesMarkdownAndReportsResults(t *testing.T) {
	store := openBehavioralFlowStore(t)

	t.Run("single session", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.store = store
		m.sessionList.SetSessions([]data.Session{{ID: "sess-000"}})
		m.sessionList.ToggleSelected()

		_, cmd := m.handleExport()
		raw := cmd()
		msg, ok := raw.(exportDoneMsg)
		if !ok {
			t.Fatalf("export command returned %T", raw)
		}
		if msg.err != nil || len(msg.paths) != 1 {
			t.Fatalf("export result paths = %v, err = %v", msg.paths, msg.err)
		}
		content, err := os.ReadFile(msg.paths[0])
		if err != nil {
			t.Fatalf("reading exported markdown: %v", err)
		}
		if !strings.Contains(string(content), "# Session: Session 0 summary") ||
			!strings.Contains(string(content), "Question 0-0") {
			t.Fatalf("exported markdown missing session behavior:\n%s", content)
		}

		result, clearCmd := m.Update(msg)
		got := result.(Model)
		if got.statusInfo != "Exported: sess-000.md" || clearCmd == nil {
			t.Fatalf("export status = %q, cmd nil = %v", got.statusInfo, clearCmd == nil)
		}
	})

	t.Run("multiple sessions", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.store = store
		m.sessionList.SetSessions([]data.Session{{ID: "sess-000"}, {ID: "sess-001"}})
		m.sessionList.ToggleSelected()
		m.sessionList.MoveDown()
		m.sessionList.ToggleSelected()

		_, cmd := m.handleExport()
		msg := cmd().(exportDoneMsg)
		if msg.err != nil || len(msg.paths) != 2 {
			t.Fatalf("multi export paths = %v, err = %v", msg.paths, msg.err)
		}

		result, _ := m.Update(msg)
		if got := result.(Model).statusInfo; got != "Exported 2 sessions" {
			t.Fatalf("multi export status = %q", got)
		}
	})

	t.Run("missing session", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.store = store
		m.sessionList.SetSessions([]data.Session{{ID: "missing-session"}})
		m.sessionList.ToggleSelected()

		_, cmd := m.handleExport()
		msg := cmd().(exportDoneMsg)
		if msg.err != nil || len(msg.paths) != 0 {
			t.Fatalf("missing export paths = %v, err = %v", msg.paths, msg.err)
		}

		result, clearCmd := m.Update(msg)
		got := result.(Model)
		if got.statusErr != "export: no sessions exported" || clearCmd == nil {
			t.Fatalf("missing export status = %q, cmd nil = %v", got.statusErr, clearCmd == nil)
		}
	})

	t.Run("explicit error", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		result, clearCmd := m.Update(exportDoneMsg{err: errTest})
		got := result.(Model)
		if got.statusErr != "export: test error" || clearCmd == nil {
			t.Fatalf("export error status = %q, cmd nil = %v", got.statusErr, clearCmd == nil)
		}
	})
}

func TestCompareFlowLoadsSessionsAndOpensView(t *testing.T) {
	store := openBehavioralFlowStore(t)

	t.Run("success", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.store = store
		m.sessionList.SetSessions([]data.Session{{ID: "sess-000"}, {ID: "sess-001"}})
		m.sessionList.ToggleSelected()
		m.sessionList.MoveDown()
		m.sessionList.ToggleSelected()

		_, cmd := m.handleCompare()
		msg := cmd().(compareDetailMsg)
		if msg.err != nil {
			t.Fatalf("compare command: %v", msg.err)
		}
		if msg.left.Session.ID != "sess-000" || msg.right.Session.ID != "sess-001" {
			t.Fatalf("compare IDs = %q and %q", msg.left.Session.ID, msg.right.Session.ID)
		}

		result, nextCmd := m.Update(msg)
		got := result.(Model)
		if nextCmd != nil || got.state != stateCompareView {
			t.Fatalf("compare state = %v, cmd nil = %v", got.state, nextCmd == nil)
		}
		if view := got.View().Content; !strings.Contains(view, "Compare Sessions") ||
			!strings.Contains(view, "sess-000") {
			t.Fatalf("compare view missing loaded sessions:\n%s", view)
		}
	})

	t.Run("wrong selection count", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.store = store
		m.sessionList.SetSessions([]data.Session{{ID: "sess-000"}})
		m.sessionList.ToggleSelected()

		result, cmd := m.handleCompare()
		got := result.(Model)
		if got.statusInfo != "Select exactly 2 sessions to compare (space to toggle)" || cmd == nil {
			t.Fatalf("selection hint = %q, cmd nil = %v", got.statusInfo, cmd == nil)
		}
	})

	t.Run("load failure", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.store = store
		m.sessionList.SetSessions([]data.Session{{ID: "missing"}, {ID: "sess-001"}})
		m.sessionList.ToggleSelected()
		m.sessionList.MoveDown()
		m.sessionList.ToggleSelected()

		_, cmd := m.handleCompare()
		msg := cmd().(compareDetailMsg)
		if msg.err == nil {
			t.Fatal("compare should report a missing selected session")
		}

		result, clearCmd := m.Update(msg)
		got := result.(Model)
		if !strings.Contains(got.statusErr, "compare: loading session missing") || clearCmd == nil {
			t.Fatalf("compare error = %q, cmd nil = %v", got.statusErr, clearCmd == nil)
		}
	})
}

func TestAsyncStoreCommandsReturnBehavioralResults(t *testing.T) {
	store := openBehavioralFlowStore(t)
	m := newBehavioralFlowModel(t)
	m.store = store

	flat := m.loadSessionsCmd()().(sessionsLoadedMsg)
	if len(flat.sessions) != 5 || flat.sessions[0].ID == "" {
		t.Fatalf("flat sessions = %#v", flat.sessions)
	}

	m.pivot = pivotFolder
	folderGroups := m.loadSessionsCmd()().(groupsLoadedMsg)
	if len(folderGroups.groups) != 3 ||
		folderGroups.groups[0].Label > folderGroups.groups[len(folderGroups.groups)-1].Label {
		t.Fatalf("folder groups are not populated and ascending: %#v", folderGroups.groups)
	}

	m.pivot = pivotDate
	m.sort.Order = data.Ascending
	dateGroups := m.loadSessionsCmd()().(groupsLoadedMsg)
	if len(dateGroups.groups) == 0 ||
		dateGroups.groups[0].Label > dateGroups.groups[len(dateGroups.groups)-1].Label {
		t.Fatalf("ascending date groups = %#v", dateGroups.groups)
	}
	for _, group := range dateGroups.groups {
		for i := 1; i < len(group.Sessions); i++ {
			if group.Sessions[i-1].LastActiveAt > group.Sessions[i].LastActiveAt {
				t.Fatalf("ascending sessions in date group %q = %#v", group.Label, group.Sessions)
			}
		}
	}

	m.sort.Order = data.Descending
	dateGroups = m.loadSessionsCmd()().(groupsLoadedMsg)
	if dateGroups.groups[0].Label < dateGroups.groups[len(dateGroups.groups)-1].Label {
		t.Fatalf("descending date groups = %#v", dateGroups.groups)
	}
	for _, group := range dateGroups.groups {
		for i := 1; i < len(group.Sessions); i++ {
			if group.Sessions[i-1].LastActiveAt < group.Sessions[i].LastActiveAt {
				t.Fatalf("descending sessions in date group %q = %#v", group.Label, group.Sessions)
			}
		}
	}

	m.pivot = pivotNone
	m.filter.Query = "Question 4-1"
	deep := m.deepSearchCmd(17)().(deepSearchResultMsg)
	if deep.version != 17 || len(deep.sessions) != 1 ||
		deep.sessions[0].ID != "sess-004" {
		t.Fatalf("deep search version = %d, sessions = %#v", deep.version, deep.sessions)
	}

	m.pivot = pivotRepo
	deepGrouped := m.deepSearchCmd(18)().(deepSearchResultMsg)
	if deepGrouped.version != 18 || len(deepGrouped.groups) != 1 ||
		len(deepGrouped.groups[0].Sessions) != 1 ||
		deepGrouped.groups[0].Sessions[0].ID != "sess-004" {
		t.Fatalf("deep grouped version = %d, groups = %#v", deepGrouped.version, deepGrouped.groups)
	}

	m.pivot = pivotNone
	m.filter.Query = ""
	m.showPreview = true
	m.sessionList.SetSessions(flat.sessions)
	m.gitStatusMap = map[string]platform.GitStatus{
		"sess-001": {IsRepo: true, Repository: "detected/repository"},
	}
	detail := m.loadSelectedDetailCmd()().(sessionDetailMsg)
	if detail.detail.Session.ID != "sess-000" || len(detail.detail.Turns) != 2 {
		t.Fatalf("selected detail = %#v", detail.detail)
	}
	if len(detail.related) != 4 {
		t.Fatalf("related sessions = %d, want 4", len(detail.related))
	}
	foundDetectedRepository := false
	for _, item := range detail.related {
		if item.ID == "sess-001" && item.DisplayRepository == "detected/repository" {
			foundDetectedRepository = true
		}
	}
	if !foundDetectedRepository {
		t.Fatalf("related sessions did not use detected repository: %#v", detail.related)
	}

	filterData := loadFilterDataCmd(store)().(filterDataMsg)
	if len(filterData.folders) != 3 {
		t.Fatalf("folders = %v, want three unique folders", filterData.folders)
	}
}
