package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ---------------------------------------------------------------------------
// EffectiveLaunchMode — comprehensive coverage
// ---------------------------------------------------------------------------

func TestEffectiveLaunchMode_ExplicitModes(t *testing.T) {
	tests := []struct {
		name          string
		launchMode    string
		launchInPlace bool
		want          string
	}{
		{"explicit in-place", "in-place", false, "in-place"},
		{"explicit tab", "tab", false, "tab"},
		{"explicit window", "window", false, "window"},
		{"explicit overrides legacy true", "tab", true, "tab"},
		{"explicit overrides legacy false", "window", false, "window"},
		{"legacy in-place", "", true, "in-place"},
		{"default is tab", "", false, "tab"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				LaunchMode:    tt.launchMode,
				LaunchInPlace: tt.launchInPlace,
			}
			got := cfg.EffectiveLaunchMode()
			if got != tt.want {
				t.Errorf("EffectiveLaunchMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Launch mode constants
// ---------------------------------------------------------------------------

func TestLaunchModeConstants(t *testing.T) {
	if LaunchModeInPlace != "in-place" {
		t.Errorf("LaunchModeInPlace = %q", LaunchModeInPlace)
	}
	if LaunchModeTab != "tab" {
		t.Errorf("LaunchModeTab = %q", LaunchModeTab)
	}
	if LaunchModeWindow != "window" {
		t.Errorf("LaunchModeWindow = %q", LaunchModeWindow)
	}
}

// ---------------------------------------------------------------------------
// Default — additional field coverage
// ---------------------------------------------------------------------------

func TestDefaultLaunchMode(t *testing.T) {
	cfg := Default()
	if cfg.LaunchMode != "" {
		t.Errorf("LaunchMode should default to empty, got %q", cfg.LaunchMode)
	}
	if cfg.EffectiveLaunchMode() != "tab" {
		t.Errorf("EffectiveLaunchMode() should default to 'tab', got %q", cfg.EffectiveLaunchMode())
	}
}

func TestDefaultTheme(t *testing.T) {
	cfg := Default()
	if cfg.Theme != "" {
		t.Errorf("Theme should default to empty, got %q", cfg.Theme)
	}
}

func TestDefaultSchemes(t *testing.T) {
	cfg := Default()
	if cfg.Schemes != nil {
		t.Errorf("Schemes should default to nil, got %v", cfg.Schemes)
	}
}

// ---------------------------------------------------------------------------
// Config JSON tag coverage
// ---------------------------------------------------------------------------

func TestConfigJSONRoundTrip_AllFields(t *testing.T) {
	// Verifies all fields survive JSON marshaling/unmarshaling
	cfg := &Config{
		DefaultShell:     "zsh",
		DefaultTerminal:  "kitty",
		DefaultTimeRange: "7d",
		DefaultSort:      "created",
		DefaultPivot:     "repo",
		ShowPreview:      false,
		MaxSessions:      50,
		YoloMode:         true,
		Agent:            "coder",
		Model:            "gpt-4",
		LaunchMode:       "window",
		LaunchInPlace:    true,
		ExcludedDirs:     []string{"/tmp"},
		CustomCommand:    "my-cli {sessionId}",
		HiddenSessions:   []string{"sess1", "sess2"},
		Theme:            "One Half Dark",
	}

	// Verify effective launch mode with explicit LaunchMode
	if cfg.EffectiveLaunchMode() != "window" {
		t.Errorf("EffectiveLaunchMode with explicit LaunchMode = %q", cfg.EffectiveLaunchMode())
	}
}

// ---------------------------------------------------------------------------
// EffectiveLaunchMode — edge cases
// ---------------------------------------------------------------------------

func TestEffectiveLaunchMode_CustomStringFallsThrough(t *testing.T) {
	// Any non-empty LaunchMode is returned as-is
	cfg := &Config{LaunchMode: "custom-mode"}
	if cfg.EffectiveLaunchMode() != "custom-mode" {
		t.Errorf("should return custom LaunchMode, got %q", cfg.EffectiveLaunchMode())
	}
}

// ---------------------------------------------------------------------------
// Load / Save / Reset — filesystem tests
// ---------------------------------------------------------------------------

// setupTempConfig sets APPDATA (Windows) or XDG_CONFIG_HOME (Linux/macOS)
// to a temp dir so configPath() returns a controlled location.
func setupTempConfig(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("APPDATA", tmp)
	if runtime.GOOS != "windows" {
		t.Setenv("XDG_CONFIG_HOME", tmp)
	}
	return tmp
}

func TestLoad_NoFile_ReturnsDefault(t *testing.T) {
	setupTempConfig(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with no file: %v", err)
	}
	def := Default()
	if cfg.MaxSessions != def.MaxSessions {
		t.Errorf("MaxSessions = %d, want default %d", cfg.MaxSessions, def.MaxSessions)
	}
	if cfg.ShowPreview != def.ShowPreview {
		t.Errorf("ShowPreview = %v, want %v", cfg.ShowPreview, def.ShowPreview)
	}
}

func TestSave_CreatesFile(t *testing.T) {
	tmp := setupTempConfig(t)

	cfg := Default()
	cfg.YoloMode = true
	cfg.Agent = "test-agent"

	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists and contains correct data.
	path := filepath.Join(tmp, "dispatch", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal saved file: %v", err)
	}
	if !loaded.YoloMode {
		t.Error("saved YoloMode should be true")
	}
	if loaded.Agent != "test-agent" {
		t.Errorf("saved Agent = %q, want 'test-agent'", loaded.Agent)
	}
}

func TestLoad_ExistingFile(t *testing.T) {
	tmp := setupTempConfig(t)

	// Write a config file manually.
	dir := filepath.Join(tmp, "dispatch")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfgData := `{"yoloMode": true, "agent": "my-agent", "max_sessions": 42}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfgData), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.YoloMode {
		t.Error("YoloMode should be true")
	}
	if cfg.Agent != "my-agent" {
		t.Errorf("Agent = %q, want 'my-agent'", cfg.Agent)
	}
	if cfg.MaxSessions != 42 {
		t.Errorf("MaxSessions = %d, want 42", cfg.MaxSessions)
	}
	// Fields not in JSON should get defaults.
	if !cfg.ShowPreview {
		t.Error("ShowPreview should default to true")
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	tmp := setupTempConfig(t)

	dir := filepath.Join(tmp, "dispatch")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("not json{"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Error("Load with malformed JSON should return error")
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	setupTempConfig(t)

	original := Default()
	original.YoloMode = true
	original.DefaultShell = "fish"
	original.Model = "gpt-4"
	original.ExcludedDirs = []string{"/tmp", "/var"}
	original.HiddenSessions = []string{"sess-a", "sess-b"}
	original.Theme = "One Half Dark"

	if err := Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.YoloMode != original.YoloMode {
		t.Error("YoloMode mismatch")
	}
	if loaded.DefaultShell != original.DefaultShell {
		t.Errorf("DefaultShell = %q, want %q", loaded.DefaultShell, original.DefaultShell)
	}
	if loaded.Model != original.Model {
		t.Errorf("Model = %q, want %q", loaded.Model, original.Model)
	}
	if loaded.Theme != original.Theme {
		t.Errorf("Theme = %q, want %q", loaded.Theme, original.Theme)
	}
	if len(loaded.ExcludedDirs) != 2 {
		t.Errorf("ExcludedDirs len = %d, want 2", len(loaded.ExcludedDirs))
	}
	if len(loaded.HiddenSessions) != 2 {
		t.Errorf("HiddenSessions len = %d, want 2", len(loaded.HiddenSessions))
	}
}

func TestReset_RemovesFile(t *testing.T) {
	setupTempConfig(t)

	// Save a config first.
	cfg := Default()
	cfg.YoloMode = true
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Reset.
	if err := Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// Load should return defaults.
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load after Reset: %v", err)
	}
	if loaded.YoloMode {
		t.Error("YoloMode should be default (false) after Reset")
	}
}

func TestReset_NoFile_NoError(t *testing.T) {
	setupTempConfig(t)

	// Reset when no file exists → no error.
	if err := Reset(); err != nil {
		t.Errorf("Reset with no file should not error: %v", err)
	}
}
