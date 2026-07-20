package data

import (
	"fmt"
	"html"
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

// RenderText formats a SessionDetail as plain text for systems that do not
// render Markdown or HTML.
func RenderText(detail *SessionDetail) string {
	if detail == nil {
		return ""
	}

	s := detail.Session
	var b strings.Builder

	b.WriteString("Session: " + s.Summary + "\n\n")

	b.WriteString("Metadata\n")
	fmt.Fprintf(&b, "ID: %s\n", s.ID)
	fmt.Fprintf(&b, "Folder: %s\n", s.Cwd)
	if s.Repository != "" {
		fmt.Fprintf(&b, "Repository: %s\n", s.Repository)
	}
	if s.Branch != "" {
		fmt.Fprintf(&b, "Branch: %s\n", s.Branch)
	}
	fmt.Fprintf(&b, "Created: %s\n", s.CreatedAt)
	fmt.Fprintf(&b, "Last Active: %s\n", s.LastActiveAt)
	fmt.Fprintf(&b, "Turns: %d\n", s.TurnCount)
	fmt.Fprintf(&b, "Files: %d\n\n", s.FileCount)

	if len(detail.Turns) > 0 {
		b.WriteString("Conversation\n\n")
		for _, turn := range detail.Turns {
			if turn.UserMessage != "" {
				b.WriteString("User:\n")
				b.WriteString(turn.UserMessage + "\n\n")
			}
			if turn.AssistantResponse != "" {
				b.WriteString("Assistant:\n")
				b.WriteString(turn.AssistantResponse + "\n\n")
			}
		}
	}

	if len(detail.Checkpoints) > 0 {
		b.WriteString("Checkpoints\n\n")
		for _, cp := range detail.Checkpoints {
			fmt.Fprintf(&b, "%d. %s\n", cp.CheckpointNumber, cp.Title)
			if cp.Overview != "" {
				b.WriteString(cp.Overview + "\n")
			}
			b.WriteString("\n")
		}
	}

	if len(detail.Files) > 0 {
		b.WriteString("Files Touched\n\n")
		seen := make(map[string]struct{})
		for _, f := range detail.Files {
			if _, ok := seen[f.FilePath]; ok {
				continue
			}
			seen[f.FilePath] = struct{}{}
			fmt.Fprintf(&b, "- %s (%s)\n", f.FilePath, f.ToolName)
		}
		b.WriteString("\n")
	}

	if len(detail.Refs) > 0 {
		b.WriteString("References\n\n")
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

// htmlExportStyle is the inline stylesheet for HTML exports. It is embedded in
// the document so the file renders without any external requests.
const htmlExportStyle = `body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;line-height:1.5;color:#1f2328;background:#fff;margin:0;padding:2rem;max-width:56rem;margin-left:auto;margin-right:auto}
h1{font-size:1.6rem;margin:0 0 1rem}
h2{font-size:1.2rem;margin:2rem 0 .75rem;border-bottom:1px solid #d0d7de;padding-bottom:.3rem}
table{border-collapse:collapse;margin:0 0 1rem}
th,td{border:1px solid #d0d7de;padding:.35rem .6rem;text-align:left;vertical-align:top}
th{background:#f6f8fa}
.turn{border:1px solid #d0d7de;border-radius:6px;margin:0 0 1rem;overflow:hidden}
.turn .role{font-weight:600;padding:.4rem .75rem;background:#f6f8fa;border-bottom:1px solid #d0d7de}
.turn.user .role{background:#ddf4ff}
.turn .body{padding:.75rem;margin:0;white-space:pre-wrap;word-wrap:break-word;font-family:inherit}
code{background:#eff1f3;padding:.1rem .3rem;border-radius:4px}
ul{padding-left:1.25rem}`

// RenderHTML formats a SessionDetail as a single self-contained HTML document
// suitable for reading in a browser. The stylesheet is inlined so the file has
// no external references. All session and conversation text is escaped so
// content cannot inject markup.
func RenderHTML(detail *SessionDetail) string {
	if detail == nil {
		return ""
	}

	s := detail.Session
	esc := html.EscapeString
	var b strings.Builder

	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>Session: %s</title>\n", esc(s.Summary))
	fmt.Fprintf(&b, "<style>\n%s\n</style>\n", htmlExportStyle)
	b.WriteString("</head>\n<body>\n")

	fmt.Fprintf(&b, "<h1>Session: %s</h1>\n", esc(s.Summary))

	// Metadata.
	b.WriteString("<h2>Metadata</h2>\n<table>\n")
	b.WriteString("<tr><th>Field</th><th>Value</th></tr>\n")
	fmt.Fprintf(&b, "<tr><td>ID</td><td><code>%s</code></td></tr>\n", esc(s.ID))
	fmt.Fprintf(&b, "<tr><td>Folder</td><td><code>%s</code></td></tr>\n", esc(s.Cwd))
	if s.Repository != "" {
		fmt.Fprintf(&b, "<tr><td>Repository</td><td>%s</td></tr>\n", esc(s.Repository))
	}
	if s.Branch != "" {
		fmt.Fprintf(&b, "<tr><td>Branch</td><td>%s</td></tr>\n", esc(s.Branch))
	}
	fmt.Fprintf(&b, "<tr><td>Created</td><td>%s</td></tr>\n", esc(s.CreatedAt))
	fmt.Fprintf(&b, "<tr><td>Last Active</td><td>%s</td></tr>\n", esc(s.LastActiveAt))
	fmt.Fprintf(&b, "<tr><td>Turns</td><td>%d</td></tr>\n", s.TurnCount)
	fmt.Fprintf(&b, "<tr><td>Files</td><td>%d</td></tr>\n", s.FileCount)
	b.WriteString("</table>\n")

	// Conversation.
	if len(detail.Turns) > 0 {
		b.WriteString("<h2>Conversation</h2>\n")
		for _, turn := range detail.Turns {
			if turn.UserMessage != "" {
				b.WriteString("<div class=\"turn user\">\n<div class=\"role\">User</div>\n")
				fmt.Fprintf(&b, "<div class=\"body\">%s</div>\n</div>\n", esc(turn.UserMessage))
			}
			if turn.AssistantResponse != "" {
				b.WriteString("<div class=\"turn assistant\">\n<div class=\"role\">Assistant</div>\n")
				fmt.Fprintf(&b, "<div class=\"body\">%s</div>\n</div>\n", esc(turn.AssistantResponse))
			}
		}
	}

	// Checkpoints.
	if len(detail.Checkpoints) > 0 {
		b.WriteString("<h2>Checkpoints</h2>\n")
		for _, cp := range detail.Checkpoints {
			fmt.Fprintf(&b, "<h3>%d. %s</h3>\n", cp.CheckpointNumber, esc(cp.Title))
			if cp.Overview != "" {
				fmt.Fprintf(&b, "<p>%s</p>\n", esc(cp.Overview))
			}
		}
	}

	// Files.
	if len(detail.Files) > 0 {
		b.WriteString("<h2>Files Touched</h2>\n<ul>\n")
		seen := make(map[string]struct{})
		for _, f := range detail.Files {
			if _, ok := seen[f.FilePath]; ok {
				continue
			}
			seen[f.FilePath] = struct{}{}
			fmt.Fprintf(&b, "<li><code>%s</code> (%s)</li>\n", esc(f.FilePath), esc(f.ToolName))
		}
		b.WriteString("</ul>\n")
	}

	// References.
	if len(detail.Refs) > 0 {
		b.WriteString("<h2>References</h2>\n<ul>\n")
		seen := make(map[string]struct{})
		for _, ref := range detail.Refs {
			key := ref.RefType + ":" + ref.RefValue
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			fmt.Fprintf(&b, "<li>%s: %s</li>\n", esc(ref.RefType), esc(ref.RefValue))
		}
		b.WriteString("</ul>\n")
	}

	b.WriteString("</body>\n</html>\n")
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
