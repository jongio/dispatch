package config

import (
	"strings"
	"testing"
)

func validColorScheme() ColorScheme {
	return ColorScheme{
		Name:         "test",
		Foreground:   "#010101",
		Background:   "#020202",
		Black:        "#030303",
		Red:          "#040404",
		Green:        "#050505",
		Yellow:       "#060606",
		Blue:         "#070707",
		Purple:       "#080808",
		Cyan:         "#090909",
		White:        "#101010",
		BrightBlack:  "#111111",
		BrightRed:    "#121212",
		BrightGreen:  "#131313",
		BrightYellow: "#141414",
		BrightBlue:   "#151515",
		BrightPurple: "#161616",
		BrightCyan:   "#171717",
		BrightWhite:  "#181818",
	}
}

func TestColorSchemePaletteUsesANSIOrder(t *testing.T) {
	t.Parallel()

	scheme := validColorScheme()
	got := scheme.Palette()
	want := [16]string{
		"#030303", "#040404", "#050505", "#060606",
		"#070707", "#080808", "#090909", "#101010",
		"#111111", "#121212", "#131313", "#141414",
		"#151515", "#161616", "#171717", "#181818",
	}

	if got != want {
		t.Fatalf("Palette() = %#v, want %#v", got, want)
	}
}

func TestColorSchemeValidateAcceptsValidColors(t *testing.T) {
	t.Parallel()

	scheme := validColorScheme()
	if err := scheme.Validate(); err != nil {
		t.Fatalf("Validate() returned unexpected error: %v", err)
	}

	scheme.Foreground = "#aBcDeF"
	if err := scheme.Validate(); err != nil {
		t.Fatalf("Validate() rejected mixed-case hex: %v", err)
	}
}

func TestColorSchemeValidateRejectsEveryInvalidRequiredField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
		set   func(*ColorScheme)
	}{
		{name: "foreground", field: "foreground", set: func(s *ColorScheme) { s.Foreground = "" }},
		{name: "background", field: "background", set: func(s *ColorScheme) { s.Background = "#12345" }},
		{name: "black", field: "black", set: func(s *ColorScheme) { s.Black = "000000" }},
		{name: "red", field: "red", set: func(s *ColorScheme) { s.Red = "#GG0000" }},
		{name: "green", field: "green", set: func(s *ColorScheme) { s.Green = "#00 000" }},
		{name: "yellow", field: "yellow", set: func(s *ColorScheme) { s.Yellow = "#0000000" }},
		{name: "blue", field: "blue", set: func(s *ColorScheme) { s.Blue = "blue" }},
		{name: "purple", field: "purple", set: func(s *ColorScheme) { s.Purple = "#-00000" }},
		{name: "cyan", field: "cyan", set: func(s *ColorScheme) { s.Cyan = "#00000?" }},
		{name: "white", field: "white", set: func(s *ColorScheme) { s.White = "#0000" }},
		{name: "brightBlack", field: "brightBlack", set: func(s *ColorScheme) { s.BrightBlack = "" }},
		{name: "brightRed", field: "brightRed", set: func(s *ColorScheme) { s.BrightRed = "#xyzxyz" }},
		{name: "brightGreen", field: "brightGreen", set: func(s *ColorScheme) { s.BrightGreen = "#12345g" }},
		{name: "brightYellow", field: "brightYellow", set: func(s *ColorScheme) { s.BrightYellow = "##123456" }},
		{name: "brightBlue", field: "brightBlue", set: func(s *ColorScheme) { s.BrightBlue = "#123456 " }},
		{name: "brightPurple", field: "brightPurple", set: func(s *ColorScheme) { s.BrightPurple = " #123456" }},
		{name: "brightCyan", field: "brightCyan", set: func(s *ColorScheme) { s.BrightCyan = "#1234.6" }},
		{name: "brightWhite", field: "brightWhite", set: func(s *ColorScheme) { s.BrightWhite = "#12345/" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scheme := validColorScheme()
			tt.set(&scheme)
			err := scheme.Validate()
			if err == nil {
				t.Fatal("Validate() returned nil for invalid color")
			}
			if !strings.Contains(err.Error(), `field "`+tt.field+`"`) {
				t.Fatalf("Validate() error = %q, want field %q", err, tt.field)
			}
			if !strings.Contains(err.Error(), `color scheme "test"`) {
				t.Fatalf("Validate() error = %q, want scheme name", err)
			}
		})
	}
}
