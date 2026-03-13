package styles

// builtins.go defines the built-in color schemes shipped with dispatch.

// BuiltinSchemes maps scheme names (case-sensitive) to their definitions.
var BuiltinSchemes = map[string]ColorScheme{
	"Dispatch Dark":  DispatchDark,
	"Dispatch Light": DispatchLight,
	"Campbell":       Campbell,
	"One Half Dark":  OneHalfDark,
	"One Half Light": OneHalfLight,
}

// BuiltinSchemeNames returns the built-in scheme names in display order.
func BuiltinSchemeNames() []string {
	return []string{
		"Dispatch Dark",
		"Dispatch Light",
		"Campbell",
		"One Half Dark",
		"One Half Light",
	}
}

// DefaultDarkScheme is the fallback dark scheme (original hardcoded palette).
var DefaultDarkScheme = DispatchDark

// DefaultLightScheme is the fallback light scheme (original hardcoded palette).
var DefaultLightScheme = DispatchLight

// DispatchDark reproduces the original hardcoded dark palette from theme.go.
var DispatchDark = ColorScheme{
	Name:         "Dispatch Dark",
	Foreground:   "#E4E4E7",
	Background:   "#111111",
	Black:        "#0C0C0C",
	Red:          "#DC2626",
	Green:        "#16A34A",
	Yellow:       "#C19C00",
	Blue:         "#5A56E0",
	Purple:       "#6D28D9",
	Cyan:         "#3A96DD",
	White:        "#CCCCCC",
	BrightBlack:  "#71717A",
	BrightRed:    "#F87171",
	BrightGreen:  "#4ADE80",
	BrightYellow: "#F9F1A5",
	BrightBlue:   "#7C6FF4",
	BrightPurple: "#A78BFA",
	BrightCyan:   "#61D6D6",
	BrightWhite:  "#F2F2F2",
}

// DispatchLight reproduces the original hardcoded light palette from theme.go.
var DispatchLight = ColorScheme{
	Name:         "Dispatch Light",
	Foreground:   "#1A1A2E",
	Background:   "#FAFAFA",
	Black:        "#0C0C0C",
	Red:          "#DC2626",
	Green:        "#16A34A",
	Yellow:       "#C19C00",
	Blue:         "#5A56E0",
	Purple:       "#6D28D9",
	Cyan:         "#3A96DD",
	White:        "#CCCCCC",
	BrightBlack:  "#71717A",
	BrightRed:    "#F87171",
	BrightGreen:  "#4ADE80",
	BrightYellow: "#F9F1A5",
	BrightBlue:   "#7C6FF4",
	BrightPurple: "#A78BFA",
	BrightCyan:   "#61D6D6",
	BrightWhite:  "#F2F2F2",
}

// Campbell is the default Windows Terminal color scheme.
var Campbell = ColorScheme{
	Name:         "Campbell",
	Foreground:   "#CCCCCC",
	Background:   "#0C0C0C",
	Black:        "#0C0C0C",
	Red:          "#C50F1F",
	Green:        "#13A10E",
	Yellow:       "#C19C00",
	Blue:         "#0037DA",
	Purple:       "#881798",
	Cyan:         "#3A96DD",
	White:        "#CCCCCC",
	BrightBlack:  "#767676",
	BrightRed:    "#E74856",
	BrightGreen:  "#16C60C",
	BrightYellow: "#F9F1A5",
	BrightBlue:   "#3B78FF",
	BrightPurple: "#B4009E",
	BrightCyan:   "#61D6D6",
	BrightWhite:  "#F2F2F2",
}

// OneHalfDark is the One Half Dark color scheme (popular WT preset).
var OneHalfDark = ColorScheme{
	Name:         "One Half Dark",
	Foreground:   "#DCDFE4",
	Background:   "#282C34",
	Black:        "#282C34",
	Red:          "#E06C75",
	Green:        "#98C379",
	Yellow:       "#E5C07B",
	Blue:         "#61AFEF",
	Purple:       "#C678DD",
	Cyan:         "#56B6C2",
	White:        "#DCDFE4",
	BrightBlack:  "#5A6374",
	BrightRed:    "#E06C75",
	BrightGreen:  "#98C379",
	BrightYellow: "#E5C07B",
	BrightBlue:   "#61AFEF",
	BrightPurple: "#C678DD",
	BrightCyan:   "#56B6C2",
	BrightWhite:  "#DCDFE4",
}

// OneHalfLight is the One Half Light color scheme (popular WT preset).
var OneHalfLight = ColorScheme{
	Name:         "One Half Light",
	Foreground:   "#383A42",
	Background:   "#FAFAFA",
	Black:        "#383A42",
	Red:          "#E45649",
	Green:        "#50A14F",
	Yellow:       "#C18401",
	Blue:         "#0184BC",
	Purple:       "#A626A4",
	Cyan:         "#0997B3",
	White:        "#FAFAFA",
	BrightBlack:  "#4F525D",
	BrightRed:    "#DF6C75",
	BrightGreen:  "#98C379",
	BrightYellow: "#E4C07A",
	BrightBlue:   "#61AFEF",
	BrightPurple: "#C577DD",
	BrightCyan:   "#56B5C1",
	BrightWhite:  "#FFFFFF",
}
