package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/platform"
)

// Function variables allow test substitution of external calls, matching the
// pattern used elsewhere in this package (see cli.go and open.go).
var (
	newLoadConfigFn = config.Load
	newLaunchFn     = defaultNewLaunch
)

// runNew starts a brand-new Copilot session in a directory using the same
// launch path the TUI uses. args is the full argument slice with
// args[0] == "new".
func runNew(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	dir, modeFlag, err := parseNewArgs(args)
	if err != nil {
		return err
	}

	cfg, err := newLoadConfigFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	mode := resolveOpenMode(modeFlag, cfg)

	resolvedDir, err := resolveNewDir(dir)
	if err != nil {
		return err
	}

	return newLaunchFn(w, cfg, resolvedDir, mode)
}

// parseNewArgs extracts the optional directory and launch mode from the "new"
// subcommand arguments. args[0] is expected to be "new". A missing directory
// means the current working directory.
func parseNewArgs(args []string) (dir, mode string, err error) {
	rest := args
	if len(rest) > 0 {
		rest = rest[1:] // drop the "new" token
	}

	var positionals []string
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		switch {
		case arg == "--mode" || arg == "-m":
			if i+1 >= len(rest) {
				return "", "", errors.New("--mode requires a value: inplace, tab, window, or pane")
			}
			mode = rest[i+1]
			i++
		case strings.HasPrefix(arg, "--mode="):
			mode = strings.TrimPrefix(arg, "--mode=")
		case strings.HasPrefix(arg, "-"):
			return "", "", fmt.Errorf("unknown flag: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}

	switch len(positionals) {
	case 0:
		dir = "" // current directory
	case 1:
		dir = positionals[0]
	default:
		return "", "", fmt.Errorf("new accepts a single directory, got %d arguments", len(positionals))
	}

	if mode != "" {
		if _, mErr := normalizeLaunchMode(mode); mErr != nil {
			return "", "", mErr
		}
	}
	return dir, mode, nil
}

// resolveNewDir validates the target directory and returns an absolute path.
// An empty dir resolves to the current working directory.
func resolveNewDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolving current directory: %w", err)
		}
		return wd, nil
	}

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory %q does not exist", dir)
		}
		return "", fmt.Errorf("checking directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is not a directory", dir)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving path %q: %w", dir, err)
	}
	return abs, nil
}

// defaultNewLaunch starts a new session using the resolved launch mode. It
// mirrors defaultOpenLaunch but passes an empty session ID, which the platform
// resume builders treat as a new session (no --resume flag).
func defaultNewLaunch(w io.Writer, cfg *config.Config, dir string, mode string) error {
	if mode == config.LaunchModeInPlace {
		rc := platform.ResumeConfig{
			YoloMode:      cfg.YoloMode,
			Agent:         cfg.Agent,
			Model:         cfg.Model,
			CustomCommand: cfg.CustomCommand,
			Cwd:           dir,
		}
		cmd, err := platform.NewResumeCmd("", rc)
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
		Cwd:           dir,
		LaunchStyle:   launchStyleForOpenMode(mode),
		PaneDirection: cfg.EffectivePaneDirection(),
	}
	if err := platform.LaunchSession(shell, "", rc); err != nil {
		return err
	}
	fmt.Fprintf(w, "Started a new session in %s\n", dir)
	return nil
}
