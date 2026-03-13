//go:build windows

package platform

import (
	"testing"
)

// ---------------------------------------------------------------------------
// launchInPlaceUnix (Windows stub) — 0% coverage
// ---------------------------------------------------------------------------

func TestLaunchInPlaceUnix_WindowsStub(t *testing.T) {
	err := launchInPlaceUnix(ShellInfo{}, "", "")
	if err == nil {
		t.Fatal("launchInPlaceUnix on Windows should return error")
	}
	want := "launchInPlaceUnix: not supported on Windows"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// ---------------------------------------------------------------------------
// buildStartCmdLine — test different shell types
// ---------------------------------------------------------------------------

func TestBuildStartCmdLine_PowerShell(t *testing.T) {
	shell := ShellInfo{Name: "PowerShell 7", Path: `C:\Program Files\PowerShell\7\pwsh.exe`, Args: []string{"-NoLogo"}}
	resumeCmd := `"ghcs" "--resume" "sess-123"`

	got := buildStartCmdLine(shell, resumeCmd)

	// Should contain psQuote wrapping.
	if got == "" {
		t.Fatal("buildStartCmdLine returned empty string")
	}
	// Should include the shell path.
	if !containsStr(got, shell.Path) {
		t.Errorf("cmd line should contain shell path %q", shell.Path)
	}
	// Should use -Command for PowerShell.
	if !containsStr(got, "-Command") {
		t.Error("cmd line should use -Command for PowerShell")
	}
	// Should include -NoLogo from shell.Args.
	if !containsStr(got, "-NoLogo") {
		t.Error("cmd line should include -NoLogo from shell.Args")
	}
}

func TestBuildStartCmdLine_Cmd(t *testing.T) {
	shell := ShellInfo{Name: "Command Prompt", Path: `C:\Windows\System32\cmd.exe`}
	resumeCmd := `"ghcs" "--resume" "sess-123"`

	got := buildStartCmdLine(shell, resumeCmd)

	if !containsStr(got, "/k") {
		t.Error("cmd line should use /k for cmd.exe")
	}
}

func TestBuildStartCmdLine_Bash(t *testing.T) {
	shell := ShellInfo{Name: "Git Bash", Path: `C:\Program Files\Git\bin\bash.exe`, Args: []string{"--login", "-i"}}
	resumeCmd := `ghcs --resume sess-123`

	got := buildStartCmdLine(shell, resumeCmd)

	if !containsStr(got, "-c") {
		t.Error("cmd line should use -c for bash")
	}
	if !containsStr(got, "--login") {
		t.Error("cmd line should include --login from shell.Args")
	}
}

// ---------------------------------------------------------------------------
// LaunchSession — additional branch coverage
// ---------------------------------------------------------------------------

func TestLaunchSession_InvalidSessionID(t *testing.T) {
	shell := ShellInfo{Name: "test", Path: "test"}
	err := LaunchSession(shell, "; malicious", ResumeConfig{})
	if err == nil {
		t.Error("LaunchSession should reject invalid session ID")
	}
}

func TestLaunchSession_EmptyShellDefaultsToDetected(t *testing.T) {
	// With empty shell path, it should use DefaultShell().
	// The session ID is invalid so we'll get an error, but the
	// important thing is that DefaultShell() is called.
	err := LaunchSession(ShellInfo{}, "; bad", ResumeConfig{})
	if err == nil {
		t.Error("should still reject bad session ID")
	}
}

// ---------------------------------------------------------------------------
// DetectTerminals — Windows path
// ---------------------------------------------------------------------------

func TestDetectTerminals_Windows(t *testing.T) {
	terminals := DetectTerminals()
	// On Windows, should detect at least one terminal.
	// We just verify it doesn't panic and returns a non-nil slice.
	if terminals == nil {
		t.Error("DetectTerminals() should return non-nil slice on Windows")
	}
}

// ---------------------------------------------------------------------------
// DetectShells — Windows path
// ---------------------------------------------------------------------------

func TestDetectShells_Windows(t *testing.T) {
	shells := DetectShells()
	if len(shells) == 0 {
		t.Error("DetectShells() should find at least one shell on Windows")
	}
}

// ---------------------------------------------------------------------------
// DefaultShell — Windows path
// ---------------------------------------------------------------------------

func TestDefaultShell_Windows(t *testing.T) {
	shell := DefaultShell()
	if shell.Path == "" {
		t.Error("DefaultShell() should return a shell with a non-empty path")
	}
	if shell.Name == "" {
		t.Error("DefaultShell() should return a shell with a non-empty name")
	}
}

// ---------------------------------------------------------------------------
// DefaultTerminal — Windows path
// ---------------------------------------------------------------------------

func TestDefaultTerminal_Windows(t *testing.T) {
	terminal := DefaultTerminal()
	// Should return either "Windows Terminal" or "conhost".
	if terminal != termWindowsTerminal && terminal != termConhost {
		t.Errorf("DefaultTerminal() = %q, want %q or %q", terminal, termWindowsTerminal, termConhost)
	}
}

// ---------------------------------------------------------------------------
// containsStr helper (avoid importing strings in test file)
// ---------------------------------------------------------------------------

func containsStr(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
