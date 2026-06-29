// Package config manages user preferences for copilot-dispatch.
//
// Configuration is stored as a JSON file inside the platform-specific
// config directory. When the file does not exist, sensible defaults are
// returned so the application can run without prior configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jongio/dispatch/internal/platform"
)

const configFileName = "config.json"

const (
	// configDirPerm restricts the config directory to user read/write/execute only.
	configDirPerm = 0o700
	// configFilePerm restricts the config file to user read/write only.
	configFilePerm = 0o600
	// maxMaxSessions is the hard upper limit for MaxSessions to prevent
	// resource exhaustion from a maliciously large config value.
	maxMaxSessions = 10_000

	// currentConfigVersion is the schema version written to new and migrated
	// config files. Increment this when making breaking schema changes and
	// add a corresponding migration case in [migrate].
	currentConfigVersion = 1
)

// NamedView holds a saved combination of list state filters that can be
// applied together to quickly switch between common session browsing setups.
type NamedView struct {
	// Name is the unique display name for this view (e.g. "Work", "Personal").
	Name string `json:"name"`

	// Search is the search text to apply.
	Search string `json:"search,omitempty"`

	// TimeRange is the time filter (e.g. "1h", "1d", "7d", "all").
	TimeRange string `json:"time_range,omitempty"`

	// Sort is the sort field (e.g. "updated", "created", "turns", "name", "folder").
	Sort string `json:"sort,omitempty"`

	// SortOrder is the sort direction ("asc" or "desc").
	SortOrder string `json:"sort_order,omitempty"`

	// Pivot is the grouping mode (e.g. "none", "folder", "repo", "branch", "date").
	Pivot string `json:"pivot,omitempty"`

	// FavoritesOnly restricts the list to favorited sessions.
	FavoritesOnly bool `json:"favorites_only,omitempty"`

	// ShowHidden includes hidden sessions in the list.
	ShowHidden bool `json:"show_hidden,omitempty"`

	// ExcludedDirs overrides the global excluded directories for this view.
	ExcludedDirs []string `json:"excluded_dirs,omitempty"`
}

// Validate returns an error if the view has invalid or missing fields.
func (v *NamedView) Validate() error {
	if v.Name == "" {
		return errors.New("named view: name is required")
	}
	if v.TimeRange != "" {
		switch v.TimeRange {
		case "1h", "1d", "7d", "all":
		default:
			return fmt.Errorf("named view %q: invalid time_range %q", v.Name, v.TimeRange)
		}
	}
	if v.Sort != "" {
		switch v.Sort {
		case "updated", "created", "turns", "name", "folder":
		default:
			return fmt.Errorf("named view %q: invalid sort %q", v.Name, v.Sort)
		}
	}
	if v.SortOrder != "" {
		switch v.SortOrder {
		case "asc", "desc":
		default:
			return fmt.Errorf("named view %q: invalid sort_order %q", v.Name, v.SortOrder)
		}
	}
	if v.Pivot != "" {
		switch v.Pivot {
		case "none", "folder", "repo", "branch", "date":
		default:
			return fmt.Errorf("named view %q: invalid pivot %q", v.Name, v.Pivot)
		}
	}
	return nil
}

// Config holds the user's preferences.
type Config struct {
	// ConfigVersion tracks the schema version of this config file. Used to
	// detect and apply migrations when the config schema changes across
	// dispatch releases. The current version is set by [currentConfigVersion].
	ConfigVersion int `json:"config_version,omitempty"`

	// DefaultShell is the preferred shell name (e.g. "pwsh", "bash", "zsh").
	DefaultShell string `json:"default_shell"`

	// DefaultTerminal is the preferred terminal emulator name
	// (e.g. "Windows Terminal", "alacritty", "iTerm2").
	DefaultTerminal string `json:"default_terminal"`

	// DefaultTimeRange is the default time filter applied to session lists.
	// Valid values: "1h", "1d", "7d", "all".
	DefaultTimeRange string `json:"default_time_range"`

	// DefaultSort is the field used to order session lists.
	// Valid values: "updated", "created", "turns", "name", "folder".
	DefaultSort string `json:"default_sort"`

	// DefaultSortOrder is the direction used to order session lists.
	// Valid values: "asc", "desc".
	DefaultSortOrder string `json:"default_sort_order,omitempty"`

	// DefaultPivot is the default grouping applied to session lists.
	// Valid values: "none", "folder", "repo", "branch", "date".
	DefaultPivot string `json:"default_pivot"`

	// ShowPreview controls whether the detail/preview panel is visible.
	ShowPreview bool `json:"show_preview"`

	// MaxSessions is the maximum number of sessions to load initially.
	MaxSessions int `json:"max_sessions"`

	// YoloMode enables the --allow-all flag when resuming sessions,
	// allowing the Copilot CLI to run commands without confirmation prompts.
	YoloMode bool `json:"yoloMode"`

	// Agent specifies the --agent <name> flag passed to the Copilot CLI
	// when resuming sessions. Empty string means no agent override.
	Agent string `json:"agent"`

	// Model specifies the --model <name> flag passed to the Copilot CLI
	// when resuming sessions. Empty string means no model override.
	Model string `json:"model"`

	// LaunchMode controls how sessions are opened:
	//   "in-place" — resume in the current terminal (replaces the TUI)
	//   "tab"      — open in a new tab of the current terminal
	//   "window"   — open in a new terminal window
	//   "pane"     — open in a split pane (Windows Terminal only)
	// Default is "tab" when unset.
	LaunchMode string `json:"launch_mode,omitempty"`

	// PaneDirection controls the split direction when LaunchMode is "pane":
	//   "auto"  — let Windows Terminal choose (default)
	//   "right" — split to the right (vertical)
	//   "down"  — split downward (horizontal)
	//   "left"  — split to the left
	//   "up"    — split upward
	// Only used when LaunchMode is "pane" and the terminal is Windows Terminal.
	PaneDirection string `json:"pane_direction,omitempty"`

	// LaunchInPlace is deprecated; kept for backward compatibility.
	// When LaunchMode is unset and LaunchInPlace is true, the effective
	// mode is "in-place". New code should use LaunchMode.
	LaunchInPlace bool `json:"launchInPlace"`

	// ExcludedDirs is a list of directory paths to exclude from session
	// listings. Sessions whose Cwd starts with any of these paths are hidden.
	ExcludedDirs []string `json:"excluded_dirs,omitempty"`

	// CustomCommand is a user-defined command that replaces the default
	// copilot CLI resume command. The placeholder {sessionId} is replaced
	// with the actual session ID at launch time. When set, YoloMode, Agent,
	// and Model are ignored (they only apply to the default copilot CLI).
	// Terminal and Shell settings are still used.
	CustomCommand string `json:"custom_command,omitempty"`

	// ExcludedWords is a list of words used to filter sessions from the
	// dispatch list. Sessions whose summary or turn content contains any
	// of these words (case-insensitive) are hidden from the session list.
	ExcludedWords []string `json:"excluded_words,omitempty"`

	// HiddenSessions is a list of session IDs that the user has chosen to
	// hide from the main session list. They can be revealed with the
	// "show hidden" toggle and unhidden individually.
	HiddenSessions []string `json:"hiddenSessions,omitempty"`

	// FavoriteSessions is a list of session IDs that the user has starred
	// as favorites. They can be filtered with the "filter favorites" toggle.
	FavoriteSessions []string `json:"favoriteSessions,omitempty"`

	// SessionNotes maps session IDs to user-defined note strings.
	// Notes provide context about why a session matters or what follow-up
	// is needed. They are displayed in the preview panel and indicated
	// by a marker in the session list.
	SessionNotes map[string]string `json:"sessionNotes,omitempty"`

	// AISearch enables Copilot SDK-powered AI search. When false (the
	// default), only the local FTS5 index is used.  Set to true to also
	// query the Copilot backend for semantically relevant sessions.
	AISearch bool `json:"ai_search,omitempty"`

	// AttentionThreshold is the duration string (e.g. "15m", "1h") after
	// which a running session with no activity is classified as "stale"
	// instead of "waiting" or "active". Default is "15m".
	AttentionThreshold string `json:"attention_threshold,omitempty"`

	// Theme is the active color scheme name.  "auto" (or empty) means
	// detect from the terminal; any other value is looked up in Schemes
	// and then the built-in scheme list.
	Theme string `json:"theme,omitempty"`

	// Schemes is a list of user-defined color schemes in Windows Terminal
	// format.  Users can paste any WT scheme JSON directly here.
	Schemes []ColorScheme `json:"schemes,omitempty"`

	// PreviewPosition controls where the session detail/preview panel
	// is displayed relative to the session list.
	// Valid values: "right", "bottom", "left", "top".
	// Default is "right" when unset.
	PreviewPosition string `json:"preview_position,omitempty"`

	// DefaultCollapsed controls whether session header rows start in
	// collapsed (single-line) or expanded (multi-line) state. When false
	// (default), sessions are expanded showing full details. When true,
	// sessions start collapsed showing only the compact indicator row.
	DefaultCollapsed bool `json:"default_collapsed,omitempty"`

	// ConversationNewestFirst controls the turn display order in the
	// preview panel's Conversation section. When true (default), turns
	// are shown newest-first (descending). When false, turns are shown
	// oldest-first (ascending by TurnIndex).
	ConversationNewestFirst bool `json:"conversation_newest_first"`

	// WorkspaceRecovery enables detection of sessions interrupted by
	// crash/reboot. When false, stale lock files are ignored. Default true.
	WorkspaceRecovery bool `json:"workspace_recovery"`

	// RedactPreviewSecrets enables replacement of common secret patterns
	// (bearer tokens, GitHub PATs, Azure connection strings, .env secrets)
	// with [redacted] in the preview pane. Only affects rendering; stored
	// session data is never modified.
	RedactPreviewSecrets bool `json:"redact_preview_secrets,omitempty"`

	// Views is a list of named views, each storing a combination of
	// filter/sort/pivot settings that can be applied together.
	Views []NamedView `json:"views,omitempty"`

	// ActiveView is the name of the currently active named view.
	// Empty or "Default" means no named view is active.
	ActiveView string `json:"active_view,omitempty"`
}

// LaunchMode describes how sessions are opened in the terminal.
type LaunchMode = string

// Launch mode constants.
const (
	// LaunchModeInPlace resumes sessions in the current terminal.
	LaunchModeInPlace LaunchMode = "in-place"
	// LaunchModeTab opens sessions in a new terminal tab.
	LaunchModeTab LaunchMode = "tab"
	// LaunchModeWindow opens sessions in a new terminal window.
	LaunchModeWindow LaunchMode = "window"
	// LaunchModePane opens sessions in a split pane (Windows Terminal only).
	LaunchModePane LaunchMode = "pane"
)

// PaneDirection constants control the split direction for pane mode.
const (
	// PaneDirectionAuto lets Windows Terminal choose the best split direction.
	PaneDirectionAuto = "auto"
	// PaneDirectionRight splits the pane to the right (vertical split).
	PaneDirectionRight = "right"
	// PaneDirectionDown splits the pane downward (horizontal split).
	PaneDirectionDown = "down"
	// PaneDirectionLeft splits the pane to the left.
	PaneDirectionLeft = "left"
	// PaneDirectionUp splits the pane upward.
	PaneDirectionUp = "up"
)

// PreviewPosition constants control where the preview panel is displayed.
const (
	// PreviewPositionRight places the preview to the right of the session list.
	PreviewPositionRight = "right"
	// PreviewPositionBottom places the preview below the session list.
	PreviewPositionBottom = "bottom"
	// PreviewPositionLeft places the preview to the left of the session list.
	PreviewPositionLeft = "left"
	// PreviewPositionTop places the preview above the session list.
	PreviewPositionTop = "top"
)

// Sort order constants for DefaultSortOrder.
const (
	// SortOrderAsc sorts results in ascending order.
	SortOrderAsc = "asc"
	// SortOrderDesc sorts results in descending order.
	SortOrderDesc = "desc"
)

// EffectivePaneDirection returns the configured pane direction, defaulting
// to "auto" when unset.
func (c *Config) EffectivePaneDirection() string {
	if c.PaneDirection != "" {
		return c.PaneDirection
	}
	return PaneDirectionAuto
}

// FindView returns the named view with the given name, or nil if not found.
func (c *Config) FindView(name string) *NamedView {
	for i := range c.Views {
		if c.Views[i].Name == name {
			return &c.Views[i]
		}
	}
	return nil
}

// ValidViews returns only the views that pass validation.
func (c *Config) ValidViews() []NamedView {
	valid := make([]NamedView, 0, len(c.Views))
	for _, v := range c.Views {
		if v.Validate() == nil {
			valid = append(valid, v)
		}
	}
	return valid
}

// EffectivePreviewPosition returns the configured preview panel position,
// defaulting to "right" when unset or invalid.
func (c *Config) EffectivePreviewPosition() string {
	switch c.PreviewPosition {
	case PreviewPositionRight, PreviewPositionBottom, PreviewPositionLeft, PreviewPositionTop:
		return c.PreviewPosition
	default:
		return PreviewPositionRight
	}
}

// EffectiveSortOrder returns the configured sort order, defaulting to "desc"
// when unset or invalid.
func (c *Config) EffectiveSortOrder() string {
	switch c.DefaultSortOrder {
	case SortOrderAsc, SortOrderDesc:
		return c.DefaultSortOrder
	default:
		return SortOrderDesc
	}
}

// defaultAttentionThreshold is used when AttentionThreshold is empty or unparseable.
const defaultAttentionThreshold = 15 * time.Minute

// EffectiveAttentionThreshold returns the configured attention threshold as
// a time.Duration, defaulting to 15 minutes when unset or invalid.
func (c *Config) EffectiveAttentionThreshold() time.Duration {
	if c.AttentionThreshold == "" {
		return defaultAttentionThreshold
	}
	d, err := time.ParseDuration(c.AttentionThreshold)
	if err != nil || d <= 0 {
		return defaultAttentionThreshold
	}
	return d
}

// EffectiveLaunchMode returns the active launch mode, resolving the
// deprecated LaunchInPlace boolean when LaunchMode is unset.
func (c *Config) EffectiveLaunchMode() string {
	if c.LaunchMode != "" {
		return c.LaunchMode
	}
	if c.LaunchInPlace {
		return LaunchModeInPlace
	}
	return LaunchModeTab
}

// Default returns a Config populated with sensible default values.
func Default() *Config {
	return &Config{
		ConfigVersion:           currentConfigVersion,
		DefaultShell:            "",
		DefaultTerminal:         "",
		DefaultTimeRange:        "1d",
		DefaultSort:             "updated",
		DefaultPivot:            "folder",
		ShowPreview:             true,
		MaxSessions:             100,
		ConversationNewestFirst: true,
		WorkspaceRecovery:       true,
	}
}

// configPath returns the full path to the configuration file.
func configPath() (string, error) {
	dir, err := platform.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// Load reads the configuration file from disk and returns the parsed Config.
// If the file does not exist, Load returns [Default] values with a nil error.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := Default() // start from defaults so missing keys keep their default
	// Reset ConfigVersion to 0 before unmarshal so that old configs lacking
	// the field are detected as version 0 (needing migration).
	cfg.ConfigVersion = 0
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	// Clamp MaxSessions to prevent resource exhaustion from oversized values.
	if cfg.MaxSessions > maxMaxSessions {
		cfg.MaxSessions = maxMaxSessions
	}
	if cfg.MaxSessions < 0 {
		cfg.MaxSessions = 0
	}
	cfg.sanitize()

	// Migrate old config schemas forward. If the version changed, persist
	// the upgraded config so future loads skip migration.
	if cfg.ConfigVersion < currentConfigVersion {
		migrate(cfg)
		cfg.ConfigVersion = currentConfigVersion
		_ = Save(cfg) // best-effort; don't fail Load on write errors
	}

	// Allow env var override for workspace recovery (used by --demo).
	if os.Getenv("DISPATCH_WORKSPACE_RECOVERY") == "1" {
		cfg.WorkspaceRecovery = true
	}

	return cfg, nil
}

// migrate applies all necessary transformations to bring an old config up to
// the current schema version. Each version bump should have a case here that
// transforms fields from the old layout to the new one, preserving user intent.
func migrate(cfg *Config) {
	// v0 → v1: LaunchInPlace (bool) was replaced by LaunchMode (string).
	// If the user had LaunchInPlace=true but LaunchMode is unset, populate it.
	if cfg.ConfigVersion < 1 {
		if cfg.LaunchInPlace && cfg.LaunchMode == "" {
			cfg.LaunchMode = LaunchModeInPlace
		}
	}
}

// shellUnsafe contains characters that must never appear in shell or terminal
// names because they could be interpreted by command interpreters (cmd.exe,
// bash, PowerShell, AppleScript). Values containing any of these are cleared
// to the empty string, which makes the launch path fall back to auto-detection.
const shellUnsafe = "&|;<>()$`\\\"'\n\r\t"

// sanitize cleans string config values to prevent command injection and
// removes control characters that could break terminal commands. It does
// NOT reject the config — it silently clears unsafe values to their zero
// value so the rest of the application falls back to safe defaults.
func (c *Config) sanitize() {
	c.DefaultShell = sanitizeConfigValue(c.DefaultShell)
	c.DefaultTerminal = sanitizeConfigValue(c.DefaultTerminal)
	c.Agent = sanitizeConfigValue(c.Agent)
	c.Model = sanitizeConfigValue(c.Model)
}

// sanitizeConfigValue returns the value unchanged if it contains only safe
// characters. If it contains shell metacharacters or control characters, the
// empty string is returned so the caller falls back to defaults.
func sanitizeConfigValue(v string) string {
	if strings.ContainsAny(v, shellUnsafe) {
		return ""
	}
	return v
}

// Save writes the given Config to disk as a JSON file.
// The parent directory is created if it does not already exist.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), configDirPerm); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	if err := os.WriteFile(path, data, configFilePerm); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// Reset deletes the config file, reverting to defaults on next Load.
func Reset() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing config: %w", err)
	}
	return nil
}
