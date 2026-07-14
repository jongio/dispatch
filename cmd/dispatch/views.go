package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jongio/dispatch/internal/config"
)

type viewsReport struct {
	ActiveView string             `json:"active_view"`
	Views      []config.NamedView `json:"views"`
}

func runViews(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	rest := args
	if len(rest) > 0 {
		rest = rest[1:]
	}
	if len(rest) == 0 || rest[0] == "list" || rest[0] == "--json" {
		return runViewsList(w, rest)
	}

	switch rest[0] {
	case "use":
		return runViewsUse(w, rest[1:])
	default:
		return fmt.Errorf("unknown views subcommand %q (want list or use)", rest[0])
	}
}

func runViewsList(w io.Writer, args []string) error {
	jsonOut := false
	if len(args) > 0 && args[0] == "list" {
		args = args[1:]
	}
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("views list does not take arguments, got %q", arg)
		}
	}

	cfg, err := configLoadFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	report := buildViewsReport(cfg)
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	writeViewsText(w, report)
	return nil
}

func runViewsUse(w io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("views use requires a view name or default")
	}
	name := strings.TrimSpace(strings.Join(args, " "))
	if name == "" {
		return fmt.Errorf("views use requires a view name or default")
	}

	cfg, err := configLoadFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if strings.EqualFold(name, "default") {
		cfg.ActiveView = ""
		if err := configSaveFn(cfg); err != nil {
			return err
		}
		fmt.Fprintln(w, "Active view: Default")
		return nil
	}
	view := cfg.FindView(name)
	if view == nil || view.Validate() != nil {
		return fmt.Errorf("view %q not found", name)
	}
	cfg.ActiveView = name
	if err := configSaveFn(cfg); err != nil {
		return err
	}
	fmt.Fprintf(w, "Active view: %s\n", name)
	return nil
}

func buildViewsReport(cfg *config.Config) viewsReport {
	active := "Default"
	if cfg != nil && cfg.ActiveView != "" {
		active = cfg.ActiveView
	}
	report := viewsReport{ActiveView: active, Views: []config.NamedView{}}
	if cfg == nil {
		return report
	}
	report.Views = cfg.ValidViews()
	return report
}

func writeViewsText(w io.Writer, report viewsReport) {
	fmt.Fprintln(w, "Dispatch views")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Active: %s\n\n", report.ActiveView)
	if len(report.Views) == 0 {
		fmt.Fprintln(w, "No named views found.")
		return
	}
	for _, v := range report.Views {
		marker := " "
		if v.Name == report.ActiveView {
			marker = "*"
		}
		fmt.Fprintf(w, "%s %s\n", marker, v.Name)
		if summary := describeView(v); summary != "" {
			fmt.Fprintf(w, "  %s\n", summary)
		}
	}
}

func describeView(v config.NamedView) string {
	parts := make([]string, 0, 8)
	if v.Search != "" {
		parts = append(parts, "search="+v.Search)
	}
	if v.TimeRange != "" {
		parts = append(parts, "time="+v.TimeRange)
	}
	if v.Sort != "" {
		sortPart := "sort=" + v.Sort
		if v.SortOrder != "" {
			sortPart += ":" + v.SortOrder
		}
		parts = append(parts, sortPart)
	}
	if v.Pivot != "" {
		parts = append(parts, "pivot="+v.Pivot)
	}
	if v.FavoritesOnly {
		parts = append(parts, "favorites")
	}
	if v.ShowHidden {
		parts = append(parts, "show_hidden")
	}
	if len(v.ExcludedDirs) > 0 {
		parts = append(parts, fmt.Sprintf("excluded_dirs=%d", len(v.ExcludedDirs)))
	}
	return strings.Join(parts, ", ")
}
