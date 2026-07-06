package components

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// TagInput provides an inline text input for editing a session's tags as a
// comma-separated list.
type TagInput struct {
	input     textinput.Model
	active    bool
	sessionID string
	width     int
}

// NewTagInput returns a TagInput ready for use.
func NewTagInput() TagInput {
	ti := textinput.New()
	ti.Placeholder = "Comma-separated tags (Enter to save, Esc to cancel)..."
	ti.Prompt = styles.IconTag() + " "
	ti.CharLimit = 200
	tiStyles := ti.Styles()
	tiStyles.Focused.Placeholder = styles.DimmedStyle
	tiStyles.Blurred.Placeholder = styles.DimmedStyle
	tiStyles.Focused.Prompt = styles.TagIndicatorStyle
	tiStyles.Blurred.Prompt = styles.TagIndicatorStyle
	ti.SetStyles(tiStyles)
	return TagInput{input: ti}
}

// Focus activates the tag input for the given session, pre-filling the
// existing comma-separated tag value.
func (n *TagInput) Focus(sessionID, current string) tea.Cmd {
	n.active = true
	n.sessionID = sessionID
	n.input.SetValue(current)
	return n.input.Focus()
}

// Blur deactivates the tag input.
func (n *TagInput) Blur() {
	n.active = false
	n.sessionID = ""
	n.input.Blur()
}

// Focused returns true when the tag input is capturing keystrokes.
func (n *TagInput) Focused() bool {
	return n.active
}

// Value returns the current comma-separated tag text.
func (n *TagInput) Value() string {
	return n.input.Value()
}

// SessionID returns the session ID being edited.
func (n *TagInput) SessionID() string {
	return n.sessionID
}

// SetWidth sets the available width for the tag input.
func (n *TagInput) SetWidth(w int) {
	n.width = w
	n.input.SetWidth(max(10, w-6))
}

// Update delegates a tea.Msg to the underlying textinput.
func (n TagInput) Update(msg tea.Msg) (TagInput, tea.Cmd) {
	var cmd tea.Cmd
	n.input, cmd = n.input.Update(msg)
	return n, cmd
}

// View renders the tag input bar.
func (n TagInput) View() string {
	v := n.input.View()
	if n.width > 0 && lipgloss.Width(v) > n.width {
		v = lipgloss.NewStyle().MaxWidth(n.width).Render(v)
	}
	return v
}
