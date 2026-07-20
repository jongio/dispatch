package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
)

// Function variables allow test substitution of external calls, matching the
// pattern used elsewhere in this package (see cli.go and open.go).
var (
	exportGetDetailFn    = defaultExportGetDetail
	exportDirFn          = data.ExportDir
	exportListSessionsFn = defaultStatsListSessions
)

// exportOptions holds the parsed flags for the export command.
type exportOptions struct {
	id     string
	format string // "md", "json", "html", or "text"
	stdout bool
	outDir string
	redact bool
	filter *data.FilterOptions // non-nil for batch mode
}

// runExport writes a session's full content. It writes to the exports directory
// by default, or to stdout with --stdout. args is the full argument slice with
// args[0] == "export".
func runExport(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	opts, err := parseExportArgs(args)
	if err != nil {
		return err
	}

	if opts.filter != nil {
		return runExportBatch(w, opts)
	}

	detail, err := exportGetDetailFn(opts.id)
	if err != nil {
		return err
	}
	if detail == nil {
		return fmt.Errorf("session %q not found", opts.id)
	}

	if opts.redact {
		detail = redactedSessionDetail(detail)
	}

	content, err := renderExport(detail, opts.format)
	if err != nil {
		return err
	}

	if opts.stdout {
		_, err := io.WriteString(w, content)
		return err
	}

	dir := opts.outDir
	if dir == "" {
		dir, err = exportDirFn()
		if err != nil {
			return err
		}
	}

	path, err := writeExportFile(dir, detail.Session.ID, opts.format, content)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "Exported session to %s\n", path)
	return nil
}

// parseExportArgs extracts the session ID and options from the "export"
// subcommand arguments. args[0] is expected to be "export".
func parseExportArgs(args []string) (exportOptions, error) {
	opts := exportOptions{format: "md"}

	rest := args
	if len(rest) > 0 {
		rest = rest[1:] // drop the "export" token
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

	var positionals []string
	var filter data.FilterOptions
	hasFilter := false
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		name, inline, hasInline := splitFlag(arg)

		switch {
		case name == "--format" || name == "-f":
			v, ni, err := takeValue(i, "--format", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return exportOptions{}, err
			}
			f, err := normalizeExportFormat(v)
			if err != nil {
				return exportOptions{}, err
			}
			opts.format = f
			i = ni
		case name == "--out" || name == "-o":
			v, ni, err := takeValue(i, "--out", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return exportOptions{}, err
			}
			opts.outDir = v
			i = ni
		case name == "--stdout":
			opts.stdout = true
		case name == "--redact":
			opts.redact = true
		case name == "--query" || name == "-q":
			v, ni, err := takeValue(i, "--query", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return exportOptions{}, err
			}
			filter.Query = v
			hasFilter = true
			i = ni
		case name == "--repo":
			v, ni, err := takeValue(i, "--repo", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return exportOptions{}, err
			}
			filter.Repository = v
			hasFilter = true
			i = ni
		case name == "--branch":
			v, ni, err := takeValue(i, "--branch", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return exportOptions{}, err
			}
			filter.Branch = v
			hasFilter = true
			i = ni
		case name == "--folder":
			v, ni, err := takeValue(i, "--folder", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return exportOptions{}, err
			}
			filter.Folder = v
			hasFilter = true
			i = ni
		case name == "--since":
			v, ni, err := takeValue(i, "--since", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return exportOptions{}, err
			}
			t, ok := parseStatsTime(v)
			if !ok {
				return exportOptions{}, fmt.Errorf("invalid --since value %q (want YYYY-MM-DD or RFC3339)", v)
			}
			filter.Since = &t
			hasFilter = true
			i = ni
		case name == "--until":
			v, ni, err := takeValue(i, "--until", inlineOrEmpty(inline, hasInline))
			if err != nil {
				return exportOptions{}, err
			}
			t, ok := parseStatsTime(v)
			if !ok {
				return exportOptions{}, fmt.Errorf("invalid --until value %q (want YYYY-MM-DD or RFC3339)", v)
			}
			filter.Until = &t
			hasFilter = true
			i = ni
		case strings.HasPrefix(arg, "-"):
			return exportOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}

	switch {
	case len(positionals) == 0 && hasFilter:
		opts.filter = &filter
	case len(positionals) == 0 && !hasFilter:
		return exportOptions{}, errors.New("export requires a session ID or filter flags (--query, --repo, --branch, --folder, --since, --until)")
	case len(positionals) == 1 && !hasFilter:
		opts.id = positionals[0]
	case len(positionals) == 1 && hasFilter:
		return exportOptions{}, errors.New("cannot combine a session ID with filter flags; use one or the other")
	default:
		return exportOptions{}, fmt.Errorf("export accepts a single session ID, got %d arguments", len(positionals))
	}

	if opts.stdout && opts.outDir != "" {
		return exportOptions{}, errors.New("--stdout and --out cannot be used together")
	}

	return opts, nil
}

// normalizeExportFormat maps a user-facing format string to a canonical export
// format.
func normalizeExportFormat(format string) (string, error) {
	switch strings.ToLower(format) {
	case "md", "markdown":
		return "md", nil
	case "json":
		return "json", nil
	case "html":
		return "html", nil
	case "txt", "text":
		return "text", nil
	default:
		return "", fmt.Errorf("invalid format %q (want md, json, html, or text)", format)
	}
}

func redactedSessionDetail(detail *data.SessionDetail) *data.SessionDetail {
	if detail == nil {
		return nil
	}
	redacted := *detail
	redacted.Session = detail.Session
	redacted.Session.Cwd = platform.RedactSecrets(redacted.Session.Cwd)
	redacted.Session.Repository = platform.RedactSecrets(redacted.Session.Repository)
	redacted.Session.Branch = platform.RedactSecrets(redacted.Session.Branch)
	redacted.Session.Summary = platform.RedactSecrets(redacted.Session.Summary)

	if len(detail.Turns) > 0 {
		redacted.Turns = append([]data.Turn(nil), detail.Turns...)
		for i := range redacted.Turns {
			redacted.Turns[i].UserMessage = platform.RedactSecrets(redacted.Turns[i].UserMessage)
			redacted.Turns[i].AssistantResponse = platform.RedactSecrets(redacted.Turns[i].AssistantResponse)
		}
	}
	if len(detail.Checkpoints) > 0 {
		redacted.Checkpoints = append([]data.Checkpoint(nil), detail.Checkpoints...)
		for i := range redacted.Checkpoints {
			redacted.Checkpoints[i].Title = platform.RedactSecrets(redacted.Checkpoints[i].Title)
			redacted.Checkpoints[i].Overview = platform.RedactSecrets(redacted.Checkpoints[i].Overview)
			redacted.Checkpoints[i].History = platform.RedactSecrets(redacted.Checkpoints[i].History)
			redacted.Checkpoints[i].WorkDone = platform.RedactSecrets(redacted.Checkpoints[i].WorkDone)
			redacted.Checkpoints[i].TechnicalDetails = platform.RedactSecrets(redacted.Checkpoints[i].TechnicalDetails)
			redacted.Checkpoints[i].ImportantFiles = platform.RedactSecrets(redacted.Checkpoints[i].ImportantFiles)
			redacted.Checkpoints[i].NextSteps = platform.RedactSecrets(redacted.Checkpoints[i].NextSteps)
		}
	}
	if len(detail.Files) > 0 {
		redacted.Files = append([]data.SessionFile(nil), detail.Files...)
		for i := range redacted.Files {
			redacted.Files[i].FilePath = platform.RedactSecrets(redacted.Files[i].FilePath)
			redacted.Files[i].ToolName = platform.RedactSecrets(redacted.Files[i].ToolName)
		}
	}
	if len(detail.Refs) > 0 {
		redacted.Refs = append([]data.SessionRef(nil), detail.Refs...)
		for i := range redacted.Refs {
			redacted.Refs[i].RefValue = platform.RedactSecrets(redacted.Refs[i].RefValue)
		}
	}
	return &redacted
}

// renderExport produces the export content for the given format.
func renderExport(detail *data.SessionDetail, format string) (string, error) {
	switch format {
	case "json":
		b, err := json.MarshalIndent(detail, "", "  ")
		if err != nil {
			return "", fmt.Errorf("encoding session as JSON: %w", err)
		}
		return string(b) + "\n", nil
	case "html":
		return data.RenderHTML(detail), nil
	case "text":
		return data.RenderText(detail), nil
	default:
		return data.RenderMarkdown(detail), nil
	}
}

// writeExportFile writes content to <dir>/<safe-id>.<ext> and returns the path.
func writeExportFile(dir, id, format, content string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating export directory: %w", err)
	}
	ext := "md"
	switch format {
	case "json":
		ext = "json"
	case "html":
		ext = "html"
	case "text":
		ext = "txt"
	}
	path := filepath.Join(dir, data.SafeFilename(id)+"."+ext)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("writing export file: %w", err)
	}
	return path, nil
}

// defaultExportGetDetail loads a full session detail by ID from the default
// session store. The ID may be a full session ID or a unique short prefix. It
// returns (nil, nil) when no session matches, and an *data.AmbiguousIDPrefixError
// when a short prefix matches more than one session.
func defaultExportGetDetail(id string) (*data.SessionDetail, error) {
	store, err := data.Open()
	if err != nil {
		return nil, fmt.Errorf("opening session store: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only, best-effort close

	ctx := context.Background()
	fullID, err := store.ResolveIDPrefix(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	detail, err := store.GetSession(ctx, fullID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return detail, nil
}

// runExportBatch exports all sessions matching the filter flags. Each session
// is exported to a separate file in the output directory.
func runExportBatch(w io.Writer, opts exportOptions) error {
	if opts.stdout {
		return errors.New("--stdout is not supported in batch mode; use --out to set the output directory")
	}

	sessions, err := exportListSessionsFn(*opts.filter)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		fmt.Fprintln(w, "No sessions match the given filters.")
		return nil
	}

	outDir := opts.outDir
	if outDir == "" {
		d, dirErr := exportDirFn()
		if dirErr != nil {
			return dirErr
		}
		outDir = d
	}

	var exported int
	for _, s := range sessions {
		detail, detailErr := exportGetDetailFn(s.ID)
		if detailErr != nil {
			fmt.Fprintf(w, "Skipping %s: %v\n", shortID(s.ID), detailErr)
			continue
		}
		if detail == nil {
			fmt.Fprintf(w, "Skipping %s: session not found\n", shortID(s.ID))
			continue
		}
		if opts.redact {
			detail = redactedSessionDetail(detail)
		}
		content, renderErr := renderExport(detail, opts.format)
		if renderErr != nil {
			fmt.Fprintf(w, "Skipping %s: %v\n", shortID(s.ID), renderErr)
			continue
		}
		path, writeErr := writeExportFile(outDir, s.ID, opts.format, content)
		if writeErr != nil {
			fmt.Fprintf(w, "Skipping %s: %v\n", shortID(s.ID), writeErr)
			continue
		}
		fmt.Fprintf(w, "Exported %s to %s\n", shortID(s.ID), path)
		exported++
	}

	fmt.Fprintf(w, "\nExported %d of %d sessions to %s\n", exported, len(sessions), outDir)
	return nil
}
