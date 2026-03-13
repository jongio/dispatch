package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// ShellPicker renders a modal list of detected shells for session launch.
type ShellPicker struct {
	shells       []platform.ShellInfo
	defaultShell string
	cursor       int
	width        int
	height       int
}

// NewShellPicker returns an empty ShellPicker.
func NewShellPicker() ShellPicker {
	return ShellPicker{}
}

// SetShells replaces the available shells. If defaultShell is non-empty,
// it is placed first in the list with a "(default)" label.
func (s *ShellPicker) SetShells(shells []platform.ShellInfo, defaultShell string) {
	s.defaultShell = defaultShell
	if defaultShell != "" {
		var ordered []platform.ShellInfo
		var rest []platform.ShellInfo
		for _, sh := range shells {
			if sh.Name == defaultShell {
				ordered = append(ordered, sh)
			} else {
				rest = append(rest, sh)
			}
		}
		s.shells = append(ordered, rest...)
	} else {
		s.shells = shells
	}
	s.cursor = 0
}

// SetSize updates the overlay dimensions.
func (s *ShellPicker) SetSize(w, h int) {
	s.width = w
	s.height = h
}

// MoveUp moves the selection up.
func (s *ShellPicker) MoveUp() {
	if s.cursor > 0 {
		s.cursor--
	}
}

// MoveDown moves the selection down.
func (s *ShellPicker) MoveDown() {
	if s.cursor < len(s.shells)-1 {
		s.cursor++
	}
}

// Selected returns the currently highlighted shell.
func (s *ShellPicker) Selected() (platform.ShellInfo, bool) {
	if len(s.shells) == 0 {
		return platform.ShellInfo{}, false
	}
	return s.shells[s.cursor], true
}

// View renders the shell picker overlay.
func (s ShellPicker) View() string {
	title := styles.OverlayTitleStyle.Render("Select Shell")

	var body strings.Builder
	body.WriteString(title + "\n")

	for i, sh := range s.shells {
		indicator := "  "
		if i == s.cursor {
			indicator = "▸ "
		}
		name := sh.Name
		if name == "" {
			name = sh.Path
		}
		if s.defaultShell != "" && sh.Name == s.defaultShell {
			name += " (default)"
		}
		line := indicator + name
		if i == s.cursor {
			line = styles.SelectedStyle.Render(line)
		}
		body.WriteString(line + "\n")
	}
	body.WriteString("\n" + styles.DimmedStyle.Render("Enter to select · Esc to cancel"))

	maxW := min(50, s.width-4)
	maxW = max(maxW, 20)

	overlay := styles.OverlayStyle.
		Width(maxW).
		Render(body.String())

	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, overlay)
}
