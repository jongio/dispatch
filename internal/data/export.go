package data

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jongio/dispatch/internal/platform"
)

// safeFilenameRE matches characters not allowed in filenames.
var safeFilenameRE = regexp.MustCompile(`[^a-zA-Z0-9_\-.]`)

// SafeFilename returns a filesystem-safe string derived from id.
// Non-alphanumeric characters (excluding hyphens, underscores, dots) are
// replaced with underscores, and the result is truncated to 200 characters.
func SafeFilename(id string) string {
	name := safeFilenameRE.ReplaceAllString(id, "_")
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

// ExportDir returns the path to the export directory under the dispatch
// config directory (e.g. ~/.config/dispatch/exports).
func ExportDir() (string, error) {
	cfgDir, err := platform.ConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving export directory: %w", err)
	}
	return filepath.Join(cfgDir, "exports"), nil
}

// RenderMarkdown formats a SessionDetail as a Markdown document suitable
// for export. The output includes metadata, conversation turns, checkpoints,
// touched files, and external references.
func RenderMarkdown(detail *SessionDetail) string {
	if detail == nil {
		return ""
	}

	s := detail.Session
	var b strings.Builder

	// ── Title ──
	b.WriteString("# Session: " + s.Summary + "\n\n")

	// ── Metadata ──
	b.WriteString("## Metadata\n\n")
	b.WriteString("| Field | Value |\n")
	b.WriteString("|-------|-------|\n")
	fmt.Fprintf(&b, "| ID | `%s` |\n", s.ID)
	fmt.Fprintf(&b, "| Folder | `%s` |\n", s.Cwd)
	if s.Repository != "" {
		fmt.Fprintf(&b, "| Repository | %s |\n", s.Repository)
	}
	if s.Branch != "" {
		fmt.Fprintf(&b, "| Branch | %s |\n", s.Branch)
	}
	fmt.Fprintf(&b, "| Created | %s |\n", s.CreatedAt)
	fmt.Fprintf(&b, "| Last Active | %s |\n", s.LastActiveAt)
	fmt.Fprintf(&b, "| Turns | %d |\n", s.TurnCount)
	fmt.Fprintf(&b, "| Files | %d |\n", s.FileCount)
	b.WriteString("\n")

	// ── Conversation ──
	if len(detail.Turns) > 0 {
		b.WriteString("## Conversation\n\n")
		for _, turn := range detail.Turns {
			if turn.UserMessage != "" {
				b.WriteString("### User\n\n")
				b.WriteString(turn.UserMessage + "\n\n")
			}
			if turn.AssistantResponse != "" {
				b.WriteString("### Assistant\n\n")
				b.WriteString(turn.AssistantResponse + "\n\n")
			}
		}
	}

	// ── Checkpoints ──
	if len(detail.Checkpoints) > 0 {
		b.WriteString("## Checkpoints\n\n")
		for _, cp := range detail.Checkpoints {
			fmt.Fprintf(&b, "### %d. %s\n\n", cp.CheckpointNumber, cp.Title)
			if cp.Overview != "" {
				b.WriteString(cp.Overview + "\n\n")
			}
		}
	}

	// ── Files ──
	if len(detail.Files) > 0 {
		b.WriteString("## Files Touched\n\n")
		seen := make(map[string]struct{})
		for _, f := range detail.Files {
			if _, ok := seen[f.FilePath]; ok {
				continue
			}
			seen[f.FilePath] = struct{}{}
			fmt.Fprintf(&b, "- `%s` (%s)\n", f.FilePath, f.ToolName)
		}
		b.WriteString("\n")
	}

	// ── References ──
	if len(detail.Refs) > 0 {
		b.WriteString("## References\n\n")
		seen := make(map[string]struct{})
		for _, ref := range detail.Refs {
			key := ref.RefType + ":" + ref.RefValue
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			fmt.Fprintf(&b, "- %s: %s\n", ref.RefType, ref.RefValue)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// ExportSession writes a session detail as Markdown to the given directory.
// The file is named <session-id>.md using a sanitized filename. Returns the
// full path of the written file.
func ExportSession(detail *SessionDetail, dir string) (string, error) {
	if detail == nil {
		return "", fmt.Errorf("nil session detail")
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating export directory: %w", err)
	}

	filename := SafeFilename(detail.Session.ID) + ".md"
	path := filepath.Join(dir, filename)

	content := RenderMarkdown(detail)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("writing export file: %w", err)
	}

	return path, nil
}
