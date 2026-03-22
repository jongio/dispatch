package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui"
	"github.com/jongio/dispatch/internal/update"
)

// handleArgs processes CLI arguments and executes early-exit subcommands
// (help, version, update, clear-cache, reindex). It returns done=true when
// the caller should exit without starting the TUI. When --demo is among
// the arguments, cleanup is non-nil and the caller must defer it. Errors
// indicate a failing subcommand; the error message is already printed to
// stderr.
//
// Function variables (below) allow test substitution of external calls.
var (
	chronicleReindexFn = data.ChronicleReindex
	maintainFn         = data.Maintain
	runUpdateFn        = update.RunUpdate
	configResetFn      = config.Reset
)

func handleArgs(args []string, origStderr io.Writer, updateCh <-chan *update.UpdateInfo) (done bool, cleanup func(), err error) {
	for _, arg := range args {
		switch arg {
		case "--help", "-h", "help":
			printUsage()
			showUpdateNotification(origStderr, updateCh)
			return true, cleanup, nil

		case "--version", "-v", "version":
			fmt.Println(tui.Version)
			showUpdateNotification(origStderr, updateCh)
			return true, cleanup, nil

		case "update":
			if uErr := runUpdateFn(tui.Version); uErr != nil {
				fmt.Fprintf(os.Stderr, "update: %v\n", uErr)
				return true, cleanup, uErr
			}
			return true, cleanup, nil

		case "--demo":
			c, demoErr := setupDemo()
			if demoErr != nil {
				fmt.Fprintf(os.Stderr, "demo: %v\n", demoErr)
				return true, cleanup, demoErr
			}
			cleanup = c

		case "--clear-cache":
			if cErr := configResetFn(); cErr != nil {
				fmt.Fprintf(os.Stderr, "clear-cache: %v\n", cErr)
				return true, cleanup, cErr
			}
			fmt.Println("Config reset to defaults.")
			return true, cleanup, nil

		case "--reindex":
			fmt.Println("Reindexing session store via Copilot CLI…")
			rErr := chronicleReindexFn(context.Background(), func(line string) {
				fmt.Println(line)
			})
			if rErr != nil {
				if errors.Is(rErr, data.ErrCopilotNotFound) {
					fmt.Println("Copilot CLI not found, running index maintenance…")
					if mErr := maintainFn(); mErr != nil {
						fmt.Fprintf(os.Stderr, "reindex: %v\n", mErr)
						return true, cleanup, mErr
					}
				} else {
					fmt.Fprintf(os.Stderr, "reindex: %v\n", rErr)
					return true, cleanup, rErr
				}
			}
			// Post-reindex maintenance (WAL checkpoint + FTS5 optimize).
			if mErr := maintainFn(); mErr != nil {
				fmt.Fprintf(os.Stderr, "warning: post-reindex maintenance: %v\n", mErr)
			}
			fmt.Println("Done.")
			return true, cleanup, nil

		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", arg)
			printUsage()
			return true, cleanup, fmt.Errorf("unknown flag: %s", arg)
		}
	}
	return false, cleanup, nil
}

// setupLogRedirect opens the log file (if configured via DISPATCH_LOG) and
// redirects stderr to it. When no log file is configured, stderr is sent to
// os.DevNull to keep Bubble Tea's alt-screen clean. Returns the writer for
// structured logging and a cleanup function that closes the redirect target.
func setupLogRedirect() (io.Writer, func()) {
	logFile := openLogFile(os.Getenv("DISPATCH_LOG"))
	if logFile != nil {
		redirectStderr(logFile)
		return logFile, func() { logFile.Close() } //nolint:errcheck // best-effort
	}
	if devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0); err == nil {
		redirectStderr(devNull)
		return io.Discard, func() { devNull.Close() } //nolint:errcheck // best-effort
	}
	return io.Discard, func() {}
}

// openLogFile opens a log file for writing at the given path. The path
// must be absolute and must not be a UNC path (to prevent outbound SMB
// authentication on Windows). Returns nil if the path is empty, invalid,
// or cannot be opened.
func openLogFile(logPath string) *os.File {
	if logPath == "" {
		return nil
	}
	cleaned := filepath.Clean(logPath)
	if !filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, `\\`) {
		return nil
	}
	f, err := os.OpenFile(cleaned, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil
	}
	return f
}
