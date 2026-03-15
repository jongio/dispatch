// Package main is the entry point for the dispatch CLI.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui"
	"github.com/jongio/dispatch/internal/update"
)

const demoDBRel = "internal/data/testdata/fake_sessions.db"

func main() {
	// Start background update check early so it can run concurrently
	// with argument parsing and TUI startup.
	updateCh := make(chan *update.UpdateInfo, 1)
	go func() {
		updateCh <- update.CheckForUpdate(tui.Version)
	}()

	origStderr := captureOriginalStderr()
	if origStderr == nil {
		origStderr = os.Stderr
	}
	if origStderr != os.Stderr {
		defer origStderr.Close() //nolint:errcheck // best-effort cleanup
	}

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--help", "-h", "help":
			printUsage()
			showUpdateNotification(origStderr, updateCh)
			return

		case "--version", "-v", "version":
			fmt.Println(tui.Version)
			showUpdateNotification(origStderr, updateCh)
			return

		case "update":
			if err := update.RunUpdate(tui.Version); err != nil {
				fmt.Fprintf(os.Stderr, "update: %v\n", err)
				os.Exit(1)
			}
			return

		case "--demo":
			demoCleanup, demoErr := setupDemo()
			if demoErr != nil {
				fmt.Fprintf(os.Stderr, "demo: %v\n", demoErr)
				os.Exit(1)
			}
			defer demoCleanup()

		case "--clear-cache":
			if err := config.Reset(); err != nil {
				fmt.Fprintf(os.Stderr, "clear-cache: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Config reset to defaults.")
			return

		case "--reindex":
			fmt.Println("Reindexing session store via Copilot CLI…")
			err := data.ChronicleReindex(context.Background(), func(line string) {
				fmt.Println(line)
			})
			if err != nil {
				if errors.Is(err, data.ErrCopilotNotFound) {
					fmt.Println("Copilot CLI not found, running index maintenance…")
					if mErr := data.Maintain(); mErr != nil {
						fmt.Fprintf(os.Stderr, "reindex: %v\n", mErr)
						os.Exit(1)
					}
				} else {
					fmt.Fprintf(os.Stderr, "reindex: %v\n", err)
					os.Exit(1)
				}
			}
			// Post-reindex maintenance (WAL checkpoint + FTS5 optimize).
			if mErr := data.Maintain(); mErr != nil {
				fmt.Fprintf(os.Stderr, "warning: post-reindex maintenance: %v\n", mErr)
			}
			fmt.Println("Done.")
			return

		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", arg)
			printUsage()
			os.Exit(1)
		}
	}

	// Redirect stderr BEFORE starting Bubble Tea.  The Copilot SDK
	// subprocess inherits our fd 2 (stderr) and writes error text like
	// "file already closed" to it.  That raw output leaks into Bubble
	// Tea's alt-screen buffer.  By redirecting fd 2 here we ensure the
	// subprocess's stderr goes to the log file (if set) or /dev/null.
	var logFile *os.File
	logWriter := io.Discard
	if logPath := os.Getenv("DISPATCH_LOG"); logPath != "" {
		// Validate log path: must be absolute and local (reject UNC paths
		// on Windows that could trigger outbound SMB authentication).
		cleaned := filepath.Clean(logPath)
		if filepath.IsAbs(cleaned) && !strings.HasPrefix(cleaned, `\\`) {
			if f, err := os.OpenFile(cleaned, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
				logFile = f
				logWriter = f
			}
		}
	}
	if logFile != nil {
		redirectStderr(logFile)
		defer logFile.Close() //nolint:errcheck // best-effort cleanup
	} else {
		if devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0); err == nil {
			redirectStderr(devNull)
			defer devNull.Close() //nolint:errcheck // best-effort cleanup
		}
	}

	// Silence slog during TUI mode — direct structured logs to the same
	// destination as stderr (log file or discard).
	slog.SetDefault(slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	p := tea.NewProgram(
		tui.NewModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(origStderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// After TUI exits, show update notification if the background
	// check found a newer release.
	showUpdateNotification(origStderr, updateCh)
}

func printUsage() {
	fmt.Printf(`dispatch %s — A TUI for browsing GitHub Copilot CLI sessions.

Usage:
  dispatch                Launch the interactive TUI
  dispatch [command]

Commands:
  help                    Show this help message
  version                 Print the version
  update                  Update dispatch to the latest release

Flags:
  -h, --help              Show this help message
  -v, --version           Print the version
  --demo                  Launch with demo data
  --clear-cache           Reset config to defaults
  --reindex               Rebuild the session store index

Environment:
  DISPATCH_DB             Path to a custom session store database
  DISPATCH_LOG            Path to a log file (enables debug logging)
`, tui.Version)
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
