package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

// ---------------------------------------------------------------------------
// NewFilePicker
// ---------------------------------------------------------------------------

func TestNewFilePicker_Empty(t *testing.T) {
	t.Parallel()
	fp := NewFilePicker()
	_, ok := fp.Selected()
	if ok {
		t.Error("empty FilePicker Selected() should return false")
	}
	if !fp.Empty() {
		t.Error("empty FilePicker Empty() should return true")
	}
}

// ---------------------------------------------------------------------------
// SetFiles
// ---------------------------------------------------------------------------

func TestFilePicker_SetFiles_Basic(t *testing.T) {
	t.Parallel()
	fp := NewFilePicker()
	files := []data.SessionFile{
		{FilePath: "src/main.go"},
		{FilePath: "src/util.go"},
	}
	fp.SetFiles(files)
	if fp.Empty() {
		t.Error("FilePicker should not be empty after SetFiles")
	}
}

func TestFilePicker_SetFiles_ResetsCursor(t *testing.T) {
	t.Parallel()
	fp := NewFilePicker()
	files := []data.SessionFile{
		{FilePath: "a.go"},
		{FilePath: "b.go"},
	}
	fp.SetFiles(files)
	fp.MoveDown()
	fp.SetFiles(files)
	if fp.cursor != 0 {
		t.Errorf("cursor after SetFiles = %d, want 0", fp.cursor)
	}
}

func TestFilePicker_SetFiles_ClearsWarning(t *testing.T) {
	t.Parallel()
	fp := NewFilePicker()
	fp.SetWarning("oops")
	fp.SetFiles([]data.SessionFile{{FilePath: "a.go"}})
	if fp.warning != "" {
		t.Error("SetFiles should clear warning")
	}
}

// ---------------------------------------------------------------------------
// Navigation
// ---------------------------------------------------------------------------

func TestFilePicker_MoveUpDown(t *testing.T) {
	t.Parallel()
	fp := NewFilePicker()
	files := []data.SessionFile{
		{FilePath: "a.go"},
		{FilePath: "b.go"},
		{FilePath: "c.go"},
	}
	fp.SetFiles(files)

	fp.MoveDown()
	if fp.cursor != 1 {
		t.Errorf("after MoveDown cursor = %d, want 1", fp.cursor)
	}

	fp.MoveDown()
	if fp.cursor != 2 {
		t.Errorf("after 2nd MoveDown cursor = %d, want 2", fp.cursor)
	}

	// Past end.
	fp.MoveDown()
	if fp.cursor != 2 {
		t.Errorf("MoveDown past end cursor = %d, want 2", fp.cursor)
	}

	fp.MoveUp()
	if fp.cursor != 1 {
		t.Errorf("after MoveUp cursor = %d, want 1", fp.cursor)
	}

	// Past top.
	fp.MoveUp()
	fp.MoveUp()
	if fp.cursor != 0 {
		t.Errorf("MoveUp past top cursor = %d, want 0", fp.cursor)
	}
}

// ---------------------------------------------------------------------------
// Selected
// ---------------------------------------------------------------------------

func TestFilePicker_Selected(t *testing.T) {
	t.Parallel()
	fp := NewFilePicker()
	files := []data.SessionFile{
		{FilePath: "first.go"},
		{FilePath: "second.go"},
	}
	fp.SetFiles(files)

	sel, ok := fp.Selected()
	if !ok {
		t.Fatal("Selected() should return true when files exist")
	}
	if sel.FilePath != "first.go" {
		t.Errorf("Selected().FilePath = %q, want %q", sel.FilePath, "first.go")
	}

	fp.MoveDown()
	sel, ok = fp.Selected()
	if !ok || sel.FilePath != "second.go" {
		t.Errorf("after MoveDown Selected().FilePath = %q, want %q", sel.FilePath, "second.go")
	}
}

// ---------------------------------------------------------------------------
// Warning
// ---------------------------------------------------------------------------

func TestFilePicker_Warning(t *testing.T) {
	t.Parallel()
	fp := NewFilePicker()
	fp.SetWarning("file not found")
	if fp.warning != "file not found" {
		t.Errorf("warning = %q, want %q", fp.warning, "file not found")
	}
	fp.ClearWarning()
	if fp.warning != "" {
		t.Error("ClearWarning should reset warning to empty")
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func TestFilePicker_View_ContainsTitle(t *testing.T) {
	t.Parallel()
	fp := NewFilePicker()
	fp.SetFiles([]data.SessionFile{{FilePath: "test.go"}})
	fp.SetSize(80, 40)
	view := fp.View()
	if !strings.Contains(view, "Open File") {
		t.Error("View should contain 'Open File' title")
	}
}

func TestFilePicker_View_ContainsFilePaths(t *testing.T) {
	t.Parallel()
	fp := NewFilePicker()
	files := []data.SessionFile{
		{FilePath: "src/main.go"},
		{FilePath: "internal/util.go"},
	}
	fp.SetFiles(files)
	fp.SetSize(80, 40)
	view := fp.View()
	if !strings.Contains(view, "src/main.go") {
		t.Error("View should contain 'src/main.go'")
	}
	if !strings.Contains(view, "internal/util.go") {
		t.Error("View should contain 'internal/util.go'")
	}
}

func TestFilePicker_View_ContainsFooter(t *testing.T) {
	t.Parallel()
	fp := NewFilePicker()
	fp.SetFiles([]data.SessionFile{{FilePath: "test.go"}})
	fp.SetSize(80, 40)
	view := fp.View()
	if !strings.Contains(view, "Enter to open") {
		t.Error("View should contain footer")
	}
}

func TestFilePicker_View_ShowsWarning(t *testing.T) {
	t.Parallel()
	fp := NewFilePicker()
	fp.SetFiles([]data.SessionFile{{FilePath: "missing.go"}})
	fp.SetWarning("file does not exist")
	fp.SetSize(80, 40)
	view := fp.View()
	if !strings.Contains(view, "file does not exist") {
		t.Error("View should contain warning message")
	}
}

func TestFilePicker_View_DoesNotPanic(t *testing.T) {
	t.Parallel()
	fp := NewFilePicker()
	_ = fp.View() // zero files, zero size
	fp.SetSize(80, 40)
	_ = fp.View() // with size but no files
	fp.SetFiles([]data.SessionFile{{FilePath: "test.go"}})
	_ = fp.View() // full setup
}
