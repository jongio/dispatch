package styles

import (
	"testing"
)

// ---------------------------------------------------------------------------
// ColorScheme.Validate
// ---------------------------------------------------------------------------

func TestValidate_ValidScheme(t *testing.T) {
	cs := Campbell
	if err := cs.Validate(); err != nil {
		t.Fatalf("Validate() on Campbell returned error: %v", err)
	}
}

func TestValidate_InvalidHex(t *testing.T) {
	cs := Campbell
	cs.Red = "not-a-color"
	if err := cs.Validate(); err == nil {
		t.Fatal("Validate() should fail for invalid hex")
	}
}

func TestValidate_EmptyField(t *testing.T) {
	cs := Campbell
	cs.Background = ""
	if err := cs.Validate(); err == nil {
		t.Fatal("Validate() should fail for empty field")
	}
}

func TestValidate_MissingHash(t *testing.T) {
	cs := Campbell
	cs.Blue = "0037DA" // missing #
	if err := cs.Validate(); err == nil {
		t.Fatal("Validate() should fail for hex without #")
	}
}

// ---------------------------------------------------------------------------
// DeriveTheme
// ---------------------------------------------------------------------------

func TestDeriveTheme_DarkScheme(t *testing.T) {
	theme := DeriveTheme(DispatchDark)

	if theme.SchemeName != "Dispatch Dark" {
		t.Errorf("SchemeName = %q, want %q", theme.SchemeName, "Dispatch Dark")
	}
	if !theme.IsDark {
		t.Error("IsDark should be true for Dispatch Dark")
	}
	// Primary should be BrightBlue on dark.
	if theme.Primary != DispatchDark.BrightBlue {
		t.Errorf("Primary = %q, want BrightBlue %q", theme.Primary, DispatchDark.BrightBlue)
	}
	// Error should be BrightRed on dark.
	if theme.Error != DispatchDark.BrightRed {
		t.Errorf("Error = %q, want BrightRed %q", theme.Error, DispatchDark.BrightRed)
	}
	// Success should be BrightGreen on dark.
	if theme.Success != DispatchDark.BrightGreen {
		t.Errorf("Success = %q, want BrightGreen %q", theme.Success, DispatchDark.BrightGreen)
	}
	// Badge should be BrightPurple on dark.
	if theme.Badge != DispatchDark.BrightPurple {
		t.Errorf("Badge = %q, want BrightPurple %q", theme.Badge, DispatchDark.BrightPurple)
	}
	// Text should be foreground.
	if theme.Text != DispatchDark.Foreground {
		t.Errorf("Text = %q, want Foreground %q", theme.Text, DispatchDark.Foreground)
	}
}

func TestDeriveTheme_LightScheme(t *testing.T) {
	theme := DeriveTheme(DispatchLight)

	if theme.IsDark {
		t.Error("IsDark should be false for Dispatch Light")
	}
	// Primary should be Blue (not Bright) on light.
	if theme.Primary != DispatchLight.Blue {
		t.Errorf("Primary = %q, want Blue %q", theme.Primary, DispatchLight.Blue)
	}
	// Error should be Red on light.
	if theme.Error != DispatchLight.Red {
		t.Errorf("Error = %q, want Red %q", theme.Error, DispatchLight.Red)
	}
}

func TestDeriveTheme_SemanticColorsAreValidHex(t *testing.T) {
	theme := DeriveTheme(Campbell)

	colors := []struct {
		name  string
		value string
	}{
		{"Primary", theme.Primary},
		{"Text", theme.Text},
		{"Dimmed", theme.Dimmed},
		{"Border", theme.Border},
		{"Selected", theme.Selected},
		{"Error", theme.Error},
		{"Success", theme.Success},
		{"Badge", theme.Badge},
		{"BadgeBg", theme.BadgeBg},
		{"StatusBg", theme.StatusBg},
		{"HeaderBg", theme.HeaderBg},
	}

	for _, c := range colors {
		if !hexPattern.MatchString(c.value) {
			t.Errorf("semantic color %s = %q is not valid #RRGGBB hex", c.name, c.value)
		}
	}
}

func TestDeriveTheme_StylesNotZero(t *testing.T) {
	theme := DeriveTheme(OneHalfDark)

	// Spot-check that styles are non-zero (i.e. they were actually built).
	rendered := theme.TitleStyle.Render("test")
	if rendered == "" {
		t.Error("TitleStyle.Render produced empty string")
	}
	rendered = theme.ErrorStyle.Render("err")
	if rendered == "" {
		t.Error("ErrorStyle.Render produced empty string")
	}
}

// ---------------------------------------------------------------------------
// Built-in scheme correctness
// ---------------------------------------------------------------------------

func TestBuiltinSchemes_AllValid(t *testing.T) {
	for _, name := range BuiltinSchemeNames() {
		cs := BuiltinSchemes[name]
		if err := cs.Validate(); err != nil {
			t.Errorf("built-in scheme %q failed validation: %v", name, err)
		}
		if cs.Name != name {
			t.Errorf("built-in scheme key %q has Name field %q", name, cs.Name)
		}
	}
}

func TestBuiltinSchemeNames_MatchesMap(t *testing.T) {
	names := BuiltinSchemeNames()
	if len(names) != len(BuiltinSchemes) {
		t.Fatalf("BuiltinSchemeNames() has %d entries, BuiltinSchemes has %d", len(names), len(BuiltinSchemes))
	}
	for _, name := range names {
		if _, ok := BuiltinSchemes[name]; !ok {
			t.Errorf("BuiltinSchemeNames() contains %q which is not in BuiltinSchemes", name)
		}
	}
}

// ---------------------------------------------------------------------------
// Color helpers
// ---------------------------------------------------------------------------

func TestIsDarkHex(t *testing.T) {
	tests := []struct {
		hex  string
		dark bool
	}{
		{"#000000", true},
		{"#111111", true},
		{"#0C0C0C", true},
		{"#282C34", true},
		{"#FFFFFF", false},
		{"#FAFAFA", false},
		{"#CCCCCC", false},
	}
	for _, tt := range tests {
		got := isDarkHex(tt.hex)
		if got != tt.dark {
			t.Errorf("isDarkHex(%q) = %v, want %v", tt.hex, got, tt.dark)
		}
	}
}

func TestBlendHex(t *testing.T) {
	// Blend black and white at 50% should produce a mid-gray.
	mid := blendHex("#000000", "#FFFFFF", 0.5)
	if !hexPattern.MatchString(mid) {
		t.Fatalf("blendHex returned invalid hex: %q", mid)
	}
	// t=0 should return approximately the first color.
	same := blendHex("#FF0000", "#0000FF", 0.0)
	if same != "#ff0000" {
		t.Errorf("blendHex(red, blue, 0.0) = %q, want #ff0000", same)
	}
}

// ---------------------------------------------------------------------------
// WCAG helpers
// ---------------------------------------------------------------------------

func TestWcagLuminance(t *testing.T) {
	tests := []struct {
		hex  string
		want float64
		tol  float64
	}{
		{"#000000", 0.0, 0.001},
		{"#FFFFFF", 1.0, 0.001},
		{"#FF0000", 0.2126, 0.01},
		{"#00FF00", 0.7152, 0.01},
		{"#0000FF", 0.0722, 0.01},
	}
	for _, tt := range tests {
		got := wcagLuminance(tt.hex)
		if got < tt.want-tt.tol || got > tt.want+tt.tol {
			t.Errorf("wcagLuminance(%q) = %.4f, want %.4f (±%.3f)", tt.hex, got, tt.want, tt.tol)
		}
	}
}

func TestWcagLuminance_InvalidHex(t *testing.T) {
	got := wcagLuminance("not-a-color")
	if got != 0 {
		t.Errorf("wcagLuminance(invalid) = %f, want 0", got)
	}
}

// contrastRatio computes the WCAG 2.1 contrast ratio between two relative
// luminance values. The result is always >= 1.
// Defined here (rather than in production code) because it is only used in tests.
func contrastRatio(l1, l2 float64) float64 {
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func TestContrastRatio(t *testing.T) {
	// Black vs white: (1+0.05)/(0+0.05) = 21.0
	got := contrastRatio(1.0, 0.0)
	if got < 20.9 || got > 21.1 {
		t.Errorf("contrastRatio(1, 0) = %f, want 21.0", got)
	}
	// Same luminance: 1.0
	got = contrastRatio(0.5, 0.5)
	if got < 0.99 || got > 1.01 {
		t.Errorf("contrastRatio(0.5, 0.5) = %f, want 1.0", got)
	}
	// Order-independent.
	r1 := contrastRatio(0.1, 0.9)
	r2 := contrastRatio(0.9, 0.1)
	if r1 != r2 {
		t.Errorf("contrastRatio should be order-independent: %f vs %f", r1, r2)
	}
}

func TestContrastText(t *testing.T) {
	tests := []struct {
		bg   string
		want string
	}{
		{"#000000", "#FFFFFF"}, // black bg → white text
		{"#111111", "#FFFFFF"}, // dark bg → white text
		{"#FFFFFF", "#000000"}, // white bg → black text
		{"#FAFAFA", "#000000"}, // light bg → black text
		{"#61AFEF", "#000000"}, // One Half Dark BrightBlue (light) → black text
		{"#5A56E0", "#FFFFFF"}, // Dispatch Blue (dark) → white text
		{"#7C6FF4", "#000000"}, // Dispatch BrightBlue (medium lavender) → black text
	}
	for _, tt := range tests {
		got := contrastText(tt.bg)
		if got != tt.want {
			t.Errorf("contrastText(%q) = %q, want %q", tt.bg, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// WCAG contrast regression — built-in schemes
// ---------------------------------------------------------------------------

func TestDeriveTheme_ActiveBadgeContrastAA(t *testing.T) {
	for _, name := range BuiltinSchemeNames() {
		cs := BuiltinSchemes[name]
		t.Run(name, func(t *testing.T) {
			theme := DeriveTheme(cs)
			fg := contrastText(theme.Primary)
			ratio := contrastRatio(wcagLuminance(fg), wcagLuminance(theme.Primary))
			if ratio < 4.5 {
				t.Errorf("ActiveBadge contrast = %.2f:1 (fg=%s on bg=%s), want >= 4.5:1",
					ratio, fg, theme.Primary)
			}
		})
	}
}

func TestDeriveTheme_DimmedContrastAALarge(t *testing.T) {
	for _, name := range BuiltinSchemeNames() {
		cs := BuiltinSchemes[name]
		t.Run(name, func(t *testing.T) {
			theme := DeriveTheme(cs)
			ratio := contrastRatio(wcagLuminance(theme.Dimmed), wcagLuminance(cs.Background))
			// Dimmed text must meet at least WCAG AA-large (3.0:1).
			if ratio < 3.0 {
				t.Errorf("Dimmed contrast = %.2f:1 (dimmed=%s on bg=%s), want >= 3.0:1",
					ratio, theme.Dimmed, cs.Background)
			}
		})
	}
}

func TestDeriveTheme_LightSchemesDimmedIsBrightBlack(t *testing.T) {
	for _, name := range BuiltinSchemeNames() {
		cs := BuiltinSchemes[name]
		if isDarkHex(cs.Background) {
			continue
		}
		t.Run(name, func(t *testing.T) {
			theme := DeriveTheme(cs)
			if theme.Dimmed != cs.BrightBlack {
				t.Errorf("Dimmed = %q, want BrightBlack %q for light scheme",
					theme.Dimmed, cs.BrightBlack)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SetTheme / CurrentTheme
// ---------------------------------------------------------------------------

func TestSetTheme_UpdatesCurrentTheme(t *testing.T) {
	original := CurrentTheme()
	defer SetTheme(original)

	if original == nil {
		t.Fatal("CurrentTheme() should not be nil after init")
	}

	newTheme := DeriveTheme(OneHalfDark)
	SetTheme(newTheme)
	if CurrentTheme() != newTheme {
		t.Error("CurrentTheme() should return the theme set by SetTheme()")
	}
}

func TestSetTheme_NilIsNoOp(t *testing.T) {
	before := CurrentTheme()
	SetTheme(nil)
	if CurrentTheme() != before {
		t.Error("SetTheme(nil) should be a no-op")
	}
}

func TestSetTheme_UpdatesExportedVars(t *testing.T) {
	before := CurrentTheme()
	defer SetTheme(before)

	theme := DeriveTheme(OneHalfLight)
	SetTheme(theme)

	// TitleStyle should render without panic.
	out := TitleStyle.Render("hello")
	if out == "" {
		t.Error("TitleStyle.Render produced empty string after SetTheme")
	}
}
