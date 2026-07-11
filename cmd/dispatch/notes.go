package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
)

// notesListSessionsFn loads sessions for the notes command. It is a package
// variable so tests can substitute a fixed set of sessions.
var notesListSessionsFn = defaultStatsListSessions

type noteEntry struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Note    string `json:"note"`
}

type notesReport struct {
	TotalNotes int         `json:"total_notes"`
	Notes      []noteEntry `json:"notes"`
}

func runNotes(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	rest := args
	if len(rest) > 0 {
		rest = rest[1:]
	}
	if len(rest) == 0 || rest[0] == "list" || rest[0] == "--json" {
		return runNotesList(w, rest)
	}

	switch rest[0] {
	case "get":
		return runNotesGet(w, rest[1:])
	case "set":
		return runNotesSet(w, rest[1:])
	case "clear":
		return runNotesClear(w, rest[1:])
	default:
		return fmt.Errorf("unknown notes subcommand %q (want list, get, set, or clear)", rest[0])
	}
}

func runNotesList(w io.Writer, args []string) error {
	jsonOut := false
	if len(args) > 0 && args[0] == "list" {
		args = args[1:]
	}
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("notes list does not take arguments, got %q", arg)
		}
	}

	cfg, err := configLoadFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	sessions, err := notesListSessionsFn(data.FilterOptions{})
	if err != nil {
		return err
	}
	report := buildNotesReport(cfg, sessions)
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	writeNotesText(w, report)
	return nil
}

func runNotesGet(w io.Writer, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("notes get requires a session ID")
	}
	cfg, err := configLoadFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	fmt.Fprintln(w, cfg.SessionNotes[args[0]])
	return nil
}

func runNotesSet(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("notes set requires a session ID and note text")
	}
	sessionID := args[0]
	note := strings.Join(args[1:], " ")
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("notes set requires a session ID")
	}
	cfg, err := configLoadFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	setSessionNote(cfg, sessionID, note)
	if err := configSaveFn(cfg); err != nil {
		return err
	}
	fmt.Fprintf(w, "Set note for %s\n", sessionID)
	return nil
}

func runNotesClear(w io.Writer, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("notes clear requires a session ID")
	}
	sessionID := args[0]
	cfg, err := configLoadFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	setSessionNote(cfg, sessionID, "")
	if err := configSaveFn(cfg); err != nil {
		return err
	}
	fmt.Fprintf(w, "Cleared note for %s\n", sessionID)
	return nil
}

func setSessionNote(cfg *config.Config, sessionID, note string) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	if note == "" {
		delete(cfg.SessionNotes, sessionID)
		return
	}
	if cfg.SessionNotes == nil {
		cfg.SessionNotes = make(map[string]string)
	}
	cfg.SessionNotes[sessionID] = note
}

func buildNotesReport(cfg *config.Config, sessions []data.Session) notesReport {
	report := notesReport{Notes: []noteEntry{}}
	if cfg == nil || len(cfg.SessionNotes) == 0 {
		return report
	}

	for _, s := range sessions {
		note := cfg.SessionNotes[s.ID]
		if note == "" {
			continue
		}
		report.Notes = append(report.Notes, noteEntry{ID: s.ID, Summary: s.Summary, Note: note})
	}
	sort.Slice(report.Notes, func(i, j int) bool {
		return report.Notes[i].ID < report.Notes[j].ID
	})
	report.TotalNotes = len(report.Notes)
	return report
}

func writeNotesText(w io.Writer, report notesReport) {
	fmt.Fprintln(w, "Dispatch notes")
	fmt.Fprintln(w)
	if report.TotalNotes == 0 {
		fmt.Fprintln(w, "No notes found.")
		return
	}
	fmt.Fprintf(w, "Notes: %d\n\n", report.TotalNotes)
	idWidth := 0
	for _, entry := range report.Notes {
		if len(entry.ID) > idWidth {
			idWidth = len(entry.ID)
		}
	}
	for _, entry := range report.Notes {
		fmt.Fprintf(w, "  %-*s  %s\n", idWidth, entry.ID, entry.Note)
		if entry.Summary != "" {
			fmt.Fprintf(w, "  %-*s  %s\n", idWidth, "", entry.Summary)
		}
	}
}
