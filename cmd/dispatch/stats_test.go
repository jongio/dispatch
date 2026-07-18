package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

func withStatsList(t *testing.T, fn func(data.FilterOptions) ([]data.Session, error)) {
	t.Helper()
	prev := statsListSessionsFn
	statsListSessionsFn = fn
	t.Cleanup(func() { statsListSessionsFn = prev })
}

func sampleSessions() []data.Session {
	return []data.Session{
		{
			ID:           "a",
			Repository:   "jongio/dispatch",
			Branch:       "main",
			HostType:     "github",
			CreatedAt:    "2026-01-05T10:00:00Z",
			LastActiveAt: "2026-01-06T10:00:00Z",
			TurnCount:    5,
			FileCount:    3,
		},
		{
			ID:           "b",
			Repository:   "jongio/dispatch",
			Branch:       "feature",
			HostType:     "github",
			CreatedAt:    "2026-02-01T10:00:00Z",
			LastActiveAt: "2026-07-01T10:00:00Z",
			TurnCount:    10,
			FileCount:    2,
		},
		{
			ID:         "c",
			Repository: "",
			Branch:     "",
			HostType:   "",
			CreatedAt:  "2026-03-01T10:00:00Z",
			UpdatedAt:  "2026-03-02T10:00:00Z",
			TurnCount:  1,
			FileCount:  0,
		},
	}
}

func TestBuildStatsReportTotals(t *testing.T) {
	report := buildStatsReport(sampleSessions())

	if report.TotalSessions != 3 {
		t.Errorf("TotalSessions = %d, want 3", report.TotalSessions)
	}
	if report.TotalTurns != 16 {
		t.Errorf("TotalTurns = %d, want 16", report.TotalTurns)
	}
	if report.TotalFiles != 5 {
		t.Errorf("TotalFiles = %d, want 5", report.TotalFiles)
	}
	if report.Earliest != "2026-01-05" {
		t.Errorf("Earliest = %q, want 2026-01-05", report.Earliest)
	}
	if report.Latest != "2026-07-01" {
		t.Errorf("Latest = %q, want 2026-07-01", report.Latest)
	}
}

func TestBuildStatsReportGrouping(t *testing.T) {
	report := buildStatsReport(sampleSessions())

	if len(report.ByRepository) != 2 {
		t.Fatalf("ByRepository len = %d, want 2", len(report.ByRepository))
	}
	// Highest count first.
	if report.ByRepository[0].Label != "jongio/dispatch" || report.ByRepository[0].Count != 2 {
		t.Errorf("ByRepository[0] = %+v, want jongio/dispatch=2", report.ByRepository[0])
	}
	if report.ByRepository[1].Label != "(none)" || report.ByRepository[1].Count != 1 {
		t.Errorf("ByRepository[1] = %+v, want (none)=1", report.ByRepository[1])
	}

	var host string
	for _, e := range report.ByHostType {
		if e.Label == "(unknown)" {
			host = e.Label
		}
	}
	if host == "" {
		t.Errorf("expected an (unknown) host type bucket, got %+v", report.ByHostType)
	}
}

func TestBuildStatsReportEmpty(t *testing.T) {
	report := buildStatsReport(nil)
	if report.TotalSessions != 0 {
		t.Errorf("TotalSessions = %d, want 0", report.TotalSessions)
	}
	if report.Earliest != "" || report.Latest != "" {
		t.Errorf("expected empty range, got %q..%q", report.Earliest, report.Latest)
	}
	if report.ByRepository == nil || report.ByBranch == nil || report.ByHostType == nil {
		t.Errorf("group slices should be non-nil for JSON output")
	}
}

func TestParseStatsArgs(t *testing.T) {
	opts, err := parseStatsArgs([]string{"stats", "--json", "--repo", "jongio/dispatch", "--branch=main", "--since", "2026-01-01", "--until=2026-07-01", "--top", "2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.json {
		t.Error("expected json=true")
	}
	if opts.filter.Repository != "jongio/dispatch" {
		t.Errorf("Repository = %q", opts.filter.Repository)
	}
	if opts.filter.Branch != "main" {
		t.Errorf("Branch = %q", opts.filter.Branch)
	}
	if opts.filter.Since == nil || opts.filter.Until == nil {
		t.Fatal("expected Since and Until to be set")
	}
	if opts.filter.Since.Format("2006-01-02") != "2026-01-01" {
		t.Errorf("Since = %v", opts.filter.Since)
	}
	if opts.top != 2 {
		t.Errorf("top = %d, want 2", opts.top)
	}
}

func TestParseStatsArgsErrors(t *testing.T) {
	cases := [][]string{
		{"stats", "--repo"},              // missing value
		{"stats", "--since", "not-date"}, // bad date
		{"stats", "--bogus"},             // unknown flag
		{"stats", "extra"},               // positional
		{"stats", "--top"},               // missing value
		{"stats", "--top", "0"},          // not positive
		{"stats", "--top", "-1"},         // negative
		{"stats", "--top", "many"},       // not a number
	}
	for _, args := range cases {
		if _, err := parseStatsArgs(args); err == nil {
			t.Errorf("parseStatsArgs(%v) expected error, got nil", args)
		}
	}
}

func TestApplyStatsTopLimit(t *testing.T) {
	report := buildStatsReport(sampleSessions())
	applyStatsTopLimit(&report, 1)

	if len(report.ByRepository) != 1 || report.ByRepository[0].Label != "jongio/dispatch" {
		t.Fatalf("ByRepository = %+v, want only jongio/dispatch", report.ByRepository)
	}
	if len(report.ByBranch) != 1 {
		t.Fatalf("ByBranch len = %d, want 1", len(report.ByBranch))
	}
	if report.TotalSessions != 3 {
		t.Errorf("TotalSessions = %d, want unchanged total 3", report.TotalSessions)
	}
}

func TestRunStatsText(t *testing.T) {
	withStatsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return sampleSessions(), nil
	})

	var buf bytes.Buffer
	if err := runStats(&buf, []string{"stats"}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Sessions: 3", "Turns:    16", "Files:    5", "By repository", "jongio/dispatch", "By host type"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestRunStatsTextTop(t *testing.T) {
	withStatsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return sampleSessions(), nil
	})

	var buf bytes.Buffer
	if err := runStats(&buf, []string{"stats", "--top", "1"}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Sessions: 3") {
		t.Errorf("top should not change totals\n%s", out)
	}
	if strings.Contains(out, "feature") {
		t.Errorf("top 1 should hide lower branch rows\n%s", out)
	}
}

func TestRunStatsJSON(t *testing.T) {
	withStatsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return sampleSessions(), nil
	})

	var buf bytes.Buffer
	if err := runStats(&buf, []string{"stats", "--json"}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	var report statsReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if report.TotalSessions != 3 || report.TotalTurns != 16 {
		t.Errorf("unexpected report: %+v", report)
	}
}

func TestRunStatsJSONTop(t *testing.T) {
	withStatsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return sampleSessions(), nil
	})

	var buf bytes.Buffer
	if err := runStats(&buf, []string{"stats", "--json", "--top=1"}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	var report statsReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(report.ByRepository) != 1 || len(report.ByBranch) != 1 || len(report.ByHostType) != 1 {
		t.Errorf("top did not cap breakdown arrays: %+v", report)
	}
	if report.TotalSessions != 3 {
		t.Errorf("TotalSessions = %d, want 3", report.TotalSessions)
	}
}

func TestRunStatsPassesFilter(t *testing.T) {
	var got data.FilterOptions
	withStatsList(t, func(f data.FilterOptions) ([]data.Session, error) {
		got = f
		return nil, nil
	})

	if err := runStats(&bytes.Buffer{}, []string{"stats", "--repo", "jongio/dispatch"}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	if got.Repository != "jongio/dispatch" {
		t.Errorf("filter.Repository = %q, want jongio/dispatch", got.Repository)
	}
}

func TestRunStatsListError(t *testing.T) {
	withStatsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return nil, errors.New("boom")
	})
	if err := runStats(&bytes.Buffer{}, []string{"stats"}); err == nil {
		t.Error("expected error from list failure")
	}
}

func TestHandleArgsStats(t *testing.T) {
	withStatsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return sampleSessions(), nil
	})
	done, _, _, err := handleArgs([]string{"stats", "--json"}, &bytes.Buffer{}, nil)
	if !done {
		t.Error("expected done=true for stats")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseStatsArgs_CSV(t *testing.T) {
	opts, err := parseStatsArgs([]string{"stats", "--csv"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.csv {
		t.Error("expected csv=true")
	}
}

func TestParseStatsArgs_CSVAndJSONConflict(t *testing.T) {
	_, err := parseStatsArgs([]string{"stats", "--csv", "--json"})
	if err == nil {
		t.Fatal("expected error for --csv + --json conflict")
	}
	if !strings.Contains(err.Error(), "cannot be combined") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestRunStatsCSV(t *testing.T) {
	withStatsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return sampleSessions(), nil
	})

	var buf bytes.Buffer
	if err := runStats(&buf, []string{"stats", "--csv"}); err != nil {
		t.Fatalf("runStats --csv: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "section,label,count") {
		t.Errorf("CSV missing header, got:\n%s", out)
	}
	if !strings.Contains(out, "totals,sessions,3") {
		t.Errorf("CSV missing totals row, got:\n%s", out)
	}
	if !strings.Contains(out, "repository,jongio/dispatch,2") {
		t.Errorf("CSV missing repo breakdown, got:\n%s", out)
	}
}

// TestRunStatsCSV_FormulaInjection verifies a repo/branch label starting with a
// spreadsheet formula trigger is neutralized (prefixed with ') so it can't
// execute as a formula when opened in Excel/Sheets (CWE-1236).
func TestRunStatsCSV_FormulaInjection(t *testing.T) {
	withStatsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return []data.Session{
			{ID: "s1", Repository: "=cmd|' /c calc'!A1", Branch: "main"},
			{ID: "s2", Repository: "=cmd|' /c calc'!A1", Branch: "main"},
		}, nil
	})

	var buf bytes.Buffer
	if err := runStats(&buf, []string{"stats", "--csv"}); err != nil {
		t.Fatalf("runStats --csv: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "\n=cmd") || strings.Contains(out, ",=cmd") {
		t.Errorf("CSV emitted an unescaped formula label, got:\n%s", out)
	}
	if !strings.Contains(out, "'=cmd") {
		t.Errorf("CSV should prefix a formula-triggering label with a quote, got:\n%s", out)
	}
}

func TestCsvSafe(t *testing.T) {
	cases := map[string]string{
		"":            "",
		"jongio/repo": "jongio/repo",
		"main":        "main",
		"=1+1":        "'=1+1",
		"+cmd":        "'+cmd",
		"-2":          "'-2",
		"@ref":        "'@ref",
		"\ttab":       "'\ttab",
	}
	for in, want := range cases {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}
}
