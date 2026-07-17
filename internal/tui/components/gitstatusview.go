package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// GitStatusView renders a detailed Git status overlay for the working directory
// a session is mapped to: branch, upstream, push/pull (ahead/behind) counts,
// working-tree category counts, and a scrollable list of changed files.
type GitStatusView struct {
	status platform.GitStatus
	set    bool
	width  int
	height int
	scroll int // vertical scroll offset into the changed-file list
}

// NewGitStatusView returns an empty GitStatusView. Call SetStatus to populate.
func NewGitStatusView() GitStatusView {
	return GitStatusView{}
}

// SetStatus configures the git status to display and resets scroll.
func (g *GitStatusView) SetStatus(s platform.GitStatus) {
	g.status = s
	g.set = true
	g.scroll = 0
}

// SetSize updates the available rendering dimensions.
func (g *GitStatusView) SetSize(w, h int) {
	g.width = w
	g.height = h
}

// ScrollUp moves the changed-file viewport up by one line.
func (g *GitStatusView) ScrollUp() {
	if g.scroll > 0 {
		g.scroll--
	}
}

// ScrollDown moves the changed-file viewport down by one line.
func (g *GitStatusView) ScrollDown() {
	g.scroll++
}

// View renders the git status overlay, centered in the available area.
func (g *GitStatusView) View() string {
	if !g.set {
		return ""
	}

	contentW := min(64, g.width-4)
	contentW = max(contentW, 30)

	header, files := g.buildLines(contentW)

	// The header (metadata) is always visible; only the file list scrolls.
	visibleH := g.height - len(header) - 8 // title + footer + border + padding
	if visibleH < 1 {
		visibleH = 1
	}
	if g.scroll > len(files)-visibleH {
		g.scroll = max(0, len(files)-visibleH)
	}
	if g.scroll < 0 {
		g.scroll = 0
	}
	end := g.scroll + visibleH
	if end > len(files) {
		end = len(files)
	}
	var visibleFiles []string
	if len(files) > 0 {
		visibleFiles = files[g.scroll:end]
	}

	title := styles.OverlayTitleStyle.Render("Git Status")
	body := title + "\n" + strings.Join(header, "\n")
	if len(visibleFiles) > 0 {
		body += "\n" + strings.Join(visibleFiles, "\n")
	}
	footer := styles.DimmedStyle.Render("esc close  |  c copy  |  ↑↓ scroll")
	body += "\n\n" + footer

	overlay := styles.OverlayStyle.Width(contentW).Render(body)
	if g.width == 0 || g.height == 0 {
		return overlay
	}
	return lipgloss.Place(g.width, g.height, lipgloss.Center, lipgloss.Center, overlay)
}

// PlainText returns an unstyled summary suitable for the clipboard.
func (g *GitStatusView) PlainText() string {
	if !g.set {
		return ""
	}
	s := g.status
	var b strings.Builder
	b.WriteString("Git Status\n")
	fmt.Fprintf(&b, "Path: %s\n", s.Dir)

	if !s.Exists {
		b.WriteString("Directory not found\n")
		return b.String()
	}
	if !s.IsRepo {
		b.WriteString("Not a git repository\n")
		return b.String()
	}

	fmt.Fprintf(&b, "Branch: %s\n", branchLabel(s))
	if s.HasUpstream {
		fmt.Fprintf(&b, "Upstream: %s\n", s.Upstream)
		fmt.Fprintf(&b, "Push/Pull: %d ahead, %d behind\n", s.Ahead, s.Behind)
	} else {
		b.WriteString("Upstream: (none)\n")
	}

	if s.Clean() {
		b.WriteString("Working tree: clean\n")
	} else {
		fmt.Fprintf(&b, "Staged: %d  Modified: %d  Untracked: %d  Deleted: %d  Conflicts: %d\n",
			s.Staged, s.Modified, s.Untracked, s.Deleted, s.Conflicts)
	}

	if len(s.Files) > 0 {
		b.WriteString("\nChanged files:\n")
		for _, f := range s.Files {
			fmt.Fprintf(&b, "  %s %s\n", f.Code, f.Path)
		}
		if s.Truncated {
			b.WriteString("  … (list truncated)\n")
		}
	}
	return b.String()
}

// buildLines renders the always-visible metadata header and the scrollable
// changed-file lines separately so the header stays pinned while files scroll.
func (g *GitStatusView) buildLines(contentW int) (header, files []string) {
	s := g.status

	header = append(header, row("Path", truncPath(s.Dir, contentW-14)))

	if !s.Exists {
		header = append(header, styles.GitMissingStyle.Render("Directory not found"))
		return header, nil
	}
	if !s.IsRepo {
		header = append(header, styles.DimmedStyle.Render("Not a git repository"))
		return header, nil
	}

	header = append(header, row("Branch", branchLabel(s)))
	header = append(header, row("Upstream", upstreamLabel(s)))
	header = append(header, row("Push/Pull", pushPullLabel(s)))
	header = append(header, "")
	header = append(header, row("Working", workingLabel(s)))

	if !s.Clean() {
		header = append(header, countsLine(s))
	}

	if len(s.Files) > 0 {
		header = append(header, "")
		label := fmt.Sprintf("Changed files (%d)", len(s.Files))
		if s.Truncated {
			label += " — truncated"
		}
		header = append(header, styles.PreviewLabelStyle.Render(label))
		for _, f := range s.Files {
			files = append(files, fileLine(f, contentW))
		}
	}
	return header, files
}

// ---------------------------------------------------------------------------
// Rendering helpers
// ---------------------------------------------------------------------------

// row renders a "label  value" line with a fixed-width styled label.
func row(label, value string) string {
	l := styles.PreviewLabelStyle.Render(fmt.Sprintf("%-10s", label))
	return l + " " + styles.PreviewValueStyle.Render(value)
}

// branchLabel returns the branch name, marking a detached HEAD.
func branchLabel(s platform.GitStatus) string {
	if s.Detached {
		return "detached HEAD"
	}
	if s.Branch == "" {
		return "(unknown)"
	}
	return s.Branch
}

// upstreamLabel returns the upstream ref or a "none" placeholder.
func upstreamLabel(s platform.GitStatus) string {
	if !s.HasUpstream || s.Upstream == "" {
		return styles.DimmedStyle.Render("(none)")
	}
	return s.Upstream
}

// pushPullLabel renders the standard ahead/behind push/pull stats with icons
// and colors, or a note when there is no upstream to compare against.
func pushPullLabel(s platform.GitStatus) string {
	if !s.HasUpstream {
		return styles.DimmedStyle.Render("no upstream")
	}
	ahead := styles.GitAheadStyle.Render(fmt.Sprintf("%s%d", styles.IconGitAhead(), s.Ahead))
	behind := styles.GitBehindStyle.Render(fmt.Sprintf("%s%d", styles.IconGitBehind(), s.Behind))
	push := styles.DimmedStyle.Render(" to push")
	pull := styles.DimmedStyle.Render(" to pull")
	if s.Ahead == 0 && s.Behind == 0 {
		return styles.SuccessStyle.Render("up to date")
	}
	return ahead + push + "   " + behind + pull
}

// workingLabel returns "clean" (styled) or a short dirty summary.
func workingLabel(s platform.GitStatus) string {
	if s.Clean() {
		return styles.GitCleanStyle.Render("clean")
	}
	return styles.GitDirtyStyle.Render("changes")
}

// countsLine renders the per-category working-tree counts, omitting zeros.
func countsLine(s platform.GitStatus) string {
	var parts []string
	add := func(label string, n int, st lipgloss.Style) {
		if n > 0 {
			parts = append(parts, st.Render(fmt.Sprintf("%s %d", label, n)))
		}
	}
	add("staged", s.Staged, styles.SuccessStyle)
	add("modified", s.Modified, styles.GitDirtyStyle)
	add("untracked", s.Untracked, styles.GitUntrackedStyle)
	add("deleted", s.Deleted, styles.ErrorStyle)
	add("conflicts", s.Conflicts, styles.ErrorStyle)
	return "           " + strings.Join(parts, "  ")
}

// fileLine renders a single changed-file entry: "<code> <path>".
func fileLine(f platform.GitFileStatus, contentW int) string {
	code := styles.DimmedStyle.Render(fmt.Sprintf("%-2s", f.Code))
	return "  " + code + " " + truncPath(f.Path, contentW-6)
}

// truncPath left-truncates a path with an ellipsis when it exceeds width.
// It operates on runes (not bytes) so multi-byte UTF-8 paths are sliced
// at character boundaries.
func truncPath(p string, width int) string {
	if width < 4 {
		width = 4
	}
	runes := []rune(p)
	if len(runes) <= width {
		return p
	}
	return "…" + string(runes[len(runes)-(width-1):])
}
