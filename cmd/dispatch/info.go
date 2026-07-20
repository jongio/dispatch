package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jongio/dispatch/internal/data"
)

// infoGetDetailFn loads a session's aggregated detail by ID. It is a seam so
// tests can substitute the store lookup. It reuses the export loader, which
// returns (nil, nil) when no session with the given ID exists.
var infoGetDetailFn = defaultExportGetDetail

// sessionInfo is the concise, count-oriented view of a session emitted by the
// info command. Unlike export, it summarizes the conversation with counts
// rather than including every turn.
type sessionInfo struct {
	ID           string    `json:"id"`
	Summary      string    `json:"summary,omitempty"`
	Repository   string    `json:"repository,omitempty"`
	Branch       string    `json:"branch,omitempty"`
	Directory    string    `json:"directory,omitempty"`
	HostType     string    `json:"host_type,omitempty"`
	CreatedAt    string    `json:"created_at,omitempty"`
	UpdatedAt    string    `json:"updated_at,omitempty"`
	LastActiveAt string    `json:"last_active_at,omitempty"`
	Turns        int       `json:"turns"`
	Files        int       `json:"files"`
	Checkpoints  int       `json:"checkpoints"`
	Commits      int       `json:"commits"`
	PRs          int       `json:"prs"`
	Issues       int       `json:"issues"`
	Refs         *infoRefs `json:"refs,omitempty"`
}

type infoRefs struct {
	Commits []string `json:"commits"`
	PRs     []string `json:"prs"`
	Issues  []string `json:"issues"`
}

// runInfo prints a concise summary of a single session as text, or as JSON
// with --json. The --refs flag adds linked reference values.
func runInfo(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	id, asJSON, includeRefs, err := parseInfoArgs(args)
	if err != nil {
		return err
	}

	detail, err := infoGetDetailFn(id)
	if err != nil {
		return err
	}
	if detail == nil {
		return fmt.Errorf("session %q not found", id)
	}

	info := buildSessionInfo(detail)
	if includeRefs {
		addInfoRefs(&info, detail.Refs)
	}
	if asJSON {
		return writeInfoJSON(w, info)
	}
	return writeInfoText(w, info)
}

// parseInfoArgs extracts the session ID and flags from the info
// subcommand arguments. args[0] is expected to be "info".
func parseInfoArgs(args []string) (id string, asJSON, includeRefs bool, err error) {
	rest := args
	if len(rest) > 0 {
		rest = rest[1:] // drop the "info" token
	}

	var positionals []string
	for _, arg := range rest {
		switch {
		case arg == "--json":
			asJSON = true
		case arg == "--refs":
			includeRefs = true
		case strings.HasPrefix(arg, "-"):
			return "", false, false, fmt.Errorf("unknown flag: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}

	switch len(positionals) {
	case 0:
		return "", false, false, errors.New("info requires a session ID")
	case 1:
		return positionals[0], asJSON, includeRefs, nil
	default:
		return "", false, false, fmt.Errorf("info accepts a single session ID, got %d arguments", len(positionals))
	}
}

// buildSessionInfo reduces a full SessionDetail to the concise info view,
// counting external references by type.
func buildSessionInfo(detail *data.SessionDetail) sessionInfo {
	s := detail.Session
	info := sessionInfo{
		ID:           s.ID,
		Summary:      s.Summary,
		Repository:   s.Repository,
		Branch:       s.Branch,
		Directory:    s.Cwd,
		HostType:     s.HostType,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
		LastActiveAt: s.LastActiveAt,
		Turns:        s.TurnCount,
		Files:        s.FileCount,
		Checkpoints:  len(detail.Checkpoints),
	}
	for _, ref := range detail.Refs {
		switch strings.ToLower(ref.RefType) {
		case "commit":
			info.Commits++
		case "pr":
			info.PRs++
		case "issue":
			info.Issues++
		}
	}
	return info
}

func addInfoRefs(info *sessionInfo, refs []data.SessionRef) {
	info.Refs = &infoRefs{
		Commits: []string{},
		PRs:     []string{},
		Issues:  []string{},
	}
	for _, ref := range refs {
		switch strings.ToLower(ref.RefType) {
		case "commit":
			info.Refs.Commits = append(info.Refs.Commits, ref.RefValue)
		case "pr":
			info.Refs.PRs = append(info.Refs.PRs, ref.RefValue)
		case "issue":
			info.Refs.Issues = append(info.Refs.Issues, ref.RefValue)
		}
	}
}

// writeInfoJSON encodes info as indented JSON.
func writeInfoJSON(w io.Writer, info sessionInfo) error {
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding session info as JSON: %w", err)
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// writeInfoText prints a human-readable summary, one field per line. Optional
// string fields are omitted when empty; counts always print.
func writeInfoText(w io.Writer, info sessionInfo) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Session %s\n", info.ID)

	writeField := func(label, value string) {
		if value != "" {
			fmt.Fprintf(&b, "  %-12s %s\n", label, value)
		}
	}
	writeField("Summary:", info.Summary)
	writeField("Repository:", info.Repository)
	writeField("Branch:", info.Branch)
	writeField("Directory:", info.Directory)
	writeField("Host:", info.HostType)
	writeField("Created:", info.CreatedAt)
	writeField("Updated:", info.UpdatedAt)
	writeField("Last active:", info.LastActiveAt)

	fmt.Fprintf(&b, "  %-12s %d\n", "Turns:", info.Turns)
	fmt.Fprintf(&b, "  %-12s %d\n", "Files:", info.Files)
	fmt.Fprintf(&b, "  %-12s %d\n", "Checkpoints:", info.Checkpoints)
	fmt.Fprintf(&b, "  %-12s %s\n", "Refs:", formatRefCounts(info))
	if info.Refs != nil {
		fmt.Fprintf(&b, "  %-12s %s\n", "Commits:", formatRefList(info.Refs.Commits))
		fmt.Fprintf(&b, "  %-12s %s\n", "PRs:", formatRefList(info.Refs.PRs))
		fmt.Fprintf(&b, "  %-12s %s\n", "Issues:", formatRefList(info.Refs.Issues))
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// formatRefCounts renders the reference breakdown as a compact summary such as
// "3 commits, 1 pr, 0 issues", using singular labels when a count is one.
func formatRefCounts(info sessionInfo) string {
	return fmt.Sprintf("%s, %s, %s",
		pluralize(info.Commits, "commit", "commits"),
		pluralize(info.PRs, "pr", "prs"),
		pluralize(info.Issues, "issue", "issues"))
}

// pluralize renders n with a singular label when n is exactly one.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func formatRefList(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}
