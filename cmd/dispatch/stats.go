package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/jongio/dispatch/internal/data"
)

// statsListSessionsFn loads sessions for the stats command. It is a package
// variable so tests can substitute a fixed set of sessions, matching the seam
// pattern used elsewhere in this package (see cli.go).
var statsListSessionsFn = defaultStatsListSessions

// statsQueryLimit is a high ceiling used so the summary covers every stored
// session rather than the smaller default page size used by the TUI.
const statsQueryLimit = 100_000

// statsOptions holds the parsed flags for the stats command.
type statsOptions struct {
	filter   data.FilterOptions
	json     bool
	calendar bool
}

// countEntry is one label and count pair in a grouped breakdown.
type countEntry struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// statsReport is the aggregate summary produced by the stats command.
type statsReport struct {
	TotalSessions int               `json:"total_sessions"`
	TotalTurns    int               `json:"total_turns"`
	TotalFiles    int               `json:"total_files"`
	Earliest      string            `json:"earliest,omitempty"`
	Latest        string            `json:"latest,omitempty"`
	ByRepository  []countEntry      `json:"by_repository"`
	ByBranch      []countEntry      `json:"by_branch"`
	ByHostType    []countEntry      `json:"by_host_type"`
	Calendar      *activityCalendar `json:"calendar,omitempty"`
}

// dayCount is the session count for a single calendar day.
type dayCount struct {
	Date      string `json:"date"` // YYYY-MM-DD (UTC)
	Count     int    `json:"count"`
	Intensity int    `json:"intensity"` // 0-4 heatmap bucket
}

// activityCalendar is a continuous per-day session-count series used to render
// a contribution-style heatmap.
type activityCalendar struct {
	Days     []dayCount `json:"days"`
	MaxCount int        `json:"max_count"`
}

// runStats prints aggregate counts for the stored sessions. args is the full
// argument slice with args[0] == "stats".
func runStats(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	opts, err := parseStatsArgs(args)
	if err != nil {
		return err
	}

	sessions, err := statsListSessionsFn(opts.filter)
	if err != nil {
		return err
	}

	report := buildStatsReport(sessions)
	if opts.calendar {
		cal := buildActivityCalendar(sessions)
		report.Calendar = &cal
	}
	if opts.json {
		return writeStatsJSON(w, report)
	}
	writeStatsText(w, report)
	if opts.calendar {
		writeActivityCalendar(w, *report.Calendar)
	}
	return nil
}

// parseStatsArgs reads the stats subcommand flags. args[0] is expected to be
// "stats". It rejects positional arguments and unknown flags.
func parseStatsArgs(args []string) (statsOptions, error) {
	var opts statsOptions

	rest := args
	if len(rest) > 0 {
		rest = rest[1:] // drop the "stats" token
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

	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		name, inline, hasInline := splitFlag(arg)

		switch {
		case name == "--json":
			opts.json = true
		case name == "--calendar":
			opts.calendar = true
		case name == "--repo" || name == "--repository":
			v, ni, err := takeValue(i, "--repo", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return statsOptions{}, err
			}
			opts.filter.Repository = v
			i = ni
		case name == "--branch":
			v, ni, err := takeValue(i, "--branch", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return statsOptions{}, err
			}
			opts.filter.Branch = v
			i = ni
		case name == "--folder":
			v, ni, err := takeValue(i, "--folder", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return statsOptions{}, err
			}
			opts.filter.Folder = v
			i = ni
		case name == "--since":
			v, ni, err := takeValue(i, "--since", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return statsOptions{}, err
			}
			t, ok := parseStatsTime(v)
			if !ok {
				return statsOptions{}, fmt.Errorf("invalid --since value %q (want YYYY-MM-DD or RFC3339)", v)
			}
			opts.filter.Since = &t
			i = ni
		case name == "--until":
			v, ni, err := takeValue(i, "--until", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return statsOptions{}, err
			}
			t, ok := parseStatsTime(v)
			if !ok {
				return statsOptions{}, fmt.Errorf("invalid --until value %q (want YYYY-MM-DD or RFC3339)", v)
			}
			opts.filter.Until = &t
			i = ni
		case strings.HasPrefix(arg, "-"):
			return statsOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			return statsOptions{}, fmt.Errorf("stats does not take positional arguments, got %q", arg)
		}
	}

	return opts, nil
}

// splitFlag separates a flag token into its name and optional inline value,
// e.g. "--repo=foo" becomes ("--repo", "foo", true).
func splitFlag(arg string) (name, value string, hasValue bool) {
	if eq := strings.IndexByte(arg, '='); eq >= 0 {
		return arg[:eq], arg[eq+1:], true
	}
	return arg, "", false
}

// inlineOrEmpty returns the inline value only when one was present, so that a
// bare flag falls through to consuming the next argument.
func inlineOrEmpty(value string, hasValue bool) string {
	if hasValue {
		return value
	}
	return ""
}

// parseStatsTime parses a timestamp in RFC3339 or common date-only forms.
func parseStatsTime(s string) (time.Time, bool) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// buildStatsReport aggregates the given sessions into a summary.
func buildStatsReport(sessions []data.Session) statsReport {
	report := statsReport{
		ByRepository: []countEntry{},
		ByBranch:     []countEntry{},
		ByHostType:   []countEntry{},
	}

	repoCounts := map[string]int{}
	branchCounts := map[string]int{}
	hostCounts := map[string]int{}

	var earliest, latest time.Time

	for _, s := range sessions {
		report.TotalSessions++
		report.TotalTurns += s.TurnCount
		report.TotalFiles += s.FileCount

		repoCounts[labelOr(s.Repository, "(none)")]++
		branchCounts[labelOr(s.Branch, "(none)")]++
		hostCounts[labelOr(s.HostType, "(unknown)")]++

		if t, ok := parseStatsTime(s.CreatedAt); ok {
			if earliest.IsZero() || t.Before(earliest) {
				earliest = t
			}
		}
		if t, ok := latestTime(s); ok {
			if latest.IsZero() || t.After(latest) {
				latest = t
			}
		}
	}

	if !earliest.IsZero() {
		report.Earliest = earliest.UTC().Format("2006-01-02")
	}
	if !latest.IsZero() {
		report.Latest = latest.UTC().Format("2006-01-02")
	}

	report.ByRepository = sortedCounts(repoCounts)
	report.ByBranch = sortedCounts(branchCounts)
	report.ByHostType = sortedCounts(hostCounts)
	return report
}

// latestTime returns the most recent timestamp for a session, preferring
// LastActiveAt and falling back to UpdatedAt then CreatedAt.
func latestTime(s data.Session) (time.Time, bool) {
	for _, ts := range []string{s.LastActiveAt, s.UpdatedAt, s.CreatedAt} {
		if t, ok := parseStatsTime(ts); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

// labelOr returns value, or fallback when value is empty.
func labelOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// sortedCounts converts a label/count map into a slice ordered by count
// descending, then label ascending for stable output.
func sortedCounts(counts map[string]int) []countEntry {
	entries := make([]countEntry, 0, len(counts))
	for label, count := range counts {
		entries = append(entries, countEntry{Label: label, Count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count != entries[j].Count {
			return entries[i].Count > entries[j].Count
		}
		return entries[i].Label < entries[j].Label
	})
	return entries
}

// writeStatsJSON prints the report as a single JSON object.
func writeStatsJSON(w io.Writer, report statsReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// writeStatsText prints the report in a plain, human-readable layout.
func writeStatsText(w io.Writer, report statsReport) {
	fmt.Fprintln(w, "Dispatch stats")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Sessions: %d\n", report.TotalSessions)
	fmt.Fprintf(w, "Turns:    %d\n", report.TotalTurns)
	fmt.Fprintf(w, "Files:    %d\n", report.TotalFiles)
	if report.Earliest != "" && report.Latest != "" {
		fmt.Fprintf(w, "Range:    %s to %s\n", report.Earliest, report.Latest)
	}

	if report.TotalSessions == 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "No sessions found.")
		return
	}

	writeCountSection(w, "By repository", report.ByRepository)
	writeCountSection(w, "By branch", report.ByBranch)
	writeCountSection(w, "By host type", report.ByHostType)
}

// writeCountSection prints a titled breakdown with aligned counts.
func writeCountSection(w io.Writer, title string, entries []countEntry) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, title)
	if len(entries) == 0 {
		fmt.Fprintln(w, "  (no data)")
		return
	}
	width := 0
	for _, e := range entries {
		if len(e.Label) > width {
			width = len(e.Label)
		}
	}
	for _, e := range entries {
		fmt.Fprintf(w, "  %-*s  %d\n", width, e.Label, e.Count)
	}
}

// defaultStatsListSessions loads every stored session matching the filter.
func defaultStatsListSessions(filter data.FilterOptions) ([]data.Session, error) {
	store, err := data.Open()
	if err != nil {
		return nil, fmt.Errorf("opening session store: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only, best-effort close

	sortOpts := data.SortOptions{Field: data.SortByCreated, Order: data.Ascending}
	sessions, err := store.ListSessions(context.Background(), filter, sortOpts, statsQueryLimit)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	return sessions, nil
}

// buildActivityCalendar groups session creation dates into per-day counts with
// an intensity bucket (0-4) scaled to the busiest day. Every day between the
// first and last active day is included so gaps render as empty cells. Days
// are ordered chronologically.
func buildActivityCalendar(sessions []data.Session) activityCalendar {
	counts := map[string]int{}
	var first, last time.Time

	for _, s := range sessions {
		t, ok := parseStatsTime(s.CreatedAt)
		if !ok {
			continue
		}
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		counts[day.Format("2006-01-02")]++
		if first.IsZero() || day.Before(first) {
			first = day
		}
		if last.IsZero() || day.After(last) {
			last = day
		}
	}

	cal := activityCalendar{Days: []dayCount{}}
	if first.IsZero() {
		return cal
	}

	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}
	cal.MaxCount = maxCount

	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		count := counts[key]
		cal.Days = append(cal.Days, dayCount{
			Date:      key,
			Count:     count,
			Intensity: intensityBucket(count, maxCount),
		})
	}
	return cal
}

// intensityBucket maps a day's count to a 0-4 heatmap level relative to the
// busiest day. Zero counts are level 0; the rest split into quartiles.
func intensityBucket(count, maxCount int) int {
	if count <= 0 || maxCount <= 0 {
		return 0
	}
	ratio := float64(count) / float64(maxCount)
	switch {
	case ratio <= 0.25:
		return 1
	case ratio <= 0.5:
		return 2
	case ratio <= 0.75:
		return 3
	default:
		return 4
	}
}

// intensityRune returns the block character used to render a heatmap level.
func intensityRune(level int) rune {
	switch level {
	case 1:
		return '░'
	case 2:
		return '▒'
	case 3:
		return '▓'
	case 4:
		return '█'
	default:
		return '·'
	}
}

// writeActivityCalendar prints a GitHub-style contribution grid: seven weekday
// rows across week columns, with a shaded block per day and an intensity
// legend below.
func writeActivityCalendar(w io.Writer, cal activityCalendar) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Activity")
	if len(cal.Days) == 0 {
		fmt.Fprintln(w, "  (no data)")
		return
	}

	byDate := make(map[string]dayCount, len(cal.Days))
	for _, d := range cal.Days {
		byDate[d.Date] = d
	}

	first, _ := time.Parse("2006-01-02", cal.Days[0].Date)
	last, _ := time.Parse("2006-01-02", cal.Days[len(cal.Days)-1].Date)

	// Align the grid to whole weeks (Sunday start) so weekday rows line up.
	gridStart := first.AddDate(0, 0, -int(first.Weekday()))
	gridEnd := last.AddDate(0, 0, 6-int(last.Weekday()))
	weeks := int(gridEnd.Sub(gridStart).Hours()/24)/7 + 1

	labels := [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	for wd := 0; wd < 7; wd++ {
		var b strings.Builder
		fmt.Fprintf(&b, "  %s ", labels[wd])
		for wk := 0; wk < weeks; wk++ {
			day := gridStart.AddDate(0, 0, wk*7+wd)
			if day.Before(first) || day.After(last) {
				b.WriteRune(' ')
				continue
			}
			b.WriteRune(intensityRune(byDate[day.Format("2006-01-02")].Intensity))
		}
		fmt.Fprintln(w, b.String())
	}

	fmt.Fprintf(w, "  Less %c%c%c%c%c More\n",
		intensityRune(0), intensityRune(1), intensityRune(2), intensityRune(3), intensityRune(4))
}
