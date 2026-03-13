//go:build !windows

package platform

// WTColorScheme mirrors the color scheme object in Windows Terminal
// settings.json.  On non-Windows platforms this type is provided so
// the rest of the codebase can reference it without build-tag guards.
type WTColorScheme struct {
	Name                string `json:"name"`
	Foreground          string `json:"foreground"`
	Background          string `json:"background"`
	CursorColor         string `json:"cursorColor,omitempty"`
	SelectionBackground string `json:"selectionBackground,omitempty"`
	Black               string `json:"black"`
	Red                 string `json:"red"`
	Green               string `json:"green"`
	Yellow              string `json:"yellow"`
	Blue                string `json:"blue"`
	Purple              string `json:"purple"`
	Cyan                string `json:"cyan"`
	White               string `json:"white"`
	BrightBlack         string `json:"brightBlack"`
	BrightRed           string `json:"brightRed"`
	BrightGreen         string `json:"brightGreen"`
	BrightYellow        string `json:"brightYellow"`
	BrightBlue          string `json:"brightBlue"`
	BrightPurple        string `json:"brightPurple"`
	BrightCyan          string `json:"brightCyan"`
	BrightWhite         string `json:"brightWhite"`
}

// DetectWTColorScheme is a no-op on non-Windows platforms.
// It always returns (nil, nil).
func DetectWTColorScheme() (*WTColorScheme, error) {
	return nil, nil
}
