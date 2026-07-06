package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
)

// Function variables allow test substitution of external calls, matching the
// pattern used elsewhere in this package (see cli.go).
var (
	openLoadConfigFn     = config.Load
	openGetSessionFn     = defaultOpenGetSession
	openGetLastSessionFn = defaultOpenGetLastSession
	openLaunchFn         = defaultOpenLaunch
	openResumeCmdFn      = platform.BuildResumeCommandString
)

// runOpen resumes a session using the same launch path the TUI uses. It
// resumes the session named by ID, or the most recently active session when
// --last is passed. With --print it writes the resolved resume command to w
// and does not launch. args is the full argument slice with args[0] == "open".
func runOpen(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	id, modeFlag, last, printCmd, err := parseOpenArgs(args)
	if err != nil {
		return err
	}

	cfg, err := openLoadConfigFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	var sess *data.Session
	if last {
		sess, err = openGetLastSessionFn()
		if err != nil {
			return err
		}
		if sess == nil {
			return errors.New("no sessions to resume")
		}
	} else {
		// Resolve the argument as an alias first; fall back to treating it as a
		// session ID when no alias matches.
		if resolved := cfg.SessionIDForAlias(id); resolved != "" {
			id = resolved
		}
		sess, err = openGetSessionFn(id)
		if err != nil {
			return err
		}
		if sess == nil {
			return fmt.Errorf("session %q not found", id)
		}
	}

	if printCmd {
		cmdStr, err := openResumeCmdFn(sess.ID, openResumeConfig(cfg, sess))
		if err != nil {
			return err
		}
		fmt.Fprintln(w, cmdStr)
		return nil
	}

	mode := resolveOpenMode(modeFlag, cfg)
	return openLaunchFn(w, cfg, sess, mode)
}

// openResumeConfig builds the resume config used to launch or print a
// session's resume command. Terminal, launch style, and pane direction are
// omitted because they affect terminal placement, not the copilot invocation.
func openResumeConfig(cfg *config.Config, sess *data.Session) platform.ResumeConfig {
	return platform.ResumeConfig{
		YoloMode:      cfg.YoloMode,
		Agent:         cfg.Agent,
		Model:         cfg.Model,
		CustomCommand: cfg.CustomCommand,
		Cwd:           sess.Cwd,
	}
}

// parseOpenArgs extracts the session ID, optional launch mode, the --last
// flag, and the --print flag from the "open" subcommand arguments. args[0] is
// expected to be "open".
func parseOpenArgs(args []string) (id, mode string, last, printCmd bool, err error) {
	rest := args
	if len(rest) > 0 {
		rest = rest[1:] // drop the "open" token
	}

	var positionals []string
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		switch {
		case arg == "--last" || arg == "-l":
			last = true
		case arg == "--mode" || arg == "-m":
			if i+1 >= len(rest) {
				return "", "", false, false, errors.New("--mode requires a value: inplace, tab, window, or pane")
			}
			mode = rest[i+1]
			i++
		case strings.HasPrefix(arg, "--mode="):
			mode = strings.TrimPrefix(arg, "--mode=")
		case arg == "--print":
			printCmd = true
		case strings.HasPrefix(arg, "-"):
			return "", "", false, false, fmt.Errorf("unknown flag: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}

	if last {
		if len(positionals) > 0 {
			return "", "", false, false, errors.New("open --last does not take a session ID")
		}
	} else {
		switch len(positionals) {
		case 0:
			return "", "", false, false, errors.New("open requires a session ID (or use --last)")
		case 1:
			id = positionals[0]
		default:
			return "", "", false, false, fmt.Errorf("open accepts a single session ID, got %d arguments", len(positionals))
		}
	}

	if mode != "" {
		if _, mErr := normalizeLaunchMode(mode); mErr != nil {
			return "", "", false, false, mErr
		}
	}
	return id, mode, last, printCmd, nil
}

// normalizeLaunchMode maps a user-facing mode string to a config launch mode.
func normalizeLaunchMode(mode string) (string, error) {
	switch strings.ToLower(mode) {
	case "inplace", "in-place":
		return config.LaunchModeInPlace, nil
	case "tab":
		return config.LaunchModeTab, nil
	case "window":
		return config.LaunchModeWindow, nil
	case "pane":
		return config.LaunchModePane, nil
	default:
		return "", fmt.Errorf("invalid mode %q (want inplace, tab, window, or pane)", mode)
	}
}

// resolveOpenMode returns the launch mode to use: the flag when set (already
// validated), otherwise the configured default.
func resolveOpenMode(modeFlag string, cfg *config.Config) string {
	if modeFlag != "" {
		m, _ := normalizeLaunchMode(modeFlag)
		return m
	}
	return cfg.EffectiveLaunchMode()
}

// defaultOpenGetSession loads a session by ID from the default session store.
// It returns (nil, nil) when no session with that ID exists.
func defaultOpenGetSession(id string) (*data.Session, error) {
	store, err := data.Open()
	if err != nil {
		return nil, fmt.Errorf("opening session store: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only, best-effort close

	detail, err := store.GetSession(context.Background(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if detail == nil {
		return nil, nil
	}
	return &detail.Session, nil
}

// defaultOpenGetLastSession loads the most recently active session from the
// default session store, ordering by last active time (the same ordering the
// TUI uses for its default "updated" sort). It returns (nil, nil) when the
// store holds no sessions.
func defaultOpenGetLastSession() (*data.Session, error) {
	store, err := data.Open()
	if err != nil {
		return nil, fmt.Errorf("opening session store: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only, best-effort close

	sortOpts := data.SortOptions{Field: data.SortByUpdated, Order: data.Descending}
	sessions, err := store.ListSessions(context.Background(), data.FilterOptions{}, sortOpts, 1)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	if len(sessions) == 0 {
		return nil, nil
	}
	return &sessions[0], nil
}

// defaultOpenLaunch resumes the session using the resolved launch mode. For
// in-place mode it runs the Copilot CLI in the current terminal and waits for
// it to exit. For tab, window, and pane modes it delegates to the platform
// launcher, matching the behavior of launching from the TUI.
func defaultOpenLaunch(w io.Writer, cfg *config.Config, sess *data.Session, mode string) error {
	if mode == config.LaunchModeInPlace {
		cmd, err := platform.NewResumeCmd(sess.ID, openResumeConfig(cfg, sess))
		if err != nil {
			return err
		}
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	shell := resolveOpenShell(cfg)
	if shell.Path == "" {
		return errors.New("no shell detected on this system")
	}
	rc := platform.ResumeConfig{
		YoloMode:      cfg.YoloMode,
		Agent:         cfg.Agent,
		Model:         cfg.Model,
		Terminal:      cfg.DefaultTerminal,
		CustomCommand: cfg.CustomCommand,
		Cwd:           sess.Cwd,
		LaunchStyle:   launchStyleForOpenMode(mode),
		PaneDirection: cfg.EffectivePaneDirection(),
	}
	if err := platform.LaunchSession(shell, sess.ID, rc); err != nil {
		return err
	}
	fmt.Fprintf(w, "Launched session %s\n", sess.ID)
	return nil
}

// resolveOpenShell picks the configured shell by name, falling back to the
// platform default. Mirrors the direct (non-picker) resolution used by the TUI
// for batch launches.
func resolveOpenShell(cfg *config.Config) platform.ShellInfo {
	if cfg.DefaultShell != "" {
		for _, sh := range platform.DetectShells() {
			if sh.Name == cfg.DefaultShell {
				return sh
			}
		}
	}
	return platform.DefaultShell()
}

// launchStyleForOpenMode maps a config launch mode to a platform launch style.
func launchStyleForOpenMode(mode string) string {
	switch mode {
	case config.LaunchModeWindow:
		return platform.LaunchStyleWindow
	case config.LaunchModePane:
		return platform.LaunchStylePane
	default:
		return platform.LaunchStyleTab
	}
}
