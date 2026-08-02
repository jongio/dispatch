package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jongio/dispatch/internal/validate"
)

// Diagnostic describes one actionable config validation problem.
type Diagnostic struct {
	Path     string `json:"path"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// ValidationResult is the machine-readable result for config validation.
type ValidationResult struct {
	Path        string       `json:"path"`
	Valid       bool         `json:"valid"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// KeybindingActions lists every supported keybindings action name.
var KeybindingActions = []string{
	"up", "down", "jump_top", "jump_bottom", "left", "right", "enter", "space",
	"quit", "force_quit", "search", "escape", "filter", "sort", "sort_order",
	"pivot", "pivot_order", "preview", "preview_fullscreen", "reindex", "help",
	"config", "time_range_1", "time_range_2", "time_range_3", "time_range_4",
	"hide", "toggle_hidden", "star", "launch_window", "launch_tab", "launch_pane",
	"preview_scroll_up", "preview_scroll_down", "jump_next_attention", "filter_attention",
	"launch_all", "select_all", "deselect_all", "conversation_sort", "preview_position",
	"resume_interrupted", "view_plan", "copy_id", "copy_path", "copy_resume_command",
	"copy_preview", "expand_collapse_all", "scan_work_status", "export", "note", "tags",
	"alias", "shift_up", "shift_down", "view_switch", "open_file", "open_dir",
	"open_ref", "timeline", "compare", "git_status", "cmd_palette",
}

var defaultKeybindings = map[string][]string{
	"up": {"up", "k"}, "down": {"down", "j"}, "jump_top": {"g", "home"}, "jump_bottom": {"G", "end"},
	"left": {"left"}, "right": {"right"}, "enter": {"enter"}, "space": {"space"},
	"quit": {"q"}, "force_quit": {"ctrl+c"}, "search": {"/"}, "escape": {"esc"},
	"filter": {"f"}, "sort": {"s"}, "sort_order": {"S"}, "pivot": {"tab"},
	"pivot_order": {"shift+tab"}, "preview": {"p"}, "preview_fullscreen": {"z"}, "reindex": {"r"},
	"help": {"?"}, "config": {","}, "time_range_1": {"1"}, "time_range_2": {"2"},
	"time_range_3": {"3"}, "time_range_4": {"4"}, "hide": {"h"}, "toggle_hidden": {"H"},
	"star": {"*"}, "launch_window": {"w"}, "launch_tab": {"t"}, "launch_pane": {"e"},
	"preview_scroll_up": {"pgup"}, "preview_scroll_down": {"pgdown"}, "jump_next_attention": {"n"}, "filter_attention": {"!"},
	"launch_all": {"L"}, "select_all": {"a"}, "deselect_all": {"d"}, "conversation_sort": {"o"},
	"preview_position": {"P"}, "resume_interrupted": {"N"}, "view_plan": {"v"}, "copy_id": {"c"},
	"copy_path": {"C"}, "copy_resume_command": {"Y"}, "copy_preview": {"y"}, "expand_collapse_all": {"x"},
	"scan_work_status": {"R"}, "export": {"X"}, "note": {"m"}, "tags": {"#"},
	"alias": {"A"}, "shift_up": {"shift+up"}, "shift_down": {"shift+down"}, "view_switch": {"V"},
	"open_file": {"F"}, "open_dir": {"O"}, "open_ref": {"b"}, "timeline": {"T"},
	"compare": {"D"}, "git_status": {"i"}, "cmd_palette": {":"},
}

var keyNamePattern = regexp.MustCompile(`^[^\s,]+$`)

// ValidateFile loads, migrates, sanitizes, and validates a config file.
func ValidateFile(path string) (ValidationResult, error) {
	result := ValidationResult{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := Default()
			result.Diagnostics = ValidateConfig(cfg)
			result.Valid = len(result.Diagnostics) == 0
			return result, nil
		}
		return result, fmt.Errorf("reading config: %w", err)
	}

	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		result.Diagnostics = append(result.Diagnostics, errorDiagnostic("$", fmt.Sprintf("invalid JSON: %v", err)))
		result.Valid = false
		return result, nil
	}
	result.Diagnostics = append(result.Diagnostics, unknownFieldDiagnostics(raw, reflect.TypeOf(Config{}), "$")...)

	cfg := Default()
	cfg.ConfigVersion = 0
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(cfg); err != nil {
		result.Diagnostics = append(result.Diagnostics, errorDiagnostic("$", fmt.Sprintf("invalid config value: %v", err)))
		result.Valid = false
		return result, nil
	}
	if cfg.MaxSessions > maxMaxSessions {
		cfg.MaxSessions = maxMaxSessions
	}
	if cfg.MaxSessions < 0 {
		cfg.MaxSessions = 0
	}
	cfg.sanitize()
	if cfg.ConfigVersion < currentConfigVersion {
		migrate(cfg)
		cfg.ConfigVersion = currentConfigVersion
	}
	result.Diagnostics = append(result.Diagnostics, ValidateConfig(cfg)...)
	result.Valid = len(result.Diagnostics) == 0
	return result, nil
}

// ValidateConfig returns semantic diagnostics for a parsed Config.
func ValidateConfig(cfg *Config) []Diagnostic {
	var diags []Diagnostic
	addEnum := func(path, value string, allowed []string) {
		if value != "" && !stringIn(value, allowed) {
			diags = append(diags, errorDiagnostic(path, fmt.Sprintf("invalid value %q (want one of: %s)", value, strings.Join(allowed, ", "))))
		}
	}

	addEnum("default_time_range", cfg.DefaultTimeRange, []string{TimeRange1h, TimeRange1d, TimeRange7d, TimeRangeAll})
	addEnum("default_sort", cfg.DefaultSort, []string{SortFieldUpdated, SortFieldCreated, SortFieldTurns, SortFieldName, SortFieldFolder, SortFieldFrecency})
	addEnum("default_sort_order", cfg.DefaultSortOrder, []string{SortOrderAsc, SortOrderDesc})
	addEnum("default_pivot", cfg.DefaultPivot, []string{PivotNone, PivotFolder, PivotRepo, PivotBranch, PivotDate, PivotHost})
	addEnum("launch_mode", cfg.LaunchMode, []string{LaunchModeInPlace, LaunchModeTab, LaunchModeWindow, LaunchModePane})
	addEnum("pane_direction", cfg.PaneDirection, []string{PaneDirectionAuto, PaneDirectionRight, PaneDirectionDown, PaneDirectionLeft, PaneDirectionUp})
	addEnum("preview_position", cfg.PreviewPosition, []string{PreviewPositionRight, PreviewPositionBottom, PreviewPositionLeft, PreviewPositionTop})
	for i, col := range cfg.HiddenColumns {
		addEnum(fmt.Sprintf("hidden_columns[%d]", i), col, ToggleableColumns)
	}
	if cfg.AttentionThreshold != "" {
		d, err := time.ParseDuration(cfg.AttentionThreshold)
		if err != nil || d <= 0 {
			diags = append(diags, errorDiagnostic("attention_threshold", fmt.Sprintf("invalid duration %q (want a positive duration like 15m or 1h)", cfg.AttentionThreshold)))
		}
	}
	if cfg.AutoRefreshSeconds != nil && *cfg.AutoRefreshSeconds < 0 {
		diags = append(diags, errorDiagnostic("auto_refresh_seconds", "must be zero or greater"))
	}
	diags = append(diags, validateSessionIDs("hiddenSessions", cfg.HiddenSessions)...)
	diags = append(diags, validateSessionIDs("favoriteSessions", cfg.FavoriteSessions)...)
	diags = append(diags, validateSessionKeyMap("sessionNotes", cfg.SessionNotes)...)
	diags = append(diags, validateSessionKeyMap("sessionTags", cfg.SessionTags)...)
	diags = append(diags, validateSessionKeyMap("sessionAliases", cfg.SessionAliases)...)
	diags = append(diags, validateSessionKeyMap("sessionLaunches", cfg.SessionLaunches)...)
	diags = append(diags, validateViews(cfg)...)
	diags = append(diags, validateSchemes(cfg.Schemes)...)
	diags = append(diags, ValidateKeybindings(cfg.Keybindings)...)
	return diags
}

// ValidateKeybindings returns diagnostics for unknown actions, bad key lists, and collisions.
func ValidateKeybindings(overrides map[string]string) []Diagnostic {
	if len(overrides) == 0 {
		return nil
	}
	actionSet := make(map[string]struct{}, len(KeybindingActions))
	for _, action := range KeybindingActions {
		actionSet[action] = struct{}{}
	}
	var diags []Diagnostic
	effective := make(map[string][]string, len(defaultKeybindings))
	for action, keys := range defaultKeybindings {
		effective[action] = append([]string(nil), keys...)
	}
	for action, raw := range overrides {
		path := "keybindings." + action
		if _, ok := actionSet[action]; !ok {
			diags = append(diags, errorDiagnostic(path, fmt.Sprintf("unknown keybinding action %q", action)))
			continue
		}
		keys := parseKeyList(raw)
		if len(keys) == 0 {
			diags = append(diags, errorDiagnostic(path, "must contain at least one key name"))
			continue
		}
		for _, key := range keys {
			if !keyNamePattern.MatchString(key) {
				diags = append(diags, errorDiagnostic(path, fmt.Sprintf("invalid key name %q", key)))
			}
		}
		effective[action] = keys
	}
	owners := map[string][]string{}
	for _, action := range KeybindingActions {
		for _, key := range effective[action] {
			owners[key] = append(owners[key], action)
		}
	}
	for action := range overrides {
		for _, key := range effective[action] {
			if len(owners[key]) > 1 {
				diags = append(diags, errorDiagnostic("keybindings."+action, fmt.Sprintf("key %q conflicts with actions: %s", key, strings.Join(owners[key], ", "))))
				break
			}
		}
	}
	sortDiagnostics(diags)
	return diags
}

// JSONSchema returns the generated JSON Schema for config.json.
func JSONSchema() (map[string]any, error) {
	schema, err := schemaForType(reflect.TypeOf(Config{}))
	if err != nil {
		return nil, err
	}
	obj := schema.(map[string]any)
	obj["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	obj["$id"] = "https://raw.githubusercontent.com/jongio/dispatch/main/docs/config.schema.json"
	obj["title"] = "Dispatch config.json"
	obj["description"] = "Schema for Dispatch user configuration."
	return obj, nil
}

// JSONSchemaBytes returns the generated JSON Schema as indented JSON.
func JSONSchemaBytes() ([]byte, error) {
	schema, err := JSONSchema()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(schema, "", "  ")
}

func validateViews(cfg *Config) []Diagnostic {
	var diags []Diagnostic
	names := map[string]int{}
	for i, v := range cfg.Views {
		base := fmt.Sprintf("views[%d]", i)
		if v.Name == "" {
			diags = append(diags, errorDiagnostic(base+".name", "name is required"))
		} else if first, ok := names[v.Name]; ok {
			diags = append(diags, errorDiagnostic(base+".name", fmt.Sprintf("duplicate named view %q (first defined at views[%d])", v.Name, first)))
		} else {
			names[v.Name] = i
		}
		addViewEnum := func(field, value string, allowed []string) {
			if value != "" && !stringIn(value, allowed) {
				diags = append(diags, errorDiagnostic(base+"."+field, fmt.Sprintf("invalid value %q (want one of: %s)", value, strings.Join(allowed, ", "))))
			}
		}
		addViewEnum("time_range", v.TimeRange, []string{TimeRange1h, TimeRange1d, TimeRange7d, TimeRangeAll})
		addViewEnum("sort", v.Sort, []string{SortFieldUpdated, SortFieldCreated, SortFieldTurns, SortFieldName, SortFieldFolder, SortFieldFrecency})
		addViewEnum("sort_order", v.SortOrder, []string{SortOrderAsc, SortOrderDesc})
		addViewEnum("pivot", v.Pivot, []string{PivotNone, PivotFolder, PivotRepo, PivotBranch, PivotDate, PivotHost})
	}
	if cfg.ActiveView != "" && cfg.ActiveView != "Default" {
		if _, ok := names[cfg.ActiveView]; !ok {
			diags = append(diags, errorDiagnostic("active_view", fmt.Sprintf("unknown named view %q", cfg.ActiveView)))
		}
	}
	return diags
}

func validateSchemes(schemes []ColorScheme) []Diagnostic {
	var diags []Diagnostic
	for i := range schemes {
		if err := schemes[i].Validate(); err != nil {
			diags = append(diags, errorDiagnostic(fmt.Sprintf("schemes[%d]", i), err.Error()))
		}
	}
	return diags
}

func validateSessionIDs(path string, ids []string) []Diagnostic {
	var diags []Diagnostic
	for i, id := range ids {
		if id != "" && !validate.SessionID(id) {
			diags = append(diags, errorDiagnostic(fmt.Sprintf("%s[%d]", path, i), fmt.Sprintf("invalid session ID %q", id)))
		}
	}
	return diags
}

func validateSessionKeyMap[T any](path string, values map[string]T) []Diagnostic {
	var diags []Diagnostic
	for id := range values {
		if id != "" && !validate.SessionID(id) {
			diags = append(diags, errorDiagnostic(path+"."+id, fmt.Sprintf("invalid session ID key %q", id)))
		}
	}
	return diags
}

func unknownFieldDiagnostics(value any, typ reflect.Type, path string) []Diagnostic {
	typ = derefType(typ)
	if typ.Kind() == reflect.Struct {
		obj, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		fields := jsonFields(typ)
		var diags []Diagnostic
		for key, child := range obj {
			field, ok := fields[key]
			childPath := joinPath(path, key)
			if !ok {
				diags = append(diags, errorDiagnostic(childPath, fmt.Sprintf("unknown field %q", key)))
				continue
			}
			diags = append(diags, unknownFieldDiagnostics(child, field.Type, childPath)...)
		}
		return diags
	}
	if typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		arr, ok := value.([]any)
		if !ok {
			return nil
		}
		var diags []Diagnostic
		for i, child := range arr {
			diags = append(diags, unknownFieldDiagnostics(child, typ.Elem(), fmt.Sprintf("%s[%d]", path, i))...)
		}
		return diags
	}
	if typ.Kind() == reflect.Map {
		return nil
	}
	return nil
}

func schemaForType(typ reflect.Type) (any, error) {
	typ = derefType(typ)
	switch typ.Kind() {
	case reflect.Struct:
		props := map[string]any{}
		fields := jsonFields(typ)
		names := make([]string, 0, len(fields))
		for name := range fields {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			field := fields[name]
			s, err := schemaForField(name, field.Type)
			if err != nil {
				return nil, err
			}
			props[name] = s
		}
		return map[string]any{"type": "object", "additionalProperties": false, "properties": props}, nil
	case reflect.String:
		return map[string]any{"type": "string"}, nil
	case reflect.Bool:
		return map[string]any{"type": "boolean"}, nil
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer"}, nil
	case reflect.Slice:
		item, err := schemaForType(typ.Elem())
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "array", "items": item}, nil
	case reflect.Map:
		val, err := schemaForType(typ.Elem())
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "object", "additionalProperties": val}, nil
	default:
		return nil, fmt.Errorf("unsupported schema type %s", typ)
	}
}

func schemaForField(name string, typ reflect.Type) (map[string]any, error) {
	baseAny, err := schemaForType(typ)
	if err != nil {
		return nil, err
	}
	base := baseAny.(map[string]any)
	switch name {
	case "default_time_range", "time_range":
		base["enum"] = []string{TimeRange1h, TimeRange1d, TimeRange7d, TimeRangeAll}
	case "default_sort", "sort":
		base["enum"] = []string{SortFieldUpdated, SortFieldCreated, SortFieldTurns, SortFieldName, SortFieldFolder, SortFieldFrecency}
	case "default_sort_order", "sort_order":
		base["enum"] = []string{SortOrderAsc, SortOrderDesc}
	case "default_pivot", "pivot":
		base["enum"] = []string{PivotNone, PivotFolder, PivotRepo, PivotBranch, PivotDate, PivotHost}
	case "launch_mode":
		base["enum"] = []string{LaunchModeInPlace, LaunchModeTab, LaunchModeWindow, LaunchModePane}
	case "pane_direction":
		base["enum"] = []string{PaneDirectionAuto, PaneDirectionRight, PaneDirectionDown, PaneDirectionLeft, PaneDirectionUp}
	case "preview_position":
		base["enum"] = []string{PreviewPositionRight, PreviewPositionBottom, PreviewPositionLeft, PreviewPositionTop}
	case "hidden_columns":
		base["items"] = map[string]any{"type": "string", "enum": ToggleableColumns}
	case "attention_threshold":
		base["description"] = "Positive Go duration such as 15m or 1h."
	case "keybindings":
		base["propertyNames"] = map[string]any{"enum": KeybindingActions}
		base["additionalProperties"] = map[string]any{"type": "string", "description": "Comma-separated key list."}
	}
	return base, nil
}

func jsonFields(typ reflect.Type) map[string]reflect.StructField {
	fields := map[string]reflect.StructField{}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		tag := field.Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" {
			name = field.Name
		}
		if name == "-" {
			continue
		}
		fields[name] = field
	}
	return fields
}

func derefType(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}

func parseKeyList(csv string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, part := range strings.Split(csv, ",") {
		key := strings.TrimSpace(part)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func joinPath(base, key string) string {
	if base == "$" {
		return key
	}
	return base + "." + key
}

func errorDiagnostic(path, message string) Diagnostic {
	return Diagnostic{Path: path, Message: message, Severity: "error"}
}

func stringIn(value string, allowed []string) bool {
	for _, v := range allowed {
		if value == v {
			return true
		}
	}
	return false
}

func sortDiagnostics(diags []Diagnostic) {
	sort.Slice(diags, func(i, j int) bool {
		if diags[i].Path == diags[j].Path {
			return diags[i].Message < diags[j].Message
		}
		return diags[i].Path < diags[j].Path
	})
}
