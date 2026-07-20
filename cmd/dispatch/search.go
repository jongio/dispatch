package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

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

type searchOutputFormat string

const (
	searchFormatJSON  searchOutputFormat = "json"
	searchFormatIDs   searchOutputFormat = "ids"
	searchFormatTable searchOutputFormat = "table"
	searchFormatCSV   searchOutputFormat = "csv"
)

// searchOptions holds the parsed flags for the search command.
type searchOptions struct {
	filter data.FilterOptions
	sort   data.SortOptions
	limit  int
	format searchOutputFormat
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

	sessions, err := searchListSessionsFn(opts.filter, opts.sort, limit)
	if err != nil {
		return err
	}

	if opts.format == searchFormatIDs {
		return writeSearchIDs(w, sessions)
	}
	if opts.format == searchFormatTable {
		return writeSearchTable(w, sessions)
	}
	if opts.format == searchFormatCSV {
		return writeSearchCSV(w, sessions)
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

func writeSearchIDs(w io.Writer, sessions []data.Session) error {
	for _, s := range sessions {
		if _, err := fmt.Fprintln(w, s.ID); err != nil {
			return err
		}
	}
	return nil
}

func writeSearchTable(w io.Writer, sessions []data.Session) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "ID\tLAST ACTIVE\tREPO\tBRANCH\tTURNS\tFILES\tSUMMARY"); err != nil {
		return err
	}
	for _, s := range sessions {
		if _, err := fmt.Fprintf(
			tw, "%s\t%s\t%s\t%s\t%d\t%d\t%s\n",
			shortSearchID(s.ID),
			searchTableTime(s),
			searchTableCell(s.Repository),
			searchTableCell(s.Branch),
			s.TurnCount,
			s.FileCount,
			searchTableCell(s.Summary),
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writeSearchCSV(w io.Writer, sessions []data.Session) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"id", "summary", "cwd", "repository", "branch", "created_at", "updated_at", "turn_count", "file_count"}); err != nil {
		return err
	}
	for _, s := range sessions {
		if err := cw.Write([]string{
			s.ID,
			s.Summary,
			s.Cwd,
			s.Repository,
			s.Branch,
			s.CreatedAt,
			s.UpdatedAt,
			strconv.Itoa(s.TurnCount),
			strconv.Itoa(s.FileCount),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func shortSearchID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

func searchTableTime(s data.Session) string {
	v := s.LastActiveAt
	if v == "" {
		v = s.UpdatedAt
	}
	if len(v) >= len("2006-01-02") {
		return v[:len("2006-01-02")]
	}
	return searchTableCell(v)
}

func searchTableCell(v string) string {
	v = strings.Join(strings.Fields(v), " ")
	if v == "" {
		return "-"
	}
	return v
}

// parseSearchArgs reads the search subcommand flags. args[0] is expected to be
// "search". A single leading token that does not start with "-" is treated as
// the search query, matching how the TUI seeds its search box.
func parseSearchArgs(args []string) (searchOptions, error) {
	opts := searchOptions{
		sort:   defaultSearchSort(),
		limit:  searchDefaultLimit,
		format: searchFormatJSON,
	}

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
			opts.format = searchFormatJSON
		case name == "--ids":
			opts.format = searchFormatIDs
		case name == "--table":
			opts.format = searchFormatTable
		case name == "--csv":
			opts.format = searchFormatCSV
		case name == "--format":
			v, ni, err := takeValue(i, "--format", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return searchOptions{}, err
			}
			format, err := parseSearchFormat(v)
			if err != nil {
				return searchOptions{}, err
			}
			opts.format = format
			i = ni
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
		case name == "--sort":
			v, ni, err := takeValue(i, "--sort", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return searchOptions{}, err
			}
			field, err := parseSearchSortField(v)
			if err != nil {
				return searchOptions{}, err
			}
			opts.sort.Field = field
			i = ni
		case name == "--order":
			v, ni, err := takeValue(i, "--order", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return searchOptions{}, err
			}
			order, err := parseSearchSortOrder(v)
			if err != nil {
				return searchOptions{}, err
			}
			opts.sort.Order = order
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

func defaultSearchSort() data.SortOptions {
	return data.SortOptions{Field: data.SortByUpdated, Order: data.Descending}
}

func parseSearchSortField(v string) (data.SortField, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "updated":
		return data.SortByUpdated, nil
	case "created":
		return data.SortByCreated, nil
	case "turns":
		return data.SortByTurns, nil
	case "name":
		return data.SortByName, nil
	case "folder":
		return data.SortByFolder, nil
	default:
		return "", fmt.Errorf("invalid --sort value %q (want updated, created, turns, name, or folder)", v)
	}
}

func parseSearchSortOrder(v string) (data.SortOrder, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "asc":
		return data.Ascending, nil
	case "desc":
		return data.Descending, nil
	default:
		return "", fmt.Errorf("invalid --order value %q (want asc or desc)", v)
	}
}

func parseSearchFormat(v string) (searchOutputFormat, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case string(searchFormatJSON):
		return searchFormatJSON, nil
	case string(searchFormatIDs):
		return searchFormatIDs, nil
	case string(searchFormatTable):
		return searchFormatTable, nil
	case string(searchFormatCSV):
		return searchFormatCSV, nil
	default:
		return "", fmt.Errorf("invalid --format value %q (want json, ids, table, or csv)", v)
	}
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
// session store.
func defaultSearchListSessions(filter data.FilterOptions, sortOpts data.SortOptions, limit int) ([]data.Session, error) {
	store, err := data.Open()
	if err != nil {
		return nil, fmt.Errorf("opening session store: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only, best-effort close

	if sortOpts.Field == "" {
		sortOpts = defaultSearchSort()
	}
	sessions, err := store.ListSessions(context.Background(), filter, sortOpts, limit)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	return sessions, nil
}
