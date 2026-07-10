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
  version                 Print the version
  open <id> [--mode M]    Resume a session by ID (M: inplace, tab, window, pane)
                          --print writes the resume command instead of launching
  open --last [--mode M]  Resume the most recently active session
  new [dir] [--mode M]    Start a new session in a directory (default: current)
  completion <shell>      Print shell completion (bash, zsh, fish, powershell)
  doctor [--json]         Print environment diagnostics (--json for machine-readable output)
  stats [flags]           Print session totals and breakdowns
  search [query] [flags]  Print matching sessions as JSON, IDs, or a table
  tags [--json]           List tags in use with per-tag session counts
  config [get|set|list|edit|path]
                          Read or change preferences (see Config commands)
  export <id> [flags]     Export a session as Markdown or JSON
  update                  Update dispatch to the latest release

Stats flags:
  --json                  Print the summary as JSON
  --calendar              Add a per-day activity heatmap
  --repo <name>           Only count sessions for a repository
  --branch <name>         Only count sessions on a branch
  --folder <path>         Only count sessions under a folder
  --since <date>          Only count sessions created on or after a date
  --until <date>          Only count sessions created on or before a date

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

Config commands:
  config list [--json]    Print every setting and its value
  config get <key>        Print one setting value
  config set <key> <val>  Validate and save one setting
  config edit             Open the config file in your editor
  config path             Print the config file path

Export flags:
  --format md|json        Output format (default md)
  --out <dir>             Write to a directory instead of the exports folder
  --stdout                Print to stdout instead of writing a file

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
