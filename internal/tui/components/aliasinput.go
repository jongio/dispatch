package components

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// AliasInput provides an inline text input for editing a session's alias.
type AliasInput struct {
	input     textinput.Model
	active    bool
	sessionID string
	width     int
}

// NewAliasInput returns an AliasInput ready for use.
func NewAliasInput() AliasInput {
	ti := textinput.New()
	ti.Placeholder = "Enter alias (Enter to save, Esc to cancel)..."
	ti.Prompt = styles.IconAlias() + " "
	ti.CharLimit = 60
	tiStyles := ti.Styles()
	tiStyles.Focused.Placeholder = styles.DimmedStyle
	tiStyles.Blurred.Placeholder = styles.DimmedStyle
	tiStyles.Focused.Prompt = styles.NoteIndicatorStyle
	tiStyles.Blurred.Prompt = styles.NoteIndicatorStyle
	ti.SetStyles(tiStyles)
	return AliasInput{input: ti}
}

// Focus activates the alias input for the given session, pre-filling the
// existing alias value.
func (n *AliasInput) Focus(sessionID, current string) tea.Cmd {
	n.active = true
	n.sessionID = sessionID
	n.input.SetValue(current)
	return n.input.Focus()
}

// Blur deactivates the alias input.
func (n *AliasInput) Blur() {
	n.active = false
	n.sessionID = ""
	n.input.Blur()
}

// Focused returns true when the alias input is capturing keystrokes.
func (n *AliasInput) Focused() bool {
	return n.active
}

// Value returns the current alias text.
func (n *AliasInput) Value() string {
	return n.input.Value()
}

// SessionID returns the session ID being edited.
func (n *AliasInput) SessionID() string {
	return n.sessionID
}

// SetWidth sets the available width for the alias input.
func (n *AliasInput) SetWidth(w int) {
	n.width = w
	n.input.SetWidth(max(10, w-6))
}

// Update delegates a tea.Msg to the underlying textinput.
func (n AliasInput) Update(msg tea.Msg) (AliasInput, tea.Cmd) {
	var cmd tea.Cmd
	n.input, cmd = n.input.Update(msg)
	return n, cmd
}

// View renders the alias input bar.
func (n AliasInput) View() string {
	v := n.input.View()
	if n.width > 0 && lipgloss.Width(v) > n.width {
		v = lipgloss.NewStyle().MaxWidth(n.width).Render(v)
	}
	return v
}
