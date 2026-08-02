package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/tui/styles"
)

type launchSetPickerMode int

const (
	launchSetModeList launchSetPickerMode = iota
	launchSetModeSave
	launchSetModeRename
	launchSetModeDelete
)

// LaunchSetPicker renders a modal overlay for managing named launch sets.
type LaunchSetPicker struct {
	sets    []config.LaunchSet
	missing map[string][]string
	cursor  int
	width   int
	height  int

	mode  launchSetPickerMode
	input textinput.Model
}

// NewLaunchSetPicker returns an empty launch set picker.
func NewLaunchSetPicker() LaunchSetPicker {
	ti := textinput.New()
	ti.Placeholder = "Launch set name..."
	ti.Prompt = "Name: "
	ti.CharLimit = 80
	tiStyles := ti.Styles()
	tiStyles.Focused.Placeholder = styles.DimmedStyle
	tiStyles.Blurred.Placeholder = styles.DimmedStyle
	ti.SetStyles(tiStyles)
	return LaunchSetPicker{
		missing: make(map[string][]string),
		input:   ti,
	}
}

// SetLaunchSets replaces the list of launch sets and marks IDs not present in
// existingIDs as missing.
func (p *LaunchSetPicker) SetLaunchSets(sets []config.LaunchSet, existingIDs map[string]struct{}) {
	p.sets = append([]config.LaunchSet(nil), sets...)
	p.missing = make(map[string][]string, len(sets))
	for _, set := range sets {
		for _, id := range set.SessionIDs {
			if _, ok := existingIDs[id]; !ok {
				p.missing[set.Name] = append(p.missing[set.Name], id)
			}
		}
	}
	if p.cursor >= len(p.sets) {
		p.cursor = max(0, len(p.sets)-1)
	}
}

// SetSize updates the overlay dimensions.
func (p *LaunchSetPicker) SetSize(w, h int) {
	p.width = w
	p.height = h
	p.input.SetWidth(max(20, min(60, w-16)))
}

// MoveUp moves the selection up, stopping at the top.
func (p *LaunchSetPicker) MoveUp() {
	if p.mode == launchSetModeList && p.cursor > 0 {
		p.cursor--
	}
}

// MoveDown moves the selection down, stopping at the bottom.
func (p *LaunchSetPicker) MoveDown() {
	if p.mode == launchSetModeList && p.cursor < len(p.sets)-1 {
		p.cursor++
	}
}

// Selected returns the currently highlighted launch set.
func (p LaunchSetPicker) Selected() (config.LaunchSet, bool) {
	if p.cursor >= 0 && p.cursor < len(p.sets) {
		return p.sets[p.cursor], true
	}
	return config.LaunchSet{}, false
}

// MissingIDs returns missing session IDs for the selected set.
func (p LaunchSetPicker) MissingIDs(name string) []string {
	return append([]string(nil), p.missing[name]...)
}

// BeginSave switches to name input for saving a new set.
func (p *LaunchSetPicker) BeginSave(defaultName string) tea.Cmd {
	p.mode = launchSetModeSave
	p.input.SetValue(defaultName)
	return p.input.Focus()
}

// BeginRename switches to name input for renaming the selected set.
func (p *LaunchSetPicker) BeginRename() tea.Cmd {
	set, ok := p.Selected()
	if !ok {
		return nil
	}
	p.mode = launchSetModeRename
	p.input.SetValue(set.Name)
	return p.input.Focus()
}

// BeginDelete switches to delete confirmation for the selected set.
func (p *LaunchSetPicker) BeginDelete() {
	if _, ok := p.Selected(); ok {
		p.mode = launchSetModeDelete
	}
}

// CancelMode returns from input/confirmation mode to the list.
func (p *LaunchSetPicker) CancelMode() {
	p.mode = launchSetModeList
	p.input.Blur()
}

// Saving reports whether the picker is entering a new set name.
func (p LaunchSetPicker) Saving() bool { return p.mode == launchSetModeSave }

// Renaming reports whether the picker is entering a replacement name.
func (p LaunchSetPicker) Renaming() bool { return p.mode == launchSetModeRename }

// Deleting reports whether the picker is confirming deletion.
func (p LaunchSetPicker) Deleting() bool { return p.mode == launchSetModeDelete }

// Value returns the current input value.
func (p LaunchSetPicker) Value() string { return strings.TrimSpace(p.input.Value()) }

// Update delegates input messages when a text field is active.
func (p LaunchSetPicker) Update(msg tea.Msg) (LaunchSetPicker, tea.Cmd) {
	if p.mode != launchSetModeSave && p.mode != launchSetModeRename {
		return p, nil
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

// View renders the launch set picker overlay.
func (p LaunchSetPicker) View() string {
	title := styles.OverlayTitleStyle.Render("Launch Sets")
	var body strings.Builder
	body.WriteString(title + "\n")

	switch p.mode {
	case launchSetModeSave:
		body.WriteString("Save selected sessions as a launch set\n\n")
		body.WriteString(p.input.View())
		body.WriteString("\n\n" + styles.DimmedStyle.Render("Enter to save · Esc to cancel"))
	case launchSetModeRename:
		body.WriteString("Rename launch set\n\n")
		body.WriteString(p.input.View())
		body.WriteString("\n\n" + styles.DimmedStyle.Render("Enter to rename · Esc to cancel"))
	case launchSetModeDelete:
		if set, ok := p.Selected(); ok {
			fmt.Fprintf(&body, "Delete %q?\n\n", set.Name)
			body.WriteString(styles.DimmedStyle.Render("Enter to delete · Esc to cancel"))
		}
	default:
		if len(p.sets) == 0 {
			body.WriteString(styles.DimmedStyle.Render("No launch sets saved") + "\n")
		}
		for i, set := range p.sets {
			indicator := "  "
			if i == p.cursor {
				indicator = "\u25b8 "
			}
			missing := p.missing[set.Name]
			suffix := fmt.Sprintf(" (%d sessions)", len(set.SessionIDs))
			if len(missing) > 0 {
				suffix = fmt.Sprintf(" (%d sessions, %d missing)", len(set.SessionIDs), len(missing))
			}
			line := indicator + set.Name + suffix
			if len(missing) == len(set.SessionIDs) && len(missing) > 0 {
				line = styles.DimmedStyle.Render(line + " — all missing")
			} else if i == p.cursor {
				line = styles.SelectedStyle.Render(line)
			}
			body.WriteString(line + "\n")
			if i == p.cursor && len(missing) > 0 {
				body.WriteString(styles.DimmedStyle.Render("    missing: "+strings.Join(missing, ", ")) + "\n")
			}
		}
		body.WriteString("\n" + styles.DimmedStyle.Render("Enter launch · Ctrl+S save selection · Ctrl+R rename · Ctrl+D delete · Esc cancel"))
	}

	maxW := min(76, p.width-4)
	maxW = max(maxW, 32)
	overlay := styles.OverlayStyle.Width(maxW).Render(body.String())
	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, overlay)
}
