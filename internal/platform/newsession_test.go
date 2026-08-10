package platform

import (
	"errors"
	"strings"
	"testing"
)

func TestLaunchNewSessionUsesDefaultsAndQuotesWorkingDirectory(t *testing.T) {
	original := launchNewSessionPlatformFn
	t.Cleanup(func() { launchNewSessionPlatformFn = original })

	var gotShell ShellInfo
	var gotCommand, gotTerminal, gotCwd, gotStyle, gotDirection string
	launchNewSessionPlatformFn = func(shell ShellInfo, command, terminal, cwd, style, direction string) (int, error) {
		gotShell = shell
		gotCommand = command
		gotTerminal = terminal
		gotCwd = cwd
		gotStyle = style
		gotDirection = direction
		return 42, nil
	}

	cwd := `C:\work dir&echo injected`
	pid, err := LaunchNewSession(LaunchNewSessionConfig{
		Cwd:           cwd,
		Shell:         ShellInfo{Name: "test", Path: "test-shell"},
		Terminal:      "test-terminal",
		LaunchStyle:   LaunchStylePane,
		PaneDirection: "vertical",
	})
	if err != nil {
		t.Fatalf("LaunchNewSession() error: %v", err)
	}
	if pid != 42 {
		t.Fatalf("PID = %d, want 42", pid)
	}
	if gotShell.Path != "test-shell" || gotTerminal != "test-terminal" {
		t.Fatalf("launcher shell/terminal = (%q, %q), want supplied values", gotShell.Path, gotTerminal)
	}
	if gotCommand != defaultNewSessionCommand {
		t.Fatalf("command = %q, want %q", gotCommand, defaultNewSessionCommand)
	}
	if gotCwd != cwd || gotStyle != LaunchStylePane || gotDirection != "vertical" {
		t.Fatalf("launcher args = cwd %q style %q direction %q", gotCwd, gotStyle, gotDirection)
	}

	_, err = LaunchNewSession(LaunchNewSessionConfig{
		Command:  "copilot --cwd {cwd}",
		Cwd:      cwd,
		Shell:    ShellInfo{Name: "test", Path: "test-shell"},
		Terminal: "test-terminal",
	})
	if err != nil {
		t.Fatalf("LaunchNewSession() with template error: %v", err)
	}
	wantQuoted := shellQuoteForOS(cwd)
	if gotCommand != "copilot --cwd "+wantQuoted {
		t.Fatalf("expanded command = %q, want %q", gotCommand, "copilot --cwd "+wantQuoted)
	}
}

func TestLaunchNewSessionRejectsInvalidCommand(t *testing.T) {
	original := launchNewSessionPlatformFn
	t.Cleanup(func() { launchNewSessionPlatformFn = original })

	called := false
	launchNewSessionPlatformFn = func(ShellInfo, string, string, string, string, string) (int, error) {
		called = true
		return 0, nil
	}

	_, err := LaunchNewSession(LaunchNewSessionConfig{
		Command:  "copilot\nmalicious",
		Shell:    ShellInfo{Path: "test-shell"},
		Terminal: "test-terminal",
	})
	if err == nil || !strings.Contains(err.Error(), "embedded newlines") {
		t.Fatalf("error = %v, want embedded-newline rejection", err)
	}
	if called {
		t.Fatal("platform launcher was called for an invalid command")
	}
}

func TestLaunchNewSessionPlatformWrapsLaunchError(t *testing.T) {
	original := platformLaunchSessionFn
	t.Cleanup(func() { platformLaunchSessionFn = original })

	sentinel := errors.New("launch failed")
	platformLaunchSessionFn = func(ShellInfo, string, string, string, string, string) error {
		return sentinel
	}

	pid, err := launchNewSessionPlatformImpl(
		ShellInfo{Path: "test-shell"},
		"copilot",
		"test-terminal",
		"",
		LaunchStyleTab,
		"",
	)
	if pid != 0 {
		t.Fatalf("PID = %d, want 0", pid)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want wrapped sentinel", err)
	}
}
