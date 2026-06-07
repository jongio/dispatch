package config

import (
	"fmt"
	"regexp"
)

// hexPattern validates a #RRGGBB hex color string.
var hexPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// ---------------------------------------------------------------------------
// ColorScheme — raw 16-color ANSI palette (Windows Terminal format)
// ---------------------------------------------------------------------------

// ColorScheme mirrors the Windows Terminal color scheme JSON object.
// All color values must be #RRGGBB hex strings.
type ColorScheme struct {
	Name                string `json:"name"`
	Foreground          string `json:"foreground"`
	Background          string `json:"background"`
	CursorColor         string `json:"cursorColor,omitempty"`
	SelectionBackground string `json:"selectionBackground,omitempty"`

	// Standard 8 ANSI colors.
	Black  string `json:"black"`
	Red    string `json:"red"`
	Green  string `json:"green"`
	Yellow string `json:"yellow"`
	Blue   string `json:"blue"`
	Purple string `json:"purple"`
	Cyan   string `json:"cyan"`
	White  string `json:"white"`

	// Bright variants.
	BrightBlack  string `json:"brightBlack"`
	BrightRed    string `json:"brightRed"`
	BrightGreen  string `json:"brightGreen"`
	BrightYellow string `json:"brightYellow"`
	BrightBlue   string `json:"brightBlue"`
	BrightPurple string `json:"brightPurple"`
	BrightCyan   string `json:"brightCyan"`
	BrightWhite  string `json:"brightWhite"`
}

// Palette returns the 16 ANSI colors in index order:
// [0]=Black, [1]=Red, …, [7]=White, [8]=BrightBlack, …, [15]=BrightWhite.
func (cs *ColorScheme) Palette() [16]string {
	return [16]string{
		cs.Black, cs.Red, cs.Green, cs.Yellow,
		cs.Blue, cs.Purple, cs.Cyan, cs.White,
		cs.BrightBlack, cs.BrightRed, cs.BrightGreen, cs.BrightYellow,
		cs.BrightBlue, cs.BrightPurple, cs.BrightCyan, cs.BrightWhite,
	}
}

// Validate checks that all required color fields are valid #RRGGBB values.
func (cs *ColorScheme) Validate() error {
	fields := []struct {
		name  string
		value string
	}{
		{"foreground", cs.Foreground},
		{"background", cs.Background},
		{"black", cs.Black},
		{"red", cs.Red},
		{"green", cs.Green},
		{"yellow", cs.Yellow},
		{"blue", cs.Blue},
		{"purple", cs.Purple},
		{"cyan", cs.Cyan},
		{"white", cs.White},
		{"brightBlack", cs.BrightBlack},
		{"brightRed", cs.BrightRed},
		{"brightGreen", cs.BrightGreen},
		{"brightYellow", cs.BrightYellow},
		{"brightBlue", cs.BrightBlue},
		{"brightPurple", cs.BrightPurple},
		{"brightCyan", cs.BrightCyan},
		{"brightWhite", cs.BrightWhite},
	}
	for _, f := range fields {
		if !hexPattern.MatchString(f.value) {
			return fmt.Errorf("color scheme %q: field %q has invalid hex color %q", cs.Name, f.name, f.value)
		}
	}
	return nil
}
