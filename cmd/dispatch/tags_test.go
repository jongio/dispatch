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

func withTagsList(t *testing.T, fn func(data.FilterOptions) ([]data.Session, error)) {
	t.Helper()
	prev := tagsListSessionsFn
	tagsListSessionsFn = fn
	t.Cleanup(func() { tagsListSessionsFn = prev })
}

func taggedSessions() []data.Session {
	return []data.Session{
		{ID: "a"},
		{ID: "b"},
		{ID: "c"},
		{ID: "d"}, // untagged
	}
}

func taggedConfig() *config.Config {
	cfg := config.Default()
	cfg.SessionTags = map[string][]string{
		"a": {"work", "urgent"},
		"b": {"work"},
		"c": {"personal"},
		// "z" is an orphan: tagged but not in the session store.
		"z": {"work", "stale"},
	}
	return cfg
}

func TestBuildTagsReportCounts(t *testing.T) {
	report := buildTagsReport(taggedConfig().SessionTags, taggedSessions())

	if report.TaggedSessions != 3 {
		t.Fatalf("TaggedSessions = %d, want 3", report.TaggedSessions)
	}
	if report.TotalTags != 3 {
		t.Fatalf("TotalTags = %d, want 3 (work, urgent, personal)", report.TotalTags)
	}

	// work=2 (a, b), personal=1 (c), urgent=1 (a). Orphan "z" is ignored.
	want := []tagCount{
		{Tag: "work", Count: 2},
		{Tag: "personal", Count: 1},
		{Tag: "urgent", Count: 1},
	}
	if len(report.Tags) != len(want) {
		t.Fatalf("Tags len = %d, want %d: %+v", len(report.Tags), len(want), report.Tags)
	}
	for i, w := range want {
		if report.Tags[i] != w {
			t.Errorf("Tags[%d] = %+v, want %+v", i, report.Tags[i], w)
		}
	}
}

func TestBuildTagsReportEmpty(t *testing.T) {
	report := buildTagsReport(map[string][]string{}, taggedSessions())
	if report.TotalTags != 0 || report.TaggedSessions != 0 {
		t.Fatalf("empty report = %+v, want zero counts", report)
	}
	if report.Tags == nil {
		t.Error("Tags should be non-nil so JSON renders [] not null")
	}
}

func TestParseTagsArgs(t *testing.T) {
	opts, err := parseTagsArgs([]string{"tags", "--json"})
	if err != nil {
		t.Fatalf("parseTagsArgs: %v", err)
	}
	if !opts.json {
		t.Error("--json not parsed")
	}

	opts, err = parseTagsArgs([]string{"tags"})
	if err != nil {
		t.Fatalf("parseTagsArgs bare: %v", err)
	}
	if opts.json {
		t.Error("json should default to false")
	}
}

func TestParseTagsArgsErrors(t *testing.T) {
	if _, err := parseTagsArgs([]string{"tags", "--nope"}); err == nil {
		t.Error("expected error for unknown flag")
	}
	if _, err := parseTagsArgs([]string{"tags", "work"}); err == nil {
		t.Error("expected error for positional argument")
	}
}

func TestRunTagsText(t *testing.T) {
	withConfigSeams(t, taggedConfig())
	withTagsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return taggedSessions(), nil
	})

	var buf bytes.Buffer
	if err := runTags(&buf, []string{"tags"}); err != nil {
		t.Fatalf("runTags: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Tags:     3", "3 tagged", "work", "personal", "urgent"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "stale") {
		t.Errorf("orphan tag should not appear:\n%s", out)
	}
}

func TestRunTagsEmptyText(t *testing.T) {
	cfg := config.Default()
	cfg.SessionTags = map[string][]string{}
	withConfigSeams(t, cfg)
	withTagsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return taggedSessions(), nil
	})

	var buf bytes.Buffer
	if err := runTags(&buf, []string{"tags"}); err != nil {
		t.Fatalf("runTags: %v", err)
	}
	if !strings.Contains(buf.String(), "No tags found.") {
		t.Errorf("expected empty notice, got:\n%s", buf.String())
	}
}

func TestRunTagsJSON(t *testing.T) {
	withConfigSeams(t, taggedConfig())
	withTagsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return taggedSessions(), nil
	})

	var buf bytes.Buffer
	if err := runTags(&buf, []string{"tags", "--json"}); err != nil {
		t.Fatalf("runTags json: %v", err)
	}

	var report tagsReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if report.TotalTags != 3 || report.TaggedSessions != 3 {
		t.Fatalf("json report = %+v, want 3 tags / 3 sessions", report)
	}
	if len(report.Tags) == 0 || report.Tags[0].Tag != "work" || report.Tags[0].Count != 2 {
		t.Fatalf("first tag = %+v, want work=2", report.Tags)
	}
}

func TestRunTagsListError(t *testing.T) {
	withConfigSeams(t, taggedConfig())
	withTagsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return nil, errors.New("boom")
	})
	if err := runTags(&bytes.Buffer{}, []string{"tags"}); err == nil {
		t.Error("expected error from session loader to propagate")
	}
}

func TestHandleArgsTags(t *testing.T) {
	withConfigSeams(t, taggedConfig())
	withTagsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return taggedSessions(), nil
	})

	done, _, _, err := handleArgs([]string{"tags"}, &bytes.Buffer{}, nil)
	if err != nil {
		t.Fatalf("handleArgs tags: %v", err)
	}
	if !done {
		t.Error("handleArgs should report done for tags")
	}
}
