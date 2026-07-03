package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jongio/dispatch/internal/config"
)

// configLoadFn and configSaveFn are package variables so tests can substitute
// the config read/write path, matching the seam pattern used elsewhere in this
// package (see cli.go and stats.go).
var (
	configLoadFn = config.Load
	configSaveFn = config.Save
	configPathFn = config.ConfigPath
)

// configFieldKind labels the value type of a settable preference so list and
// JSON output can render it correctly and set can parse the value.
type configFieldKind string

const (
	configKindString configFieldKind = "string"
	configKindBool   configFieldKind = "bool"
	configKindInt    configFieldKind = "int"
)

// configField describes one preference that the config command can read and
// write. get returns the current value as a string ("" for unset). set parses
// and validates a string value into the config, returning an error on bad
// input.
type configField struct {
	name string
	kind configFieldKind
	get  func(*config.Config) string
	set  func(*config.Config, string) error
}

// configFields returns the settable preferences in a stable display order.
// Only scalar settings are exposed; list/map settings (views, hidden sessions,
// notes, color schemes) are edited in the TUI or by hand.
func configFields() []configField {
	return []configField{
		strField("default_shell", func(c *config.Config) *string { return &c.DefaultShell }),
		strField("default_terminal", func(c *config.Config) *string { return &c.DefaultTerminal }),
		enumField("default_time_range", func(c *config.Config) *string { return &c.DefaultTimeRange },
			config.TimeRange1h, config.TimeRange1d, config.TimeRange7d, config.TimeRangeAll),
		enumField("default_sort", func(c *config.Config) *string { return &c.DefaultSort },
			config.SortFieldUpdated, config.SortFieldCreated, config.SortFieldTurns, config.SortFieldName, config.SortFieldFolder),
		enumField("default_sort_order", func(c *config.Config) *string { return &c.DefaultSortOrder },
			config.SortOrderAsc, config.SortOrderDesc),
		enumField("default_pivot", func(c *config.Config) *string { return &c.DefaultPivot },
			config.PivotNone, config.PivotFolder, config.PivotRepo, config.PivotBranch, config.PivotDate, config.PivotHost),
		boolField("show_preview", func(c *config.Config) *bool { return &c.ShowPreview }),
		maxSessionsField(),
		boolField("yoloMode", func(c *config.Config) *bool { return &c.YoloMode }),
		strField("agent", func(c *config.Config) *string { return &c.Agent }),
		strField("model", func(c *config.Config) *string { return &c.Model }),
		enumField("launch_mode", func(c *config.Config) *string { return &c.LaunchMode },
			config.LaunchModeInPlace, config.LaunchModeTab, config.LaunchModeWindow, config.LaunchModePane),
		enumField("pane_direction", func(c *config.Config) *string { return &c.PaneDirection },
			config.PaneDirectionAuto, config.PaneDirectionRight, config.PaneDirectionDown, config.PaneDirectionLeft, config.PaneDirectionUp),
		strField("custom_command", func(c *config.Config) *string { return &c.CustomCommand }),
		boolField("ai_search", func(c *config.Config) *bool { return &c.AISearch }),
		durationField("attention_threshold", func(c *config.Config) *string { return &c.AttentionThreshold }),
		strField("theme", func(c *config.Config) *string { return &c.Theme }),
		enumField("preview_position", func(c *config.Config) *string { return &c.PreviewPosition },
			config.PreviewPositionRight, config.PreviewPositionBottom, config.PreviewPositionLeft, config.PreviewPositionTop),
		boolField("default_collapsed", func(c *config.Config) *bool { return &c.DefaultCollapsed }),
		boolField("conversation_newest_first", func(c *config.Config) *bool { return &c.ConversationNewestFirst }),
		boolField("workspace_recovery", func(c *config.Config) *bool { return &c.WorkspaceRecovery }),
		boolField("redact_preview_secrets", func(c *config.Config) *bool { return &c.RedactPreviewSecrets }),
		autoRefreshField(),
	}
}

// strField builds a free-form string preference.
func strField(name string, ptr func(*config.Config) *string) configField {
	return configField{
		name: name,
		kind: configKindString,
		get:  func(c *config.Config) string { return *ptr(c) },
		set: func(c *config.Config, v string) error {
			*ptr(c) = v
			return nil
		},
	}
}

// enumField builds a string preference restricted to a fixed set of values.
// An empty value is always allowed and clears the setting back to its default.
func enumField(name string, ptr func(*config.Config) *string, allowed ...string) configField {
	return configField{
		name: name,
		kind: configKindString,
		get:  func(c *config.Config) string { return *ptr(c) },
		set: func(c *config.Config, v string) error {
			if v != "" {
				valid := false
				for _, a := range allowed {
					if v == a {
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("invalid value %q for %s (want one of: %s)", v, name, strings.Join(allowed, ", "))
				}
			}
			*ptr(c) = v
			return nil
		},
	}
}

// boolField builds a boolean preference. It accepts the usual truthy/falsy
// spellings understood by strconv.ParseBool.
func boolField(name string, ptr func(*config.Config) *bool) configField {
	return configField{
		name: name,
		kind: configKindBool,
		get:  func(c *config.Config) string { return strconv.FormatBool(*ptr(c)) },
		set: func(c *config.Config, v string) error {
			b, err := strconv.ParseBool(strings.TrimSpace(v))
			if err != nil {
				return fmt.Errorf("invalid value %q for %s (want true or false)", v, name)
			}
			*ptr(c) = b
			return nil
		},
	}
}

// maxSessionsField builds the max_sessions preference with a non-negative
// integer constraint.
func maxSessionsField() configField {
	return configField{
		name: "max_sessions",
		kind: configKindInt,
		get:  func(c *config.Config) string { return strconv.Itoa(c.MaxSessions) },
		set: func(c *config.Config, v string) error {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return fmt.Errorf("invalid value %q for max_sessions (want a whole number)", v)
			}
			if n < 0 {
				return fmt.Errorf("max_sessions must be zero or greater, got %d", n)
			}
			c.MaxSessions = n
			return nil
		},
	}
}

// durationField builds a preference whose value must parse as a positive Go
// duration (e.g. "15m", "1h"). An empty value clears the setting.
func durationField(name string, ptr func(*config.Config) *string) configField {
	return configField{
		name: name,
		kind: configKindString,
		get:  func(c *config.Config) string { return *ptr(c) },
		set: func(c *config.Config, v string) error {
			if v != "" {
				d, err := time.ParseDuration(v)
				if err != nil || d <= 0 {
					return fmt.Errorf("invalid value %q for %s (want a positive duration like 15m or 1h)", v, name)
				}
			}
			*ptr(c) = v
			return nil
		},
	}
}

// autoRefreshField builds the auto_refresh_seconds preference. It is stored as
// a pointer so "unset" and "0" are distinct: set "default" clears it to unset,
// and any integer sets an explicit interval (0 disables polling).
func autoRefreshField() configField {
	return configField{
		name: "auto_refresh_seconds",
		kind: configKindInt,
		get: func(c *config.Config) string {
			if c.AutoRefreshSeconds == nil {
				return ""
			}
			return strconv.Itoa(*c.AutoRefreshSeconds)
		},
		set: func(c *config.Config, v string) error {
			trimmed := strings.TrimSpace(v)
			if trimmed == "" || trimmed == "default" {
				c.AutoRefreshSeconds = nil
				return nil
			}
			n, err := strconv.Atoi(trimmed)
			if err != nil {
				return fmt.Errorf("invalid value %q for auto_refresh_seconds (want a whole number or \"default\")", v)
			}
			c.AutoRefreshSeconds = &n
			return nil
		},
	}
}

// findConfigField returns the field with the given name, or false if unknown.
func findConfigField(name string) (configField, bool) {
	for _, f := range configFields() {
		if f.name == name {
			return f, true
		}
	}
	return configField{}, false
}

// runConfig reads or writes user preferences from the command line. args is the
// full argument slice with args[0] == "config".
func runConfig(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	rest := args
	if len(rest) > 0 {
		rest = rest[1:] // drop the "config" token
	}

	// No subcommand behaves like "list".
	sub := "list"
	if len(rest) > 0 {
		sub = rest[0]
		rest = rest[1:]
	}

	switch sub {
	case "list":
		return runConfigList(w, rest)
	case "get":
		return runConfigGet(w, rest)
	case "set":
		return runConfigSet(w, rest)
	case "path":
		return runConfigPath(w, rest)
	default:
		return fmt.Errorf("unknown config subcommand %q (want list, get, set, or path)", sub)
	}
}

// runConfigList prints every setting and its current value. With --json the
// output is a single JSON object keyed by setting name.
func runConfigList(w io.Writer, args []string) error {
	jsonOut := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("config list does not take arguments, got %q", arg)
		}
	}

	cfg, err := configLoadFn()
	if err != nil {
		return err
	}

	fields := configFields()
	if jsonOut {
		obj := make(map[string]any, len(fields))
		for _, f := range fields {
			obj[f.name] = fieldJSONValue(f, cfg)
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(obj)
	}

	width := 0
	for _, f := range fields {
		if len(f.name) > width {
			width = len(f.name)
		}
	}
	for _, f := range fields {
		fmt.Fprintf(w, "%-*s = %s\n", width, f.name, f.get(cfg))
	}
	return nil
}

// runConfigGet prints the current value of a single setting.
func runConfigGet(w io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("config get requires a key (see config list for keys)")
	}
	if len(args) > 1 {
		return fmt.Errorf("config get takes a single key, got %d arguments", len(args))
	}
	key := args[0]

	field, ok := findConfigField(key)
	if !ok {
		return unknownConfigKeyErr(key)
	}

	cfg, err := configLoadFn()
	if err != nil {
		return err
	}
	fmt.Fprintln(w, field.get(cfg))
	return nil
}

// runConfigSet validates and writes a single setting, persisting through the
// existing config save path so validation and migrations still run.
func runConfigSet(w io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("config set requires a key and a value")
	}
	if len(args) > 2 {
		return fmt.Errorf("config set takes a key and a single value, got %d arguments", len(args))
	}
	key, value := args[0], args[1]

	field, ok := findConfigField(key)
	if !ok {
		return unknownConfigKeyErr(key)
	}

	cfg, err := configLoadFn()
	if err != nil {
		return err
	}
	if err := field.set(cfg, value); err != nil {
		return err
	}
	if err := configSaveFn(cfg); err != nil {
		return err
	}
	fmt.Fprintf(w, "%s = %s\n", field.name, field.get(cfg))
	return nil
}

// runConfigPath prints the resolved config file path.
func runConfigPath(w io.Writer, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("config path does not take arguments, got %q", args[0])
	}
	path, err := configPathFn()
	if err != nil {
		return err
	}
	fmt.Fprintln(w, path)
	return nil
}

// fieldJSONValue returns the setting value typed for JSON output: bool as a
// JSON boolean, int as a number (or null when unset), and everything else as a
// string.
func fieldJSONValue(f configField, cfg *config.Config) any {
	raw := f.get(cfg)
	switch f.kind {
	case configKindBool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return raw
		}
		return b
	case configKindInt:
		if raw == "" {
			return nil
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return raw
		}
		return n
	default:
		return raw
	}
}

// unknownConfigKeyErr builds a consistent error for an unrecognized key.
func unknownConfigKeyErr(key string) error {
	names := make([]string, 0, len(configFields()))
	for _, f := range configFields() {
		names = append(names, f.name)
	}
	sort.Strings(names)
	return fmt.Errorf("unknown config key %q (valid keys: %s)", key, strings.Join(names, ", "))
}
