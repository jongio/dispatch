package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
)

func withNotesList(t *testing.T, fn func(data.FilterOptions) ([]data.Session, error)) {
	t.Helper()
	prev := notesListSessionsFn
	notesListSessionsFn = fn
	t.Cleanup(func() { notesListSessionsFn = prev })
}

func notedSessions() []data.Session {
	return []data.Session{
		{ID: "b", Summary: "Build command"},
		{ID: "a", Summary: "Auth fix"},
		{ID: "c", Summary: "No note"},
	}
}

func notedConfig() *config.Config {
	cfg := config.Default()
	cfg.SessionNotes = map[string]string{
		"a": "follow up",
		"b": "ready to ship",
		"z": "orphan note",
	}
	return cfg
}

func TestBuildNotesReport(t *testing.T) {
	report := buildNotesReport(notedConfig(), notedSessions())
	if report.TotalNotes != 2 {
		t.Fatalf("TotalNotes = %d, want 2", report.TotalNotes)
	}
	if len(report.Notes) != 2 || report.Notes[0].ID != "a" || report.Notes[1].ID != "b" {
		t.Fatalf("notes sorted by ID without orphans = %+v", report.Notes)
	}
}

func TestRunNotesListText(t *testing.T) {
	withConfigSeams(t, notedConfig())
	withNotesList(t, func(data.FilterOptions) ([]data.Session, error) { return notedSessions(), nil })

	var buf bytes.Buffer
	if err := runNotes(&buf, []string{"notes"}); err != nil {
		t.Fatalf("runNotes: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Dispatch notes", "Notes: 2", "a", "follow up", "ready to ship"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "orphan note") {
		t.Fatalf("orphan note should not appear:\n%s", out)
	}
}

func TestRunNotesListJSON(t *testing.T) {
	withConfigSeams(t, notedConfig())
	withNotesList(t, func(data.FilterOptions) ([]data.Session, error) { return notedSessions(), nil })

	var buf bytes.Buffer
	if err := runNotes(&buf, []string{"notes", "--json"}); err != nil {
		t.Fatalf("runNotes json: %v", err)
	}
	var report notesReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if report.TotalNotes != 2 || len(report.Notes) != 2 {
		t.Fatalf("report = %+v, want 2 notes", report)
	}
}

func TestRunNotesGetSetClear(t *testing.T) {
	cfg := withConfigSeams(t, config.Default())

	var buf bytes.Buffer
	if err := runNotes(&buf, []string{"notes", "set", "ses-1", "needs", "review"}); err != nil {
		t.Fatalf("set note: %v", err)
	}
	if cfg.SessionNotes["ses-1"] != "needs review" {
		t.Fatalf("stored note = %q", cfg.SessionNotes["ses-1"])
	}

	buf.Reset()
	if err := runNotes(&buf, []string{"notes", "get", "ses-1"}); err != nil {
		t.Fatalf("get note: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "needs review" {
		t.Fatalf("get output = %q", buf.String())
	}

	buf.Reset()
	if err := runNotes(&buf, []string{"notes", "clear", "ses-1"}); err != nil {
		t.Fatalf("clear note: %v", err)
	}
	if _, ok := cfg.SessionNotes["ses-1"]; ok {
		t.Fatal("note should be removed")
	}
}

func TestRunNotesErrors(t *testing.T) {
	withConfigSeams(t, config.Default())
	withNotesList(t, func(data.FilterOptions) ([]data.Session, error) { return nil, errors.New("boom") })
	for _, args := range [][]string{
		{"notes", "bogus"},
		{"notes", "get"},
		{"notes", "set", "ses-1"},
		{"notes", "clear"},
		{"notes", "list", "extra"},
	} {
		if err := runNotes(&bytes.Buffer{}, args); err == nil {
			t.Fatalf("expected error for args %v", args)
		}
	}
}

func TestHandleArgsNotes(t *testing.T) {
	withConfigSeams(t, notedConfig())
	withNotesList(t, func(data.FilterOptions) ([]data.Session, error) { return notedSessions(), nil })

	done, _, _, err := handleArgs([]string{"notes"}, &bytes.Buffer{}, nil)
	if err != nil {
		t.Fatalf("handleArgs notes: %v", err)
	}
	if !done {
		t.Fatal("handleArgs should report done for notes")
	}
}
