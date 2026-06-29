package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// Command represents a single executable action in the command palette.
type Command struct {
	Name        string
	Shortcut    string
	Description string
	Action      string          // unique action ID dispatched on selection
	Enabled     func() bool     // returns false to disable/hide the command
}

// CmdPalette renders a filterable, executable command overlay.
type CmdPalette struct {
	commands []Command
	filtered []int // indices into commands
	cursor   int
	filter   string
	width    int
	height   int
}

// NewCmdPalette returns an empty CmdPalette.
func NewCmdPalette() CmdPalette {
	return CmdPalette{}
}

// SetCommands populates the palette and resets filter state.
func (p *CmdPalette) SetCommands(cmds []Command) {
	p.commands = cmds
	p.filter = ""
	p.cursor = 0
	p.refilter()
}

// SetSize updates the overlay dimensions.
func (p *CmdPalette) SetSize(w, h int) {
	p.width = w
	p.height = h
}

// Filter returns the current filter text.
func (p *CmdPalette) Filter() string {
	return p.filter
}

// SetFilter updates the filter string and recalculates visible commands.
func (p *CmdPalette) SetFilter(s string) {
	p.filter = s
	p.cursor = 0
	p.refilter()
}

// TypeRune appends a rune to the filter.
func (p *CmdPalette) TypeRune(r rune) {
	p.filter += string(r)
	p.cursor = 0
	p.refilter()
}

// Backspace removes the last rune from the filter.
func (p *CmdPalette) Backspace() {
	if len(p.filter) == 0 {
		return
	}
	runes := []rune(p.filter)
	p.filter = string(runes[:len(runes)-1])
	p.cursor = 0
	p.refilter()
}

// MoveUp moves the cursor up, stopping at the top.
func (p *CmdPalette) MoveUp() {
	if p.cursor > 0 {
		p.cursor--
	}
}

// MoveDown moves the cursor down, stopping at the bottom.
func (p *CmdPalette) MoveDown() {
	if p.cursor < len(p.filtered)-1 {
		p.cursor++
	}
}

// Selected returns the action ID of the currently highlighted command.
// The second return value is false if no command is highlighted.
func (p *CmdPalette) Selected() (string, bool) {
	if len(p.filtered) == 0 {
		return "", false
	}
	if p.cursor < 0 || p.cursor >= len(p.filtered) {
		return "", false
	}
	cmd := p.commands[p.filtered[p.cursor]]
	if cmd.Enabled != nil && !cmd.Enabled() {
		return "", false
	}
	return cmd.Action, true
}

// FilteredCount returns the number of visible commands.
func (p *CmdPalette) FilteredCount() int {
	return len(p.filtered)
}

// View renders the command palette overlay.
func (p CmdPalette) View() string {
	title := styles.OverlayTitleStyle.Render("Command Palette")

	var body strings.Builder
	body.WriteString(title + "\n")

	// Filter input line.
	promptStyle := lipgloss.NewStyle().Foreground(styles.ColorPrimary)
	body.WriteString(promptStyle.Render(": ") + p.filter + "\u2588\n\n") // block cursor

	if len(p.filtered) == 0 {
		body.WriteString(styles.DimmedStyle.Render("  No matching commands"))
	} else {
		// Determine visible window for scrolling.
		maxVisible := p.maxVisibleItems()
		offset := 0
		if p.cursor >= maxVisible {
			offset = p.cursor - maxVisible + 1
		}
		end := offset + maxVisible
		if end > len(p.filtered) {
			end = len(p.filtered)
		}

		shortcutStyle := lipgloss.NewStyle().
			Foreground(styles.ColorPrimary).
			Width(8).
			Align(lipgloss.Right)
		descStyle := lipgloss.NewStyle().Foreground(styles.ColorText)

		for vi := offset; vi < end; vi++ {
			idx := p.filtered[vi]
			cmd := p.commands[idx]
			disabled := cmd.Enabled != nil && !cmd.Enabled()

			indicator := "  "
			if vi == p.cursor {
				indicator = "\u25b8 " // ▸
			}

			shortcut := shortcutStyle.Render(cmd.Shortcut)
			name := cmd.Name
			if cmd.Description != "" {
				name += " " + styles.DimmedStyle.Render(cmd.Description)
			}

			line := indicator + shortcut + " " + descStyle.Render(name)
			if vi == p.cursor && !disabled {
				line = styles.SelectedStyle.Render(indicator + cmd.Shortcut + " " + cmd.Name)
			} else if disabled {
				line = styles.DimmedStyle.Render(indicator + cmd.Shortcut + " " + cmd.Name)
			}

			body.WriteString(line + "\n")
		}
	}

	body.WriteString("\n" + styles.DimmedStyle.Render("Enter to run \u00b7 Esc to cancel"))

	maxW := min(60, p.width-4)
	maxW = max(maxW, 30)

	overlay := styles.OverlayStyle.
		Width(maxW).
		Render(body.String())

	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, overlay)
}

// maxVisibleItems returns how many items can be shown given the overlay height.
func (p CmdPalette) maxVisibleItems() int {
	// Reserve lines for title, filter input, blank line, footer, borders/padding.
	reserved := 8
	available := p.height - reserved
	if available < 5 {
		available = 5
	}
	return available
}

// refilter rebuilds the filtered index list based on the current filter string.
// Commands whose Enabled func returns false are excluded.
func (p *CmdPalette) refilter() {
	p.filtered = p.filtered[:0]
	lower := strings.ToLower(p.filter)
	for i, cmd := range p.commands {
		if cmd.Enabled != nil && !cmd.Enabled() {
			continue
		}
		if lower == "" {
			p.filtered = append(p.filtered, i)
			continue
		}
		// Match against name, shortcut, and description.
		if strings.Contains(strings.ToLower(cmd.Name), lower) ||
			strings.Contains(strings.ToLower(cmd.Shortcut), lower) ||
			strings.Contains(strings.ToLower(cmd.Description), lower) {
			p.filtered = append(p.filtered, i)
		}
	}
}
