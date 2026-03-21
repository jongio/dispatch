// Package markdown provides Glamour-based markdown rendering for
// TUI preview panes.
package markdown

import (
	"strings"

	"github.com/charmbracelet/glamour"
	gansi "github.com/charmbracelet/glamour/ansi"
	gstyles "github.com/charmbracelet/glamour/styles"
)

// Style returns a customised Glamour style config based on the dark theme
// with heading prefixes removed for cleaner TUI rendering.
func Style() gansi.StyleConfig {
	s := gstyles.DarkStyleConfig
	s.H2.Prefix = ""
	s.H3.Prefix = ""
	s.H4.Prefix = ""
	s.H5.Prefix = ""
	s.H6.Prefix = ""
	return s
}

// RenderStatic renders markdown content using Glamour and returns the
// result as a slice of lines. Falls back to plain text if rendering
// fails. Safe for concurrent use in tea.Cmd goroutines.
func RenderStatic(source string, width int) []string {
	if width <= 0 {
		width = 80
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(Style()),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return strings.Split(source, "\n")
	}

	rendered, err := renderer.Render(source)
	if err != nil {
		return strings.Split(source, "\n")
	}

	rendered = strings.TrimRight(rendered, "\n")
	return strings.Split(rendered, "\n")
}
