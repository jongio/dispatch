package main

import (
	"bufio"
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
	openListSessionsFn   = defaultOpenListScopedSessions
	openDetectGitFn      = detectGitContext

	// openStdin is the reader used by --stdin batch resume. It is a package
	// variable so tests can feed it a fixed list of IDs.
	openStdin io.Reader = os.Stdin
)

// runOpen resumes a session using the same launch path the TUI uses. It
// resumes the session named by ID, or the most recently active session when
// --last is passed. With --print it writes the resolved resume command to w
// and does not launch. With --stdin it reads session IDs from standard input
// (one per line) and resumes each, which composes with `dispatch search --ids`.
// args is the full argument slice with args[0] == "open".
func runOpen(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	id, modeFlag, last, printCmd, stdin, ov, scopeFilter, err := parseOpenArgs(args)
	if err != nil {
		return err
	}

	cfg, err := openLoadConfigFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	ov.apply(cfg)

	if stdin {
		return runOpenBatch(w, cfg, modeFlag, printCmd)
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
	} else if scopeFilter != nil {
		// Scoped resume: find the newest session matching the filter.
		sessions, lErr := openListSessionsFn(*scopeFilter)
		if lErr != nil {
			return lErr
		}
		if len(sessions) == 0 {
			return fmt.Errorf("no sessions found matching the filter")
		}
		sess = &sessions[0]
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

// runOpenBatch resumes every session whose ID is read from standard input,
// one ID per line. Blank lines and lines beginning with '#' are ignored, and
// each line's first whitespace-separated field is used as the ID, so annotated
// lists still work. It composes with `dispatch search --ids`:
//
//	dispatch search --ids feat | dispatch open --stdin --mode tab
//
// Resolution and launch reuse the single-session path. A missing session or a
// launch failure for one ID does not abort the batch; failures are collected
// and returned together so the exit code is non-zero while the rest still run.
func runOpenBatch(w io.Writer, cfg *config.Config, modeFlag string, printCmd bool) error {
	ids, err := readSessionIDs(openStdin)
	if err != nil {
		return fmt.Errorf("reading session IDs: %w", err)
	}
	if len(ids) == 0 {
		return errors.New("open --stdin: no session IDs on standard input")
	}

	mode := resolveOpenMode(modeFlag, cfg)
	if !printCmd && mode == config.LaunchModeInPlace {
		return errors.New("open --stdin cannot launch in inplace mode; pass --mode tab, window, or pane")
	}

	var failures []error
	resumed := 0
	for _, rawID := range ids {
		target := rawID
		if resolved := cfg.SessionIDForAlias(target); resolved != "" {
			target = resolved
		}

		sess, gErr := openGetSessionFn(target)
		if gErr != nil {
			failures = append(failures, fmt.Errorf("%s: %w", rawID, gErr))
			continue
		}
		if sess == nil {
			failures = append(failures, fmt.Errorf("%s: session not found", rawID))
			continue
		}

		if printCmd {
			cmdStr, cErr := openResumeCmdFn(sess.ID, openResumeConfig(cfg, sess))
			if cErr != nil {
				failures = append(failures, fmt.Errorf("%s: %w", rawID, cErr))
				continue
			}
			fmt.Fprintln(w, cmdStr)
			resumed++
			continue
		}

		if lErr := openLaunchFn(w, cfg, sess, mode); lErr != nil {
			failures = append(failures, fmt.Errorf("%s: %w", rawID, lErr))
			continue
		}
		resumed++
	}

	if len(failures) > 0 {
		fmt.Fprintf(w, "resumed %d of %d sessions\n", resumed, len(ids))
		return fmt.Errorf("open --stdin: %d of %d failed: %w",
			len(failures), len(ids), errors.Join(failures...))
	}
	return nil
}

// readSessionIDs reads session IDs from r, one per line. It trims surrounding
// whitespace, skips blank lines and lines starting with '#', takes the first
// whitespace-separated field of each remaining line, and drops duplicates while
// preserving first-seen order.
func readSessionIDs(r io.Reader) ([]string, error) {
	if r == nil {
		return nil, nil
	}

	var ids []string
	seen := make(map[string]struct{})

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // tolerate long lines

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.IndexAny(line, " \t"); i >= 0 {
			line = line[:i]
		}
		if _, dup := seen[line]; dup {
			continue
		}
		seen[line] = struct{}{}
		ids = append(ids, line)
	}
	if scErr := sc.Err(); scErr != nil {
		return nil, scErr
	}
	return ids, nil
}

// openResumeConfig builds the resume config used to launch or print a
// session's resume command. Terminal, launch style, and pane direction are
// omitted because they affect terminal placement, not the copilot invocation.
func openResumeConfig(cfg *config.Config, sess *data.Session) platform.ResumeConfig {
	return platform.ResumeConfig{
		YoloMode:      cfg.YoloMode,
		Agent:         cfg.Agent,
		Model:         cfg.Model,
		ResumeSessionCommand: cfg.ResumeSessionCommand,
		Cwd:           sess.Cwd,
	}
}

// parseOpenArgs extracts the session ID, optional launch mode, the --last
// flag, the --print flag, the --stdin flag, and any per-launch agent/model/yolo
// overrides from the "open" subcommand arguments. args[0] is expected to be "open".
func parseOpenArgs(args []string) (id, mode string, last, printCmd, stdin bool, ov launchOverrides, scopeFilter *data.FilterOptions, err error) {
	rest := args
	if len(rest) > 0 {
		rest = rest[1:] // drop the "open" token
	}

	var positionals []string
	var repo, branch, folder string
	var current bool
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		if matched, next, mErr := matchLaunchOverride(rest, i, &ov); matched {
			if mErr != nil {
				return "", "", false, false, false, launchOverrides{}, nil, mErr
			}
			i = next
			continue
		}
		switch {
		case arg == "--last" || arg == "-l":
			last = true
		case arg == "--stdin":
			stdin = true
		case arg == "--current":
			current = true
		case arg == "--mode" || arg == "-m":
			if i+1 >= len(rest) {
				return "", "", false, false, false, launchOverrides{}, nil, errors.New("--mode requires a value: inplace, tab, window, or pane")
			}
			mode = rest[i+1]
			i++
		case strings.HasPrefix(arg, "--mode="):
			mode = strings.TrimPrefix(arg, "--mode=")
		case arg == "--repo":
			if i+1 >= len(rest) {
				return "", "", false, false, false, launchOverrides{}, nil, errors.New("--repo requires a value")
			}
			repo = rest[i+1]
			i++
		case strings.HasPrefix(arg, "--repo="):
			repo = strings.TrimPrefix(arg, "--repo=")
		case arg == "--branch":
			if i+1 >= len(rest) {
				return "", "", false, false, false, launchOverrides{}, nil, errors.New("--branch requires a value")
			}
			branch = rest[i+1]
			i++
		case strings.HasPrefix(arg, "--branch="):
			branch = strings.TrimPrefix(arg, "--branch=")
		case arg == "--folder":
			if i+1 >= len(rest) {
				return "", "", false, false, false, launchOverrides{}, nil, errors.New("--folder requires a value")
			}
			folder = rest[i+1]
			i++
		case strings.HasPrefix(arg, "--folder="):
			folder = strings.TrimPrefix(arg, "--folder=")
		case arg == "--print":
			printCmd = true
		case strings.HasPrefix(arg, "-"):
			return "", "", false, false, false, launchOverrides{}, nil, fmt.Errorf("unknown flag: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}

	hasScope := repo != "" || branch != "" || folder != "" || current
	switch {
	case stdin:
		if last || hasScope {
			return "", "", false, false, false, launchOverrides{}, nil, errors.New("open --stdin cannot be combined with --last or scope filters")
		}
		if len(positionals) > 0 {
			return "", "", false, false, false, launchOverrides{}, nil, errors.New("open --stdin reads session IDs from standard input; do not also pass an ID")
		}
	case last:
		if hasScope {
			return "", "", false, false, false, launchOverrides{}, nil, errors.New("open --last cannot be combined with --repo, --branch, --folder, or --current")
		}
		if len(positionals) > 0 {
			return "", "", false, false, false, launchOverrides{}, nil, errors.New("open --last does not take a session ID")
		}
	case hasScope:
		if len(positionals) > 0 {
			return "", "", false, false, false, launchOverrides{}, nil, errors.New("open with scope filters does not take a session ID")
		}
		filter := data.FilterOptions{
			Repository: repo,
			Branch:     branch,
			Folder:     folder,
		}
		if current {
			detectedRepo, detectedBranch, cErr := openDetectGitFn()
			if cErr != nil {
				return "", "", false, false, false, launchOverrides{}, nil, fmt.Errorf("--current: %w", cErr)
			}
			if filter.Repository == "" {
				filter.Repository = detectedRepo
			}
			if filter.Branch == "" {
				filter.Branch = detectedBranch
			}
			// Guard against an empty scope: without a detected repo/branch (no
			// origin remote, detached HEAD) the filter would match every
			// session and resume an unrelated one.
			if filter.Repository == "" && filter.Branch == "" && filter.Folder == "" {
				return "", "", false, false, false, launchOverrides{}, nil,
					errors.New("--current: could not detect a repository or branch from the current directory")
			}
		}
		scopeFilter = &filter
	default:
		switch len(positionals) {
		case 0:
			return "", "", false, false, false, launchOverrides{}, nil, errors.New("open requires a session ID (or use --last, --current, --repo, --branch, or --stdin)")
		case 1:
			id = positionals[0]
		default:
			return "", "", false, false, false, launchOverrides{}, nil, fmt.Errorf("open accepts a single session ID, got %d arguments", len(positionals))
		}
	}

	if mode != "" {
		if _, mErr := normalizeLaunchMode(mode); mErr != nil {
			return "", "", false, false, false, launchOverrides{}, nil, mErr
		}
	}
	return id, mode, last, printCmd, stdin, ov, scopeFilter, nil
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

	ctx := context.Background()
	fullID, err := store.ResolveIDPrefix(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	detail, err := store.GetSession(ctx, fullID)
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

// defaultOpenListScopedSessions lists sessions matching a scope filter, ordered
// most-recently-active first (the same ordering --last uses), so the scoped
// resume path can pick sessions[0] as the most recent match.
func defaultOpenListScopedSessions(filter data.FilterOptions) ([]data.Session, error) {
	store, err := data.Open()
	if err != nil {
		return nil, fmt.Errorf("opening session store: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only, best-effort close

	sortOpts := data.SortOptions{Field: data.SortByUpdated, Order: data.Descending}
	sessions, err := store.ListSessions(context.Background(), filter, sortOpts, statsQueryLimit)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	return sessions, nil
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
		ResumeSessionCommand: cfg.ResumeSessionCommand,
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

// detectGitContext resolves the repo and branch for the current working
// directory, reusing the same seam the startup filter uses.
func detectGitContext() (string, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	return detectGitRepoFn(cwd)
}
