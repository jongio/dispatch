package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jongio/dispatch/internal/data"
)

// compareGetDetailFn loads a session's aggregated detail by ID. It is a seam
// so tests can substitute the store lookup without touching the environment.
var compareGetDetailFn = defaultExportGetDetail

// sessionComparison is the structured result of comparing two sessions, used
// for both the text and JSON outputs.
type sessionComparison struct {
	Left  sessionSide `json:"left"`
	Right sessionSide `json:"right"`

	MetadataDiffs  []metadataDiff `json:"metadata_diffs"`
	FilesOnlyLeft  []string       `json:"files_only_left"`
	FilesOnlyRight []string       `json:"files_only_right"`
	RefsOnlyLeft   []string       `json:"refs_only_left"`
	RefsOnlyRight  []string       `json:"refs_only_right"`
}

// sessionSide summarises one side of the comparison.
type sessionSide struct {
	ID               string   `json:"id"`
	Summary          string   `json:"summary,omitempty"`
	Repository       string   `json:"repository,omitempty"`
	Branch           string   `json:"branch,omitempty"`
	Directory        string   `json:"directory,omitempty"`
	HostType         string   `json:"host_type,omitempty"`
	CreatedAt        string   `json:"created_at,omitempty"`
	UpdatedAt        string   `json:"updated_at,omitempty"`
	Turns            int      `json:"turns"`
	Files            int      `json:"files"`
	Checkpoints      int      `json:"checkpoints"`
	CheckpointTitles []string `json:"checkpoint_titles,omitempty"`
}

// metadataDiff records a single field that differs between the two sessions.
type metadataDiff struct {
	Field string `json:"field"`
	Left  string `json:"left"`
	Right string `json:"right"`
}

// runCompare prints a comparison of two sessions as text, or as JSON with
// --json. args is the full argument slice with args[0] == "compare".
func runCompare(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	leftID, rightID, asJSON, err := parseCompareArgs(args)
	if err != nil {
		return err
	}

	leftDetail, err := compareGetDetailFn(leftID)
	if err != nil {
		return err
	}
	if leftDetail == nil {
		return fmt.Errorf("session %q not found", leftID)
	}

	rightDetail, err := compareGetDetailFn(rightID)
	if err != nil {
		return err
	}
	if rightDetail == nil {
		return fmt.Errorf("session %q not found", rightID)
	}

	cmp := buildComparison(leftDetail, rightDetail)
	if asJSON {
		return writeCompareJSON(w, cmp)
	}
	return writeCompareText(w, cmp)
}

// parseCompareArgs extracts the two session IDs and the --json flag from the
// compare subcommand arguments. args[0] is expected to be "compare".
func parseCompareArgs(args []string) (leftID, rightID string, asJSON bool, err error) {
	rest := args
	if len(rest) > 0 {
		rest = rest[1:] // drop the "compare" token
	}

	var positionals []string
	for _, arg := range rest {
		switch {
		case arg == "--json":
			asJSON = true
		case strings.HasPrefix(arg, "-"):
			return "", "", false, fmt.Errorf("unknown flag: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}

	switch len(positionals) {
	case 0:
		return "", "", false, errors.New("compare requires two session IDs")
	case 1:
		return "", "", false, errors.New("compare requires two session IDs")
	case 2:
		return positionals[0], positionals[1], asJSON, nil
	default:
		return "", "", false, fmt.Errorf("compare accepts exactly two session IDs, got %d", len(positionals))
	}
}

// buildComparison produces a sessionComparison from two loaded session details.
func buildComparison(left, right *data.SessionDetail) sessionComparison {
	cmp := sessionComparison{
		Left:  buildSide(left),
		Right: buildSide(right),
	}

	// Metadata diffs.
	addDiff := func(field, lv, rv string) {
		if lv != rv {
			cmp.MetadataDiffs = append(cmp.MetadataDiffs, metadataDiff{Field: field, Left: lv, Right: rv})
		}
	}
	addDiff("summary", left.Session.Summary, right.Session.Summary)
	addDiff("repository", left.Session.Repository, right.Session.Repository)
	addDiff("branch", left.Session.Branch, right.Session.Branch)
	addDiff("directory", left.Session.Cwd, right.Session.Cwd)
	addDiff("host_type", left.Session.HostType, right.Session.HostType)
	addDiff("created_at", left.Session.CreatedAt, right.Session.CreatedAt)
	addDiff("updated_at", left.Session.UpdatedAt, right.Session.UpdatedAt)

	addCountDiff := func(field string, lv, rv int) {
		if lv != rv {
			cmp.MetadataDiffs = append(cmp.MetadataDiffs, metadataDiff{
				Field: field,
				Left:  fmt.Sprintf("%d", lv),
				Right: fmt.Sprintf("%d", rv),
			})
		}
	}
	addCountDiff("turns", left.Session.TurnCount, right.Session.TurnCount)
	addCountDiff("files", left.Session.FileCount, right.Session.FileCount)
	addCountDiff("checkpoints", len(left.Checkpoints), len(right.Checkpoints))

	// File diffs.
	leftFiles := filePathSet(left.Files)
	rightFiles := filePathSet(right.Files)
	cmp.FilesOnlyLeft = setDiff(leftFiles, rightFiles)
	cmp.FilesOnlyRight = setDiff(rightFiles, leftFiles)

	// Ref diffs.
	leftRefs := refStringSet(left.Refs)
	rightRefs := refStringSet(right.Refs)
	cmp.RefsOnlyLeft = setDiff(leftRefs, rightRefs)
	cmp.RefsOnlyRight = setDiff(rightRefs, leftRefs)

	return cmp
}

// buildSide creates the summary view for one side of the comparison.
func buildSide(detail *data.SessionDetail) sessionSide {
	s := detail.Session
	side := sessionSide{
		ID:          s.ID,
		Summary:     s.Summary,
		Repository:  s.Repository,
		Branch:      s.Branch,
		Directory:   s.Cwd,
		HostType:    s.HostType,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
		Turns:       s.TurnCount,
		Files:       s.FileCount,
		Checkpoints: len(detail.Checkpoints),
	}
	for _, cp := range detail.Checkpoints {
		if cp.Title != "" {
			side.CheckpointTitles = append(side.CheckpointTitles, cp.Title)
		}
	}
	return side
}

// filePathSet returns the unique file paths as a set (map keys).
func filePathSet(files []data.SessionFile) map[string]struct{} {
	result := make(map[string]struct{}, len(files))
	for _, f := range files {
		result[f.FilePath] = struct{}{}
	}
	return result
}

// refStringSet returns "type:value" strings as a set for comparison.
func refStringSet(refs []data.SessionRef) map[string]struct{} {
	result := make(map[string]struct{}, len(refs))
	for _, r := range refs {
		key := strings.ToLower(r.RefType) + ":" + r.RefValue
		result[key] = struct{}{}
	}
	return result
}

// setDiff returns the sorted keys present in a but absent from b.
func setDiff(a, b map[string]struct{}) []string {
	var diff []string
	for k := range a {
		if _, ok := b[k]; !ok {
			diff = append(diff, k)
		}
	}
	sort.Strings(diff)
	return diff
}

// writeCompareJSON encodes the comparison as indented JSON.
func writeCompareJSON(w io.Writer, cmp sessionComparison) error {
	b, err := json.MarshalIndent(cmp, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding comparison as JSON: %w", err)
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// writeCompareText prints a human-readable diff summary.
func writeCompareText(w io.Writer, cmp sessionComparison) error {
	var b strings.Builder

	fmt.Fprintf(&b, "Comparing sessions\n")
	fmt.Fprintf(&b, "  Left:  %s\n", cmp.Left.ID)
	fmt.Fprintf(&b, "  Right: %s\n", cmp.Right.ID)
	b.WriteString("\n")

	// Metadata diffs.
	if len(cmp.MetadataDiffs) == 0 {
		b.WriteString("Metadata: identical\n")
	} else {
		b.WriteString("Metadata differences:\n")
		for _, d := range cmp.MetadataDiffs {
			fmt.Fprintf(&b, "  %-14s %s -> %s\n", d.Field+":", d.Left, d.Right)
		}
	}
	b.WriteString("\n")

	// Checkpoint titles.
	writeCompareList(&b, "Checkpoint titles (left):", cmp.Left.CheckpointTitles)
	writeCompareList(&b, "Checkpoint titles (right):", cmp.Right.CheckpointTitles)

	// File diffs.
	writeCompareList(&b, "Files only in left:", cmp.FilesOnlyLeft)
	writeCompareList(&b, "Files only in right:", cmp.FilesOnlyRight)

	// Ref diffs.
	writeCompareList(&b, "Refs only in left:", cmp.RefsOnlyLeft)
	writeCompareList(&b, "Refs only in right:", cmp.RefsOnlyRight)

	_, err := io.WriteString(w, b.String())
	return err
}

// writeCompareList appends a labeled list to the builder, or "(none)" when
// the slice is empty.
func writeCompareList(b *strings.Builder, label string, items []string) {
	fmt.Fprintf(b, "%s\n", label)
	if len(items) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for _, item := range items {
			fmt.Fprintf(b, "  %s\n", item)
		}
	}
}
