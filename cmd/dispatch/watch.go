package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/jongio/dispatch/internal/data"
)

// watchScanAttentionFn scans session attention state. It is a package variable
// so tests can substitute a controlled snapshot.
var watchScanAttentionFn = func(threshold time.Duration) map[string]data.AttentionStatus {
	return data.ScanAttention(threshold, true)
}

// watchListSessionsFn loads sessions so watch can apply filters and resolve
// metadata. It is a package variable so tests can substitute a fixed set.
var watchListSessionsFn = defaultStatsListSessions

// bellFn writes a BEL character. It is a package variable so tests can
// suppress the audible bell.
var bellFn = func() { fmt.Fprint(os.Stderr, "\a") }

// watchOptions holds parsed flags for the watch command.
type watchOptions struct {
	once     bool
	json     bool
	interval time.Duration
	repo     string
	branch   string
	folder   string
}

// watchSnapshot is the JSON representation of attention state at a point in time.
type watchSnapshot struct {
	Timestamp   string              `json:"timestamp"`
	Total       int                 `json:"total"`
	Waiting     int                 `json:"waiting"`
	Working     int                 `json:"working"`
	Thinking    int                 `json:"thinking"`
	Active      int                 `json:"active"`
	Stale       int                 `json:"stale"`
	Idle        int                 `json:"idle"`
	Compacting  int                 `json:"compacting"`
	Interrupted int                 `json:"interrupted"`
	Sessions    []watchSessionEntry `json:"sessions,omitempty"`
}

// watchSessionEntry describes one session in the watch output.
type watchSessionEntry struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Summary string `json:"summary,omitempty"`
}

// runWatch monitors attention state. With --once it prints a snapshot and
// exits; otherwise it polls and prints transitions.
func runWatch(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	opts, err := parseWatchArgs(args)
	if err != nil {
		return err
	}

	if opts.once {
		return runWatchOnce(w, opts)
	}
	return runWatchStream(w, opts)
}

// parseWatchArgs reads the watch subcommand flags.
func parseWatchArgs(args []string) (watchOptions, error) {
	opts := watchOptions{interval: 5 * time.Second}

	rest := args
	if len(rest) > 0 {
		rest = rest[1:]
	}

	i := 0
	for i < len(rest) {
		arg := rest[i]
		switch {
		case arg == "--once":
			opts.once = true
		case arg == "--json":
			opts.json = true
		case arg == "--interval":
			i++
			if i >= len(rest) {
				return watchOptions{}, fmt.Errorf("--interval requires a duration (e.g. 5s, 1m)")
			}
			d, dErr := time.ParseDuration(rest[i])
			if dErr != nil {
				return watchOptions{}, fmt.Errorf("invalid interval %q: %w", rest[i], dErr)
			}
			if d < time.Second {
				d = time.Second
			}
			opts.interval = d
		case arg == "--repo":
			i++
			if i >= len(rest) {
				return watchOptions{}, fmt.Errorf("--repo requires a value")
			}
			opts.repo = rest[i]
		case arg == "--branch":
			i++
			if i >= len(rest) {
				return watchOptions{}, fmt.Errorf("--branch requires a value")
			}
			opts.branch = rest[i]
		case arg == "--folder":
			i++
			if i >= len(rest) {
				return watchOptions{}, fmt.Errorf("--folder requires a value")
			}
			opts.folder = rest[i]
		case strings.HasPrefix(arg, "-"):
			return watchOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			return watchOptions{}, fmt.Errorf("watch does not take positional arguments, got %q", arg)
		}
		i++
	}

	return opts, nil
}

// runWatchOnce prints a single attention snapshot and exits.
func runWatchOnce(w io.Writer, opts watchOptions) error {
	snap, err := buildWatchSnapshot(opts)
	if err != nil {
		return err
	}
	if opts.json {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(snap)
	}
	writeWatchSnapshotText(w, snap)
	return nil
}

// runWatchStream polls attention state and prints transitions until Ctrl-C.
func runWatchStream(w io.Writer, opts watchOptions) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	prev := map[string]data.AttentionStatus{}
	ticker := time.NewTicker(opts.interval)
	defer ticker.Stop()

	// Print initial snapshot.
	snap, err := buildWatchSnapshot(opts)
	if err != nil {
		return err
	}
	if opts.json {
		enc := json.NewEncoder(w)
		if encErr := enc.Encode(snap); encErr != nil {
			return encErr
		}
	} else {
		writeWatchSnapshotText(w, snap)
		fmt.Fprintln(w)
	}

	// Build initial state for transition detection.
	attention := scanFiltered(opts)
	for id, status := range attention {
		prev[id] = status
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			current := scanFiltered(opts)
			for id, status := range current {
				old, existed := prev[id]
				if !existed || old != status {
					if status == data.AttentionWaiting || status == data.AttentionInterrupted {
						bellFn()
					}
					ts := time.Now().Format("15:04:05")
					if opts.json {
						enc := json.NewEncoder(w)
						_ = enc.Encode(map[string]string{
							"time":   ts,
							"id":     id,
							"status": status.String(),
						})
					} else {
						fmt.Fprintf(w, "[%s] %s  %s\n", ts, shortID(id), status.String())
					}
				}
			}
			// Detect sessions that disappeared.
			for id := range prev {
				if _, ok := current[id]; !ok {
					ts := time.Now().Format("15:04:05")
					if opts.json {
						enc := json.NewEncoder(w)
						_ = enc.Encode(map[string]string{
							"time":   ts,
							"id":     id,
							"status": "gone",
						})
					} else {
						fmt.Fprintf(w, "[%s] %s  gone\n", ts, shortID(id))
					}
				}
			}
			prev = current
		}
	}
}

// scanFiltered runs the attention scan and filters by session metadata.
func scanFiltered(opts watchOptions) map[string]data.AttentionStatus {
	threshold := 15 * time.Minute
	attention := watchScanAttentionFn(threshold)

	if opts.repo == "" && opts.branch == "" && opts.folder == "" {
		return attention
	}

	sessions, err := watchListSessionsFn(data.FilterOptions{
		Repository: opts.repo,
		Branch:     opts.branch,
		Folder:     opts.folder,
	})
	if err != nil {
		return attention
	}

	allowed := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		allowed[s.ID] = true
	}

	filtered := make(map[string]data.AttentionStatus, len(attention))
	for id, status := range attention {
		if allowed[id] {
			filtered[id] = status
		}
	}
	return filtered
}

// buildWatchSnapshot creates a snapshot of the current attention state.
func buildWatchSnapshot(opts watchOptions) (watchSnapshot, error) {
	attention := scanFiltered(opts)

	sessions, err := watchListSessionsFn(data.FilterOptions{
		Repository: opts.repo,
		Branch:     opts.branch,
		Folder:     opts.folder,
	})
	if err != nil {
		return watchSnapshot{}, err
	}

	summaries := make(map[string]string, len(sessions))
	for _, s := range sessions {
		summaries[s.ID] = s.Summary
	}

	snap := watchSnapshot{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	entries := make([]watchSessionEntry, 0, len(attention))
	for id, status := range attention {
		snap.Total++
		switch status {
		case data.AttentionWaiting:
			snap.Waiting++
		case data.AttentionWorking:
			snap.Working++
		case data.AttentionThinking:
			snap.Thinking++
		case data.AttentionActive:
			snap.Active++
		case data.AttentionStale:
			snap.Stale++
		case data.AttentionCompacting:
			snap.Compacting++
		case data.AttentionInterrupted:
			snap.Interrupted++
		default:
			snap.Idle++
		}

		if status == data.AttentionWaiting || status == data.AttentionInterrupted ||
			status == data.AttentionWorking || status == data.AttentionThinking ||
			status == data.AttentionActive {
			entries = append(entries, watchSessionEntry{
				ID:      id,
				Status:  status.String(),
				Summary: summaries[id],
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})
	snap.Sessions = entries
	return snap, nil
}

// writeWatchSnapshotText prints a human-readable attention snapshot.
func writeWatchSnapshotText(w io.Writer, snap watchSnapshot) {
	fmt.Fprintln(w, "Dispatch session attention")
	fmt.Fprintln(w)

	if snap.Total == 0 {
		fmt.Fprintln(w, "No sessions found.")
		return
	}

	fmt.Fprintf(w, "Total: %d", snap.Total)
	parts := []string{}
	if snap.Waiting > 0 {
		parts = append(parts, fmt.Sprintf("%d waiting", snap.Waiting))
	}
	if snap.Working > 0 {
		parts = append(parts, fmt.Sprintf("%d working", snap.Working))
	}
	if snap.Thinking > 0 {
		parts = append(parts, fmt.Sprintf("%d thinking", snap.Thinking))
	}
	if snap.Active > 0 {
		parts = append(parts, fmt.Sprintf("%d active", snap.Active))
	}
	if snap.Stale > 0 {
		parts = append(parts, fmt.Sprintf("%d stale", snap.Stale))
	}
	if snap.Compacting > 0 {
		parts = append(parts, fmt.Sprintf("%d compacting", snap.Compacting))
	}
	if snap.Interrupted > 0 {
		parts = append(parts, fmt.Sprintf("%d interrupted", snap.Interrupted))
	}
	if snap.Idle > 0 {
		parts = append(parts, fmt.Sprintf("%d idle", snap.Idle))
	}
	if len(parts) > 0 {
		fmt.Fprintf(w, " (%s)", strings.Join(parts, ", "))
	}
	fmt.Fprintln(w)

	if len(snap.Sessions) > 0 {
		fmt.Fprintln(w)
		for _, s := range snap.Sessions {
			label := s.Summary
			if label == "" {
				label = "(no summary)"
			}
			fmt.Fprintf(w, "  %-12s  %-10s  %s\n", shortID(s.ID), s.Status, label)
		}
	}
}
