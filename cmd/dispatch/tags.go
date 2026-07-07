package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jongio/dispatch/internal/data"
)

// tagsListSessionsFn loads sessions for the tags command. It is a package
// variable so tests can substitute a fixed set of sessions, matching the seam
// pattern used elsewhere in this package (see stats.go and cli.go).
var tagsListSessionsFn = defaultStatsListSessions

// tagsOptions holds the parsed flags for the tags command.
type tagsOptions struct {
	json bool
}

// tagCount is one tag and the number of sessions that carry it.
type tagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// tagsReport is the aggregate tag summary produced by the tags command.
type tagsReport struct {
	TotalTags      int        `json:"total_tags"`
	TaggedSessions int        `json:"tagged_sessions"`
	Tags           []tagCount `json:"tags"`
}

// runTags prints the tags in use with a per-tag session count. args is the
// full argument slice with args[0] == "tags".
func runTags(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	opts, err := parseTagsArgs(args)
	if err != nil {
		return err
	}

	cfg, err := configLoadFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	sessions, err := tagsListSessionsFn(data.FilterOptions{})
	if err != nil {
		return err
	}

	report := buildTagsReport(cfg.SessionTags, sessions)
	if opts.json {
		return writeTagsJSON(w, report)
	}
	writeTagsText(w, report)
	return nil
}

// parseTagsArgs reads the tags subcommand flags. args[0] is expected to be
// "tags". It rejects positional arguments and unknown flags.
func parseTagsArgs(args []string) (tagsOptions, error) {
	var opts tagsOptions

	rest := args
	if len(rest) > 0 {
		rest = rest[1:] // drop the "tags" token
	}

	for _, arg := range rest {
		switch {
		case arg == "--json":
			opts.json = true
		case strings.HasPrefix(arg, "-"):
			return tagsOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			return tagsOptions{}, fmt.Errorf("tags does not take positional arguments, got %q", arg)
		}
	}

	return opts, nil
}

// buildTagsReport counts how many of the given sessions carry each tag. Tags on
// sessions that are not present (for example, sessions deleted since they were
// tagged) are ignored so the counts reflect the current session store.
func buildTagsReport(sessionTags map[string][]string, sessions []data.Session) tagsReport {
	report := tagsReport{Tags: []tagCount{}}

	counts := map[string]int{}
	for _, s := range sessions {
		tags := sessionTags[s.ID]
		if len(tags) == 0 {
			continue
		}
		report.TaggedSessions++
		for _, tag := range tags {
			counts[tag]++
		}
	}

	report.TotalTags = len(counts)
	report.Tags = sortedTagCounts(counts)
	return report
}

// sortedTagCounts converts a tag/count map into a slice ordered by count
// descending, then tag ascending for stable output.
func sortedTagCounts(counts map[string]int) []tagCount {
	entries := make([]tagCount, 0, len(counts))
	for tag, count := range counts {
		entries = append(entries, tagCount{Tag: tag, Count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count != entries[j].Count {
			return entries[i].Count > entries[j].Count
		}
		return entries[i].Tag < entries[j].Tag
	})
	return entries
}

// writeTagsJSON prints the report as a single JSON object.
func writeTagsJSON(w io.Writer, report tagsReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// writeTagsText prints the report in a plain, human-readable layout.
func writeTagsText(w io.Writer, report tagsReport) {
	fmt.Fprintln(w, "Dispatch tags")
	fmt.Fprintln(w)

	if report.TotalTags == 0 {
		fmt.Fprintln(w, "No tags found.")
		return
	}

	fmt.Fprintf(w, "Tags:     %d\n", report.TotalTags)
	fmt.Fprintf(w, "Sessions: %d tagged\n", report.TaggedSessions)
	fmt.Fprintln(w)

	width := 0
	for _, e := range report.Tags {
		if len(e.Tag) > width {
			width = len(e.Tag)
		}
	}
	for _, e := range report.Tags {
		fmt.Fprintf(w, "  %-*s  %d\n", width, e.Tag, e.Count)
	}
}
