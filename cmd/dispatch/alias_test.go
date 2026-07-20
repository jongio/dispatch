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

// withAliasSeams swaps the config and session-resolution seams that runAlias
// depends on and restores them via t.Cleanup. sessions are matched by exact ID.
func withAliasSeams(t *testing.T, cfg *config.Config, sessions []data.Session) {
	t.Helper()

	prevLoad := configLoadFn
	prevSave := configSaveFn
	configLoadFn = func() (*config.Config, error) { return cfg, nil }
	configSaveFn = func(*config.Config) error { return nil }
	t.Cleanup(func() { configLoadFn = prevLoad; configSaveFn = prevSave })

	prevGet := openGetSessionFn
	openGetSessionFn = func(id string) (*data.Session, error) {
		for i := range sessions {
			if sessions[i].ID == id {
				return &sessions[i], nil
			}
		}
		return nil, nil
	}
	t.Cleanup(func() { openGetSessionFn = prevGet })
}

func TestParseAliasArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantSession string
		wantName    string
		wantClear   bool
		wantRemove  bool
		wantJSON    bool
		wantErr     bool
	}{
		{name: "set", args: []string{"alias", "ses-1", "review"}, wantSession: "ses-1", wantName: "review"},
		{name: "set with json", args: []string{"alias", "ses-1", "review", "--json"}, wantSession: "ses-1", wantName: "review", wantJSON: true},
		{name: "clear", args: []string{"alias", "ses-1", "--clear"}, wantSession: "ses-1", wantClear: true},
		{name: "remove spaced", args: []string{"alias", "--remove", "review"}, wantName: "review", wantRemove: true},
		{name: "remove equals", args: []string{"alias", "--remove=review"}, wantName: "review", wantRemove: true},
		{name: "no args", args: []string{"alias"}, wantErr: true},
		{name: "id without name", args: []string{"alias", "ses-1"}, wantErr: true},
		{name: "remove without name", args: []string{"alias", "--remove"}, wantErr: true},
		{name: "remove with session", args: []string{"alias", "--remove", "review", "ses-1"}, wantErr: true},
		{name: "clear with extra name", args: []string{"alias", "ses-1", "--clear", "extra"}, wantErr: true},
		{name: "too many positionals", args: []string{"alias", "ses-1", "a", "b"}, wantErr: true},
		{name: "unknown flag", args: []string{"alias", "ses-1", "--bogus"}, wantErr: true},
		{name: "remove and clear", args: []string{"alias", "--remove", "review", "--clear"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionArg, name, clearFlag, remove, jsonOut, err := parseAliasArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (session=%q name=%q)", sessionArg, name)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sessionArg != tt.wantSession {
				t.Errorf("sessionArg = %q, want %q", sessionArg, tt.wantSession)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if clearFlag != tt.wantClear {
				t.Errorf("clear = %v, want %v", clearFlag, tt.wantClear)
			}
			if remove != tt.wantRemove {
				t.Errorf("remove = %v, want %v", remove, tt.wantRemove)
			}
			if jsonOut != tt.wantJSON {
				t.Errorf("jsonOut = %v, want %v", jsonOut, tt.wantJSON)
			}
		})
	}
}

func TestRunAlias_Set(t *testing.T) {
	cfg := &config.Config{}
	withAliasSeams(t, cfg, []data.Session{{ID: "ses-1"}})

	var buf bytes.Buffer
	if err := runAlias(&buf, []string{"alias", "ses-1", "Review"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.AliasFor("ses-1"); got != "review" {
		t.Errorf("alias = %q, want %q (normalized lowercase)", got, "review")
	}
	if !strings.Contains(buf.String(), "Set alias") {
		t.Errorf("output = %q, want it to mention Set alias", buf.String())
	}
}

func TestRunAlias_Reassign(t *testing.T) {
	cfg := &config.Config{SessionAliases: map[string]string{"ses-1": "old"}}
	withAliasSeams(t, cfg, []data.Session{{ID: "ses-1"}})

	if err := runAlias(&bytes.Buffer{}, []string{"alias", "ses-1", "new"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.AliasFor("ses-1"); got != "new" {
		t.Errorf("alias = %q, want %q", got, "new")
	}
}

func TestRunAlias_Clear(t *testing.T) {
	cfg := &config.Config{SessionAliases: map[string]string{"ses-1": "review"}}
	withAliasSeams(t, cfg, []data.Session{{ID: "ses-1"}})

	var buf bytes.Buffer
	if err := runAlias(&buf, []string{"alias", "ses-1", "--clear"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.AliasFor("ses-1"); got != "" {
		t.Errorf("alias = %q, want empty after clear", got)
	}
	if !strings.Contains(buf.String(), "Cleared alias") {
		t.Errorf("output = %q, want it to mention Cleared alias", buf.String())
	}
}

func TestRunAlias_RemoveByName(t *testing.T) {
	cfg := &config.Config{SessionAliases: map[string]string{"ses-1": "review"}}
	withAliasSeams(t, cfg, []data.Session{{ID: "ses-1"}})

	if err := runAlias(&bytes.Buffer{}, []string{"alias", "--remove", "review"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.AliasFor("ses-1"); got != "" {
		t.Errorf("alias = %q, want empty after remove", got)
	}
}

func TestRunAlias_RemoveUnknownName(t *testing.T) {
	cfg := &config.Config{SessionAliases: map[string]string{"ses-1": "review"}}
	withAliasSeams(t, cfg, []data.Session{{ID: "ses-1"}})

	err := runAlias(&bytes.Buffer{}, []string{"alias", "--remove", "nope"})
	if err == nil {
		t.Fatal("expected error removing an unknown alias, got nil")
	}
}

func TestRunAlias_DuplicateRejected(t *testing.T) {
	cfg := &config.Config{SessionAliases: map[string]string{"ses-1": "review"}}
	withAliasSeams(t, cfg, []data.Session{{ID: "ses-1"}, {ID: "ses-2"}})

	err := runAlias(&bytes.Buffer{}, []string{"alias", "ses-2", "review"})
	if err == nil {
		t.Fatal("expected error assigning an alias already in use, got nil")
	}
	if got := cfg.AliasFor("ses-2"); got != "" {
		t.Errorf("ses-2 alias = %q, want empty (assignment should fail)", got)
	}
}

func TestRunAlias_UnknownID(t *testing.T) {
	cfg := &config.Config{}
	withAliasSeams(t, cfg, []data.Session{{ID: "ses-1"}})

	err := runAlias(&bytes.Buffer{}, []string{"alias", "missing", "review"})
	if err == nil {
		t.Fatal("expected error for unknown session ID, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to mention not found", err.Error())
	}
}

func TestRunAlias_JSON(t *testing.T) {
	cfg := &config.Config{}
	withAliasSeams(t, cfg, []data.Session{{ID: "ses-1"}})

	var buf bytes.Buffer
	if err := runAlias(&buf, []string{"alias", "ses-1", "review", "--json"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var res aliasResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if res.ID != "ses-1" || res.Alias != "review" {
		t.Errorf("result = %+v, want {ID:ses-1 Alias:review}", res)
	}
}

func TestRunAlias_LoadError(t *testing.T) {
	prevLoad := configLoadFn
	configLoadFn = func() (*config.Config, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { configLoadFn = prevLoad })

	err := runAlias(&bytes.Buffer{}, []string{"alias", "ses-1", "review"})
	if err == nil {
		t.Fatal("expected error when config load fails, got nil")
	}
}
