package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jongio/dispatch/internal/data"
)

// withSearchList swaps the search command's session loader for a test double
// and restores it afterward, matching the seam helper used by the stats tests.
func withSearchList(t *testing.T, fn func(data.FilterOptions, int) ([]data.Session, error)) {
	t.Helper()
	prev := searchListSessionsFn
	searchListSessionsFn = fn
	t.Cleanup(func() { searchListSessionsFn = prev })
}

func TestParseSearchArgsQueryAndFilters(t *testing.T) {
	opts, err := parseSearchArgs([]string{
		"search", "--json", "auth", "bug",
		"--repo", "jongio/dispatch",
		"--branch=main",
		"--folder", "/code",
		"--host", "cli",
		"--deep",
		"--limit", "10",
		"--since", "2026-01-01",
		"--until", "2026-12-31",
	})
	if err != nil {
		t.Fatalf("parseSearchArgs returned error: %v", err)
	}
	if opts.filter.Query != "auth bug" {
		t.Errorf("Query = %q, want %q", opts.filter.Query, "auth bug")
	}
	if opts.filter.Repository != "jongio/dispatch" {
		t.Errorf("Repository = %q, want jongio/dispatch", opts.filter.Repository)
	}
	if opts.filter.Branch != "main" {
		t.Errorf("Branch = %q, want main", opts.filter.Branch)
	}
	if opts.filter.Folder != "/code" {
		t.Errorf("Folder = %q, want /code", opts.filter.Folder)
	}
	if opts.filter.HostType != "cli" {
		t.Errorf("HostType = %q, want cli", opts.filter.HostType)
	}
	if !opts.filter.DeepSearch {
		t.Error("DeepSearch = false, want true")
	}
	if opts.limit != 10 {
		t.Errorf("limit = %d, want 10", opts.limit)
	}
	if opts.filter.Since == nil || !opts.filter.Since.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Since = %v, want 2026-01-01", opts.filter.Since)
	}
	if opts.filter.Until == nil {
		t.Error("Until = nil, want 2026-12-31")
	}
}

func TestParseSearchArgsDefaultLimit(t *testing.T) {
	opts, err := parseSearchArgs([]string{"search"})
	if err != nil {
		t.Fatalf("parseSearchArgs returned error: %v", err)
	}
	if opts.limit != searchDefaultLimit {
		t.Errorf("limit = %d, want default %d", opts.limit, searchDefaultLimit)
	}
	if opts.filter.Query != "" {
		t.Errorf("Query = %q, want empty", opts.filter.Query)
	}
}

func TestParseSearchArgsErrors(t *testing.T) {
	cases := [][]string{
		{"search", "--bogus"},
		{"search", "--since", "not-a-date"},
		{"search", "--limit", "-3"},
		{"search", "--limit", "abc"},
		{"search", "--repo"},
	}
	for _, args := range cases {
		if _, err := parseSearchArgs(args); err == nil {
			t.Errorf("parseSearchArgs(%v) = nil error, want error", args)
		}
	}
}

func TestRunSearchJSONOutput(t *testing.T) {
	sessions := []data.Session{
		{
			ID:         "a",
			Summary:    "fix auth bug",
			Cwd:        "/code/app",
			Repository: "jongio/dispatch",
			Branch:     "main",
			CreatedAt:  "2026-01-05T10:00:00Z",
			UpdatedAt:  "2026-01-06T10:00:00Z",
			TurnCount:  5,
			FileCount:  3,
		},
	}
	var gotFilter data.FilterOptions
	var gotLimit int
	withSearchList(t, func(f data.FilterOptions, limit int) ([]data.Session, error) {
		gotFilter = f
		gotLimit = limit
		return sessions, nil
	})

	var buf bytes.Buffer
	if err := runSearch(&buf, []string{"search", "auth", "--limit", "5"}); err != nil {
		t.Fatalf("runSearch returned error: %v", err)
	}

	if gotFilter.Query != "auth" {
		t.Errorf("filter.Query = %q, want auth", gotFilter.Query)
	}
	if gotLimit != 5 {
		t.Errorf("limit = %d, want 5", gotLimit)
	}

	var out []searchSession
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(out) != 1 {
		t.Fatalf("got %d results, want 1", len(out))
	}
	if out[0].ID != "a" || out[0].Summary != "fix auth bug" || out[0].TurnCount != 5 || out[0].FileCount != 3 {
		t.Errorf("unexpected result: %+v", out[0])
	}
}

func TestRunSearchEmptyIsEmptyArray(t *testing.T) {
	withSearchList(t, func(data.FilterOptions, int) ([]data.Session, error) {
		return nil, nil
	})

	var buf bytes.Buffer
	if err := runSearch(&buf, []string{"search", "nomatch"}); err != nil {
		t.Fatalf("runSearch returned error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "[]" {
		t.Errorf("output = %q, want []", got)
	}
}

func TestRunSearchNoLimitUsesCeiling(t *testing.T) {
	var gotLimit int
	withSearchList(t, func(_ data.FilterOptions, limit int) ([]data.Session, error) {
		gotLimit = limit
		return nil, nil
	})

	var buf bytes.Buffer
	if err := runSearch(&buf, []string{"search", "--limit", "0"}); err != nil {
		t.Fatalf("runSearch returned error: %v", err)
	}
	if gotLimit != searchAllLimit {
		t.Errorf("limit = %d, want ceiling %d", gotLimit, searchAllLimit)
	}
}

func TestRunSearchPropagatesStoreError(t *testing.T) {
	withSearchList(t, func(data.FilterOptions, int) ([]data.Session, error) {
		return nil, errors.New("store boom")
	})

	var buf bytes.Buffer
	err := runSearch(&buf, []string{"search"})
	if err == nil {
		t.Fatal("runSearch returned nil error, want store error")
	}
}
