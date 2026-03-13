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
)

const demoDBRel = "internal/data/testdata/fake_sessions.db"

func main() {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--demo":
			dbPath := findDemoDB()
			if dbPath == "" {
				fmt.Fprintln(os.Stderr, "demo db not found; set DISPATCH_DB or run from the repo root")
				os.Exit(1)
			}
			_ = os.Setenv("DISPATCH_DB", dbPath)

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
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
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
