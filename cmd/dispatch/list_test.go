package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
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

func withListSelector(t *testing.T, fn func(io.Writer, []data.Session) (data.Session, bool, error)) {
	t.Helper()
	previous := listSelectFn
	listSelectFn = fn
	t.Cleanup(func() { listSelectFn = previous })
}

func TestParseListArgsDefaultsToWorkingDirectory(t *testing.T) {
	folder := t.TempDir()
	withListGetwd(t, func() (string, error) { return folder, nil })

	opts, err := parseListArgs([]string{"resume"})
	if err != nil {
		t.Fatalf("parseListArgs returned error: %v", err)
	}
	if opts.filter.Folder != folder {
		t.Errorf("Folder = %q, want %q", opts.filter.Folder, folder)
	}
}

func TestParseListArgsDefaultsToTable(t *testing.T) {
	withListGetwd(t, func() (string, error) { return t.TempDir(), nil })

	opts, err := parseListArgs([]string{"resume"})
	if err != nil {
		t.Fatalf("parseListArgs returned error: %v", err)
	}
	if opts.format != searchFormatTable {
		t.Errorf("format = %q, want %q", opts.format, searchFormatTable)
	}
	if opts.formatExplicit {
		t.Error("default table format should not bypass the interactive picker")
	}
	if opts.limit != 0 {
		t.Errorf("limit = %d, want all matching sessions", opts.limit)
	}
}

func TestParseListArgsExplicitFolder(t *testing.T) {
	folder := t.TempDir()

	opts, err := parseListArgs([]string{"resume", "--folder", folder})
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

	opts, err := parseListArgs([]string{"resume", "--folder", "project"})
	if err != nil {
		t.Fatalf("parseListArgs returned error: %v", err)
	}
	if opts.filter.Folder != folder {
		t.Errorf("Folder = %q, want %q", opts.filter.Folder, folder)
	}
}

func TestParseListArgsPositionalQuery(t *testing.T) {
	withListGetwd(t, func() (string, error) { return t.TempDir(), nil })

	opts, err := parseListArgs([]string{"resume", "auth", "bug"})
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
		{name: "json", args: []string{"resume", "--json"}, want: searchFormatJSON},
		{name: "jsonl", args: []string{"resume", "--jsonl"}, want: searchFormatJSONL},
		{name: "csv", args: []string{"resume", "--csv"}, want: searchFormatCSV},
		{name: "ids", args: []string{"resume", "--ids"}, want: searchFormatIDs},
		{name: "paths", args: []string{"resume", "--paths"}, want: searchFormatPaths},
		{name: "commands", args: []string{"resume", "--commands"}, want: searchFormatCommands},
		{name: "format", args: []string{"resume", "--format", "json"}, want: searchFormatJSON},
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
		"resume", "auth", "--folder", folder, "--repo", "jongio/dispatch",
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
	_, err := parseListArgs([]string{"resume", "--folder", missing})
	if err == nil || !strings.Contains(err.Error(), "folder") || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want actionable not-exist folder error", err)
	}
}

func TestParseListArgsRejectsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.txt")
	if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
		t.Fatalf("os.WriteFile returned error: %v", err)
	}

	_, err := parseListArgs([]string{"resume", "--folder", path})
	if err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("error = %v, want not-a-directory error", err)
	}
}

func TestParseListArgsWorkingDirectoryError(t *testing.T) {
	wantErr := errors.New("working directory unavailable")
	withListGetwd(t, func() (string, error) { return "", wantErr })

	_, err := parseListArgs([]string{"resume"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestParseListArgsDemoMode(t *testing.T) {
	withListDemoMode(t, true)

	opts, err := parseListArgs([]string{"resume"})
	if err != nil {
		t.Fatalf("parseListArgs returned error: %v", err)
	}
	if opts.filter.Folder != "" {
		t.Errorf("Folder = %q, want no folder filter for demo data", opts.filter.Folder)
	}

	synthetic := `D:\code\project-alpha\api`
	opts, err = parseListArgs([]string{"resume", "--folder", synthetic})
	if err != nil {
		t.Fatalf("parseListArgs rejected synthetic demo folder: %v", err)
	}
	if opts.filter.Folder != synthetic {
		t.Errorf("Folder = %q, want %q", opts.filter.Folder, synthetic)
	}
}

func TestRunListSelectsAndResumesSession(t *testing.T) {
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
	withListSelector(t, func(_ io.Writer, sessions []data.Session) (data.Session, bool, error) {
		if len(sessions) != 1 || sessions[0].ID != "1234567890abcdef" {
			t.Fatalf("selector sessions = %#v", sessions)
		}
		return sessions[0], true, nil
	})
	previousLoad := openLoadConfigFn
	previousLaunch := openInteractiveLaunchFn
	openLoadConfigFn = func() (*config.Config, error) {
		return &config.Config{LaunchMode: config.LaunchModeInPlace}, nil
	}
	var launched *data.Session
	openInteractiveLaunchFn = func(_ io.Writer, _ *config.Config, session *data.Session, mode string) error {
		launched = session
		if mode != config.LaunchModeInPlace {
			t.Errorf("mode = %q, want inplace", mode)
		}
		return nil
	}
	t.Cleanup(func() {
		openLoadConfigFn = previousLoad
		openInteractiveLaunchFn = previousLaunch
	})

	var output bytes.Buffer
	if err := runList(&output, []string{"resume", "auth"}); err != nil {
		t.Fatalf("runList returned error: %v", err)
	}
	if captured.Folder != folder || captured.Query != "auth" {
		t.Errorf("captured filter = %+v, want folder %q and query auth", captured, folder)
	}
	if launched == nil || launched.ID != "1234567890abcdef" {
		t.Errorf("launched session = %#v", launched)
	}
}

func TestRunListCSVUsesSafeRenderer(t *testing.T) {
	withListGetwd(t, func() (string, error) { return t.TempDir(), nil })
	withSearchList(t, func(data.FilterOptions, data.SortOptions, int) ([]data.Session, error) {
		return []data.Session{{ID: "session-id", Summary: "=SUM(1,1)"}}, nil
	})

	var output bytes.Buffer
	if err := runList(&output, []string{"resume", "--csv"}); err != nil {
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
	withListSelector(t, func(io.Writer, []data.Session) (data.Session, bool, error) {
		return data.Session{}, false, nil
	})

	if err := runList(nil, []string{"resume"}); err != nil {
		t.Fatalf("runList returned error with nil writer: %v", err)
	}
}

func TestRunListNilWriterWithNoSessions(t *testing.T) {
	withListGetwd(t, func() (string, error) { return t.TempDir(), nil })
	withSearchList(t, func(data.FilterOptions, data.SortOptions, int) ([]data.Session, error) {
		return nil, nil
	})

	if err := runList(nil, []string{"resume"}); err != nil {
		t.Fatalf("runList returned error with nil writer and no sessions: %v", err)
	}
}

func TestListPickerShowsFullIDAndSelectsWithEnter(t *testing.T) {
	session := data.Session{
		ID:         "12345678-90ab-cdef-1234-567890abcdef",
		Repository: "jongio/dispatch",
		Branch:     "feature/list",
		Summary:    "Fix auth",
	}
	model := newListPickerModel([]data.Session{session})

	view := model.View().Content
	if !strings.Contains(view, session.ID) {
		t.Errorf("picker view omitted full session ID:\n%s", view)
	}
	for _, header := range []string{"SESSION ID", "REPOSITORY", "BRANCH", "SUMMARY"} {
		if !strings.Contains(view, header) {
			t.Errorf("picker view omitted table header %q:\n%s", header, view)
		}
	}
	idIndex := strings.Index(view, "SESSION ID")
	summaryIndex := strings.Index(view, "SUMMARY")
	repoIndex := strings.Index(view, "REPOSITORY")
	branchIndex := strings.Index(view, "BRANCH")
	if idIndex >= summaryIndex || summaryIndex >= repoIndex || repoIndex >= branchIndex {
		t.Errorf("picker headers are not ordered ID, summary, repository, branch:\n%s", view)
	}
	for _, detail := range []string{session.Repository, "featu…", session.Summary} {
		if !strings.Contains(view, detail) {
			t.Errorf("picker view omitted %q:\n%s", detail, view)
		}
	}

	result, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should quit the picker")
	}
	selected := result.(listPickerModel)
	if !selected.selected || selected.cursor != 0 {
		t.Errorf("selected model = %#v", selected)
	}
}

func TestListPickerFitsNarrowTerminalWithWideText(t *testing.T) {
	model := newListPickerModel([]data.Session{{
		ID:         "12345678-90ab-cdef-1234-567890abcdef",
		Summary:    "\x1b[31m修复登录问题 🚀 with a very long summary\x1b[0m",
		Repository: "jongio/a-very-long-repository-name",
		Branch:     "feature/a-very-long-branch-name",
	}})
	model.width = 80

	view := model.View().Content
	if strings.Contains(view, "\x1b") {
		t.Fatalf("picker leaked terminal control sequences:\n%s", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if width := ansi.StringWidth(line); width > model.width {
			t.Errorf("rendered line width = %d, want <= %d:\n%s", width, model.width, line)
		}
	}
}

func TestDefaultOpenInteractiveLaunchBlocksMissingWorkspace(t *testing.T) {
	session := data.Session{ID: "session-id", Cwd: filepath.Join(t.TempDir(), "missing")}
	err := defaultOpenInteractiveLaunch(io.Discard, &config.Config{}, &session, config.LaunchModeTab)
	if err == nil || !strings.Contains(err.Error(), "workspace folder no longer exists") {
		t.Fatalf("error = %v, want missing-workspace error", err)
	}
}

func TestDefaultOpenInteractiveLaunchPromptsForMultipleShells(t *testing.T) {
	folder := t.TempDir()
	session := data.Session{ID: "session-id", Cwd: folder}
	shells := []platform.ShellInfo{
		{Name: "first", Path: "first.exe"},
		{Name: "second", Path: "second.exe"},
	}
	previousDetect := openDetectShellsFn
	previousSelect := openSelectShellFn
	previousLaunch := openLaunchFn
	openDetectShellsFn = func() []platform.ShellInfo { return shells }
	openSelectShellFn = func(_ io.Writer, got []platform.ShellInfo) (platform.ShellInfo, bool, error) {
		if len(got) != len(shells) {
			t.Fatalf("shells = %#v, want %#v", got, shells)
		}
		return platform.ShellInfo{}, false, nil
	}
	openLaunchFn = func(io.Writer, *config.Config, *data.Session, string) error {
		t.Fatal("direct launcher should not run when shell selection is cancelled")
		return nil
	}
	t.Cleanup(func() {
		openDetectShellsFn = previousDetect
		openSelectShellFn = previousSelect
		openLaunchFn = previousLaunch
	})

	if err := defaultOpenInteractiveLaunch(io.Discard, &config.Config{}, &session, config.LaunchModeTab); err != nil {
		t.Fatalf("defaultOpenInteractiveLaunch returned error: %v", err)
	}
}

func TestDefaultOpenInteractiveLaunchUsesDetectedSingleShell(t *testing.T) {
	folder := t.TempDir()
	session := data.Session{ID: "session-id", Cwd: folder}
	shell := platform.ShellInfo{Name: "only", Path: "only.exe"}

	previousDetect := openDetectShellsFn
	previousLaunch := openLaunchFn
	previousLaunchWithShell := openLaunchWithShellFn
	openDetectShellsFn = func() []platform.ShellInfo { return []platform.ShellInfo{shell} }
	openLaunchFn = func(io.Writer, *config.Config, *data.Session, string) error {
		t.Fatal("single detected shell should not be re-resolved")
		return nil
	}
	var launched platform.ShellInfo
	openLaunchWithShellFn = func(_ io.Writer, _ *config.Config, _ *data.Session, _ string, got platform.ShellInfo) error {
		launched = got
		return nil
	}
	t.Cleanup(func() {
		openDetectShellsFn = previousDetect
		openLaunchFn = previousLaunch
		openLaunchWithShellFn = previousLaunchWithShell
	})

	if err := defaultOpenInteractiveLaunch(io.Discard, &config.Config{}, &session, config.LaunchModeTab); err != nil {
		t.Fatalf("defaultOpenInteractiveLaunch returned error: %v", err)
	}
	if launched.Name != shell.Name || launched.Path != shell.Path {
		t.Errorf("launched shell = %#v, want %#v", launched, shell)
	}
}

func TestListPickerNavigation(t *testing.T) {
	model := newListPickerModel([]data.Session{{ID: "first"}, {ID: "second"}})
	result, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	moved := result.(listPickerModel)
	if moved.cursor != 1 {
		t.Errorf("cursor = %d, want 1", moved.cursor)
	}
	result, _ = moved.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	cancelled := result.(listPickerModel)
	if cancelled.selected || !cancelled.quitting {
		t.Errorf("cancelled model = %#v", cancelled)
	}
}

func TestListPickerShowsMoreSessions(t *testing.T) {
	model := newListPickerModel(make([]data.Session, 101))
	model.height = 200

	view := model.View().Content
	for _, want := range []string{
		"Select a session (50 of 101)",
		"Show 50 more sessions",
		"51 remaining",
		"m more",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("picker view omitted %q:\n%s", want, view)
		}
	}

	model.cursor = model.visible
	result, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("Enter on show-more row should keep the picker open")
	}
	expanded := result.(listPickerModel)
	if expanded.visible != 100 || expanded.selected || expanded.quitting {
		t.Errorf("expanded model = %#v", expanded)
	}

	result, _ = expanded.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	all := result.(listPickerModel)
	if all.visible != 101 || all.hasMore() {
		t.Errorf("fully expanded model = %#v", all)
	}
	if strings.Contains(all.View().Content, "more sessions") {
		t.Errorf("fully expanded picker still offers more sessions:\n%s", all.View().Content)
	}
}
