package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/update"
)

func withTagSeams(t *testing.T, cfg *config.Config, sessions []data.Session) {
	t.Helper()

	prevLoad := configLoadFn
	prevSave := configSaveFn
	configLoadFn = func() (*config.Config, error) { return cfg, nil }
	configSaveFn = func(c *config.Config) error { return nil }
	t.Cleanup(func() { configLoadFn = prevLoad; configSaveFn = prevSave })

	prevList := tagListSessionsFn
	tagListSessionsFn = func(data.FilterOptions) ([]data.Session, error) {
		return sessions, nil
	}
	t.Cleanup(func() { tagListSessionsFn = prevList })
}

func TestRunTag_ListTags(t *testing.T) {
	cfg := &config.Config{
		SessionTags: map[string][]string{
			"ses-1": {"api", "work"},
		},
	}
	sessions := []data.Session{{ID: "ses-1"}}
	withTagSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runTag(&buf, []string{"tag", "ses-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("api")) {
		t.Errorf("expected 'api' in output, got:\n%s", out)
	}
	if !bytes.Contains([]byte(out), []byte("work")) {
		t.Errorf("expected 'work' in output, got:\n%s", out)
	}
}

func TestRunTag_ListEmpty(t *testing.T) {
	cfg := &config.Config{}
	sessions := []data.Session{{ID: "ses-1"}}
	withTagSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runTag(&buf, []string{"tag", "ses-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("no tags")) {
		t.Errorf("expected 'no tags' message, got:\n%s", out)
	}
}

func TestRunTag_Add(t *testing.T) {
	cfg := &config.Config{
		SessionTags: map[string][]string{
			"ses-1": {"existing"},
		},
	}
	sessions := []data.Session{{ID: "ses-1"}}
	withTagSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runTag(&buf, []string{"tag", "ses-1", "--add", "new,another"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tags := cfg.SessionTags["ses-1"]
	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %v", tags)
	}
}

func TestRunTag_AddDeduplicates(t *testing.T) {
	cfg := &config.Config{
		SessionTags: map[string][]string{
			"ses-1": {"api"},
		},
	}
	sessions := []data.Session{{ID: "ses-1"}}
	withTagSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runTag(&buf, []string{"tag", "ses-1", "--add", "api,work"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tags := cfg.SessionTags["ses-1"]
	if len(tags) != 2 {
		t.Errorf("expected 2 tags (no duplicate), got %v", tags)
	}
}

func TestRunTag_Remove(t *testing.T) {
	cfg := &config.Config{
		SessionTags: map[string][]string{
			"ses-1": {"api", "work", "temp"},
		},
	}
	sessions := []data.Session{{ID: "ses-1"}}
	withTagSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runTag(&buf, []string{"tag", "ses-1", "--remove", "temp"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tags := cfg.SessionTags["ses-1"]
	if len(tags) != 2 {
		t.Errorf("expected 2 tags after removal, got %v", tags)
	}
}

func TestRunTag_RemoveAll(t *testing.T) {
	cfg := &config.Config{
		SessionTags: map[string][]string{
			"ses-1": {"only"},
		},
	}
	sessions := []data.Session{{ID: "ses-1"}}
	withTagSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runTag(&buf, []string{"tag", "ses-1", "--remove", "only"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cfg.SessionTags["ses-1"]; ok {
		t.Error("expected session tag entry to be deleted when all tags removed")
	}
}

func TestRunTag_Set(t *testing.T) {
	cfg := &config.Config{
		SessionTags: map[string][]string{
			"ses-1": {"old1", "old2"},
		},
	}
	sessions := []data.Session{{ID: "ses-1"}}
	withTagSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runTag(&buf, []string{"tag", "ses-1", "--set", "new1,new2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tags := cfg.SessionTags["ses-1"]
	if len(tags) != 2 || tags[0] != "new1" || tags[1] != "new2" {
		t.Errorf("expected [new1, new2], got %v", tags)
	}
}

func TestRunTag_JSON(t *testing.T) {
	cfg := &config.Config{
		SessionTags: map[string][]string{
			"ses-1": {"api"},
		},
	}
	sessions := []data.Session{{ID: "ses-1"}}
	withTagSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runTag(&buf, []string{"tag", "ses-1", "--json"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got tagResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.ID != "ses-1" {
		t.Errorf("id = %q, want ses-1", got.ID)
	}
}

func TestRunTag_AliasResolution(t *testing.T) {
	cfg := &config.Config{
		SessionAliases: map[string]string{
			"ses-1": "myalias",
		},
		SessionTags: map[string][]string{
			"ses-1": {"existing"},
		},
	}
	sessions := []data.Session{{ID: "ses-1"}}
	withTagSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runTag(&buf, []string{"tag", "myalias"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("existing")) {
		t.Errorf("expected tags via alias resolution, got:\n%s", out)
	}
}

func TestRunTag_UnknownSession(t *testing.T) {
	cfg := &config.Config{}
	sessions := []data.Session{{ID: "ses-1"}}
	withTagSeams(t, cfg, sessions)

	err := runTag(&bytes.Buffer{}, []string{"tag", "missing"})
	if err == nil {
		t.Fatal("expected error for unknown session")
	}
}

func TestRunTag_NoID(t *testing.T) {
	err := runTag(&bytes.Buffer{}, []string{"tag"})
	if err == nil {
		t.Fatal("expected error when no session ID given")
	}
}

func TestRunTag_UnknownFlag(t *testing.T) {
	_, err := parseTagArgs([]string{"tag", "ses-1", "--nope"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestRunTag_ConflictingActions(t *testing.T) {
	_, err := parseTagArgs([]string{"tag", "ses-1", "--add", "a", "--remove", "b"})
	if err == nil {
		t.Fatal("expected error for conflicting actions")
	}
}

func TestRunTag_ConfigLoadError(t *testing.T) {
	prev := configLoadFn
	configLoadFn = func() (*config.Config, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { configLoadFn = prev })

	err := runTag(&bytes.Buffer{}, []string{"tag", "ses-1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleArgs_Tag(t *testing.T) {
	cfg := &config.Config{}
	sessions := []data.Session{{ID: "ses-1"}}
	withTagSeams(t, cfg, sessions)

	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	done, _, _, err := handleArgs([]string{"tag", "ses-1"}, &bytes.Buffer{}, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true")
	}
}
