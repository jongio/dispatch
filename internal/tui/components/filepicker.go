package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// FilePicker renders a modal list of session files for opening.
type FilePicker struct {
	files   []data.SessionFile
	cursor  int
	width   int
	height  int
	warning string // transient warning shown when a file does not exist
}

// NewFilePicker returns an empty FilePicker.
func NewFilePicker() FilePicker {
	return FilePicker{}
}

// SetFiles replaces the file list and resets the cursor.
func (f *FilePicker) SetFiles(files []data.SessionFile) {
	f.files = files
	f.cursor = 0
	f.warning = ""
}

// SetSize updates the overlay dimensions.
func (f *FilePicker) SetSize(w, h int) {
	f.width = w
	f.height = h
}

// MoveUp moves the selection up, stopping at the top.
func (f *FilePicker) MoveUp() {
	if f.cursor > 0 {
		f.cursor--
	}
}

// MoveDown moves the selection down, stopping at the bottom.
func (f *FilePicker) MoveDown() {
	if f.cursor < len(f.files)-1 {
		f.cursor++
	}
}

// Selected returns the currently highlighted file, if any.
func (f *FilePicker) Selected() (data.SessionFile, bool) {
	if len(f.files) == 0 {
		return data.SessionFile{}, false
	}
	return f.files[f.cursor], true
}

// SetWarning sets a transient warning message displayed below the list.
func (f *FilePicker) SetWarning(msg string) {
	f.warning = msg
}

// ClearWarning removes the warning message.
func (f *FilePicker) ClearWarning() {
	f.warning = ""
}

// Empty returns true when the picker has no files.
func (f *FilePicker) Empty() bool {
	return len(f.files) == 0
}

// View renders the file picker overlay.
func (f FilePicker) View() string {
	title := styles.OverlayTitleStyle.Render("Open File")

	var body strings.Builder
	body.WriteString(title + "\n")

	for i, file := range f.files {
		indicator := "  "
		if i == f.cursor {
			indicator = "\u25b8 " // ▸
		}
		line := indicator + file.FilePath
		if i == f.cursor {
			line = styles.SelectedStyle.Render(line)
		}
		body.WriteString(line + "\n")
	}

	if f.warning != "" {
		body.WriteString("\n" + styles.ErrorStyle.Render(f.warning))
	}

	body.WriteString("\n" + styles.DimmedStyle.Render("Enter to open \u00b7 Esc to close"))

	maxW := min(70, f.width-4)
	maxW = max(maxW, 20)

	overlay := styles.OverlayStyle.
		Width(maxW).
		Render(body.String())

	return lipgloss.Place(f.width, f.height, lipgloss.Center, lipgloss.Center, overlay)
}
