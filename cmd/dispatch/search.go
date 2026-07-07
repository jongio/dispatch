package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/jongio/dispatch/internal/data"
)

// searchListSessionsFn loads sessions for the search command. It is a package
// variable so tests can substitute a fixed set of sessions, matching the seam
// pattern used elsewhere in this package (see stats.go and cli.go).
var searchListSessionsFn = defaultSearchListSessions

// searchDefaultLimit caps how many matches the search command returns unless
// the caller overrides it with --limit. A limit of 0 means no cap.
const searchDefaultLimit = 50

// searchAllLimit is a high ceiling used when the caller requests every match
// (--limit 0), mirroring the ceiling the stats command uses.
const searchAllLimit = 100_000

// searchOptions holds the parsed flags for the search command.
type searchOptions struct {
	filter data.FilterOptions
	limit  int
}

// searchSession is the machine-readable shape emitted for each matching
// session. It is a dedicated struct (rather than data.Session) so the output
// contract stays stable regardless of internal model changes.
type searchSession struct {
	ID         string `json:"id"`
	Summary    string `json:"summary"`
	Cwd        string `json:"cwd"`
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	TurnCount  int    `json:"turn_count"`
	FileCount  int    `json:"file_count"`
}

// runSearch prints matching sessions as JSON without starting the TUI. args is
// the full argument slice with args[0] == "search".
func runSearch(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	opts, err := parseSearchArgs(args)
	if err != nil {
		return err
	}

	limit := opts.limit
	if limit <= 0 {
		limit = searchAllLimit
	}

	sessions, err := searchListSessionsFn(opts.filter, limit)
	if err != nil {
		return err
	}

	results := make([]searchSession, 0, len(sessions))
	for _, s := range sessions {
		results = append(results, searchSession{
			ID:         s.ID,
			Summary:    s.Summary,
			Cwd:        s.Cwd,
			Repository: s.Repository,
			Branch:     s.Branch,
			CreatedAt:  s.CreatedAt,
			UpdatedAt:  s.UpdatedAt,
			TurnCount:  s.TurnCount,
			FileCount:  s.FileCount,
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

// parseSearchArgs reads the search subcommand flags. args[0] is expected to be
// "search". A single leading token that does not start with "-" is treated as
// the search query, matching how the TUI seeds its search box.
func parseSearchArgs(args []string) (searchOptions, error) {
	opts := searchOptions{limit: searchDefaultLimit}

	rest := args
	if len(rest) > 0 {
		rest = rest[1:] // drop the "search" token
	}

	takeValue := func(i int, name, inline string) (string, int, error) {
		if inline != "" {
			return inline, i, nil
		}
		if i+1 >= len(rest) {
			return "", i, fmt.Errorf("%s requires a value", name)
		}
		return rest[i+1], i + 1, nil
	}

	var queryParts []string
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		name, inline, hasInline := splitFlag(arg)

		switch {
		case name == "--json":
			// Accepted for explicitness and forward compatibility; JSON is
			// the only output mode this command emits.
		case name == "--deep":
			opts.filter.DeepSearch = true
		case name == "--query" || name == "-q":
			v, ni, err := takeValue(i, "--query", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return searchOptions{}, err
			}
			queryParts = append(queryParts, v)
			i = ni
		case name == "--repo" || name == "--repository":
			v, ni, err := takeValue(i, "--repo", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return searchOptions{}, err
			}
			opts.filter.Repository = v
			i = ni
		case name == "--branch":
			v, ni, err := takeValue(i, "--branch", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return searchOptions{}, err
			}
			opts.filter.Branch = v
			i = ni
		case name == "--folder":
			v, ni, err := takeValue(i, "--folder", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return searchOptions{}, err
			}
			opts.filter.Folder = v
			i = ni
		case name == "--host":
			v, ni, err := takeValue(i, "--host", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return searchOptions{}, err
			}
			opts.filter.HostType = v
			i = ni
		case name == "--since":
			v, ni, err := takeValue(i, "--since", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return searchOptions{}, err
			}
			t, ok := parseStatsTime(v)
			if !ok {
				return searchOptions{}, fmt.Errorf("invalid --since value %q (want YYYY-MM-DD or RFC3339)", v)
			}
			opts.filter.Since = &t
			i = ni
		case name == "--until":
			v, ni, err := takeValue(i, "--until", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return searchOptions{}, err
			}
			t, ok := parseStatsTime(v)
			if !ok {
				return searchOptions{}, fmt.Errorf("invalid --until value %q (want YYYY-MM-DD or RFC3339)", v)
			}
			opts.filter.Until = &t
			i = ni
		case name == "--limit" || name == "-n":
			v, ni, err := takeValue(i, "--limit", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return searchOptions{}, err
			}
			n, err := parseSearchLimit(v)
			if err != nil {
				return searchOptions{}, err
			}
			opts.limit = n
			i = ni
		case strings.HasPrefix(arg, "-"):
			return searchOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			queryParts = append(queryParts, arg)
		}
	}

	if len(queryParts) > 0 {
		opts.filter.Query = strings.Join(queryParts, " ")
	}

	return opts, nil
}

// parseSearchLimit parses the --limit value. Zero means "no cap"; negative
// values are rejected.
func parseSearchLimit(v string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid --limit value %q (want a whole number, 0 for no limit)", v)
	}
	return n, nil
}

// defaultSearchListSessions loads sessions matching the filter from the default
// session store, ordered by most recent activity first.
func defaultSearchListSessions(filter data.FilterOptions, limit int) ([]data.Session, error) {
	store, err := data.Open()
	if err != nil {
		return nil, fmt.Errorf("opening session store: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only, best-effort close

	sortOpts := data.SortOptions{Field: data.SortByUpdated, Order: data.Descending}
	sessions, err := store.ListSessions(context.Background(), filter, sortOpts, limit)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	return sessions, nil
}
