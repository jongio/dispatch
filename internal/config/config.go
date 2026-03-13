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

	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/tui/styles"
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
)

// Config holds the user's preferences.
type Config struct {
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

	// HiddenSessions is a list of session IDs that the user has chosen to
	// hide from the main session list. They can be revealed with the
	// "show hidden" toggle and unhidden individually.
	HiddenSessions []string `json:"hiddenSessions,omitempty"`

	// AISearch enables Copilot SDK-powered AI search. When false (the
	// default), only the local FTS5 index is used.  Set to true to also
	// query the Copilot backend for semantically relevant sessions.
	AISearch bool `json:"ai_search,omitempty"`

	// Theme is the active color scheme name.  "auto" (or empty) means
	// detect from the terminal; any other value is looked up in Schemes
	// and then the built-in scheme list.
	Theme string `json:"theme,omitempty"`

	// Schemes is a list of user-defined color schemes in Windows Terminal
	// format.  Users can paste any WT scheme JSON directly here.
	Schemes []styles.ColorScheme `json:"schemes,omitempty"`
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

// EffectivePaneDirection returns the configured pane direction, defaulting
// to "auto" when unset.
func (c *Config) EffectivePaneDirection() string {
	if c.PaneDirection != "" {
		return c.PaneDirection
	}
	return PaneDirectionAuto
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
		DefaultShell:     "",
		DefaultTerminal:  "",
		DefaultTimeRange: "1d",
		DefaultSort:      "updated",
		DefaultPivot:     "folder",
		ShowPreview:      true,
		MaxSessions:      100,
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
	return cfg, nil
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
