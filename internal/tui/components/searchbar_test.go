package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// ---------------------------------------------------------------------------
// NewSearchBar
// ---------------------------------------------------------------------------

func TestNewSearchBar_Defaults(t *testing.T) {
	t.Parallel()
	sb := NewSearchBar()
	if sb.Focused() {
		t.Error("new SearchBar should not be focused")
	}
	if sb.Value() != "" {
		t.Errorf("new SearchBar value = %q, want empty", sb.Value())
	}
}

// ---------------------------------------------------------------------------
// Focus / Blur / Focused
// ---------------------------------------------------------------------------

func TestSearchBar_FocusBlur(t *testing.T) {
	t.Parallel()
	sb := NewSearchBar()

	_ = sb.Focus()
	if !sb.Focused() {
		t.Error("SearchBar should be focused after Focus()")
	}

	sb.Blur()
	if sb.Focused() {
		t.Error("SearchBar should not be focused after Blur()")
	}
}

// ---------------------------------------------------------------------------
// Value / SetValue
// ---------------------------------------------------------------------------

func TestSearchBar_ValueRoundTrip(t *testing.T) {
	t.Parallel()
	sb := NewSearchBar()
	sb.SetValue("test query")
	if sb.Value() != "test query" {
		t.Errorf("Value() = %q, want %q", sb.Value(), "test query")
	}
}

func TestSearchBar_SetValue_Empty(t *testing.T) {
	t.Parallel()
	sb := NewSearchBar()
	sb.SetValue("something")
	sb.SetValue("")
	if sb.Value() != "" {
		t.Errorf("Value() after SetValue('') = %q, want empty", sb.Value())
	}
}

// ---------------------------------------------------------------------------
// SetResultCount
// ---------------------------------------------------------------------------

func TestSearchBar_SetResultCount(t *testing.T) {
	t.Parallel()
	sb := NewSearchBar()
	sb.SetResultCount(42)
	if sb.resultCount != 42 {
		t.Errorf("resultCount = %d, want 42", sb.resultCount)
	}
}

// ---------------------------------------------------------------------------
// SetSearching
// ---------------------------------------------------------------------------

func TestSearchBar_SetSearching(t *testing.T) {
	t.Parallel()
	sb := NewSearchBar()
	sb.SetSearching(true)
	if !sb.searching {
		t.Error("searching should be true")
	}
	sb.SetSearching(false)
	if sb.searching {
		t.Error("searching should be false")
	}
}

// ---------------------------------------------------------------------------
// SetWidth
// ---------------------------------------------------------------------------

func TestSearchBar_SetWidth(t *testing.T) {
	t.Parallel()
	sb := NewSearchBar()
	sb.SetWidth(80)
	if sb.width != 80 {
		t.Errorf("width = %d, want 80", sb.width)
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func TestSearchBar_View_DoesNotPanic(t *testing.T) {
	t.Parallel()
	sb := NewSearchBar()
	_ = sb.View()
}

func TestSearchBar_View_ShowsResultCount(t *testing.T) {
	t.Parallel()
	sb := NewSearchBar()
	_ = sb.Focus()
	sb.SetValue("test")
	sb.SetResultCount(15)
	view := sb.View()
	if !strings.Contains(view, "15 results") {
		t.Errorf("View should contain '15 results', got: %q", view)
	}
}

func TestSearchBar_View_ShowsSearchingIndicator(t *testing.T) {
	t.Parallel()
	sb := NewSearchBar()
	_ = sb.Focus()
	sb.SetValue("query")
	sb.SetResultCount(5)
	sb.SetSearching(true)
	view := sb.View()
	if !strings.Contains(view, "searching") {
		t.Errorf("View should contain 'searching' when searching, got: %q", view)
	}
}

func TestSearchBar_View_NoCountWhenInactive(t *testing.T) {
	t.Parallel()
	sb := NewSearchBar()
	sb.SetValue("test")
	sb.SetResultCount(10)
	// Not focused — should not show results count.
	view := sb.View()
	if strings.Contains(view, "10 results") {
		t.Error("View should not show result count when inactive")
	}
}

func TestSearchBar_View_NoCountWhenEmpty(t *testing.T) {
	t.Parallel()
	sb := NewSearchBar()
	_ = sb.Focus()
	sb.SetResultCount(10)
	// No value — should not show results count.
	view := sb.View()
	if strings.Contains(view, "10 results") {
		t.Error("View should not show result count when value is empty")
	}
}

func TestSearchBar_View_FitsWithinWidth(t *testing.T) {
	t.Parallel()
	// Even at narrow widths the rendered output must not exceed allocation.
	sb := NewSearchBar()
	_ = sb.Focus()
	sb.SetValue("a long query string that might push width")
	sb.SetResultCount(999)
	sb.SetSearching(true)
	sb.SetWidth(50)

	view := sb.View()
	visW := lipgloss.Width(view)
	if visW > 50 {
		t.Errorf("rendered width %d exceeds allocated 50", visW)
	}
}
