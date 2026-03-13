//go:build windows

package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// colorSchemeRef.UnmarshalJSON — additional edge cases
// ---------------------------------------------------------------------------

func TestColorSchemeRef_UnmarshalJSON_PlainString(t *testing.T) {
	var ref colorSchemeRef
	if err := json.Unmarshal([]byte(`"Campbell"`), &ref); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}
	if ref.plain != "Campbell" {
		t.Errorf("plain = %q, want %q", ref.plain, "Campbell")
	}
}

func TestColorSchemeRef_UnmarshalJSON_Object(t *testing.T) {
	var ref colorSchemeRef
	if err := json.Unmarshal([]byte(`{"dark":"Dark Scheme","light":"Light Scheme"}`), &ref); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}
	if ref.dark != "Dark Scheme" {
		t.Errorf("dark = %q, want %q", ref.dark, "Dark Scheme")
	}
	if ref.light != "Light Scheme" {
		t.Errorf("light = %q, want %q", ref.light, "Light Scheme")
	}
}

func TestColorSchemeRef_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var ref colorSchemeRef
	err := json.Unmarshal([]byte(`[1,2,3]`), &ref)
	if err == nil {
		t.Error("expected error for invalid JSON (array)")
	}
}

func TestColorSchemeRef_UnmarshalJSON_EmptyString(t *testing.T) {
	var ref colorSchemeRef
	if err := json.Unmarshal([]byte(`""`), &ref); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}
	if ref.plain != "" {
		t.Errorf("plain = %q, want empty", ref.plain)
	}
}

func TestColorSchemeRef_UnmarshalJSON_DarkOnly(t *testing.T) {
	var ref colorSchemeRef
	if err := json.Unmarshal([]byte(`{"dark":"Dark Only"}`), &ref); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}
	if ref.dark != "Dark Only" {
		t.Errorf("dark = %q, want %q", ref.dark, "Dark Only")
	}
	if ref.light != "" {
		t.Errorf("light = %q, want empty", ref.light)
	}
}

// ---------------------------------------------------------------------------
// colorSchemeRef.resolve — all branches
// ---------------------------------------------------------------------------

func TestColorSchemeRef_Resolve_Plain(t *testing.T) {
	ref := colorSchemeRef{plain: "Campbell"}
	if got := ref.resolve(); got != "Campbell" {
		t.Errorf("resolve() = %q, want %q", got, "Campbell")
	}
}

func TestColorSchemeRef_Resolve_DarkPreferred(t *testing.T) {
	ref := colorSchemeRef{dark: "Dark", light: "Light"}
	if got := ref.resolve(); got != "Dark" {
		t.Errorf("resolve() = %q, want %q (dark preferred)", got, "Dark")
	}
}

func TestColorSchemeRef_Resolve_LightFallback(t *testing.T) {
	ref := colorSchemeRef{light: "Light Only"}
	if got := ref.resolve(); got != "Light Only" {
		t.Errorf("resolve() = %q, want %q", got, "Light Only")
	}
}

func TestColorSchemeRef_Resolve_Empty(t *testing.T) {
	ref := colorSchemeRef{}
	if got := ref.resolve(); got != "" {
		t.Errorf("resolve() = %q, want empty", got)
	}
}

func TestColorSchemeRef_Resolve_PlainTakesPrecedence(t *testing.T) {
	ref := colorSchemeRef{plain: "Plain", dark: "Dark", light: "Light"}
	if got := ref.resolve(); got != "Plain" {
		t.Errorf("resolve() = %q, want %q (plain takes precedence)", got, "Plain")
	}
}

// ---------------------------------------------------------------------------
// parseWTSettings — file-based test
// ---------------------------------------------------------------------------

func TestParseWTSettings_ValidFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	data := []byte(`{
		"defaultProfile": "{abc}",
		"profiles": {
			"defaults": {},
			"list": [
				{
					"guid": "{abc}",
					"colorScheme": "Campbell"
				}
			]
		},
		"schemes": [
			{
				"name": "Campbell",
				"foreground": "#CCCCCC",
				"background": "#0C0C0C",
				"black": "#0C0C0C",
				"red": "#C50F1F",
				"green": "#13A10E",
				"yellow": "#C19C00",
				"blue": "#0037DA",
				"purple": "#881798",
				"cyan": "#3A96DD",
				"white": "#CCCCCC",
				"brightBlack": "#767676",
				"brightRed": "#E74856",
				"brightGreen": "#16C60C",
				"brightYellow": "#F9F1A5",
				"brightBlue": "#3B78FF",
				"brightPurple": "#B4009E",
				"brightCyan": "#61D6D6",
				"brightWhite": "#F2F2F2"
			}
		]
	}`)

	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	scheme, err := parseWTSettings(settingsPath)
	if err != nil {
		t.Fatalf("parseWTSettings error: %v", err)
	}
	if scheme == nil {
		t.Fatal("expected non-nil scheme")
	}
	if scheme.Name != "Campbell" {
		t.Errorf("scheme.Name = %q, want %q", scheme.Name, "Campbell")
	}
}

func TestParseWTSettings_NonexistentFile(t *testing.T) {
	_, err := parseWTSettings("/nonexistent/settings.json")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestParseWTSettings_InvalidJSONFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	_ = os.WriteFile(settingsPath, []byte(`{invalid json`), 0o644)

	_, err := parseWTSettings(settingsPath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// wtSettingsPaths
// ---------------------------------------------------------------------------

func TestWtSettingsPaths_ReturnsPathsWhenLocalAppDataSet(t *testing.T) {
	// On Windows, LOCALAPPDATA should always be set.
	if os.Getenv("LOCALAPPDATA") == "" {
		t.Skip("LOCALAPPDATA not set")
	}
	paths := wtSettingsPaths()
	if len(paths) == 0 {
		t.Error("wtSettingsPaths should return paths when LOCALAPPDATA is set")
	}
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			t.Errorf("wtSettingsPaths returned non-absolute path: %q", p)
		}
	}
}

// ---------------------------------------------------------------------------
// DetectWTColorScheme (smoke test)
// ---------------------------------------------------------------------------

func TestDetectWTColorScheme_DoesNotPanic(t *testing.T) {
	// Result depends on system WT installation.
	scheme, err := DetectWTColorScheme()
	if err != nil {
		t.Logf("DetectWTColorScheme error: %v (WT may not be configured)", err)
	}
	if scheme != nil {
		if scheme.Name == "" {
			t.Error("detected scheme has empty name")
		}
	}
}

// ---------------------------------------------------------------------------
// parseWTSettingsData — additional edge cases
// ---------------------------------------------------------------------------

func TestParseWTSettings_ObjectSchemeWithLightOnly(t *testing.T) {
	data := []byte(`{
		"defaultProfile": "{abc}",
		"profiles": {
			"defaults": {},
			"list": [
				{
					"guid": "{abc}",
					"colorScheme": {"light": "One Half Light"}
				}
			]
		},
		"schemes": [
			{
				"name": "One Half Light",
				"foreground": "#383A42",
				"background": "#FAFAFA",
				"black": "#383A42",
				"red": "#E45649",
				"green": "#50A14F",
				"yellow": "#C18401",
				"blue": "#0184BC",
				"purple": "#A626A4",
				"cyan": "#0997B3",
				"white": "#FAFAFA",
				"brightBlack": "#4F525D",
				"brightRed": "#DF6C75",
				"brightGreen": "#98C379",
				"brightYellow": "#E4C07A",
				"brightBlue": "#61AFEF",
				"brightPurple": "#C577DD",
				"brightCyan": "#56B5C1",
				"brightWhite": "#FFFFFF"
			}
		]
	}`)

	scheme, err := parseWTSettingsData(data)
	if err != nil {
		t.Fatalf("parseWTSettingsData error: %v", err)
	}
	if scheme == nil {
		t.Fatal("expected non-nil scheme for light-only object colorScheme")
	}
	if scheme.Name != "One Half Light" {
		t.Errorf("scheme.Name = %q, want %q", scheme.Name, "One Half Light")
	}
}

func TestParseWTSettings_EmptySchemesList(t *testing.T) {
	data := []byte(`{
		"defaultProfile": "{abc}",
		"profiles": {
			"defaults": {},
			"list": []
		},
		"schemes": []
	}`)
	scheme, err := parseWTSettingsData(data)
	if err != nil {
		t.Fatalf("parseWTSettingsData error: %v", err)
	}
	if scheme != nil {
		t.Error("expected nil scheme for empty profiles list")
	}
}
