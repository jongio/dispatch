package tui

import (
	"testing"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
)

func TestParseSearchTokens_Tag(t *testing.T) {
	sf := ParseSearchTokens("tag:work")
	if sf.Tag != "work" {
		t.Fatalf("expected tag work, got %q", sf.Tag)
	}
	if !sf.HasTokens() {
		t.Fatal("expected HasTokens true")
	}
	if got := sf.TokenSummary(); got != "tag:work" {
		t.Fatalf("expected summary tag:work, got %q", got)
	}
}

func TestFilterTaggedSessions(t *testing.T) {
	m := newTestModel()
	m.cfg = config.Default()
	m.cfg.SessionTags = map[string][]string{
		"a": {"work"},
		"b": {"personal"},
	}
	sessions := []data.Session{{ID: "a"}, {ID: "b"}, {ID: "c"}}

	if got := m.filterTaggedSessions(sessions); len(got) != 3 {
		t.Fatalf("expected 3 sessions with no filter, got %d", len(got))
	}

	m.tagFilter = "work"
	got := m.filterTaggedSessions(sessions)
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("expected only session a, got %+v", got)
	}
}

func TestFilterTaggedGroups(t *testing.T) {
	m := newTestModel()
	m.cfg = config.Default()
	m.cfg.SessionTags = map[string][]string{"a": {"work"}}
	m.tagFilter = "work"
	groups := []data.SessionGroup{{
		Label:    "g",
		Sessions: []data.Session{{ID: "a"}, {ID: "b"}},
		Count:    2,
	}}
	got := m.filterTaggedGroups(groups)
	if len(got) != 1 || len(got[0].Sessions) != 1 || got[0].Sessions[0].ID != "a" {
		t.Fatalf("expected group with only session a, got %+v", got)
	}
}
