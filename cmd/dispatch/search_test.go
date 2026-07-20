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
func withSearchList(t *testing.T, fn func(data.FilterOptions, data.SortOptions, int) ([]data.Session, error)) {
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
		"--sort", "turns",
		"--order=asc",
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
	if opts.sort.Field != data.SortByTurns {
		t.Errorf("sort field = %q, want %q", opts.sort.Field, data.SortByTurns)
	}
	if opts.sort.Order != data.Ascending {
		t.Errorf("sort order = %q, want %q", opts.sort.Order, data.Ascending)
	}
	if opts.format != searchFormatJSON {
		t.Errorf("format = %q, want json", opts.format)
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
	if opts.sort != defaultSearchSort() {
		t.Errorf("sort = %+v, want %+v", opts.sort, defaultSearchSort())
	}
	if opts.filter.Query != "" {
		t.Errorf("Query = %q, want empty", opts.filter.Query)
	}
	if opts.format != searchFormatJSON {
		t.Errorf("format = %q, want json", opts.format)
	}
}

func TestParseSearchArgsIDFormats(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "ids shortcut", args: []string{"search", "--ids"}},
		{name: "format ids separate", args: []string{"search", "--format", "ids"}},
		{name: "format ids inline", args: []string{"search", "--format=ids"}},
		{name: "table shortcut", args: []string{"search", "--table"}},
		{name: "format table separate", args: []string{"search", "--format", "table"}},
		{name: "format table inline", args: []string{"search", "--format=table"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseSearchArgs(tc.args)
			if err != nil {
				t.Fatalf("parseSearchArgs returned error: %v", err)
			}
			want := searchFormatIDs
			if strings.Contains(strings.Join(tc.args, " "), "table") {
				want = searchFormatTable
			}
			if opts.format != want {
				t.Errorf("format = %q, want %s", opts.format, want)
			}
		})
	}
}

func TestParseSearchArgsErrors(t *testing.T) {
	cases := [][]string{
		{"search", "--bogus"},
		{"search", "--since", "not-a-date"},
		{"search", "--limit", "-3"},
		{"search", "--limit", "abc"},
		{"search", "--repo"},
		{"search", "--format"},
		{"search", "--format", "yaml"},
		{"search", "--sort"},
		{"search", "--sort", "attention"},
		{"search", "--order", "sideways"},
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
	var gotSort data.SortOptions
	var gotLimit int
	withSearchList(t, func(f data.FilterOptions, sort data.SortOptions, limit int) ([]data.Session, error) {
		gotFilter = f
		gotSort = sort
		gotLimit = limit
		return sessions, nil
	})

	var buf bytes.Buffer
	if err := runSearch(&buf, []string{"search", "auth", "--limit", "5", "--sort", "name", "--order", "asc"}); err != nil {
		t.Fatalf("runSearch returned error: %v", err)
	}

	if gotFilter.Query != "auth" {
		t.Errorf("filter.Query = %q, want auth", gotFilter.Query)
	}
	if gotLimit != 5 {
		t.Errorf("limit = %d, want 5", gotLimit)
	}
	if gotSort.Field != data.SortByName || gotSort.Order != data.Ascending {
		t.Errorf("sort = %+v, want name asc", gotSort)
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

func TestRunSearchIDsOutput(t *testing.T) {
	sessions := []data.Session{
		{ID: "session-a"},
		{ID: "session-b"},
	}
	withSearchList(t, func(data.FilterOptions, data.SortOptions, int) ([]data.Session, error) {
		return sessions, nil
	})

	var buf bytes.Buffer
	if err := runSearch(&buf, []string{"search", "--ids"}); err != nil {
		t.Fatalf("runSearch returned error: %v", err)
	}

	if got, want := buf.String(), "session-a\nsession-b\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestRunSearchIDsNoMatchesIsEmpty(t *testing.T) {
	withSearchList(t, func(data.FilterOptions, data.SortOptions, int) ([]data.Session, error) {
		return nil, nil
	})

	var buf bytes.Buffer
	if err := runSearch(&buf, []string{"search", "--format", "ids"}); err != nil {
		t.Fatalf("runSearch returned error: %v", err)
	}
	if got := buf.String(); got != "" {
		t.Errorf("output = %q, want empty", got)
	}
}

func TestRunSearchTableOutput(t *testing.T) {
	sessions := []data.Session{
		{
			ID:           "0123456789abcdef",
			Summary:      "fix auth bug",
			Repository:   "jongio/dispatch",
			Branch:       "main",
			LastActiveAt: "2026-01-06T10:00:00Z",
			TurnCount:    5,
			FileCount:    3,
		},
		{
			ID:        "short",
			Summary:   "  ",
			UpdatedAt: "2026-01-05T09:00:00Z",
		},
	}
	withSearchList(t, func(data.FilterOptions, data.SortOptions, int) ([]data.Session, error) {
		return sessions, nil
	})

	var buf bytes.Buffer
	if err := runSearch(&buf, []string{"search", "--table"}); err != nil {
		t.Fatalf("runSearch returned error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"ID", "LAST ACTIVE", "REPO", "BRANCH", "TURNS", "FILES", "SUMMARY",
		"0123456789ab", "2026-01-06", "jongio/dispatch", "main", "fix auth bug",
		"short", "2026-01-05",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("table output missing %q:\n%s", want, got)
		}
	}
}

func TestRunSearchTableEmptyPrintsHeader(t *testing.T) {
	withSearchList(t, func(data.FilterOptions, data.SortOptions, int) ([]data.Session, error) {
		return nil, nil
	})

	var buf bytes.Buffer
	if err := runSearch(&buf, []string{"search", "--format", "table"}); err != nil {
		t.Fatalf("runSearch returned error: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "ID") || strings.Contains(got, "session-a") {
		t.Errorf("unexpected table output:\n%s", got)
	}
}

func TestRunSearchEmptyIsEmptyArray(t *testing.T) {
	withSearchList(t, func(data.FilterOptions, data.SortOptions, int) ([]data.Session, error) {
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
	withSearchList(t, func(_ data.FilterOptions, _ data.SortOptions, limit int) ([]data.Session, error) {
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
	withSearchList(t, func(data.FilterOptions, data.SortOptions, int) ([]data.Session, error) {
		return nil, errors.New("store boom")
	})

	var buf bytes.Buffer
	err := runSearch(&buf, []string{"search"})
	if err == nil {
		t.Fatal("runSearch returned nil error, want store error")
	}
}
