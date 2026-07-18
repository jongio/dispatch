package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
)

// pruneAllSessionIDsFn returns the IDs of every session in the store, including
// empty ones. It is a package variable so tests can substitute a fixed set. It
// must NOT use the filtered session list (which drops empty sessions), or prune
// would delete config metadata for real-but-empty sessions.
var pruneAllSessionIDsFn = defaultPruneAllSessionIDs

// defaultPruneAllSessionIDs loads all session IDs from the default store.
func defaultPruneAllSessionIDs() ([]string, error) {
	store, err := data.Open()
	if err != nil {
		return nil, fmt.Errorf("opening session store: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only, best-effort close

	ids, err := store.AllSessionIDs(context.Background())
	if err != nil {
		return nil, fmt.Errorf("listing session IDs: %w", err)
	}
	return ids, nil
}

// pruneCategory describes stale entries found in one config section.
type pruneCategory struct {
	Name    string   `json:"name"`
	Stale   int      `json:"stale"`
	Kept    int      `json:"kept"`
	Removed []string `json:"removed,omitempty"`
}

// pruneReport is the aggregate output for the prune command.
type pruneReport struct {
	TotalStale int             `json:"total_stale"`
	Applied    bool            `json:"applied"`
	Categories []pruneCategory `json:"categories"`
}

// pruneOptions holds parsed flags.
type pruneOptions struct {
	apply bool
	json  bool
}

// runPrune scans session-keyed config entries and reports (or removes) entries
// whose session ID is no longer in the store.
func runPrune(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	opts, err := parsePruneArgs(args)
	if err != nil {
		return err
	}

	cfg, err := configLoadFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	ids, err := pruneAllSessionIDsFn()
	if err != nil {
		return err
	}

	liveIDs := make(map[string]bool, len(ids))
	for _, id := range ids {
		liveIDs[id] = true
	}

	report := buildPruneReport(cfg, liveIDs)
	report.Applied = opts.apply

	// Safety guard: an empty store would classify every config entry as stale
	// and wipe all of it. That is almost always a misconfiguration (wrong DB
	// path) rather than genuine intent, so refuse to apply.
	if opts.apply && len(liveIDs) == 0 && report.TotalStale > 0 {
		return fmt.Errorf("refusing to prune: the session store has no sessions, so applying would remove all %d config entries; if this is intentional, edit the config file directly", report.TotalStale)
	}

	if opts.apply && report.TotalStale > 0 {
		applyPrune(cfg, liveIDs)
		if err := configSaveFn(cfg); err != nil {
			return err
		}
	}

	if opts.json {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	writePruneText(w, report)
	return nil
}

// parsePruneArgs reads the prune subcommand flags.
func parsePruneArgs(args []string) (pruneOptions, error) {
	var opts pruneOptions
	rest := args
	if len(rest) > 0 {
		rest = rest[1:]
	}
	for _, arg := range rest {
		switch {
		case arg == "--apply":
			opts.apply = true
		case arg == "--json":
			opts.json = true
		case strings.HasPrefix(arg, "-"):
			return pruneOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			return pruneOptions{}, fmt.Errorf("prune does not take positional arguments, got %q", arg)
		}
	}
	return opts, nil
}

// buildPruneReport scans each config section for stale entries.
func buildPruneReport(cfg *config.Config, liveIDs map[string]bool) pruneReport {
	report := pruneReport{Categories: []pruneCategory{}}

	report.Categories = append(
		report.Categories,
		pruneMapStringString("aliases", cfg.SessionAliases, liveIDs),
		pruneMapStringSlice("tags", cfg.SessionTags, liveIDs),
		pruneMapStringString("notes", cfg.SessionNotes, liveIDs),
		pruneMapStringLaunch("launches", cfg.SessionLaunches, liveIDs),
		pruneStringSlice("favorites", cfg.FavoriteSessions, liveIDs),
		pruneStringSlice("hidden", cfg.HiddenSessions, liveIDs),
	)

	for _, cat := range report.Categories {
		report.TotalStale += cat.Stale
	}
	return report
}

// applyPrune removes stale entries from the config in place.
func applyPrune(cfg *config.Config, liveIDs map[string]bool) {
	pruneMapInPlace(cfg.SessionAliases, liveIDs)
	pruneMapSliceInPlace(cfg.SessionTags, liveIDs)
	pruneMapInPlace(cfg.SessionNotes, liveIDs)
	pruneMapLaunchInPlace(cfg.SessionLaunches, liveIDs)
	cfg.FavoriteSessions = filterSlice(cfg.FavoriteSessions, liveIDs)
	cfg.HiddenSessions = filterSlice(cfg.HiddenSessions, liveIDs)
}

// pruneMapStringString reports stale entries in a map[string]string keyed by
// session ID.
func pruneMapStringString(name string, m map[string]string, liveIDs map[string]bool) pruneCategory {
	cat := pruneCategory{Name: name}
	for id := range m {
		if liveIDs[id] {
			cat.Kept++
		} else {
			cat.Stale++
			cat.Removed = append(cat.Removed, id)
		}
	}
	sort.Strings(cat.Removed)
	return cat
}

// pruneMapStringSlice reports stale entries in a map[string][]string keyed by
// session ID.
func pruneMapStringSlice(name string, m map[string][]string, liveIDs map[string]bool) pruneCategory {
	cat := pruneCategory{Name: name}
	for id := range m {
		if liveIDs[id] {
			cat.Kept++
		} else {
			cat.Stale++
			cat.Removed = append(cat.Removed, id)
		}
	}
	sort.Strings(cat.Removed)
	return cat
}

// pruneMapStringLaunch reports stale entries in the SessionLaunches map.
func pruneMapStringLaunch(name string, m map[string]config.SessionLaunch, liveIDs map[string]bool) pruneCategory {
	cat := pruneCategory{Name: name}
	for id := range m {
		if liveIDs[id] {
			cat.Kept++
		} else {
			cat.Stale++
			cat.Removed = append(cat.Removed, id)
		}
	}
	sort.Strings(cat.Removed)
	return cat
}

// pruneStringSlice reports stale entries in a string slice of session IDs.
func pruneStringSlice(name string, ids []string, liveIDs map[string]bool) pruneCategory {
	cat := pruneCategory{Name: name}
	for _, id := range ids {
		if liveIDs[id] {
			cat.Kept++
		} else {
			cat.Stale++
			cat.Removed = append(cat.Removed, id)
		}
	}
	sort.Strings(cat.Removed)
	return cat
}

// pruneMapInPlace deletes keys from a map[string]string that are not in liveIDs.
func pruneMapInPlace(m map[string]string, liveIDs map[string]bool) {
	for id := range m {
		if !liveIDs[id] {
			delete(m, id)
		}
	}
}

// pruneMapSliceInPlace deletes keys from a map[string][]string that are not in liveIDs.
func pruneMapSliceInPlace(m map[string][]string, liveIDs map[string]bool) {
	for id := range m {
		if !liveIDs[id] {
			delete(m, id)
		}
	}
}

// pruneMapLaunchInPlace deletes keys from the SessionLaunches map that are not in liveIDs.
func pruneMapLaunchInPlace(m map[string]config.SessionLaunch, liveIDs map[string]bool) {
	for id := range m {
		if !liveIDs[id] {
			delete(m, id)
		}
	}
}

// filterSlice returns a new slice containing only IDs present in liveIDs.
func filterSlice(ids []string, liveIDs map[string]bool) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if liveIDs[id] {
			result = append(result, id)
		}
	}
	return result
}

// writePruneText prints the prune report in a human-readable layout.
func writePruneText(w io.Writer, report pruneReport) {
	if report.TotalStale == 0 {
		fmt.Fprintln(w, "Nothing to prune. All config entries reference existing sessions.")
		return
	}

	if report.Applied {
		fmt.Fprintf(w, "Pruned %d stale config entries.\n\n", report.TotalStale)
	} else {
		fmt.Fprintf(w, "Found %d stale config entries (run with --apply to remove).\n\n", report.TotalStale)
	}

	for _, cat := range report.Categories {
		if cat.Stale == 0 {
			continue
		}
		fmt.Fprintf(w, "  %s: %d stale, %d kept\n", cat.Name, cat.Stale, cat.Kept)
		limit := 5
		for i, id := range cat.Removed {
			if i >= limit {
				fmt.Fprintf(w, "    ... and %d more\n", cat.Stale-limit)
				break
			}
			fmt.Fprintf(w, "    %s\n", id)
		}
	}
}
