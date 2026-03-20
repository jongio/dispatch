//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestCheckTerminalCompat_NoMSYSTEM(t *testing.T) {
	// When MSYSTEM is not set (PowerShell, cmd, etc.), the check should pass.
	t.Setenv("MSYSTEM", "")
	if msg := checkTerminalCompat(); msg != "" {
		t.Errorf("checkTerminalCompat() should return empty when MSYSTEM is unset, got: %s", msg)
	}
}

func TestCheckTerminalCompat_MinTTY(t *testing.T) {
	// Simulate standalone Git Bash (MinTTY): MSYSTEM is set but no
	// ConPTY terminal env vars are present.
	t.Setenv("MSYSTEM", "MINGW64")
	t.Setenv("WT_SESSION", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("ConEmuPID", "")

	msg := checkTerminalCompat()
	if msg == "" {
		t.Fatal("checkTerminalCompat() should return error message for MinTTY")
	}
	// Verify the message contains key information.
	for _, want := range []string{
		"MinTTY",
		"MSYS2 pseudo-TTY",
		"PowerShell",
		"Windows Terminal",
		"winpty",
		"https://www.msys2.org/docs/terminals/",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message should mention %q;\ngot: %s", want, msg)
		}
	}
}

func TestCheckTerminalCompat_WindowsTerminal(t *testing.T) {
	// Git Bash shell inside Windows Terminal — WT_SESSION is set.
	t.Setenv("MSYSTEM", "MINGW64")
	t.Setenv("WT_SESSION", "some-guid")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("ConEmuPID", "")

	if msg := checkTerminalCompat(); msg != "" {
		t.Errorf("checkTerminalCompat() should pass for Windows Terminal, got: %s", msg)
	}
}

func TestCheckTerminalCompat_VSCode(t *testing.T) {
	// Git Bash shell inside VS Code integrated terminal.
	t.Setenv("MSYSTEM", "MINGW64")
	t.Setenv("WT_SESSION", "")
	t.Setenv("TERM_PROGRAM", "vscode")
	t.Setenv("ConEmuPID", "")

	if msg := checkTerminalCompat(); msg != "" {
		t.Errorf("checkTerminalCompat() should pass for VS Code terminal, got: %s", msg)
	}
}

func TestCheckTerminalCompat_VSCodeCaseInsensitive(t *testing.T) {
	// TERM_PROGRAM comparison should be case-insensitive.
	t.Setenv("MSYSTEM", "MINGW64")
	t.Setenv("WT_SESSION", "")
	t.Setenv("TERM_PROGRAM", "VSCode")
	t.Setenv("ConEmuPID", "")

	if msg := checkTerminalCompat(); msg != "" {
		t.Errorf("checkTerminalCompat() should be case-insensitive for TERM_PROGRAM=VSCode, got: %s", msg)
	}
}

func TestCheckTerminalCompat_ConEmu(t *testing.T) {
	// Git Bash shell inside ConEmu/Cmder.
	t.Setenv("MSYSTEM", "MINGW64")
	t.Setenv("WT_SESSION", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("ConEmuPID", "12345")

	if msg := checkTerminalCompat(); msg != "" {
		t.Errorf("checkTerminalCompat() should pass for ConEmu, got: %s", msg)
	}
}

func TestCheckTerminalCompat_UnknownTermProgram(t *testing.T) {
	// MSYSTEM set with an unrecognized TERM_PROGRAM — should still block.
	t.Setenv("MSYSTEM", "MINGW64")
	t.Setenv("WT_SESSION", "")
	t.Setenv("TERM_PROGRAM", "alacritty")
	t.Setenv("ConEmuPID", "")

	msg := checkTerminalCompat()
	if msg == "" {
		t.Error("checkTerminalCompat() should block for unknown TERM_PROGRAM with MSYSTEM set")
	}
}

func TestIsConPTYTerminal_AllUnset(t *testing.T) {
	t.Setenv("WT_SESSION", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("ConEmuPID", "")

	if isConPTYTerminal() {
		t.Error("isConPTYTerminal() should return false when no ConPTY env vars are set")
	}
}

func TestIsConPTYTerminal_WTSession(t *testing.T) {
	t.Setenv("WT_SESSION", "abc-123")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("ConEmuPID", "")

	if !isConPTYTerminal() {
		t.Error("isConPTYTerminal() should return true when WT_SESSION is set")
	}
}

func TestIsConPTYTerminal_ConEmuPID(t *testing.T) {
	t.Setenv("WT_SESSION", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("ConEmuPID", "9999")

	if !isConPTYTerminal() {
		t.Error("isConPTYTerminal() should return true when ConEmuPID is set")
	}
}

// TestCheckTerminalCompat_MSYSTEMVariants ensures detection works for
// different MSYSTEM values (MINGW32, MINGW64, UCRT64, CLANG64, etc.)
func TestCheckTerminalCompat_MSYSTEMVariants(t *testing.T) {
	variants := []string{"MINGW32", "MINGW64", "UCRT64", "CLANG64", "MSYS"}
	for _, v := range variants {
		t.Run(v, func(t *testing.T) {
			t.Setenv("MSYSTEM", v)
			t.Setenv("WT_SESSION", "")
			t.Setenv("TERM_PROGRAM", "")
			t.Setenv("ConEmuPID", "")

			if msg := checkTerminalCompat(); msg == "" {
				t.Errorf("checkTerminalCompat() should block for MSYSTEM=%s without ConPTY", v)
			}
		})
	}
}

// TestCheckTerminalCompat_PreservesEnv verifies t.Setenv restores correctly
// (sanity check that our test isolation works).
func TestCheckTerminalCompat_PreservesEnv(t *testing.T) {
	original := os.Getenv("MSYSTEM")
	t.Setenv("MSYSTEM", "TEST_VALUE_FOR_DISPATCH")
	// After the test, t.Setenv restores the original value automatically.
	// Verify the temporary value is active during the test.
	if os.Getenv("MSYSTEM") != "TEST_VALUE_FOR_DISPATCH" {
		t.Error("t.Setenv should set the value during the test")
	}
	_ = original // appease linter
}
