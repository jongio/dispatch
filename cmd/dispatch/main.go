// Package main is the entry point for the dispatch CLI.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/jongio/dispatch/internal/tui"
	"github.com/jongio/dispatch/internal/update"
	"github.com/jongio/dispatch/internal/version"
)

const (
	demoDBRel        = "internal/data/testdata/fake_sessions.db"
	noUpdateCheckEnv = "DISPATCH_NO_UPDATE_CHECK"
)

var checkForUpdateFn = update.CheckForUpdate

func main() {
	updateCh := startUpdateCheck(context.Background(), version.Version)

	origStderr := captureOriginalStderr()
	if origStderr == nil {
		origStderr = os.Stderr
	}
	if origStderr != os.Stderr {
		defer origStderr.Close() //nolint:errcheck // best-effort cleanup
	}

	done, demoCleanup, startup, err := handleArgs(os.Args[1:], origStderr, updateCh)
	if demoCleanup != nil {
		defer demoCleanup()
	}
	if err != nil {
		os.Exit(1)
	}
	if done {
		return
	}

	// Redirect stderr BEFORE starting Bubble Tea.  The Copilot SDK
	// subprocess inherits our fd 2 (stderr) and writes error text like
	// "file already closed" to it.  That raw output leaks into Bubble
	// Tea's alt-screen buffer.  By redirecting fd 2 here we ensure the
	// subprocess's stderr goes to the log file (if set) or /dev/null.
	logWriter, logCleanup := setupLogRedirect()
	defer logCleanup()

	// Silence slog during TUI mode — direct structured logs to the same
	// destination as stderr (log file or discard).
	slog.SetDefault(slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	// Detect incompatible terminal environments (e.g. MSYS/Git Bash without
	// a real Windows console handle) and bail out with a helpful message
	// rather than showing a blank screen.
	if msg := checkTerminalCompat(); msg != "" {
		fmt.Fprintln(origStderr, msg)
		os.Exit(1)
	}

	p := tea.NewProgram(
		tui.NewModelWithQuery(startup.SeedQuery()),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(origStderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// After TUI exits, show update notification if the background
	// check found a newer release.
	showUpdateNotification(origStderr, updateCh)
}

func startUpdateCheck(ctx context.Context, currentVersion string) <-chan *update.UpdateInfo {
	ch := make(chan *update.UpdateInfo, 1)
	if updateCheckDisabled() {
		return ch
	}
	// Start background update check early so it can run concurrently with
	// argument parsing and TUI startup.
	go func() {
		ch <- checkForUpdateFn(ctx, currentVersion)
	}()
	return ch
}

func updateCheckDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(noUpdateCheckEnv))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func printUsage() {
	fmt.Printf(`dispatch %s — A TUI for browsing GitHub Copilot CLI sessions.

Usage:
  dispatch                Launch the interactive TUI
  dispatch [query]        Launch the TUI with the search box pre-filled
  dispatch [filters]      Launch the TUI filtered to a repo, branch, or folder
  dispatch [command]

Commands:
  help                    Show this help message
  version [--json]        Print the version
  open <id> [--mode M]    Resume a session by ID or prefix (M: inplace, tab, window, pane)
                          --print writes the resume command instead of launching
  open --last [--mode M]  Resume the most recently active session
  open --stdin [--mode M] Resume every session ID read from standard input
                          (one per line; pairs with search --ids)
  open --repo R [--mode M]
                          Resume the most recent session matching a scope filter
  new [dir] [--mode M]    Start a new session in a directory (default: current)
  completion <shell>      Print shell completion (bash, zsh, fish, powershell)
  doctor [--json]         Print environment diagnostics (--json for machine-readable output)
  stats [flags]           Print session totals and breakdowns
  search [query] [flags]  Print matching sessions as JSON, IDs, or a table
  tags [--json]           List tags in use with per-tag session counts
  aliases [--json]        List session aliases with orphan detection
  notes [command]          List, get, set, or clear session notes
  views [command]          List named views or set the active view
  config [get|set|list|edit|path]
                          Read or change preferences (see Config commands)
  export <id> [flags]     Export a session as Markdown, JSON, or HTML
  export --repo R [flags] Export all sessions matching a scope filter (batch mode)
  info <id> [--json]      Print a concise session summary (--json for machine-readable output)
  path <id|--last|--current>
                          Print only a session's working directory (for cd "$(dispatch path x)")
  compare <a> <b> [--json]
                          Compare two sessions side by side
  tag <id> [flags]        Add, remove, set, or list tags on a session
  prune [--apply] [--json]
                          Report (or remove) config entries for missing sessions
  watch [--once] [flags]  Monitor session attention state
  man                     Write the man page (roff) to standard output
  update                  Update dispatch to the latest release

Open and new flags:
  --mode <M>              Launch mode for this run (inplace, tab, window, pane)
  --agent <name>          Override the configured agent for this run only
  --model <name>          Override the configured model for this run only
  --yolo[=true|false]     Override yolo mode for this run only (bare form implies true)
  --last                  (open only) Resume the most recently active session
  --print                 (open only) Print the resume command instead of launching
  --repo <name>           (open only) Resume the most recent session for a repository
  --branch <name>         (open only) Resume the most recent session on a branch
  --folder <path>         (open only) Resume the most recent session under a folder
  --current               (open only) Auto-detect repo and branch from the current directory

Session IDs:
  Commands that take <id> (open, export, info, compare, tag) accept a full session
  ID or any unique prefix of one, like a short git SHA. An ambiguous prefix
  lists the matching sessions so you can add more characters.

Stats flags:
  --json                  Print the summary as JSON
  --csv                   Print the summary as CSV
  --calendar              Add a per-day activity heatmap
  --repo <name>           Only count sessions for a repository
  --branch <name>         Only count sessions on a branch
  --folder <path>         Only count sessions under a folder
  --since <date>          Only count sessions created on or after a date
  --until <date>          Only count sessions created on or before a date
  --top <n>               Limit each breakdown to the top N entries

Search flags:
  --json                  Print results as JSON (default)
  --ids                   Print one session ID per line
  --table                 Print a readable table
  --format json|ids|table Choose the output format
  --query <text>          Text to match (also accepted as a positional argument)
  --deep                  Search turns, checkpoints, files, and refs too
  --repo <name>           Only include sessions for a repository
  --branch <name>         Only include sessions on a branch
  --folder <path>         Only include sessions under a folder
  --host <type>           Only include sessions by host type (cli, cloud, actions)
  --since <date>          Only include sessions active on or after a date
  --until <date>          Only include sessions active on or before a date
  --limit <n>             Cap the number of results (default 50, 0 for no limit)

Views commands:
  views [list] [--json]    List configured named views
  views use <name>         Set the active named view
  views use default        Clear the active named view

Config commands:
  config list [--json]    Print every setting and its value
  config get <key>        Print one setting value
  config set <key> <val>  Validate and save one setting
  config unset <key>      Reset one setting to its default
  config edit             Open the config file in your editor
  config path             Print the config file path

Notes commands:
  notes [list] [--json]    List notes attached to current sessions
  notes get <id>           Print one session note
  notes set <id> <text>    Set one session note
  notes clear <id>         Clear one session note

Export flags:
  --format md|json|html   Output format (default md)
  --out <dir>             Write to a directory instead of the exports folder
  --stdout                Print to stdout instead of writing a file
  --redact                Mask common secret patterns in the export
  --query <text>          (batch) Text filter for session matching
  --repo <name>           (batch) Only export sessions for a repository
  --branch <name>         (batch) Only export sessions on a branch
  --folder <path>         (batch) Only export sessions under a folder
  --since <date>          (batch) Only export sessions created on or after a date
  --until <date>          (batch) Only export sessions created on or before a date

Tag commands:
  tag <id>                List tags for one session
  tag <id> --add a,b      Add tags (comma-separated)
  tag <id> --remove a     Remove tags
  tag <id> --set a,b      Replace all tags
  tag <id> --json         Print the result as JSON

Prune flags:
  --apply                 Remove stale entries (default is dry run)
  --json                  Print the summary as JSON

Watch flags:
  --once                  Print a snapshot and exit (default: stream transitions)
  --json                  Print as JSON
  --interval <dur>        Poll interval (default 5s, minimum 1s)
  --repo <name>           Only watch sessions for a repository
  --branch <name>         Only watch sessions on a branch
  --folder <path>         Only watch sessions under a folder

Flags:
  -h, --help              Show this help message
  -v, --version           Print the version
  --demo                  Launch with demo data
  --clear-cache           Reset config to defaults
  --reindex               Rebuild the session store index

Startup filters:
  --current               Filter to the current git repo and branch (from cwd)
  --cwd <path>            Filter to sessions under a folder (base dir for --current)
  --repo <name>           Filter to a repository (owner/repo)
  --branch <name>         Filter to a branch
  --query <text>          Pre-fill the search box with free text

Environment:
  DISPATCH_DB             Path to a custom session store database
  DISPATCH_SESSION_STATE  Path to a custom session state directory
  DISPATCH_LOG            Path to a log file (enables debug logging)
  DISPATCH_NO_UPDATE_CHECK
                          Skip the background release check when set to 1, true, yes, or on
`, version.Version)
}

// showUpdateNotification prints an update notification to stderr if the
// background version check found a newer release. It performs a non-blocking
// read of the channel to avoid delaying program exit.
func showUpdateNotification(w io.Writer, ch <-chan *update.UpdateInfo) {
	if w == nil {
		w = os.Stderr
	}

	select {
	case info := <-ch:
		if info != nil {
			fmt.Fprintf(w, "\nA new version of dispatch is available: v%s → v%s\nRun \"dispatch update\" to install it.\n",
				info.CurrentVersion, info.LatestVersion)
		}
	default:
	}
}

// findDemoDB looks for the fake session store in two places:
//  1. Next to the executable (for installed binaries).
//  2. Relative to the working directory (for go run / dev builds).
func findDemoDB() string {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), demoDBRel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if _, err := os.Stat(demoDBRel); err == nil {
		abs, err := filepath.Abs(demoDBRel)
		if err != nil {
			return ""
		}
		return abs
	}
	return ""
}
