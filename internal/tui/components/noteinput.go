package components

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// NoteInput provides an inline text input for editing session notes.
type NoteInput struct {
	input     textinput.Model
	active    bool
	sessionID string
	width     int
}

// NewNoteInput returns a NoteInput ready for use.
func NewNoteInput() NoteInput {
	ti := textinput.New()
	ti.Placeholder = "Enter note (Enter to save, Esc to cancel)..."
	ti.Prompt = styles.IconNote() + " "
	ti.CharLimit = 200
	tiStyles := ti.Styles()
	tiStyles.Focused.Placeholder = styles.DimmedStyle
	tiStyles.Blurred.Placeholder = styles.DimmedStyle
	tiStyles.Focused.Prompt = styles.NoteIndicatorStyle
	tiStyles.Blurred.Prompt = styles.NoteIndicatorStyle
	ti.SetStyles(tiStyles)
	return NoteInput{input: ti}
}

// Focus activates the note input for the given session, pre-filling the
// existing note value.
func (n *NoteInput) Focus(sessionID, currentNote string) tea.Cmd {
	n.active = true
	n.sessionID = sessionID
	n.input.SetValue(currentNote)
	return n.input.Focus()
}

// Blur deactivates the note input.
func (n *NoteInput) Blur() {
	n.active = false
	n.sessionID = ""
	n.input.Blur()
}

// Focused returns true when the note input is capturing keystrokes.
func (n *NoteInput) Focused() bool {
	return n.active
}

// Value returns the current note text.
func (n *NoteInput) Value() string {
	return n.input.Value()
}

// SessionID returns the session ID being edited.
func (n *NoteInput) SessionID() string {
	return n.sessionID
}

// SetWidth sets the available width for the note input.
func (n *NoteInput) SetWidth(w int) {
	n.width = w
	n.input.SetWidth(max(10, w-6))
}

// Update delegates a tea.Msg to the underlying textinput.
func (n NoteInput) Update(msg tea.Msg) (NoteInput, tea.Cmd) {
	var cmd tea.Cmd
	n.input, cmd = n.input.Update(msg)
	return n, cmd
}

// View renders the note input bar.
func (n NoteInput) View() string {
	v := n.input.View()
	if n.width > 0 && lipgloss.Width(v) > n.width {
		v = lipgloss.NewStyle().MaxWidth(n.width).Render(v)
	}
	return v
}
