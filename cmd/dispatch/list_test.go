package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jongio/dispatch/internal/data"
)

func withListGetwd(t *testing.T, fn func() (string, error)) {
	t.Helper()
	previous := listGetwdFn
	listGetwdFn = fn
	t.Cleanup(func() { listGetwdFn = previous })
}

func withListDemoMode(t *testing.T, enabled bool) {
	t.Helper()
	previous := listDemoModeFn
	listDemoModeFn = func() bool { return enabled }
	t.Cleanup(func() { listDemoModeFn = previous })
}

func TestParseListArgsDefaultsToWorkingDirectory(t *testing.T) {
	folder := t.TempDir()
	withListGetwd(t, func() (string, error) { return folder, nil })

	opts, err := parseListArgs([]string{"list"})
	if err != nil {
		t.Fatalf("parseListArgs returned error: %v", err)
	}
	if opts.filter.Folder != folder {
		t.Errorf("Folder = %q, want %q", opts.filter.Folder, folder)
	}
}

func TestParseListArgsDefaultsToTable(t *testing.T) {
	withListGetwd(t, func() (string, error) { return t.TempDir(), nil })

	opts, err := parseListArgs([]string{"list"})
	if err != nil {
		t.Fatalf("parseListArgs returned error: %v", err)
	}
	if opts.format != searchFormatTable {
		t.Errorf("format = %q, want %q", opts.format, searchFormatTable)
	}
}

func TestParseListArgsExplicitFolder(t *testing.T) {
	folder := t.TempDir()

	opts, err := parseListArgs([]string{"list", "--folder", folder})
	if err != nil {
		t.Fatalf("parseListArgs returned error: %v", err)
	}
	if opts.filter.Folder != folder {
		t.Errorf("Folder = %q, want %q", opts.filter.Folder, folder)
	}
}

func TestParseListArgsRelativeFolder(t *testing.T) {
	base := t.TempDir()
	folder := filepath.Join(base, "project")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatalf("create project folder: %v", err)
	}
	t.Chdir(base)

	opts, err := parseListArgs([]string{"list", "--folder", "project"})
	if err != nil {
		t.Fatalf("parseListArgs returned error: %v", err)
	}
	if opts.filter.Folder != folder {
		t.Errorf("Folder = %q, want %q", opts.filter.Folder, folder)
	}
}

func TestParseListArgsPositionalQuery(t *testing.T) {
	withListGetwd(t, func() (string, error) { return t.TempDir(), nil })

	opts, err := parseListArgs([]string{"list", "auth", "bug"})
	if err != nil {
		t.Fatalf("parseListArgs returned error: %v", err)
	}
	if opts.filter.Query != "auth bug" {
		t.Errorf("Query = %q, want %q", opts.filter.Query, "auth bug")
	}
}

func TestParseListArgsOutputOverrides(t *testing.T) {
	withListGetwd(t, func() (string, error) { return t.TempDir(), nil })

	cases := []struct {
		name string
		args []string
		want searchOutputFormat
	}{
		{name: "json", args: []string{"list", "--json"}, want: searchFormatJSON},
		{name: "jsonl", args: []string{"list", "--jsonl"}, want: searchFormatJSONL},
		{name: "csv", args: []string{"list", "--csv"}, want: searchFormatCSV},
		{name: "ids", args: []string{"list", "--ids"}, want: searchFormatIDs},
		{name: "paths", args: []string{"list", "--paths"}, want: searchFormatPaths},
		{name: "commands", args: []string{"list", "--commands"}, want: searchFormatCommands},
		{name: "format", args: []string{"list", "--format", "json"}, want: searchFormatJSON},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseListArgs(tc.args)
			if err != nil {
				t.Fatalf("parseListArgs returned error: %v", err)
			}
			if opts.format != tc.want {
				t.Errorf("format = %q, want %q", opts.format, tc.want)
			}
		})
	}
}

func TestParseListArgsReusesSearchFilters(t *testing.T) {
	folder := t.TempDir()
	opts, err := parseListArgs([]string{
		"list", "auth", "--folder", folder, "--repo", "jongio/dispatch",
		"--branch", "main", "--host", "cli", "--tag", "work", "--deep",
		"--limit", "10", "--sort", "turns", "--order", "asc",
		"--since", "2026-01-01", "--until", "2026-12-31",
	})
	if err != nil {
		t.Fatalf("parseListArgs returned error: %v", err)
	}
	if opts.filter.Query != "auth" ||
		opts.filter.Repository != "jongio/dispatch" ||
		opts.filter.Branch != "main" ||
		opts.filter.HostType != "cli" ||
		!opts.filter.DeepSearch ||
		opts.tag != "work" ||
		opts.limit != 10 ||
		opts.sort.Field != data.SortByTurns ||
		opts.sort.Order != data.Ascending {
		t.Errorf("unexpected options: %+v", opts)
	}
	if opts.filter.Since == nil || opts.filter.Until == nil {
		t.Fatal("expected since and until filters")
	}
	if !opts.filter.Since.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Since = %v, want 2026-01-01", opts.filter.Since)
	}
}

func TestParseListArgsMissingFolder(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	_, err := parseListArgs([]string{"list", "--folder", missing})
	if err == nil || !strings.Contains(err.Error(), "folder") || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want actionable not-exist folder error", err)
	}
}

func TestParseListArgsRejectsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.txt")
	if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
		t.Fatalf("os.WriteFile returned error: %v", err)
	}

	_, err := parseListArgs([]string{"list", "--folder", path})
	if err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("error = %v, want not-a-directory error", err)
	}
}

func TestParseListArgsWorkingDirectoryError(t *testing.T) {
	wantErr := errors.New("working directory unavailable")
	withListGetwd(t, func() (string, error) { return "", wantErr })

	_, err := parseListArgs([]string{"list"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestParseListArgsDemoMode(t *testing.T) {
	withListDemoMode(t, true)

	opts, err := parseListArgs([]string{"list"})
	if err != nil {
		t.Fatalf("parseListArgs returned error: %v", err)
	}
	if opts.filter.Folder != "" {
		t.Errorf("Folder = %q, want no folder filter for demo data", opts.filter.Folder)
	}

	synthetic := `D:\code\project-alpha\api`
	opts, err = parseListArgs([]string{"list", "--folder", synthetic})
	if err != nil {
		t.Fatalf("parseListArgs rejected synthetic demo folder: %v", err)
	}
	if opts.filter.Folder != synthetic {
		t.Errorf("Folder = %q, want %q", opts.filter.Folder, synthetic)
	}
}

func TestRunListReusesSearchPipeline(t *testing.T) {
	folder := t.TempDir()
	withListGetwd(t, func() (string, error) { return folder, nil })

	var captured data.FilterOptions
	withSearchList(t, func(filter data.FilterOptions, _ data.SortOptions, _ int) ([]data.Session, error) {
		captured = filter
		return []data.Session{{
			ID:         "1234567890abcdef",
			Summary:    "Fix auth",
			Repository: "jongio/dispatch",
			Branch:     "main",
			UpdatedAt:  "2026-08-13T12:00:00Z",
		}}, nil
	})

	var output bytes.Buffer
	if err := runList(&output, []string{"list", "auth"}); err != nil {
		t.Fatalf("runList returned error: %v", err)
	}
	if captured.Folder != folder || captured.Query != "auth" {
		t.Errorf("captured filter = %+v, want folder %q and query auth", captured, folder)
	}
	if !strings.Contains(output.String(), "ID") || !strings.Contains(output.String(), "1234567890ab") {
		t.Errorf("output did not use table renderer:\n%s", output.String())
	}
}

func TestRunListCSVUsesSafeRenderer(t *testing.T) {
	withListGetwd(t, func() (string, error) { return t.TempDir(), nil })
	withSearchList(t, func(data.FilterOptions, data.SortOptions, int) ([]data.Session, error) {
		return []data.Session{{ID: "session-id", Summary: "=SUM(1,1)"}}, nil
	})

	var output bytes.Buffer
	if err := runList(&output, []string{"list", "--csv"}); err != nil {
		t.Fatalf("runList returned error: %v", err)
	}
	if !strings.Contains(output.String(), `"'=SUM(1,1)"`) {
		t.Errorf("list CSV did not sanitize spreadsheet formula:\n%s", output.String())
	}
}

func TestRunListNilWriter(t *testing.T) {
	withListGetwd(t, func() (string, error) { return t.TempDir(), nil })
	withSearchList(t, func(data.FilterOptions, data.SortOptions, int) ([]data.Session, error) {
		return []data.Session{{ID: "session-id"}}, nil
	})

	if err := runList(nil, []string{"list"}); err != nil {
		t.Fatalf("runList returned error with nil writer: %v", err)
	}
}
