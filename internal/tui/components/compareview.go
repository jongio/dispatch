package components

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// CompareView renders a side-by-side comparison of two session details.
type CompareView struct {
	left   *data.SessionDetail
	right  *data.SessionDetail
	width  int
	height int
	scroll int // vertical scroll offset
}

// NewCompareView creates an empty CompareView. Call SetSessions to populate.
func NewCompareView() CompareView {
	return CompareView{}
}

// SetSessions configures the two sessions to compare.
func (c *CompareView) SetSessions(a, b *data.SessionDetail) {
	c.left = a
	c.right = b
	c.scroll = 0
}

// SetSize updates the available rendering dimensions.
func (c *CompareView) SetSize(w, h int) {
	c.width = w
	c.height = h
}

// ScrollUp moves the viewport up by one line.
func (c *CompareView) ScrollUp() {
	if c.scroll > 0 {
		c.scroll--
	}
}

// ScrollDown moves the viewport down by one line.
func (c *CompareView) ScrollDown() {
	c.scroll++
}

// View renders the compare overlay content.
func (c *CompareView) View() string {
	if c.left == nil || c.right == nil {
		return ""
	}

	contentW := c.width - 6 // overlay border + padding
	if contentW < 20 {
		contentW = 20
	}

	lines := c.buildLines(contentW)

	// Apply scroll and height clipping.
	visibleH := c.height - 6 // title + footer + border
	if visibleH < 1 {
		visibleH = 1
	}
	if c.scroll > len(lines)-visibleH {
		c.scroll = max(0, len(lines)-visibleH)
	}
	end := c.scroll + visibleH
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[c.scroll:end]

	title := styles.OverlayTitleStyle.Render("Compare Sessions")
	footer := styles.DimmedStyle.Render("esc close  |  c copy  |  ↑↓ scroll")

	body := title + "\n" + strings.Join(visible, "\n") + "\n\n" + footer

	return styles.OverlayStyle.
		Width(contentW).
		Render(body)
}

// PlainText returns an unformatted text summary suitable for clipboard.
func (c *CompareView) PlainText() string {
	if c.left == nil || c.right == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("Session Compare\n")
	b.WriteString(strings.Repeat("=", 60) + "\n\n")

	lbl := func(label, lv, rv string) {
		fmt.Fprintf(&b, "%-16s | %-20s | %s\n", label, lv, rv)
	}

	lbl("ID", short(c.left.Session.ID), short(c.right.Session.ID))
	lbl("Repository", c.left.Session.Repository, c.right.Session.Repository)
	lbl("Branch", c.left.Session.Branch, c.right.Session.Branch)
	lbl("Host Type", c.left.Session.HostType, c.right.Session.HostType)
	lbl("Updated", c.left.Session.UpdatedAt, c.right.Session.UpdatedAt)
	lbl("Turns", fmt.Sprint(len(c.left.Turns)), fmt.Sprint(len(c.right.Turns)))
	lbl("Checkpoints", fmt.Sprint(len(c.left.Checkpoints)), fmt.Sprint(len(c.right.Checkpoints)))
	lbl("Files", fmt.Sprint(len(c.left.Files)), fmt.Sprint(len(c.right.Files)))
	lbl("Refs", fmt.Sprint(len(c.left.Refs)), fmt.Sprint(len(c.right.Refs)))

	// Files comparison.
	leftFiles, rightFiles := filePathSet(c.left.Files), filePathSet(c.right.Files)
	common, onlyLeft, onlyRight := diffSets(leftFiles, rightFiles)

	if len(common) > 0 {
		b.WriteString("\nCommon files:\n")
		for _, f := range common {
			fmt.Fprintf(&b, "  %s\n", f)
		}
	}
	if len(onlyLeft) > 0 {
		fmt.Fprintf(&b, "\nOnly in %s:\n", short(c.left.Session.ID))
		for _, f := range onlyLeft {
			fmt.Fprintf(&b, "  %s\n", f)
		}
	}
	if len(onlyRight) > 0 {
		fmt.Fprintf(&b, "\nOnly in %s:\n", short(c.right.Session.ID))
		for _, f := range onlyRight {
			fmt.Fprintf(&b, "  %s\n", f)
		}
	}

	// Refs comparison.
	leftRefs, rightRefs := refValueSet(c.left.Refs), refValueSet(c.right.Refs)
	commonRefs, onlyLR, onlyRR := diffSets(leftRefs, rightRefs)

	if len(commonRefs) > 0 {
		b.WriteString("\nCommon refs:\n")
		for _, r := range commonRefs {
			fmt.Fprintf(&b, "  %s\n", r)
		}
	}
	if len(onlyLR) > 0 {
		fmt.Fprintf(&b, "\nRefs only in %s:\n", short(c.left.Session.ID))
		for _, r := range onlyLR {
			fmt.Fprintf(&b, "  %s\n", r)
		}
	}
	if len(onlyRR) > 0 {
		fmt.Fprintf(&b, "\nRefs only in %s:\n", short(c.right.Session.ID))
		for _, r := range onlyRR {
			fmt.Fprintf(&b, "  %s\n", r)
		}
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Internal rendering
// ---------------------------------------------------------------------------

func (c *CompareView) buildLines(contentW int) []string {
	colW := (contentW - 3) / 2 // 3 for " | " separator
	if colW < 10 {
		colW = 10
	}

	var lines []string
	sep := styles.DimmedStyle.Render(" | ")

	// Header row with session IDs.
	lID := styles.PreviewTitleStyle.Render(pad(short(c.left.Session.ID), colW))
	rID := styles.PreviewTitleStyle.Render(pad(short(c.right.Session.ID), colW))
	lines = append(lines, lID+sep+rID)
	lines = append(lines, strings.Repeat("─", contentW))

	// Metadata rows.
	metaRows := []struct {
		label string
		lv    string
		rv    string
	}{
		{"Repository", c.left.Session.Repository, c.right.Session.Repository},
		{"Branch", c.left.Session.Branch, c.right.Session.Branch},
		{"Host Type", c.left.Session.HostType, c.right.Session.HostType},
		{"Updated", c.left.Session.UpdatedAt, c.right.Session.UpdatedAt},
		{"Turns", fmt.Sprint(len(c.left.Turns)), fmt.Sprint(len(c.right.Turns))},
		{"Checkpoints", fmt.Sprint(len(c.left.Checkpoints)), fmt.Sprint(len(c.right.Checkpoints))},
		{"Files", fmt.Sprint(len(c.left.Files)), fmt.Sprint(len(c.right.Files))},
		{"Refs", fmt.Sprint(len(c.left.Refs)), fmt.Sprint(len(c.right.Refs))},
	}

	for _, row := range metaRows {
		label := styles.PreviewLabelStyle.Render(row.label)
		lv := renderValue(row.lv, colW, row.lv != row.rv)
		rv := renderValue(row.rv, colW, row.lv != row.rv)
		lines = append(lines, label)
		lines = append(lines, lv+sep+rv)
	}

	// File diff section.
	leftFiles, rightFiles := filePathSet(c.left.Files), filePathSet(c.right.Files)
	common, onlyLeft, onlyRight := diffSets(leftFiles, rightFiles)

	lines = append(lines, "")
	lines = append(lines, styles.PreviewLabelStyle.Render("Files"))

	if len(common) > 0 {
		lines = append(lines, styles.DimmedStyle.Render(fmt.Sprintf("  Common (%d):", len(common))))
		for _, f := range common {
			lines = append(lines, "    "+f)
		}
	}
	if len(onlyLeft) > 0 {
		lines = append(lines, styles.SuccessStyle.Render(fmt.Sprintf("  Only left (%d):", len(onlyLeft))))
		for _, f := range onlyLeft {
			lines = append(lines, "    "+f)
		}
	}
	if len(onlyRight) > 0 {
		lines = append(lines, styles.ErrorStyle.Render(fmt.Sprintf("  Only right (%d):", len(onlyRight))))
		for _, f := range onlyRight {
			lines = append(lines, "    "+f)
		}
	}

	// Ref diff section.
	leftRefs, rightRefs := refValueSet(c.left.Refs), refValueSet(c.right.Refs)
	commonRefs, onlyLR, onlyRR := diffSets(leftRefs, rightRefs)

	lines = append(lines, "")
	lines = append(lines, styles.PreviewLabelStyle.Render("Refs"))

	if len(commonRefs) > 0 {
		lines = append(lines, styles.DimmedStyle.Render(fmt.Sprintf("  Common (%d):", len(commonRefs))))
		for _, r := range commonRefs {
			lines = append(lines, "    "+r)
		}
	}
	if len(onlyLR) > 0 {
		lines = append(lines, styles.SuccessStyle.Render(fmt.Sprintf("  Only left (%d):", len(onlyLR))))
		for _, r := range onlyLR {
			lines = append(lines, "    "+r)
		}
	}
	if len(onlyRR) > 0 {
		lines = append(lines, styles.ErrorStyle.Render(fmt.Sprintf("  Only right (%d):", len(onlyRR))))
		for _, r := range onlyRR {
			lines = append(lines, "    "+r)
		}
	}

	if len(common) == 0 && len(onlyLeft) == 0 && len(onlyRight) == 0 &&
		len(commonRefs) == 0 && len(onlyLR) == 0 && len(onlyRR) == 0 {
		lines = append(lines, styles.DimmedStyle.Render("  (no files or refs)"))
	}

	return lines
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// short truncates a session ID to 8 characters for display.
func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// pad right-pads a string with spaces to the given width using lipgloss.
func pad(s string, w int) string {
	return lipgloss.NewStyle().Width(w).Render(s)
}

// renderValue styles a cell value; highlights when different.
func renderValue(v string, w int, different bool) string {
	style := styles.PreviewValueStyle.Width(w)
	if different {
		style = style.Bold(true)
	}
	if v == "" {
		v = "(empty)"
	}
	return style.Render(v)
}

// filePathSet extracts file paths from a slice of SessionFile into a set.
func filePathSet(files []data.SessionFile) map[string]struct{} {
	m := make(map[string]struct{}, len(files))
	for _, f := range files {
		m[f.FilePath] = struct{}{}
	}
	return m
}

// refValueSet extracts "type:value" strings from refs into a set.
func refValueSet(refs []data.SessionRef) map[string]struct{} {
	m := make(map[string]struct{}, len(refs))
	for _, r := range refs {
		m[r.RefType+":"+r.RefValue] = struct{}{}
	}
	return m
}

// diffSets computes the intersection, left-only, and right-only elements
// from two string sets, returning sorted slices.
func diffSets(a, b map[string]struct{}) (common, onlyA, onlyB []string) {
	for k := range a {
		if _, ok := b[k]; ok {
			common = append(common, k)
		} else {
			onlyA = append(onlyA, k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			onlyB = append(onlyB, k)
		}
	}
	sort.Strings(common)
	sort.Strings(onlyA)
	sort.Strings(onlyB)
	return
}
