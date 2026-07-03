package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/config"
)

// collapseSpaces rewrites runs of spaces to a single space so tests can assert
// on aligned "key = value" output without hard-coding column widths.
func collapseSpaces(s string) string {
	return regexp.MustCompile(` +`).ReplaceAllString(s, " ")
}

// withConfigSeams substitutes the config load/save/path seams for a test and
// returns a pointer to the in-memory config that set operations mutate.
func withConfigSeams(t *testing.T, initial *config.Config) *config.Config {
	t.Helper()
	if initial == nil {
		initial = config.Default()
	}
	prevLoad, prevSave, prevPath := configLoadFn, configSaveFn, configPathFn
	configLoadFn = func() (*config.Config, error) { return initial, nil }
	configSaveFn = func(c *config.Config) error { initial = c; return nil }
	configPathFn = func() (string, error) { return "/tmp/dispatch/config.json", nil }
	t.Cleanup(func() {
		configLoadFn, configSaveFn, configPathFn = prevLoad, prevSave, prevPath
	})
	return initial
}

func TestRunConfigList_Text(t *testing.T) {
	cfg := config.Default()
	cfg.DefaultShell = "pwsh"
	withConfigSeams(t, cfg)

	var buf bytes.Buffer
	if err := runConfig(&buf, []string{"config", "list"}); err != nil {
		t.Fatalf("runConfig list: %v", err)
	}
	out := collapseSpaces(buf.String())
	if !strings.Contains(out, "default_shell = pwsh") {
		t.Errorf("list output missing default_shell line:\n%s", out)
	}
	if !strings.Contains(out, "show_preview = true") {
		t.Errorf("list output missing show_preview line:\n%s", out)
	}
}

func TestRunConfigList_DefaultsToList(t *testing.T) {
	withConfigSeams(t, config.Default())

	var buf bytes.Buffer
	if err := runConfig(&buf, []string{"config"}); err != nil {
		t.Fatalf("runConfig with no subcommand: %v", err)
	}
	if !strings.Contains(collapseSpaces(buf.String()), "max_sessions = 100") {
		t.Errorf("bare config should list settings, got:\n%s", buf.String())
	}
}

func TestRunConfigList_JSON(t *testing.T) {
	cfg := config.Default()
	cfg.MaxSessions = 42
	cfg.AISearch = true
	withConfigSeams(t, cfg)

	var buf bytes.Buffer
	if err := runConfig(&buf, []string{"config", "list", "--json"}); err != nil {
		t.Fatalf("runConfig list --json: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(buf.Bytes(), &obj); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if got, ok := obj["max_sessions"].(float64); !ok || int(got) != 42 {
		t.Errorf("max_sessions = %v, want 42", obj["max_sessions"])
	}
	if got, ok := obj["ai_search"].(bool); !ok || !got {
		t.Errorf("ai_search = %v, want true", obj["ai_search"])
	}
	// auto_refresh_seconds is unset by default and should serialize as null.
	if v, present := obj["auto_refresh_seconds"]; !present || v != nil {
		t.Errorf("auto_refresh_seconds = %v, want null", v)
	}
}

func TestRunConfigGet(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "dracula"
	withConfigSeams(t, cfg)

	var buf bytes.Buffer
	if err := runConfig(&buf, []string{"config", "get", "theme"}); err != nil {
		t.Fatalf("runConfig get: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "dracula" {
		t.Errorf("get theme = %q, want dracula", buf.String())
	}
}

func TestRunConfigGet_UnknownKey(t *testing.T) {
	withConfigSeams(t, config.Default())

	err := runConfig(&bytes.Buffer{}, []string{"config", "get", "nope"})
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	if !strings.Contains(err.Error(), "unknown config key") {
		t.Errorf("error = %v, want unknown config key", err)
	}
}

func TestRunConfigGet_RequiresKey(t *testing.T) {
	withConfigSeams(t, config.Default())
	if err := runConfig(&bytes.Buffer{}, []string{"config", "get"}); err == nil {
		t.Fatal("expected error when get has no key")
	}
}

func TestRunConfigSet_String(t *testing.T) {
	cfg := withConfigSeams(t, config.Default())

	var buf bytes.Buffer
	if err := runConfig(&buf, []string{"config", "set", "agent", "gpt-5"}); err != nil {
		t.Fatalf("runConfig set: %v", err)
	}
	if cfg.Agent != "gpt-5" {
		t.Errorf("Agent = %q, want gpt-5", cfg.Agent)
	}
	if !strings.Contains(buf.String(), "agent = gpt-5") {
		t.Errorf("set output = %q, want confirmation", buf.String())
	}
}

func TestRunConfigSet_Bool(t *testing.T) {
	cfg := withConfigSeams(t, config.Default())

	if err := runConfig(&bytes.Buffer{}, []string{"config", "set", "show_preview", "false"}); err != nil {
		t.Fatalf("runConfig set bool: %v", err)
	}
	if cfg.ShowPreview {
		t.Error("ShowPreview = true, want false after set")
	}
}

func TestRunConfigSet_BoolInvalid(t *testing.T) {
	withConfigSeams(t, config.Default())
	err := runConfig(&bytes.Buffer{}, []string{"config", "set", "show_preview", "maybe"})
	if err == nil || !strings.Contains(err.Error(), "true or false") {
		t.Errorf("error = %v, want true/false guidance", err)
	}
}

func TestRunConfigSet_Int(t *testing.T) {
	cfg := withConfigSeams(t, config.Default())

	if err := runConfig(&bytes.Buffer{}, []string{"config", "set", "max_sessions", "250"}); err != nil {
		t.Fatalf("runConfig set int: %v", err)
	}
	if cfg.MaxSessions != 250 {
		t.Errorf("MaxSessions = %d, want 250", cfg.MaxSessions)
	}
}

func TestRunConfigSet_IntNegativeRejected(t *testing.T) {
	withConfigSeams(t, config.Default())
	if err := runConfig(&bytes.Buffer{}, []string{"config", "set", "max_sessions", "-1"}); err == nil {
		t.Fatal("expected error for negative max_sessions")
	}
}

func TestRunConfigSet_EnumValid(t *testing.T) {
	cfg := withConfigSeams(t, config.Default())

	if err := runConfig(&bytes.Buffer{}, []string{"config", "set", "launch_mode", "window"}); err != nil {
		t.Fatalf("runConfig set enum: %v", err)
	}
	if cfg.LaunchMode != config.LaunchModeWindow {
		t.Errorf("LaunchMode = %q, want window", cfg.LaunchMode)
	}
}

func TestRunConfigSet_EnumInvalid(t *testing.T) {
	withConfigSeams(t, config.Default())
	err := runConfig(&bytes.Buffer{}, []string{"config", "set", "launch_mode", "hologram"})
	if err == nil || !strings.Contains(err.Error(), "invalid value") {
		t.Errorf("error = %v, want invalid value", err)
	}
}

func TestRunConfigSet_Duration(t *testing.T) {
	cfg := withConfigSeams(t, config.Default())

	if err := runConfig(&bytes.Buffer{}, []string{"config", "set", "attention_threshold", "30m"}); err != nil {
		t.Fatalf("runConfig set duration: %v", err)
	}
	if cfg.AttentionThreshold != "30m" {
		t.Errorf("AttentionThreshold = %q, want 30m", cfg.AttentionThreshold)
	}

	if err := runConfig(&bytes.Buffer{}, []string{"config", "set", "attention_threshold", "soon"}); err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestRunConfigSet_AutoRefreshDefaultUnsets(t *testing.T) {
	cfg := config.Default()
	n := 10
	cfg.AutoRefreshSeconds = &n
	cfg = withConfigSeams(t, cfg)

	if err := runConfig(&bytes.Buffer{}, []string{"config", "set", "auto_refresh_seconds", "default"}); err != nil {
		t.Fatalf("runConfig set auto_refresh_seconds default: %v", err)
	}
	if cfg.AutoRefreshSeconds != nil {
		t.Errorf("AutoRefreshSeconds = %v, want nil after default", *cfg.AutoRefreshSeconds)
	}
}

func TestRunConfigSet_AutoRefreshNumber(t *testing.T) {
	cfg := withConfigSeams(t, config.Default())

	if err := runConfig(&bytes.Buffer{}, []string{"config", "set", "auto_refresh_seconds", "0"}); err != nil {
		t.Fatalf("runConfig set auto_refresh_seconds 0: %v", err)
	}
	if cfg.AutoRefreshSeconds == nil || *cfg.AutoRefreshSeconds != 0 {
		t.Errorf("AutoRefreshSeconds = %v, want 0", cfg.AutoRefreshSeconds)
	}
}

func TestRunConfigSet_UnknownKey(t *testing.T) {
	withConfigSeams(t, config.Default())
	if err := runConfig(&bytes.Buffer{}, []string{"config", "set", "nope", "1"}); err == nil {
		t.Fatal("expected error for unknown key on set")
	}
}

func TestRunConfigSet_RequiresKeyAndValue(t *testing.T) {
	withConfigSeams(t, config.Default())
	if err := runConfig(&bytes.Buffer{}, []string{"config", "set", "agent"}); err == nil {
		t.Fatal("expected error when set is missing a value")
	}
}

func TestRunConfigSet_SaveError(t *testing.T) {
	withConfigSeams(t, config.Default())
	configSaveFn = func(*config.Config) error { return errors.New("disk full") }

	err := runConfig(&bytes.Buffer{}, []string{"config", "set", "agent", "x"})
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Errorf("error = %v, want save failure", err)
	}
}

func TestRunConfigPath(t *testing.T) {
	withConfigSeams(t, config.Default())

	var buf bytes.Buffer
	if err := runConfig(&buf, []string{"config", "path"}); err != nil {
		t.Fatalf("runConfig path: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "/tmp/dispatch/config.json" {
		t.Errorf("path = %q, want config.json path", buf.String())
	}
}

func TestRunConfig_UnknownSubcommand(t *testing.T) {
	withConfigSeams(t, config.Default())
	err := runConfig(&bytes.Buffer{}, []string{"config", "frobnicate"})
	if err == nil || !strings.Contains(err.Error(), "unknown config subcommand") {
		t.Errorf("error = %v, want unknown subcommand", err)
	}
}

func TestConfigFields_RoundTrip(t *testing.T) {
	// Every field's get must return a value its own set accepts, so a
	// list -> set loop never rejects a value dispatch itself produced.
	cfg := config.Default()
	for _, f := range configFields() {
		val := f.get(cfg)
		if err := f.set(cfg, val); err != nil {
			t.Errorf("field %q: set(get()) rejected %q: %v", f.name, val, err)
		}
	}
}

func TestHandleArgs_Config(t *testing.T) {
	withConfigSeams(t, config.Default())

	done, cleanup, _, err := handleArgs([]string{"config", "list"}, &bytes.Buffer{}, nil)
	if !done {
		t.Error("expected done=true for config")
	}
	if cleanup != nil {
		t.Error("expected cleanup=nil for config")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleArgs_ConfigError(t *testing.T) {
	withConfigSeams(t, config.Default())

	done, _, _, err := handleArgs([]string{"config", "get", "bogus"}, &bytes.Buffer{}, nil)
	if !done {
		t.Error("expected done=true for config error")
	}
	if err == nil {
		t.Error("expected error for unknown config key")
	}
}
