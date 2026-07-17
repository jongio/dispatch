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

// withAliasesSeams swaps the config loader and session lister for the duration
// of a test.
func withAliasesSeams(t *testing.T, cfg *config.Config, sessions []data.Session) {
	t.Helper()

	prevLoad := configLoadFn
	configLoadFn = func() (*config.Config, error) { return cfg, nil }
	t.Cleanup(func() { configLoadFn = prevLoad })

	prevList := aliasesListSessionsFn
	aliasesListSessionsFn = func(data.FilterOptions) ([]data.Session, error) {
		return sessions, nil
	}
	t.Cleanup(func() { aliasesListSessionsFn = prevList })
}

func TestRunAliases_Text(t *testing.T) {
	cfg := &config.Config{
		SessionAliases: map[string]string{
			"ses-1": "auth",
			"ses-2": "deploy",
		},
	}
	sessions := []data.Session{
		{ID: "ses-1", Summary: "Add auth", Repository: "jongio/dispatch"},
		{ID: "ses-2", Summary: "Fix deploy", Repository: "jongio/life"},
	}
	withAliasesSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runAliases(&buf, []string{"aliases"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"auth", "deploy", "ses-1", "ses-2", "Add auth", "Fix deploy"} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestRunAliases_JSON(t *testing.T) {
	cfg := &config.Config{
		SessionAliases: map[string]string{
			"ses-1": "auth",
		},
	}
	sessions := []data.Session{
		{ID: "ses-1", Summary: "Add auth", Repository: "jongio/dispatch"},
	}
	withAliasesSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runAliases(&buf, []string{"aliases", "--json"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got aliasesReport
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.TotalAliases != 1 {
		t.Errorf("total_aliases = %d, want 1", got.TotalAliases)
	}
	if got.Aliases[0].Alias != "auth" {
		t.Errorf("alias = %q, want auth", got.Aliases[0].Alias)
	}
}

func TestRunAliases_Empty(t *testing.T) {
	cfg := &config.Config{}
	withAliasesSeams(t, cfg, nil)

	var buf bytes.Buffer
	if err := runAliases(&buf, []string{"aliases"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("No aliases configured")) {
		t.Errorf("expected empty-state message, got:\n%s", out)
	}
}

func TestRunAliases_Orphaned(t *testing.T) {
	cfg := &config.Config{
		SessionAliases: map[string]string{
			"ses-gone": "old",
			"ses-1":    "auth",
		},
	}
	sessions := []data.Session{
		{ID: "ses-1", Summary: "Add auth"},
	}
	withAliasesSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runAliases(&buf, []string{"aliases"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("orphaned")) {
		t.Errorf("expected orphaned marker, got:\n%s", out)
	}
}

func TestRunAliases_OrphanedJSON(t *testing.T) {
	cfg := &config.Config{
		SessionAliases: map[string]string{
			"ses-gone": "old",
		},
	}
	withAliasesSeams(t, cfg, nil)

	var buf bytes.Buffer
	if err := runAliases(&buf, []string{"aliases", "--json"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got aliasesReport
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Orphaned != 1 {
		t.Errorf("orphaned = %d, want 1", got.Orphaned)
	}
	if !got.Aliases[0].Orphaned {
		t.Error("expected alias to be marked orphaned")
	}
}

func TestParseAliasesArgs_UnknownFlag(t *testing.T) {
	_, err := parseAliasesArgs([]string{"aliases", "--nope"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestParseAliasesArgs_Positional(t *testing.T) {
	_, err := parseAliasesArgs([]string{"aliases", "extra"})
	if err == nil {
		t.Fatal("expected error for positional argument")
	}
}

func TestRunAliases_ConfigLoadError(t *testing.T) {
	prev := configLoadFn
	configLoadFn = func() (*config.Config, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { configLoadFn = prev })

	err := runAliases(&bytes.Buffer{}, []string{"aliases"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleArgs_Aliases(t *testing.T) {
	cfg := &config.Config{}
	withAliasesSeams(t, cfg, nil)

	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	done, _, _, err := handleArgs([]string{"aliases"}, &bytes.Buffer{}, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true")
	}
}
