package platform

import (
	"errors"
	"fmt"
	"strings"
)

// defaultNewSessionCommand is used when the user has not configured a custom
// new session command.
const defaultNewSessionCommand = "copilot"

// LaunchNewSessionConfig holds parameters for launching a brand new Copilot
// CLI session (as opposed to resuming an existing one).
type LaunchNewSessionConfig struct {
	// Command is the command template. Empty means use default.
	// Supports {cwd} placeholder.
	Command string

	// Cwd is the working directory for the new session.
	Cwd string

	// Terminal is the terminal emulator to use (e.g. "Windows Terminal").
	Terminal string

	// Shell is the shell to run the command in.
	Shell ShellInfo

	// LaunchStyle controls how the session is opened (tab, window, pane).
	LaunchStyle string

	// PaneDirection controls split direction for pane mode.
	PaneDirection string
}

// LaunchNewSession starts a brand new Copilot CLI session in the specified
// working directory. It returns the PID of the launched process (for tracking)
// or an error.
func LaunchNewSession(cfg LaunchNewSessionConfig) (int, error) {
	if cfg.Shell.Path == "" {
		cfg.Shell = DefaultShell()
	}
	if cfg.Shell.Path == "" {
		return 0, errors.New("no shell available")
	}
	if cfg.Terminal == "" {
		cfg.Terminal = DefaultTerminal()
	}

	cmd := cfg.Command
	if cmd == "" {
		cmd = defaultNewSessionCommand
	}

	// Replace template variables.
	if cfg.Cwd != "" {
		cmd = strings.ReplaceAll(cmd, "{cwd}", cfg.Cwd)
	}

	return launchNewSessionPlatform(cfg.Shell, cmd, cfg.Terminal, cfg.Cwd, cfg.LaunchStyle, cfg.PaneDirection)
}

// launchNewSessionPlatformFn is the platform-specific launcher for new sessions.
// Tests can replace this to prevent real process spawning.
var launchNewSessionPlatformFn = launchNewSessionPlatformImpl

func launchNewSessionPlatform(shell ShellInfo, cmd string, terminal string, cwd string, launchStyle string, paneDirection string) (int, error) {
	return launchNewSessionPlatformFn(shell, cmd, terminal, cwd, launchStyle, paneDirection)
}

func launchNewSessionPlatformImpl(shell ShellInfo, command string, terminal string, cwd string, launchStyle string, paneDirection string) (int, error) {
	// Build the launch command. For new sessions we want the terminal to
	// stay open after the copilot CLI exits (interactive mode), so we
	// don't append an exit.
	err := platformLaunchSessionFn(shell, command, terminal, cwd, launchStyle, paneDirection)
	if err != nil {
		return 0, fmt.Errorf("launch new session: %w", err)
	}

	// Attempt to find the PID of the launched process. Since
	// platformLaunchSessionFn uses wt.exe (which spawns a child), the
	// actual copilot process PID is not directly available. We return 0
	// here and let the EventWatcher pick up the session via lock files.
	return 0, nil
}
