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

func withPruneSeams(t *testing.T, cfg *config.Config, sessions []data.Session) {
	t.Helper()

	prevLoad := configLoadFn
	prevSave := configSaveFn
	configLoadFn = func() (*config.Config, error) { return cfg, nil }
	configSaveFn = func(c *config.Config) error { return nil }
	t.Cleanup(func() { configLoadFn = prevLoad; configSaveFn = prevSave })

	prevList := pruneListSessionsFn
	pruneListSessionsFn = func(data.FilterOptions) ([]data.Session, error) {
		return sessions, nil
	}
	t.Cleanup(func() { pruneListSessionsFn = prevList })
}

func makePruneConfig() *config.Config {
	return &config.Config{
		SessionAliases: map[string]string{
			"live-1": "auth",
			"gone-1": "old",
		},
		SessionTags: map[string][]string{
			"live-1": {"work"},
			"gone-2": {"stale"},
		},
		SessionNotes: map[string]string{
			"live-1": "keep this",
			"gone-3": "remove this",
		},
		FavoriteSessions: []string{"live-1", "gone-4"},
		HiddenSessions:   []string{"gone-5"},
		SessionLaunches: map[string]config.SessionLaunch{
			"live-1": {Count: 5},
			"gone-6": {Count: 1},
		},
	}
}

func TestRunPrune_DryRun(t *testing.T) {
	cfg := makePruneConfig()
	sessions := []data.Session{{ID: "live-1"}}
	withPruneSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runPrune(&buf, []string{"prune"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("--apply")) {
		t.Errorf("expected --apply hint, got:\n%s", out)
	}
	// Verify config was NOT modified (dry run).
	if _, ok := cfg.SessionAliases["gone-1"]; !ok {
		t.Error("dry run should not remove entries")
	}
}

func TestRunPrune_Apply(t *testing.T) {
	cfg := makePruneConfig()
	sessions := []data.Session{{ID: "live-1"}}
	withPruneSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runPrune(&buf, []string{"prune", "--apply"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("Pruned")) {
		t.Errorf("expected 'Pruned' message, got:\n%s", out)
	}
	if _, ok := cfg.SessionAliases["gone-1"]; ok {
		t.Error("expected stale alias to be removed after --apply")
	}
	if _, ok := cfg.SessionAliases["live-1"]; !ok {
		t.Error("live alias should be kept")
	}
	if len(cfg.FavoriteSessions) != 1 || cfg.FavoriteSessions[0] != "live-1" {
		t.Errorf("favorites = %v, want [live-1]", cfg.FavoriteSessions)
	}
}

func TestRunPrune_JSON(t *testing.T) {
	cfg := makePruneConfig()
	sessions := []data.Session{{ID: "live-1"}}
	withPruneSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runPrune(&buf, []string{"prune", "--json"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got pruneReport
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.TotalStale == 0 {
		t.Error("expected stale entries")
	}
}

func TestRunPrune_NothingToPrune(t *testing.T) {
	cfg := &config.Config{
		SessionAliases: map[string]string{"live-1": "auth"},
	}
	sessions := []data.Session{{ID: "live-1"}}
	withPruneSeams(t, cfg, sessions)

	var buf bytes.Buffer
	if err := runPrune(&buf, []string{"prune"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("Nothing to prune")) {
		t.Errorf("expected nothing-to-prune message, got:\n%s", out)
	}
}

func TestRunPrune_EmptyConfig(t *testing.T) {
	cfg := &config.Config{}
	withPruneSeams(t, cfg, nil)

	var buf bytes.Buffer
	if err := runPrune(&buf, []string{"prune"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("Nothing to prune")) {
		t.Errorf("expected nothing-to-prune message, got:\n%s", out)
	}
}

func TestRunPrune_ConfigLoadError(t *testing.T) {
	prev := configLoadFn
	configLoadFn = func() (*config.Config, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { configLoadFn = prev })

	err := runPrune(&bytes.Buffer{}, []string{"prune"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParsePruneArgs_UnknownFlag(t *testing.T) {
	_, err := parsePruneArgs([]string{"prune", "--nope"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleArgs_Prune(t *testing.T) {
	cfg := &config.Config{}
	withPruneSeams(t, cfg, nil)

	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	done, _, _, err := handleArgs([]string{"prune"}, &bytes.Buffer{}, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true")
	}
}
