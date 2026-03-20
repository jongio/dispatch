//go:build windows

package main

import (
	"os"
	"strings"
)

// checkTerminalCompat detects MSYS/Git Bash environments where the TUI
// won't work. MinTTY (the default Git Bash terminal) uses MSYS pseudo-TTYs
// that Go sees as pipes at the POSIX layer, but the Windows standard handles
// (GetStdHandle) still point at the original console — so GetConsoleMode
// misleadingly succeeds. We therefore use environment variables to detect
// MinTTY vs ConPTY-capable terminals.
//
// Returns a non-empty message when the terminal is known to be incompatible.
func checkTerminalCompat() string {
	if os.Getenv("MSYSTEM") == "" {
		return "" // not running under MSYS/Git Bash
	}

	// ConPTY-capable terminals provide genuine console I/O even when
	// running a Git Bash shell. Detect them by their env vars.
	if isConPTYTerminal() {
		return ""
	}

	// MSYSTEM is set but we're not in a ConPTY terminal — almost certainly
	// MinTTY (standalone Git Bash). The TUI will render a blank screen.
	return `dispatch does not work in Git Bash (MinTTY).

WHY: Git Bash uses MinTTY as its terminal, which communicates with programs
through MSYS2 pseudo-TTYs (POSIX-style pipes). Go programs on Windows use the
Win32 console API for terminal I/O (raw mode, alternate screen, key events).
These two layers are incompatible -- Go reads from the Windows console handle
while MinTTY writes to its own pipe, resulting in a blank screen.

This affects all Go TUI applications (Bubble Tea, tview, tcell), not just
dispatch. It is a known limitation of MinTTY with native Windows programs.

ALTERNATIVES (any of these will work):
  - PowerShell:        dispatch
  - Command Prompt:    dispatch
  - Windows Terminal:  Add a Git Bash profile -- dispatch works via ConPTY
  - winpty wrapper:    winpty dispatch

MORE INFO: https://www.msys2.org/docs/terminals/`
}

// isConPTYTerminal returns true when the process is running inside a
// terminal that provides ConPTY (real Windows console) even for MSYS shells.
func isConPTYTerminal() bool {
	// Windows Terminal
	if os.Getenv("WT_SESSION") != "" {
		return true
	}
	// VS Code integrated terminal
	if strings.EqualFold(os.Getenv("TERM_PROGRAM"), "vscode") {
		return true
	}
	// ConEmu / Cmder
	if os.Getenv("ConEmuPID") != "" {
		return true
	}
	return false
}
