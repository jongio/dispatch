package components

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// SearchBar wraps a bubbles textinput for FTS5 session search.
type SearchBar struct {
	input       textinput.Model
	resultCount int
	active      bool
	searching   bool   // deep search in progress
	aiSearching bool   // Copilot SDK search in progress
	aiStatus    string // "connecting", "ready", "error", or "" (not started)
	aiError     string // error message when aiStatus == "error"
	aiResults   int    // count of AI-found sessions in last search
	width       int
}

// NewSearchBar returns a SearchBar ready for use.
func NewSearchBar() SearchBar {
	ti := textinput.New()
	ti.Placeholder = "Search sessions…"
	ti.Prompt = styles.IconSearch() + " "
	ti.CharLimit = 0 // unlimited
	ti.PlaceholderStyle = styles.DimmedStyle
	ti.PromptStyle = styles.SearchPromptStyle
	return SearchBar{input: ti}
}

// Focus activates the search input for typing.
func (s *SearchBar) Focus() tea.Cmd {
	s.active = true
	return s.input.Focus()
}

// Blur deactivates the search input.
func (s *SearchBar) Blur() {
	s.active = false
	s.input.Blur()
}

// Focused returns true when the search bar is capturing keystrokes.
func (s *SearchBar) Focused() bool {
	return s.active
}

// Value returns the current search query.
func (s *SearchBar) Value() string {
	return s.input.Value()
}

// SetValue sets the search text.
func (s *SearchBar) SetValue(v string) {
	s.input.SetValue(v)
}

// SetResultCount updates the displayed result count.
func (s *SearchBar) SetResultCount(n int) {
	s.resultCount = n
}

// SetSearching sets the deep-search-in-progress indicator.
func (s *SearchBar) SetSearching(v bool) {
	s.searching = v
}

// SetAISearching sets the Copilot SDK search-in-progress indicator.
func (s *SearchBar) SetAISearching(v bool) {
	s.aiSearching = v
}

// SetAIStatus sets the Copilot SDK connection status.
// Valid values: "" (not started), "connecting", "ready", "error".
func (s *SearchBar) SetAIStatus(status string) {
	s.aiStatus = status
}

// SetAIError sets the AI error message shown when status is "error".
func (s *SearchBar) SetAIError(msg string) {
	s.aiError = msg
}

// SetAIResults sets the count of AI-found sessions from the last search.
func (s *SearchBar) SetAIResults(n int) {
	s.aiResults = n
}

// SetWidth sets the available width for the search bar.
func (s *SearchBar) SetWidth(w int) {
	s.width = w
	s.input.Width = max(10, w-6) // account for prompt + padding
}

// Update delegates a tea.Msg to the underlying textinput.
func (s SearchBar) Update(msg tea.Msg) (SearchBar, tea.Cmd) {
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

// View renders the search bar.
func (s SearchBar) View() string {
	// Build the status suffix first so we can shrink the textinput
	// window to leave room for it within the total allocated width.
	// The textinput pads its output to exactly input.Width characters,
	// so without this adjustment the suffix would be clipped by the
	// MaxWidth clamp below.
	var suffix string
	if s.active && s.input.Value() != "" {
		count := FormatInt(s.resultCount) + " results"
		if s.searching {
			count += " (searching…)"
		}
		// AI search status
		switch {
		case s.aiSearching:
			if s.aiStatus == "connecting" {
				count += " (✦ connecting…)"
			} else {
				count += " (✦ searching…)"
			}
		case s.aiStatus == "error":
			if s.aiError != "" {
				count += " (✦ " + s.aiError + ")"
			} else {
				count += " (✦ unavailable)"
			}
		case s.aiResults > 0:
			count += " (✦ " + FormatInt(s.aiResults) + " AI)"
		}
		suffix = styles.DimmedStyle.Render(" " + count)
	}

	// Shrink the textinput to leave room for the suffix so that the
	// combined output (input + suffix) fits within s.width.
	if s.width > 0 {
		suffixW := lipgloss.Width(suffix)
		// 4 = prompt icon + space + small padding
		inputW := max(10, s.width-suffixW-4)
		s.input.Width = inputW
	}

	v := s.input.View() + suffix

	// Clamp output to allocated width so ANSI-heavy status text
	// never leaks extra visual width into the header layout.
	if s.width > 0 && lipgloss.Width(v) > s.width {
		v = lipgloss.NewStyle().MaxWidth(s.width).Render(v)
	}
	return v
}
