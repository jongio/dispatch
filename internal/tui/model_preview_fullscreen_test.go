package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestPreviewFullscreen_ToggleAndLayout verifies that pressing "z" enters a
// fullscreen preview (list hidden, preview fills the content area) and that
// pressing "z" again restores the split layout.
func TestPreviewFullscreen_ToggleAndLayout(t *testing.T) {
	m := newTestModelWithSize(120, 40)
	m.showPreview = true
	m.recalcLayout()

	if m.previewFullscreen {
		t.Fatal("expected previewFullscreen false initially")
	}

	res, _ := m.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	rm := res.(Model)
	if !rm.previewFullscreen {
		t.Fatal("pressing z should enable fullscreen preview")
	}
	if rm.layout.previewWidth != rm.width {
		t.Errorf("fullscreen preview width = %d, want %d", rm.layout.previewWidth, rm.width)
	}
	if rm.layout.listWidth != 0 {
		t.Errorf("fullscreen list width = %d, want 0", rm.layout.listWidth)
	}
	if rm.layout.previewHeight != rm.layout.contentHeight {
		t.Errorf("fullscreen preview height = %d, want content height %d", rm.layout.previewHeight, rm.layout.contentHeight)
	}

	res2, _ := rm.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	rm2 := res2.(Model)
	if rm2.previewFullscreen {
		t.Fatal("pressing z again should disable fullscreen preview")
	}
	if rm2.layout.listWidth == 0 {
		t.Error("exiting fullscreen should restore the session list width")
	}
}

// TestPreviewFullscreen_EscapeExits verifies that Escape leaves fullscreen and
// restores the split layout.
func TestPreviewFullscreen_EscapeExits(t *testing.T) {
	m := newTestModelWithSize(120, 40)
	m.showPreview = true
	m.recalcLayout()

	res, _ := m.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	rm := res.(Model)
	if !rm.previewFullscreen {
		t.Fatal("expected fullscreen after z")
	}

	res2, _ := rm.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	rm2 := res2.(Model)
	if rm2.previewFullscreen {
		t.Error("esc should exit fullscreen preview")
	}
	if rm2.layout.previewWidth == rm2.width {
		t.Error("esc should restore the split layout (preview no longer full width)")
	}
}

// TestPreviewFullscreen_WorksWithPreviewOff verifies that fullscreen can be
// entered even when the split preview is toggled off, that paging does not exit
// fullscreen, and that the view renders non-empty content.
func TestPreviewFullscreen_WorksWithPreviewOff(t *testing.T) {
	m := newTestModelWithSize(120, 40)
	m.showPreview = false
	m.recalcLayout()

	res, _ := m.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	rm := res.(Model)
	if !rm.previewFullscreen {
		t.Fatal("expected fullscreen after z even with preview toggled off")
	}
	if rm.layout.previewWidth != rm.width {
		t.Errorf("fullscreen preview width = %d, want %d", rm.layout.previewWidth, rm.width)
	}

	res2, _ := rm.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	rm2 := res2.(Model)
	if !rm2.previewFullscreen {
		t.Error("paging should not exit fullscreen preview")
	}

	out := rm2.renderMainView()
	if strings.TrimSpace(out) == "" {
		t.Error("fullscreen view should render non-empty content")
	}
}
