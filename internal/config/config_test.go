package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ---------------------------------------------------------------------------
// Default() tests
// ---------------------------------------------------------------------------

func TestDefaultValues(t *testing.T) {
	cfg := Default()

	if cfg.DefaultShell != "" {
		t.Errorf("DefaultShell = %q, want empty", cfg.DefaultShell)
	}
	if cfg.DefaultTerminal != "" {
		t.Errorf("DefaultTerminal = %q, want empty", cfg.DefaultTerminal)
	}
	if cfg.DefaultTimeRange != "1d" {
		t.Errorf("DefaultTimeRange = %q, want '1d'", cfg.DefaultTimeRange)
	}
	if cfg.DefaultSort != "updated" {
		t.Errorf("DefaultSort = %q, want 'updated'", cfg.DefaultSort)
	}
	if cfg.DefaultPivot != "folder" {
		t.Errorf("DefaultPivot = %q, want 'folder'", cfg.DefaultPivot)
	}
	if !cfg.ShowPreview {
		t.Error("ShowPreview should default to true")
	}
	if cfg.MaxSessions != 100 {
		t.Errorf("MaxSessions = %d, want 100", cfg.MaxSessions)
	}
	if cfg.YoloMode {
		t.Error("YoloMode should default to false")
	}
	if cfg.Agent != "" {
		t.Errorf("Agent = %q, want empty", cfg.Agent)
	}
	if cfg.Model != "" {
		t.Errorf("Model = %q, want empty", cfg.Model)
	}
	if cfg.LaunchInPlace {
		t.Error("LaunchInPlace should default to false")
	}
	if cfg.CustomCommand != "" {
		t.Errorf("CustomCommand = %q, want empty", cfg.CustomCommand)
	}
	if len(cfg.ExcludedDirs) != 0 {
		t.Errorf("ExcludedDirs = %v, want empty", cfg.ExcludedDirs)
	}
	if len(cfg.HiddenSessions) != 0 {
		t.Errorf("HiddenSessions = %v, want empty", cfg.HiddenSessions)
	}
}

// ---------------------------------------------------------------------------
// JSON round-trip tests
// ---------------------------------------------------------------------------

func TestConfigJSONRoundTrip(t *testing.T) {
	original := &Config{
		DefaultShell:     "zsh",
		DefaultTerminal:  "alacritty",
		DefaultTimeRange: "7d",
		DefaultSort:      "created",
		DefaultPivot:     "repo",
		ShowPreview:      false,
		MaxSessions:      50,
		YoloMode:         true,
		Agent:            "coder",
		Model:            "gpt-4",
		LaunchInPlace:    true,
		ExcludedDirs:     []string{"/tmp", "/var"},
		CustomCommand:    "ghcs --resume {sessionId} --custom",
		HiddenSessions:   []string{"sess-1", "sess-2"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var restored Config
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if restored.DefaultShell != original.DefaultShell {
		t.Errorf("DefaultShell = %q, want %q", restored.DefaultShell, original.DefaultShell)
	}
	if restored.DefaultTerminal != original.DefaultTerminal {
		t.Errorf("DefaultTerminal = %q, want %q", restored.DefaultTerminal, original.DefaultTerminal)
	}
	if restored.DefaultTimeRange != original.DefaultTimeRange {
		t.Errorf("DefaultTimeRange = %q, want %q", restored.DefaultTimeRange, original.DefaultTimeRange)
	}
	if restored.DefaultSort != original.DefaultSort {
		t.Errorf("DefaultSort = %q, want %q", restored.DefaultSort, original.DefaultSort)
	}
	if restored.DefaultPivot != original.DefaultPivot {
		t.Errorf("DefaultPivot = %q, want %q", restored.DefaultPivot, original.DefaultPivot)
	}
	if restored.ShowPreview != original.ShowPreview {
		t.Errorf("ShowPreview = %v, want %v", restored.ShowPreview, original.ShowPreview)
	}
	if restored.MaxSessions != original.MaxSessions {
		t.Errorf("MaxSessions = %d, want %d", restored.MaxSessions, original.MaxSessions)
	}
	if restored.YoloMode != original.YoloMode {
		t.Errorf("YoloMode = %v, want %v", restored.YoloMode, original.YoloMode)
	}
	if restored.Agent != original.Agent {
		t.Errorf("Agent = %q, want %q", restored.Agent, original.Agent)
	}
	if restored.Model != original.Model {
		t.Errorf("Model = %q, want %q", restored.Model, original.Model)
	}
	if restored.LaunchInPlace != original.LaunchInPlace {
		t.Errorf("LaunchInPlace = %v, want %v", restored.LaunchInPlace, original.LaunchInPlace)
	}
	if restored.CustomCommand != original.CustomCommand {
		t.Errorf("CustomCommand = %q, want %q", restored.CustomCommand, original.CustomCommand)
	}
	if len(restored.ExcludedDirs) != len(original.ExcludedDirs) {
		t.Fatalf("ExcludedDirs len = %d, want %d", len(restored.ExcludedDirs), len(original.ExcludedDirs))
	}
	for i := range original.ExcludedDirs {
		if restored.ExcludedDirs[i] != original.ExcludedDirs[i] {
			t.Errorf("ExcludedDirs[%d] = %q, want %q", i, restored.ExcludedDirs[i], original.ExcludedDirs[i])
		}
	}
	if len(restored.HiddenSessions) != len(original.HiddenSessions) {
		t.Fatalf("HiddenSessions len = %d, want %d", len(restored.HiddenSessions), len(original.HiddenSessions))
	}
	for i := range original.HiddenSessions {
		if restored.HiddenSessions[i] != original.HiddenSessions[i] {
			t.Errorf("HiddenSessions[%d] = %q, want %q", i, restored.HiddenSessions[i], original.HiddenSessions[i])
		}
	}
}

func TestDefaultValuesPreservedOnPartialJSON(t *testing.T) {
	// When JSON has only some keys, defaults should fill the rest.
	partialJSON := `{"default_shell": "fish", "yoloMode": true}`

	cfg := Default()
	if err := json.Unmarshal([]byte(partialJSON), cfg); err != nil {
		t.Fatalf("Unmarshal partial JSON: %v", err)
	}

	if cfg.DefaultShell != "fish" {
		t.Errorf("DefaultShell = %q, want 'fish'", cfg.DefaultShell)
	}
	if !cfg.YoloMode {
		t.Error("YoloMode should be true from JSON")
	}
	// These should keep their defaults.
	if cfg.DefaultTimeRange != "1d" {
		t.Errorf("DefaultTimeRange = %q, want '1d' (default)", cfg.DefaultTimeRange)
	}
	if cfg.MaxSessions != 100 {
		t.Errorf("MaxSessions = %d, want 100 (default)", cfg.MaxSessions)
	}
	if !cfg.ShowPreview {
		t.Error("ShowPreview should keep default true")
	}
}

func TestJSONFieldNames(t *testing.T) {
	cfg := &Config{
		DefaultShell:   "zsh",
		YoloMode:       true,
		LaunchInPlace:  true,
		HiddenSessions: []string{"s1"},
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	// Verify camelCase JSON field names match the struct tags.
	expectedKeys := []string{"default_shell", "yoloMode", "launchInPlace", "hiddenSessions"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q not found in marshaled output", key)
		}
	}
}

// ---------------------------------------------------------------------------
// Load / Save integration tests (using temp directory)
// ---------------------------------------------------------------------------

// withTempConfigDir temporarily overrides the OS config directory env var
// and returns a cleanup function to restore it.
func withTempConfigDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", tmpDir)
	case "darwin":
		// On macOS, os.UserConfigDir() returns ~/Library/Application Support.
		// We can't easily override it without HOME.
		t.Setenv("HOME", tmpDir)
		// Create the Library/Application Support structure.
		appSupportDir := filepath.Join(tmpDir, "Library", "Application Support")
		if err := os.MkdirAll(appSupportDir, 0o755); err != nil {
			t.Fatalf("creating macOS config dir: %v", err)
		}
	default:
		// Linux: XDG_CONFIG_HOME takes precedence.
		t.Setenv("XDG_CONFIG_HOME", tmpDir)
	}

	return tmpDir
}

func TestSaveAndLoad(t *testing.T) {
	withTempConfigDir(t)

	original := &Config{
		DefaultShell:     "bash",
		DefaultTerminal:  "alacritty",
		DefaultTimeRange: "30d",
		DefaultSort:      "turns",
		DefaultPivot:     "repo",
		ShowPreview:      false,
		MaxSessions:      200,
		YoloMode:         true,
		Agent:            "reviewer",
		Model:            "claude-3",
		LaunchInPlace:    true,
		ExcludedDirs:     []string{"/opt/scratch"},
		CustomCommand:    "my-cli --resume {sessionId}",
		HiddenSessions:   []string{"hidden-1"},
	}

	if err := Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.DefaultShell != original.DefaultShell {
		t.Errorf("DefaultShell = %q, want %q", loaded.DefaultShell, original.DefaultShell)
	}
	if loaded.DefaultTerminal != original.DefaultTerminal {
		t.Errorf("DefaultTerminal = %q, want %q", loaded.DefaultTerminal, original.DefaultTerminal)
	}
	if loaded.DefaultTimeRange != original.DefaultTimeRange {
		t.Errorf("DefaultTimeRange = %q, want %q", loaded.DefaultTimeRange, original.DefaultTimeRange)
	}
	if loaded.MaxSessions != original.MaxSessions {
		t.Errorf("MaxSessions = %d, want %d", loaded.MaxSessions, original.MaxSessions)
	}
	if loaded.YoloMode != original.YoloMode {
		t.Errorf("YoloMode = %v, want %v", loaded.YoloMode, original.YoloMode)
	}
	if loaded.Agent != original.Agent {
		t.Errorf("Agent = %q, want %q", loaded.Agent, original.Agent)
	}
	if loaded.Model != original.Model {
		t.Errorf("Model = %q, want %q", loaded.Model, original.Model)
	}
	if loaded.LaunchInPlace != original.LaunchInPlace {
		t.Errorf("LaunchInPlace = %v, want %v", loaded.LaunchInPlace, original.LaunchInPlace)
	}
	if loaded.CustomCommand != original.CustomCommand {
		t.Errorf("CustomCommand = %q, want %q", loaded.CustomCommand, original.CustomCommand)
	}
}

func TestLoadReturnsDefaultsWhenNoFile(t *testing.T) {
	withTempConfigDir(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with no config file: %v", err)
	}

	def := Default()
	if cfg.DefaultTimeRange != def.DefaultTimeRange {
		t.Errorf("DefaultTimeRange = %q, want default %q", cfg.DefaultTimeRange, def.DefaultTimeRange)
	}
	if cfg.MaxSessions != def.MaxSessions {
		t.Errorf("MaxSessions = %d, want default %d", cfg.MaxSessions, def.MaxSessions)
	}
	if cfg.ShowPreview != def.ShowPreview {
		t.Errorf("ShowPreview = %v, want default %v", cfg.ShowPreview, def.ShowPreview)
	}
}

func TestLoadCorruptJSON(t *testing.T) {
	withTempConfigDir(t)

	// Write invalid JSON to the config path.
	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("{invalid json!!!"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = Load()
	if err == nil {
		t.Fatal("Load should fail with corrupt JSON")
	}
}

func TestLoadPartialJSON(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Write JSON with only one field.
	if err := os.WriteFile(path, []byte(`{"default_shell": "fish"}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load partial JSON: %v", err)
	}
	if cfg.DefaultShell != "fish" {
		t.Errorf("DefaultShell = %q, want 'fish'", cfg.DefaultShell)
	}
	// Other fields should have defaults.
	if cfg.DefaultTimeRange != "1d" {
		t.Errorf("DefaultTimeRange = %q, want default '1d'", cfg.DefaultTimeRange)
	}
	if cfg.MaxSessions != 100 {
		t.Errorf("MaxSessions = %d, want default 100", cfg.MaxSessions)
	}
}

func TestSaveCreatesDirectories(t *testing.T) {
	withTempConfigDir(t)

	cfg := Default()
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("config file not created at %s: %v", path, err)
	}
}

func TestSaveOverwrites(t *testing.T) {
	withTempConfigDir(t)

	cfg1 := Default()
	cfg1.DefaultShell = "bash"
	if err := Save(cfg1); err != nil {
		t.Fatalf("Save cfg1: %v", err)
	}

	cfg2 := Default()
	cfg2.DefaultShell = "zsh"
	if err := Save(cfg2); err != nil {
		t.Fatalf("Save cfg2: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.DefaultShell != "zsh" {
		t.Errorf("DefaultShell = %q, want 'zsh' (from second save)", loaded.DefaultShell)
	}
}

func TestSaveEmptyConfig(t *testing.T) {
	withTempConfigDir(t)

	cfg := &Config{} // zero-value config
	if err := Save(cfg); err != nil {
		t.Fatalf("Save empty config: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Zero-value strings and ints should round-trip.
	if loaded.DefaultShell != "" {
		t.Errorf("DefaultShell = %q, want empty", loaded.DefaultShell)
	}
	if loaded.MaxSessions != 0 {
		t.Errorf("MaxSessions = %d, want 0", loaded.MaxSessions)
	}
}

func TestConfigPathContainsDispatch(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	dir := filepath.Dir(path)
	if filepath.Base(dir) != "dispatch" {
		t.Errorf("config dir should end with 'dispatch', got %q", dir)
	}
	if filepath.Base(path) != "config.json" {
		t.Errorf("config file name = %q, want 'config.json'", filepath.Base(path))
	}
}

// ---------------------------------------------------------------------------
// Additional Load / Save coverage tests
// ---------------------------------------------------------------------------

func TestLoadReadError_NotPermission(t *testing.T) {
	// When the config path exists but is a directory (not a file),
	// os.ReadFile returns a non-ErrNotExist error that Load should propagate.
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	// Create the config path as a directory instead of a file.
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	_, err = Load()
	if err == nil {
		t.Fatal("Load should fail when config path is a directory")
	}
}

func TestSaveAndLoadPreservesAllFields(t *testing.T) {
	withTempConfigDir(t)

	cfg := &Config{
		DefaultShell:     "fish",
		DefaultTerminal:  "wezterm",
		DefaultTimeRange: "all",
		DefaultSort:      "name",
		DefaultPivot:     "date",
		ShowPreview:      false,
		MaxSessions:      500,
		YoloMode:         true,
		Agent:            "developer",
		Model:            "o1-preview",
		LaunchInPlace:    true,
		ExcludedDirs:     []string{"/a", "/b", "/c"},
		CustomCommand:    "custom-cli {sessionId} --flag",
		HiddenSessions:   []string{"h1", "h2", "h3"},
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Verify every single field round-trips.
	if loaded.DefaultShell != cfg.DefaultShell {
		t.Errorf("DefaultShell = %q, want %q", loaded.DefaultShell, cfg.DefaultShell)
	}
	if loaded.DefaultTerminal != cfg.DefaultTerminal {
		t.Errorf("DefaultTerminal = %q, want %q", loaded.DefaultTerminal, cfg.DefaultTerminal)
	}
	if loaded.DefaultTimeRange != cfg.DefaultTimeRange {
		t.Errorf("DefaultTimeRange = %q, want %q", loaded.DefaultTimeRange, cfg.DefaultTimeRange)
	}
	if loaded.DefaultSort != cfg.DefaultSort {
		t.Errorf("DefaultSort = %q, want %q", loaded.DefaultSort, cfg.DefaultSort)
	}
	if loaded.DefaultPivot != cfg.DefaultPivot {
		t.Errorf("DefaultPivot = %q, want %q", loaded.DefaultPivot, cfg.DefaultPivot)
	}
	if loaded.ShowPreview != cfg.ShowPreview {
		t.Errorf("ShowPreview = %v, want %v", loaded.ShowPreview, cfg.ShowPreview)
	}
	if loaded.MaxSessions != cfg.MaxSessions {
		t.Errorf("MaxSessions = %d, want %d", loaded.MaxSessions, cfg.MaxSessions)
	}
	if loaded.YoloMode != cfg.YoloMode {
		t.Errorf("YoloMode = %v, want %v", loaded.YoloMode, cfg.YoloMode)
	}
	if loaded.Agent != cfg.Agent {
		t.Errorf("Agent = %q, want %q", loaded.Agent, cfg.Agent)
	}
	if loaded.Model != cfg.Model {
		t.Errorf("Model = %q, want %q", loaded.Model, cfg.Model)
	}
	if loaded.LaunchInPlace != cfg.LaunchInPlace {
		t.Errorf("LaunchInPlace = %v, want %v", loaded.LaunchInPlace, cfg.LaunchInPlace)
	}
	if loaded.CustomCommand != cfg.CustomCommand {
		t.Errorf("CustomCommand = %q, want %q", loaded.CustomCommand, cfg.CustomCommand)
	}
	if len(loaded.ExcludedDirs) != len(cfg.ExcludedDirs) {
		t.Fatalf("ExcludedDirs len = %d, want %d", len(loaded.ExcludedDirs), len(cfg.ExcludedDirs))
	}
	for i := range cfg.ExcludedDirs {
		if loaded.ExcludedDirs[i] != cfg.ExcludedDirs[i] {
			t.Errorf("ExcludedDirs[%d] = %q, want %q", i, loaded.ExcludedDirs[i], cfg.ExcludedDirs[i])
		}
	}
	if len(loaded.HiddenSessions) != len(cfg.HiddenSessions) {
		t.Fatalf("HiddenSessions len = %d, want %d", len(loaded.HiddenSessions), len(cfg.HiddenSessions))
	}
	for i := range cfg.HiddenSessions {
		if loaded.HiddenSessions[i] != cfg.HiddenSessions[i] {
			t.Errorf("HiddenSessions[%d] = %q, want %q", i, loaded.HiddenSessions[i], cfg.HiddenSessions[i])
		}
	}
}

func TestSaveFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission checks not meaningful on Windows")
	}

	withTempConfigDir(t)

	cfg := Default()
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	// Save uses 0o600, so only owner should have read/write.
	perm := info.Mode().Perm()
	if perm&0o077 != 0 {
		t.Errorf("config file permissions = %o, want no group/other access (0600)", perm)
	}
}

func TestLoadEmptyFile(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Write empty file — json.Unmarshal("") should fail.
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = Load()
	if err == nil {
		t.Fatal("Load should fail on empty file (invalid JSON)")
	}
}

func TestLoadFileWithExtraFields(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// JSON with unknown fields should not cause errors (Go ignores unknown fields).
	content := `{"default_shell": "zsh", "unknown_field": true, "future_feature": 42}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with extra fields: %v", err)
	}
	if cfg.DefaultShell != "zsh" {
		t.Errorf("DefaultShell = %q, want 'zsh'", cfg.DefaultShell)
	}
	// Defaults should be preserved for unspecified fields.
	if cfg.DefaultTimeRange != "1d" {
		t.Errorf("DefaultTimeRange = %q, want '1d'", cfg.DefaultTimeRange)
	}
}

func TestConfigPathFormat(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("configPath should return absolute path, got %q", path)
	}
	if filepath.Base(path) != "config.json" {
		t.Errorf("config file name = %q, want 'config.json'", filepath.Base(path))
	}
	dir := filepath.Dir(path)
	if filepath.Base(dir) != "dispatch" {
		t.Errorf("config dir = %q, want parent 'dispatch'", dir)
	}
}

func TestDefaultFieldTypes(t *testing.T) {
	cfg := Default()

	// Verify zero-value fields are indeed their zero values.
	if cfg.YoloMode != false {
		t.Error("YoloMode should default to false")
	}
	if cfg.LaunchInPlace != false {
		t.Error("LaunchInPlace should default to false")
	}
	if cfg.Agent != "" {
		t.Errorf("Agent = %q, want empty", cfg.Agent)
	}
	if cfg.Model != "" {
		t.Errorf("Model = %q, want empty", cfg.Model)
	}
	if cfg.CustomCommand != "" {
		t.Errorf("CustomCommand = %q, want empty", cfg.CustomCommand)
	}
	if len(cfg.ExcludedDirs) != 0 {
		t.Errorf("ExcludedDirs should be nil or empty, got %v", cfg.ExcludedDirs)
	}
	if len(cfg.HiddenSessions) != 0 {
		t.Errorf("HiddenSessions should be nil or empty, got %v", cfg.HiddenSessions)
	}
}

// ---------------------------------------------------------------------------
// configPath / Load / Save error paths via broken config dir
// ---------------------------------------------------------------------------

// clearConfigDirEnv sets the OS-specific config dir env var to empty,
// causing os.UserConfigDir() (and therefore configPath) to fail.
func clearConfigDirEnv(t *testing.T) {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", "")
	case "darwin":
		t.Setenv("HOME", "")
	default:
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")
	}
}

func TestConfigPath_ErrorWhenConfigDirFails(t *testing.T) {
	clearConfigDirEnv(t)

	_, err := configPath()
	if err == nil {
		t.Fatal("configPath should fail when config dir env is cleared")
	}
}

func TestLoad_ErrorWhenConfigPathFails(t *testing.T) {
	clearConfigDirEnv(t)

	_, err := Load()
	if err == nil {
		t.Fatal("Load should fail when configPath fails")
	}
}

func TestSave_ErrorWhenConfigPathFails(t *testing.T) {
	clearConfigDirEnv(t)

	err := Save(Default())
	if err == nil {
		t.Fatal("Save should fail when configPath fails")
	}
}

// ---------------------------------------------------------------------------
// EffectiveLaunchMode tests
// ---------------------------------------------------------------------------

func TestEffectiveLaunchMode_ExplicitMode(t *testing.T) {
	tests := []struct {
		mode string
		want string
	}{
		{"in-place", "in-place"},
		{"tab", "tab"},
		{"window", "window"},
		{"pane", "pane"},
	}
	for _, tt := range tests {
		cfg := &Config{LaunchMode: tt.mode, LaunchInPlace: true}
		if got := cfg.EffectiveLaunchMode(); got != tt.want {
			t.Errorf("LaunchMode=%q, LaunchInPlace=true: got %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestEffectiveLaunchMode_BackwardCompat_InPlace(t *testing.T) {
	cfg := &Config{LaunchInPlace: true}
	if got := cfg.EffectiveLaunchMode(); got != "in-place" {
		t.Errorf("legacy LaunchInPlace=true: got %q, want 'in-place'", got)
	}
}

func TestEffectiveLaunchMode_Default(t *testing.T) {
	cfg := Default()
	if got := cfg.EffectiveLaunchMode(); got != "tab" {
		t.Errorf("default config: got %q, want 'tab'", got)
	}
}

func TestEffectiveLaunchMode_JSONBackwardCompat(t *testing.T) {
	// Simulate a config file from before LaunchMode existed.
	legacyJSON := `{"launchInPlace": true}`
	cfg := Default()
	if err := json.Unmarshal([]byte(legacyJSON), cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg.EffectiveLaunchMode(); got != "in-place" {
		t.Errorf("legacy JSON: got %q, want 'in-place'", got)
	}
}

func TestEffectiveLaunchMode_NewFieldOverridesLegacy(t *testing.T) {
	// LaunchMode takes precedence over LaunchInPlace.
	mixedJSON := `{"launchInPlace": true, "launch_mode": "window"}`
	cfg := Default()
	if err := json.Unmarshal([]byte(mixedJSON), cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg.EffectiveLaunchMode(); got != "window" {
		t.Errorf("mixed JSON: got %q, want 'window'", got)
	}
}

// ---------------------------------------------------------------------------
// EffectivePaneDirection tests
// ---------------------------------------------------------------------------

func TestEffectivePaneDirection_ExplicitDirection(t *testing.T) {
	tests := []struct {
		dir  string
		want string
	}{
		{"auto", "auto"},
		{"right", "right"},
		{"down", "down"},
		{"left", "left"},
		{"up", "up"},
	}
	for _, tt := range tests {
		cfg := &Config{PaneDirection: tt.dir}
		if got := cfg.EffectivePaneDirection(); got != tt.want {
			t.Errorf("PaneDirection=%q: got %q, want %q", tt.dir, got, tt.want)
		}
	}
}

func TestEffectivePaneDirection_Default(t *testing.T) {
	cfg := Default()
	if got := cfg.EffectivePaneDirection(); got != "auto" {
		t.Errorf("default config: got %q, want 'auto'", got)
	}
}

func TestEffectivePaneDirection_EmptyDefaultsToAuto(t *testing.T) {
	cfg := &Config{}
	if got := cfg.EffectivePaneDirection(); got != "auto" {
		t.Errorf("empty PaneDirection: got %q, want 'auto'", got)
	}
}

func TestEffectivePaneDirection_JSONRoundTrip(t *testing.T) {
	jsonStr := `{"launch_mode": "pane", "pane_direction": "left"}`
	cfg := Default()
	if err := json.Unmarshal([]byte(jsonStr), cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg.EffectivePaneDirection(); got != "left" {
		t.Errorf("JSON pane_direction: got %q, want 'left'", got)
	}
}

func TestPaneDirectionConstants(t *testing.T) {
	// Verify constant values match expected strings.
	if PaneDirectionAuto != "auto" {
		t.Errorf("PaneDirectionAuto = %q, want 'auto'", PaneDirectionAuto)
	}
	if PaneDirectionRight != "right" {
		t.Errorf("PaneDirectionRight = %q, want 'right'", PaneDirectionRight)
	}
	if PaneDirectionDown != "down" {
		t.Errorf("PaneDirectionDown = %q, want 'down'", PaneDirectionDown)
	}
	if PaneDirectionLeft != "left" {
		t.Errorf("PaneDirectionLeft = %q, want 'left'", PaneDirectionLeft)
	}
	if PaneDirectionUp != "up" {
		t.Errorf("PaneDirectionUp = %q, want 'up'", PaneDirectionUp)
	}
}

// ---------------------------------------------------------------------------
// MaxSessions clamping tests
// ---------------------------------------------------------------------------

func TestLoad_ClampsMaxSessionsUpperBound(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Set max_sessions to a value exceeding maxMaxSessions.
	content := `{"max_sessions": 999999}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MaxSessions != maxMaxSessions {
		t.Errorf("MaxSessions = %d, want %d (clamped)", cfg.MaxSessions, maxMaxSessions)
	}
}

func TestLoad_ClampsNegativeMaxSessions(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := `{"max_sessions": -5}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MaxSessions != 0 {
		t.Errorf("MaxSessions = %d, want 0 (clamped)", cfg.MaxSessions)
	}
}
