//go:build windows

package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ---------------------------------------------------------------------------
// Windows Terminal settings.json parser
// ---------------------------------------------------------------------------

// WTColorScheme mirrors the color scheme object in Windows Terminal
// settings.json.  The field names/tags intentionally match the WT schema
// so we can unmarshal directly.
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

// colorSchemeRef can be a plain string or a {"dark":"...","light":"..."} object.
type colorSchemeRef struct {
	plain string
	dark  string
	light string
}

func (r *colorSchemeRef) UnmarshalJSON(b []byte) error {
	// Try plain string first.
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		r.plain = s
		return nil
	}
	// Try object form.
	var obj struct {
		Dark  string `json:"dark"`
		Light string `json:"light"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	r.dark = obj.Dark
	r.light = obj.Light
	return nil
}

// resolve returns the scheme name, preferring the dark variant when the
// terminal background is dark (the common case for WT).
func (r *colorSchemeRef) resolve() string {
	if r.plain != "" {
		return r.plain
	}
	if r.dark != "" {
		return r.dark
	}
	return r.light
}

// wtProfile represents a single profile in settings.json.
type wtProfile struct {
	GUID        string          `json:"guid"`
	ColorScheme *colorSchemeRef `json:"colorScheme,omitempty"`
}

// wtSettings is a minimal representation of WT's settings.json.
type wtSettings struct {
	DefaultProfile string `json:"defaultProfile"`
	Profiles       struct {
		Defaults wtProfile   `json:"defaults"`
		List     []wtProfile `json:"list"`
	} `json:"profiles"`
	Schemes []WTColorScheme `json:"schemes"`
}

// DetectWTColorScheme attempts to read Windows Terminal's settings.json
// and return the active color scheme for the default profile.
//
// Returns (nil, nil) if WT is not installed or the profile has no
// explicit color scheme.
func DetectWTColorScheme() (*WTColorScheme, error) {
	paths := wtSettingsPaths()
	for _, p := range paths {
		scheme, err := parseWTSettings(p)
		if err != nil {
			continue // file not found or unreadable — try next path
		}
		if scheme != nil {
			return scheme, nil
		}
	}
	return nil, nil
}

// wtSettingsPaths returns candidate paths for WT settings.json.
func wtSettingsPaths() []string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return nil
	}
	return []string{
		// Store install (most common).
		filepath.Join(localAppData, "Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", "settings.json"),
		// Scoop / Chocolatey install.
		filepath.Join(localAppData, "Microsoft", "Windows Terminal", "settings.json"),
	}
}

// parseWTSettings reads a single WT settings.json and returns the
// resolved color scheme for the default profile, or nil if none is set.
func parseWTSettings(path string) (*WTColorScheme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseWTSettingsData(data)
}

// parseWTSettingsData parses raw WT settings JSON. Exported for testing.
func parseWTSettingsData(data []byte) (*WTColorScheme, error) {
	var s wtSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	// 1. Find the default profile.
	schemeName := ""
	for _, p := range s.Profiles.List {
		if p.GUID == s.DefaultProfile {
			if p.ColorScheme != nil {
				schemeName = p.ColorScheme.resolve()
			}
			break
		}
	}

	// 2. Fall back to profiles.defaults.
	if schemeName == "" && s.Profiles.Defaults.ColorScheme != nil {
		schemeName = s.Profiles.Defaults.ColorScheme.resolve()
	}

	// 3. No scheme configured → return nil.
	if schemeName == "" {
		return nil, nil
	}

	// 4. Look up the scheme by name.
	for i := range s.Schemes {
		if s.Schemes[i].Name == schemeName {
			return &s.Schemes[i], nil
		}
	}

	return nil, fmt.Errorf("color scheme %q not found in WT settings", schemeName)
}
