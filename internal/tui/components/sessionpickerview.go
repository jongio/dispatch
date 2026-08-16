package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// SessionPickerRow contains the display-ready text for one resume candidate.
type SessionPickerRow struct {
	ID         string
	Repository string
	Branch     string
	Summary    string
}

// DefaultSessionPickerBatchSize is the number of additional rows revealed at once.
const DefaultSessionPickerBatchSize = 50

// SessionPickerView renders the terminal content shared by the resume command
// and deterministic website screenshots.
type SessionPickerView struct {
	Rows    []SessionPickerRow
	Cursor  int
	Offset  int
	Visible int
	Width   int
	Height  int
	IDWidth int
}

func (v SessionPickerView) Render() string {
	var b strings.Builder
	if v.hasMore() {
		fmt.Fprintf(&b, "Select a session (%d of %d)\n\n", v.Visible, len(v.Rows))
	} else {
		fmt.Fprintf(&b, "Select a session (%d)\n\n", len(v.Rows))
	}

	idWidth, summaryWidth, repoWidth, branchWidth := v.columnWidths()
	fmt.Fprintf(
		&b,
		"  %s  %s  %s  %s\n",
		padSessionPickerCell("SESSION ID", idWidth),
		padSessionPickerCell("SUMMARY", summaryWidth),
		padSessionPickerCell("REPOSITORY", repoWidth),
		padSessionPickerCell("BRANCH", branchWidth),
	)
	fmt.Fprintf(
		&b,
		"  %s  %s  %s  %s\n",
		strings.Repeat("-", idWidth),
		strings.Repeat("-", summaryWidth),
		strings.Repeat("-", repoWidth),
		strings.Repeat("-", branchWidth),
	)

	end := min(v.selectableCount(), v.Offset+v.visibleRows())
	for i := v.Offset; i < end; i++ {
		prefix := "  "
		if i == v.Cursor {
			prefix = "> "
		}
		if i == v.Visible {
			remaining := len(v.Rows) - v.Visible
			count := min(DefaultSessionPickerBatchSize, remaining)
			fmt.Fprintf(&b, "%sShow %d more sessions (%d remaining)\n", prefix, count, remaining)
			continue
		}
		row := v.Rows[i]
		fmt.Fprintf(
			&b,
			"%s%s  %s  %s  %s\n",
			prefix,
			padSessionPickerCell(row.ID, idWidth),
			padSessionPickerCell(row.Summary, summaryWidth),
			padSessionPickerCell(row.Repository, repoWidth),
			padSessionPickerCell(row.Branch, branchWidth),
		)
	}

	b.WriteString("\n↑/↓ move · Enter resume")
	if v.hasMore() {
		b.WriteString(" · m more")
	}
	b.WriteString(" · q cancel")
	return b.String()
}

func (v SessionPickerView) hasMore() bool {
	return v.Visible < len(v.Rows)
}

func (v SessionPickerView) selectableCount() int {
	count := v.Visible
	if v.hasMore() {
		count++
	}
	return count
}

func (v SessionPickerView) visibleRows() int {
	return SessionPickerVisibleRows(v.Height)
}

// SessionPickerVisibleRows returns the number of table rows available at a
// terminal height after accounting for the title, header, and footer.
func SessionPickerVisibleRows(height int) int {
	return max(1, height-6)
}

func (v SessionPickerView) columnWidths() (int, int, int, int) {
	idWidth := max(v.IDWidth, ansi.StringWidth("SESSION ID"))
	if v.IDWidth <= 0 {
		for _, row := range v.Rows {
			idWidth = max(idWidth, ansi.StringWidth(row.ID))
		}
	}

	const (
		minSummaryWidth = 8
		minRepoWidth    = 10
		minBranchWidth  = 6
		targetSummary   = 24
		targetRepo      = 28
		targetBranch    = 22
	)
	summaryWidth := minSummaryWidth
	repoWidth := minRepoWidth
	branchWidth := minBranchWidth

	extra := max(0, v.Width-2-idWidth-6-summaryWidth-repoWidth-branchWidth)
	grow := func(width *int, target int) {
		add := min(extra, target-*width)
		*width += add
		extra -= add
	}
	grow(&summaryWidth, targetSummary)
	grow(&repoWidth, targetRepo)
	grow(&branchWidth, targetBranch)
	summaryWidth += extra

	return idWidth, summaryWidth, repoWidth, branchWidth
}

func padSessionPickerCell(value string, width int) string {
	value = ansi.Truncate(value, width, "…")
	return PadToWidth(value, width)
}
