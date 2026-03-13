//go:build windows

package platform

import (
	"testing"
)

// ---------------------------------------------------------------------------
// WT settings.json parsing tests
// ---------------------------------------------------------------------------

func TestParseWTSettings_SimpleScheme(t *testing.T) {
	data := []byte(`{
		"defaultProfile": "{61c54bbd-c2c6-5271-96e7-009a87ff44bf}",
		"profiles": {
			"defaults": {},
			"list": [
				{
					"guid": "{61c54bbd-c2c6-5271-96e7-009a87ff44bf}",
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

	scheme, err := parseWTSettingsData(data)
	if err != nil {
		t.Fatalf("parseWTSettingsData returned error: %v", err)
	}
	if scheme == nil {
		t.Fatal("expected non-nil scheme")
	}
	if scheme.Name != "Campbell" {
		t.Errorf("scheme name = %q, want Campbell", scheme.Name)
	}
	if scheme.Foreground != "#CCCCCC" {
		t.Errorf("foreground = %q, want #CCCCCC", scheme.Foreground)
	}
}

func TestParseWTSettings_ObjectColorScheme(t *testing.T) {
	// WT supports {"dark":"...", "light":"..."} object for colorScheme.
	data := []byte(`{
		"defaultProfile": "{abc}",
		"profiles": {
			"defaults": {},
			"list": [
				{
					"guid": "{abc}",
					"colorScheme": {"dark": "One Half Dark", "light": "One Half Light"}
				}
			]
		},
		"schemes": [
			{
				"name": "One Half Dark",
				"foreground": "#DCDFE4",
				"background": "#282C34",
				"black": "#282C34",
				"red": "#E06C75",
				"green": "#98C379",
				"yellow": "#E5C07B",
				"blue": "#61AFEF",
				"purple": "#C678DD",
				"cyan": "#56B6C2",
				"white": "#DCDFE4",
				"brightBlack": "#5A6374",
				"brightRed": "#E06C75",
				"brightGreen": "#98C379",
				"brightYellow": "#E5C07B",
				"brightBlue": "#61AFEF",
				"brightPurple": "#C678DD",
				"brightCyan": "#56B6C2",
				"brightWhite": "#DCDFE4"
			}
		]
	}`)

	scheme, err := parseWTSettingsData(data)
	if err != nil {
		t.Fatalf("parseWTSettingsData returned error: %v", err)
	}
	if scheme == nil {
		t.Fatal("expected non-nil scheme for object colorScheme")
	}
	if scheme.Name != "One Half Dark" {
		t.Errorf("scheme name = %q, want 'One Half Dark'", scheme.Name)
	}
}

func TestParseWTSettings_DefaultsFallback(t *testing.T) {
	// When profile has no colorScheme, fall back to profiles.defaults.
	data := []byte(`{
		"defaultProfile": "{abc}",
		"profiles": {
			"defaults": {
				"colorScheme": "Campbell"
			},
			"list": [
				{
					"guid": "{abc}"
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

	scheme, err := parseWTSettingsData(data)
	if err != nil {
		t.Fatalf("parseWTSettingsData returned error: %v", err)
	}
	if scheme == nil {
		t.Fatal("expected non-nil scheme via defaults fallback")
	}
	if scheme.Name != "Campbell" {
		t.Errorf("scheme name = %q, want Campbell", scheme.Name)
	}
}

func TestParseWTSettings_NoSchemeConfigured(t *testing.T) {
	data := []byte(`{
		"defaultProfile": "{abc}",
		"profiles": {
			"defaults": {},
			"list": [
				{
					"guid": "{abc}"
				}
			]
		},
		"schemes": []
	}`)

	scheme, err := parseWTSettingsData(data)
	if err != nil {
		t.Fatalf("parseWTSettingsData returned error: %v", err)
	}
	if scheme != nil {
		t.Errorf("expected nil scheme when none configured, got %+v", scheme)
	}
}

func TestParseWTSettings_MissingProfile(t *testing.T) {
	data := []byte(`{
		"defaultProfile": "{does-not-exist}",
		"profiles": {
			"defaults": {},
			"list": [
				{
					"guid": "{abc}"
				}
			]
		},
		"schemes": []
	}`)

	scheme, err := parseWTSettingsData(data)
	if err != nil {
		t.Fatalf("parseWTSettingsData returned error: %v", err)
	}
	if scheme != nil {
		t.Errorf("expected nil scheme when profile not found, got %+v", scheme)
	}
}

func TestParseWTSettings_SchemeNotFound(t *testing.T) {
	data := []byte(`{
		"defaultProfile": "{abc}",
		"profiles": {
			"defaults": {},
			"list": [
				{
					"guid": "{abc}",
					"colorScheme": "Nonexistent Scheme"
				}
			]
		},
		"schemes": []
	}`)

	scheme, err := parseWTSettingsData(data)
	if err == nil {
		t.Fatal("expected error when scheme name is not in schemes array")
	}
	if scheme != nil {
		t.Error("expected nil scheme when scheme not found")
	}
}

func TestParseWTSettings_InvalidJSON(t *testing.T) {
	data := []byte(`{not valid json}`)
	_, err := parseWTSettingsData(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
