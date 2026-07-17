package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jongio/dispatch/internal/data"
)

// aliasesListSessionsFn loads sessions for the aliases command. It is a
// package variable so tests can substitute a fixed set of sessions.
var aliasesListSessionsFn = defaultStatsListSessions

// aliasEntry describes one alias mapping.
type aliasEntry struct {
	Alias    string `json:"alias"`
	ID       string `json:"id"`
	Summary  string `json:"summary,omitempty"`
	Repo     string `json:"repo,omitempty"`
	Orphaned bool   `json:"orphaned"`
}

// aliasesReport is the aggregate output for the aliases command.
type aliasesReport struct {
	TotalAliases int          `json:"total_aliases"`
	Orphaned     int          `json:"orphaned"`
	Aliases      []aliasEntry `json:"aliases"`
}

// runAliases lists every session alias with its target session. args[0] is
// expected to be "aliases".
func runAliases(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	opts, err := parseAliasesArgs(args)
	if err != nil {
		return err
	}

	cfg, err := configLoadFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	sessions, err := aliasesListSessionsFn(data.FilterOptions{})
	if err != nil {
		return err
	}

	report := buildAliasesReport(cfg.SessionAliases, sessions)
	if opts.json {
		return writeAliasesJSON(w, report)
	}
	writeAliasesText(w, report)
	return nil
}

// aliasesOptions holds parsed flags for the aliases command.
type aliasesOptions struct {
	json bool
}

// parseAliasesArgs reads the aliases subcommand flags.
func parseAliasesArgs(args []string) (aliasesOptions, error) {
	var opts aliasesOptions

	rest := args
	if len(rest) > 0 {
		rest = rest[1:]
	}

	for _, arg := range rest {
		switch {
		case arg == "--json":
			opts.json = true
		case strings.HasPrefix(arg, "-"):
			return aliasesOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			return aliasesOptions{}, fmt.Errorf("aliases does not take positional arguments, got %q", arg)
		}
	}

	return opts, nil
}

// buildAliasesReport builds the alias listing. SessionAliases maps session
// ID to alias string.
func buildAliasesReport(aliases map[string]string, sessions []data.Session) aliasesReport {
	report := aliasesReport{Aliases: []aliasEntry{}}

	liveIDs := make(map[string]*data.Session, len(sessions))
	for i := range sessions {
		liveIDs[sessions[i].ID] = &sessions[i]
	}

	for id, alias := range aliases {
		if alias == "" {
			continue
		}
		entry := aliasEntry{Alias: alias, ID: id}
		if s, ok := liveIDs[id]; ok {
			entry.Summary = s.Summary
			entry.Repo = s.Repository
		} else {
			entry.Orphaned = true
			report.Orphaned++
		}
		report.Aliases = append(report.Aliases, entry)
	}

	sort.Slice(report.Aliases, func(i, j int) bool {
		return report.Aliases[i].Alias < report.Aliases[j].Alias
	})
	report.TotalAliases = len(report.Aliases)
	return report
}

// writeAliasesJSON prints the report as a single JSON object.
func writeAliasesJSON(w io.Writer, report aliasesReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// writeAliasesText prints the report in a plain, human-readable layout.
func writeAliasesText(w io.Writer, report aliasesReport) {
	fmt.Fprintln(w, "Dispatch aliases")
	fmt.Fprintln(w)

	if report.TotalAliases == 0 {
		fmt.Fprintln(w, "No aliases configured.")
		return
	}

	fmt.Fprintf(w, "Aliases: %d", report.TotalAliases)
	if report.Orphaned > 0 {
		fmt.Fprintf(w, " (%d orphaned)", report.Orphaned)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w)

	aliasWidth := 0
	for _, e := range report.Aliases {
		if len(e.Alias) > aliasWidth {
			aliasWidth = len(e.Alias)
		}
	}

	for _, e := range report.Aliases {
		if e.Orphaned {
			fmt.Fprintf(w, "  %-*s  %s  [orphaned]\n", aliasWidth, e.Alias, shortID(e.ID))
		} else {
			label := e.Summary
			if e.Repo != "" {
				label = e.Repo + ": " + label
			}
			fmt.Fprintf(w, "  %-*s  %s  %s\n", aliasWidth, e.Alias, shortID(e.ID), label)
		}
	}
}

// shortID returns a truncated session ID for display.
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
