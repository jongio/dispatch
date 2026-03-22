//go:build windows

package platform

import (
	"os/exec"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// buildWSLWTArgs — pure function, fully testable
// ---------------------------------------------------------------------------

func TestBuildWSLWTArgs_DefaultTab(t *testing.T) {
	t.Parallel()
	shell := ShellInfo{Name: "bash", Path: "/usr/bin/bash"}
	args := buildWSLWTArgs(shell, "ghcs --resume s1", "", "", "", "")
	// Default launch style → new-tab in current window
	assertContains(t, args, "-w", "0", "new-tab", "wsl.exe", "--", "/usr/bin/bash", "-c", "ghcs --resume s1")
	assertNotContains(t, args, "--startingDirectory")
	assertNotContains(t, args, "-d")
}

func TestBuildWSLWTArgs_Window(t *testing.T) {
	t.Parallel()
	shell := ShellInfo{Name: "bash", Path: "/usr/bin/bash"}
	args := buildWSLWTArgs(shell, "ghcs --resume s1", "", "", LaunchStyleWindow, "")
	assertContains(t, args, "-w", "new", "new-tab")
}

func TestBuildWSLWTArgs_Pane(t *testing.T) {
	t.Parallel()
	shell := ShellInfo{Name: "bash", Path: "/usr/bin/bash"}
	args := buildWSLWTArgs(shell, "ghcs --resume s1", "", "", LaunchStylePane, "down")
	assertContains(t, args, "-w", "0", "split-pane", "-H")
}

func TestBuildWSLWTArgs_PaneRight(t *testing.T) {
	t.Parallel()
	shell := ShellInfo{Name: "bash", Path: "/usr/bin/bash"}
	args := buildWSLWTArgs(shell, "ghcs --resume s1", "", "", LaunchStylePane, "right")
	assertContains(t, args, "-V")
}

func TestBuildWSLWTArgs_PaneAuto(t *testing.T) {
	t.Parallel()
	shell := ShellInfo{Name: "bash", Path: "/usr/bin/bash"}
	args := buildWSLWTArgs(shell, "ghcs --resume s1", "", "", LaunchStylePane, "auto")
	// "auto" → no direction flag
	assertNotContains(t, args, "-H")
	assertNotContains(t, args, "-V")
}

func TestBuildWSLWTArgs_WithCwd(t *testing.T) {
	t.Parallel()
	shell := ShellInfo{Name: "bash", Path: "/usr/bin/bash"}
	args := buildWSLWTArgs(shell, "ghcs --resume s1", `C:\Users\test`, "", "", "")
	assertContains(t, args, "--startingDirectory", `C:\Users\test`)
}

func TestBuildWSLWTArgs_WithDistro(t *testing.T) {
	t.Parallel()
	shell := ShellInfo{Name: "bash", Path: "/usr/bin/bash"}
	args := buildWSLWTArgs(shell, "ghcs --resume s1", "", "Ubuntu-22.04", "", "")
	assertContains(t, args, "wsl.exe", "-d", "Ubuntu-22.04", "--", "/usr/bin/bash", "-c")
}

func TestBuildWSLWTArgs_WithDistroAndCwd(t *testing.T) {
	t.Parallel()
	shell := ShellInfo{Name: "zsh", Path: "/usr/bin/zsh"}
	args := buildWSLWTArgs(shell, "ghcs --resume s2", `C:\Users\dev`, "Debian", LaunchStyleWindow, "")
	assertContains(t, args, "-w", "new", "new-tab")
	assertContains(t, args, "--startingDirectory", `C:\Users\dev`)
	assertContains(t, args, "wsl.exe", "-d", "Debian", "--", "/usr/bin/zsh", "-c", "ghcs --resume s2")
}

func TestBuildWSLWTArgs_EmptyDistroOmitsDFlag(t *testing.T) {
	t.Parallel()
	shell := ShellInfo{Name: "bash", Path: "/usr/bin/bash"}
	args := buildWSLWTArgs(shell, "cmd", "", "", "", "")
	assertNotContains(t, args, "-d")
}

// ---------------------------------------------------------------------------
// setCmdLine — verify it doesn't panic
// ---------------------------------------------------------------------------

func TestSetCmdLine_SetsRawCmdLine(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("cmd.exe")
	setCmdLine(cmd, `cmd.exe /c start "" "C:\test.exe"`)
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr should be set")
	}
	if cmd.SysProcAttr.CmdLine == "" {
		t.Error("CmdLine should be non-empty")
	}
}

// ---------------------------------------------------------------------------
// escapeAppleScript — pure function, testable anywhere
// ---------------------------------------------------------------------------

func TestEscapeAppleScript_Basic(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"plain", "hello", "hello"},
		{"double quote", `say "hello"`, `say \"hello\"`},
		{"backslash", `path\to\file`, `path\\to\\file`},
		{"single quote", "it's", "it'\\''s"},
		{"control chars stripped", "hello\x01\x02world", "helloworld"},
		{"del stripped", "hello\x7Fworld", "helloworld"},
		{"tab stripped", "hello\tworld", "helloworld"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := escapeAppleScript(tt.input)
			if got != tt.want {
				t.Errorf("escapeAppleScript(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func assertContains(t *testing.T, args []string, want ...string) {
	t.Helper()
	joined := strings.Join(args, " ")
	for _, w := range want {
		found := false
		for _, a := range args {
			if a == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("args %v should contain %q (full: %s)", args, w, joined)
		}
	}
}

func assertNotContains(t *testing.T, args []string, excluded ...string) {
	t.Helper()
	for _, e := range excluded {
		for _, a := range args {
			if a == e {
				t.Errorf("args %v should NOT contain %q", args, e)
			}
		}
	}
}
