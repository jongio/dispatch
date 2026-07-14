package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/config"
)

func viewsConfig() *config.Config {
	cfg := config.Default()
	cfg.ActiveView = "Work"
	cfg.Views = []config.NamedView{
		{Name: "Broken", TimeRange: "never"},
		{Name: "Work", Search: "repo:jongio/dispatch", TimeRange: config.TimeRange7d, Sort: config.SortFieldUpdated, SortOrder: config.SortOrderDesc, Pivot: config.PivotRepo, FavoritesOnly: true},
		{Name: "Personal", Pivot: config.PivotFolder, ShowHidden: true, ExcludedDirs: []string{"archive"}},
	}
	return cfg
}

func TestBuildViewsReportFiltersInvalidViews(t *testing.T) {
	report := buildViewsReport(viewsConfig())
	if report.ActiveView != "Work" {
		t.Fatalf("ActiveView = %q, want Work", report.ActiveView)
	}
	if len(report.Views) != 2 {
		t.Fatalf("views len = %d, want 2 valid views: %+v", len(report.Views), report.Views)
	}
	for _, v := range report.Views {
		if v.Name == "Broken" {
			t.Fatal("invalid view should be filtered")
		}
	}
}

func TestRunViewsListText(t *testing.T) {
	withConfigSeams(t, viewsConfig())
	var buf bytes.Buffer
	if err := runViews(&buf, []string{"views"}); err != nil {
		t.Fatalf("runViews: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Dispatch views", "Active: Work", "* Work", "repo:jongio/dispatch", "Personal"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Broken") {
		t.Fatalf("invalid view should not appear:\n%s", out)
	}
}

func TestRunViewsListJSON(t *testing.T) {
	withConfigSeams(t, viewsConfig())
	var buf bytes.Buffer
	if err := runViews(&buf, []string{"views", "--json"}); err != nil {
		t.Fatalf("runViews json: %v", err)
	}
	var report viewsReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if report.ActiveView != "Work" || len(report.Views) != 2 {
		t.Fatalf("report = %+v, want active Work and 2 views", report)
	}
}

func TestRunViewsUse(t *testing.T) {
	cfg := withConfigSeams(t, viewsConfig())
	var buf bytes.Buffer
	if err := runViews(&buf, []string{"views", "use", "Personal"}); err != nil {
		t.Fatalf("views use Personal: %v", err)
	}
	if cfg.ActiveView != "Personal" {
		t.Fatalf("ActiveView = %q, want Personal", cfg.ActiveView)
	}
	if !strings.Contains(buf.String(), "Active view: Personal") {
		t.Fatalf("unexpected output: %q", buf.String())
	}

	buf.Reset()
	if err := runViews(&buf, []string{"views", "use", "default"}); err != nil {
		t.Fatalf("views use default: %v", err)
	}
	if cfg.ActiveView != "" {
		t.Fatalf("ActiveView = %q, want cleared", cfg.ActiveView)
	}
}

func TestRunViewsUseInvalidDoesNotSave(t *testing.T) {
	cfg := withConfigSeams(t, viewsConfig())
	if err := runViews(&bytes.Buffer{}, []string{"views", "use", "Missing"}); err == nil {
		t.Fatal("expected error for missing view")
	}
	if err := runViews(&bytes.Buffer{}, []string{"views", "use", "Broken"}); err == nil {
		t.Fatal("expected error for invalid view")
	}
	if cfg.ActiveView != "Work" {
		t.Fatalf("ActiveView changed on error: %q", cfg.ActiveView)
	}
}

func TestRunViewsErrors(t *testing.T) {
	withConfigSeams(t, viewsConfig())
	for _, args := range [][]string{
		{"views", "bogus"},
		{"views", "list", "extra"},
		{"views", "use"},
	} {
		if err := runViews(&bytes.Buffer{}, args); err == nil {
			t.Fatalf("expected error for args %v", args)
		}
	}
}

func TestHandleArgsViews(t *testing.T) {
	withConfigSeams(t, viewsConfig())
	done, _, _, err := handleArgs([]string{"views"}, &bytes.Buffer{}, nil)
	if err != nil {
		t.Fatalf("handleArgs views: %v", err)
	}
	if !done {
		t.Fatal("handleArgs should report done for views")
	}
}
