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
)

// Function variables allow test substitution of external calls, matching the
// pattern used elsewhere in this package (see cli.go and open.go).
var (
	exportGetDetailFn = defaultExportGetDetail
	exportDirFn       = data.ExportDir
)

// exportOptions holds the parsed flags for the export command.
type exportOptions struct {
	id     string
	format string // "md", "json", or "html"
	stdout bool
	outDir string
}

// runExport writes a session's full content as Markdown or JSON. It writes to
// the exports directory by default, or to stdout with --stdout. args is the
// full argument slice with args[0] == "export".
func runExport(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	opts, err := parseExportArgs(args)
	if err != nil {
		return err
	}

	detail, err := exportGetDetailFn(opts.id)
	if err != nil {
		return err
	}
	if detail == nil {
		return fmt.Errorf("session %q not found", opts.id)
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
		case strings.HasPrefix(arg, "-"):
			return exportOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}

	switch len(positionals) {
	case 0:
		return exportOptions{}, errors.New("export requires a session ID")
	case 1:
		opts.id = positionals[0]
	default:
		return exportOptions{}, fmt.Errorf("export accepts a single session ID, got %d arguments", len(positionals))
	}

	if opts.stdout && opts.outDir != "" {
		return exportOptions{}, errors.New("--stdout and --out cannot be used together")
	}

	return opts, nil
}

// normalizeExportFormat maps a user-facing format string to "md", "json", or
// "html".
func normalizeExportFormat(format string) (string, error) {
	switch strings.ToLower(format) {
	case "md", "markdown":
		return "md", nil
	case "json":
		return "json", nil
	case "html":
		return "html", nil
	default:
		return "", fmt.Errorf("invalid format %q (want md, json, or html)", format)
	}
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
	}
	path := filepath.Join(dir, data.SafeFilename(id)+"."+ext)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("writing export file: %w", err)
	}
	return path, nil
}

// defaultExportGetDetail loads a full session detail by ID from the default
// session store. It returns (nil, nil) when no session with that ID exists.
func defaultExportGetDetail(id string) (*data.SessionDetail, error) {
	store, err := data.Open()
	if err != nil {
		return nil, fmt.Errorf("opening session store: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only, best-effort close

	detail, err := store.GetSession(context.Background(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return detail, nil
}
