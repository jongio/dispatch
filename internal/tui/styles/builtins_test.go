package styles

import (
	"testing"
)

// ---------------------------------------------------------------------------
// BuiltinSchemeNames
// ---------------------------------------------------------------------------

func TestBuiltinSchemeNames_ExactOrder(t *testing.T) {
	expected := []string{
		"Dispatch Dark",
		"Dispatch Light",
		"Campbell",
		"One Half Dark",
		"One Half Light",
	}
	got := BuiltinSchemeNames()
	if len(got) != len(expected) {
		t.Fatalf("BuiltinSchemeNames() len = %d, want %d", len(got), len(expected))
	}
	for i, name := range expected {
		if got[i] != name {
			t.Errorf("BuiltinSchemeNames()[%d] = %q, want %q", i, got[i], name)
		}
	}
}

func TestBuiltinSchemeNames_Count(t *testing.T) {
	names := BuiltinSchemeNames()
	if len(names) != 5 {
		t.Errorf("BuiltinSchemeNames() has %d entries, want 5", len(names))
	}
}

// ---------------------------------------------------------------------------
// DefaultDarkScheme / DefaultLightScheme
// ---------------------------------------------------------------------------

func TestDefaultDarkScheme_IsDispatchDark(t *testing.T) {
	if DefaultDarkScheme.Name != DispatchDark.Name {
		t.Errorf("DefaultDarkScheme.Name = %q, want %q", DefaultDarkScheme.Name, DispatchDark.Name)
	}
	if DefaultDarkScheme.Background != DispatchDark.Background {
		t.Errorf("DefaultDarkScheme.Background = %q, want %q", DefaultDarkScheme.Background, DispatchDark.Background)
	}
}

func TestDefaultLightScheme_IsDispatchLight(t *testing.T) {
	if DefaultLightScheme.Name != DispatchLight.Name {
		t.Errorf("DefaultLightScheme.Name = %q, want %q", DefaultLightScheme.Name, DispatchLight.Name)
	}
	if DefaultLightScheme.Background != DispatchLight.Background {
		t.Errorf("DefaultLightScheme.Background = %q, want %q", DefaultLightScheme.Background, DispatchLight.Background)
	}
}

func TestDefaultSchemes_AreValid(t *testing.T) {
	if err := DefaultDarkScheme.Validate(); err != nil {
		t.Errorf("DefaultDarkScheme validation failed: %v", err)
	}
	if err := DefaultLightScheme.Validate(); err != nil {
		t.Errorf("DefaultLightScheme validation failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// BuiltinSchemes map
// ---------------------------------------------------------------------------

func TestBuiltinSchemes_ContainsAllNames(t *testing.T) {
	for _, name := range BuiltinSchemeNames() {
		if _, ok := BuiltinSchemes[name]; !ok {
			t.Errorf("BuiltinSchemes missing key %q", name)
		}
	}
}

func TestBuiltinSchemes_AllSchemesHaveDarkOrLightBackground(t *testing.T) {
	for _, name := range BuiltinSchemeNames() {
		cs := BuiltinSchemes[name]
		dark := isDarkHex(cs.Background)
		// "Dark" in name should be dark, "Light" should be light.
		if contains(name, "Dark") && !dark {
			t.Errorf("scheme %q has 'Dark' in name but isDarkHex(bg) = false", name)
		}
		if contains(name, "Light") && dark {
			t.Errorf("scheme %q has 'Light' in name but isDarkHex(bg) = true", name)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Layout constants
// ---------------------------------------------------------------------------

func TestLayoutConstants(t *testing.T) {
	if MinTermWidth < 1 {
		t.Errorf("MinTermWidth = %d, want > 0", MinTermWidth)
	}
	if PreviewMinWidth <= MinTermWidth {
		t.Errorf("PreviewMinWidth (%d) should be > MinTermWidth (%d)", PreviewMinWidth, MinTermWidth)
	}
	if PreviewWidthRatio <= 0 || PreviewWidthRatio >= 1 {
		t.Errorf("PreviewWidthRatio = %f, want 0 < ratio < 1", PreviewWidthRatio)
	}
	if HeaderLines < 1 {
		t.Errorf("HeaderLines = %d, want >= 1", HeaderLines)
	}
	if FooterLines < 1 {
		t.Errorf("FooterLines = %d, want >= 1", FooterLines)
	}
}

// ---------------------------------------------------------------------------
// Theme — additional exported var checks
// ---------------------------------------------------------------------------

func TestSetTheme_UpdatesAllColorTokens(t *testing.T) {
	theme := DeriveTheme(Campbell)
	original := CurrentTheme()
	SetTheme(theme)
	defer SetTheme(original)

	// All three color tokens should be non-nil.
	if ColorPrimary == nil {
		t.Error("ColorPrimary is nil after SetTheme")
	}
	if ColorText == nil {
		t.Error("ColorText is nil after SetTheme")
	}
	if ColorDimmed == nil {
		t.Error("ColorDimmed is nil after SetTheme")
	}
}

func TestCurrentTheme_NeverNilAfterInit(t *testing.T) {
	ct := CurrentTheme()
	if ct == nil {
		t.Fatal("CurrentTheme() should never be nil after package init")
	}
	if ct.SchemeName == "" {
		t.Error("CurrentTheme().SchemeName should not be empty")
	}
}

func TestDeriveTheme_AllBuiltinSchemes(t *testing.T) {
	for _, name := range BuiltinSchemeNames() {
		cs := BuiltinSchemes[name]
		t.Run(name, func(t *testing.T) {
			theme := DeriveTheme(cs)
			if theme == nil {
				t.Fatal("DeriveTheme returned nil")
			}
			if theme.SchemeName != name {
				t.Errorf("SchemeName = %q, want %q", theme.SchemeName, name)
			}
			// Every style should render a non-empty string.
			if theme.TitleStyle.Render("x") == "" {
				t.Error("TitleStyle renders empty")
			}
			if theme.ErrorStyle.Render("x") == "" {
				t.Error("ErrorStyle renders empty")
			}
			if theme.NormalStyle.Render("x") == "" {
				t.Error("NormalStyle renders empty")
			}
		})
	}
}
