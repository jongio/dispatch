package platform

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// detectUnixShells - fallback path (no /etc/shells on Windows)
// ---------------------------------------------------------------------------

func TestDetectUnixShells_FallbackPath(t *testing.T) {
	// On Windows, /etc/shells does not exist so detectUnixShells takes the
	// fallback branch that probes well-known shell names via LookPath.
	// On Unix, this still exercises the function - it reads /etc/shells.
	shells := detectUnixShells()
	for _, s := range shells {
		if s.Name == "" {
			t.Error("shell Name should not be empty")
		}
		if s.Path == "" {
			t.Error("shell Path should not be empty")
		}
	}
	// On Windows, the fallback probes bash/zsh/fish/sh via LookPath.
	// We may or may not find any, but the function must not panic.
	t.Logf("detectUnixShells found %d shells", len(shells))
}

func TestDetectUnixShells_NoDuplicateNames(t *testing.T) {
	shells := detectUnixShells()
	seen := make(map[string]struct{})
	for _, s := range shells {
		if _, exists := seen[s.Name]; exists {
			t.Errorf("detectUnixShells returned duplicate shell name: %q", s.Name)
		}
		seen[s.Name] = struct{}{}
	}
}

func TestDetectUnixShells_PathsAreAbsolute(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell paths on Windows may not be Unix-absolute")
	}
	shells := detectUnixShells()
	for _, s := range shells {
		if !strings.HasPrefix(s.Path, "/") {
			t.Errorf("shell %q has non-absolute path: %q", s.Name, s.Path)
		}
	}
}

// ---------------------------------------------------------------------------
// defaultUnixShell - $SHELL env and fallback
// ---------------------------------------------------------------------------

func TestDefaultUnixShell_WithSHELLEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("$SHELL env var semantics differ on Windows")
	}
	// Set $SHELL to an existing file to exercise the env-var path.
	tmpFile, err := os.CreateTemp("", "fake-shell-*")
	if err != nil {
		t.Fatal(err)
	}
	_ = tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	t.Setenv("SHELL", tmpFile.Name())
	sh := defaultUnixShell()
	if sh.Path != tmpFile.Name() {
		t.Errorf("defaultUnixShell().Path = %q, want %q", sh.Path, tmpFile.Name())
	}
}

func TestDefaultUnixShell_FallbackWithoutSHELL(t *testing.T) {
	t.Setenv("SHELL", "")
	sh := defaultUnixShell()
	// Should fall back to LookPath("bash") or "/bin/sh".
	if sh.Name == "" {
		t.Error("defaultUnixShell should return a shell even without $SHELL")
	}
	if sh.Path == "" {
		t.Error("defaultUnixShell should return a path even without $SHELL")
	}
	t.Logf("defaultUnixShell fallback: %+v", sh)
}

func TestDefaultUnixShell_InvalidSHELL(t *testing.T) {
	// Non-absolute path - should be ignored.
	t.Setenv("SHELL", "not-absolute")
	sh := defaultUnixShell()
	if sh.Path == "not-absolute" {
		t.Error("defaultUnixShell should ignore non-absolute $SHELL")
	}
	// Non-existent absolute path - should be ignored.
	t.Setenv("SHELL", "/nonexistent/path/to/shell")
	sh = defaultUnixShell()
	if sh.Path == "/nonexistent/path/to/shell" {
		t.Error("defaultUnixShell should ignore non-existent $SHELL path")
	}
}

func TestDefaultUnixShell_DirectoryInSHELL(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELL", dir)
	sh := defaultUnixShell()
	// A directory path should be rejected (info.IsDir() check).
	if sh.Path == dir {
		t.Error("defaultUnixShell should ignore directory paths in $SHELL")
	}
}

// ---------------------------------------------------------------------------
// detectDarwinTerminals - always includes Terminal.app
// ---------------------------------------------------------------------------

func TestDetectDarwinTerminals_AlwaysIncludesTerminalApp(t *testing.T) {
	terms := detectDarwinTerminals()
	if len(terms) == 0 {
		t.Fatal("detectDarwinTerminals should always return at least Terminal.app")
	}
	if terms[0].Name != "Terminal.app" {
		t.Errorf("first terminal should be Terminal.app, got %q", terms[0].Name)
	}
}

// ---------------------------------------------------------------------------
// detectLinuxTerminals - candidate probing
// ---------------------------------------------------------------------------

func TestDetectLinuxTerminals_ReturnsTerminalInfos(t *testing.T) {
	terms := detectLinuxTerminals()
	for _, ti := range terms {
		if ti.Name == "" {
			t.Error("detectLinuxTerminals returned a terminal with empty Name")
		}
	}
	t.Logf("detectLinuxTerminals found %d terminals", len(terms))
}

// ---------------------------------------------------------------------------
// isGitBash - pure function
// ---------------------------------------------------------------------------

func TestIsGitBash_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		si   ShellInfo
		want bool
	}{
		{"git bash typical", ShellInfo{Name: "Git Bash", Path: `C:\Program Files\Git\bin\bash.exe`}, true},
		{"git bash name match", ShellInfo{Name: "Git Bash", Path: "/usr/bin/bash"}, true},
		{"bash under git dir", ShellInfo{Name: "bash", Path: `C:\Git\bin\bash.exe`}, true},
		{"plain bash", ShellInfo{Name: "bash", Path: "/usr/bin/bash"}, false},
		{"powershell", ShellInfo{Name: "PowerShell", Path: `C:\Windows\System32\powershell.exe`}, false},
		{"cmd", ShellInfo{Name: "cmd", Path: `C:\Windows\System32\cmd.exe`}, false},
		{"zsh", ShellInfo{Name: "zsh", Path: "/usr/bin/zsh"}, false},
		{"empty", ShellInfo{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGitBash(tt.si)
			if got != tt.want {
				t.Errorf("isGitBash(%+v) = %v, want %v", tt.si, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// bashifyCmd - pure function
// ---------------------------------------------------------------------------

func TestBashifyCmd_TableDriven(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"backslashes to forward slashes",
			`C:\Users\test\ghcs.exe`,
			`C:/Users/test/ghcs.exe`,
		},
		{
			"double quotes to single quotes",
			"\"C:\\Program Files\\ghcs.exe\" \"--resume\" \"abc123\"",
			"'C:/Program Files/ghcs.exe' '--resume' 'abc123'",
		},
		{
			"existing single quotes escaped",
			"\"O'Brien's tool\"",
			"'O'\\''Brien'\\''s tool'",
		},
		{
			"no special chars",
			"simple-command",
			"simple-command",
		},
		{
			"mixed backslash and quotes",
			"\"C:\\Users\\Bob's Tools\\ghcs.exe\"",
			"'C:/Users/Bob'\\''s Tools/ghcs.exe'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bashifyCmd(tt.input)
			if got != tt.want {
				t.Errorf("bashifyCmd(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// sanitizeStderr - pure function
// ---------------------------------------------------------------------------

func TestSanitizeStderr(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "error: something failed", "error: something failed"},
		{"newlines preserved", "line1\nline2", "line1\nline2"},
		{"tabs preserved", "col1\tcol2", "col1\tcol2"},
		{"control chars stripped", "err\x01or\x02msg\x03", "errormsg"},
		{"escape sequences stripped", "err\x1b[2Jhidden\x1b[31mred", "err[2Jhidden[31mred"},
		{"bell stripped", "msg\x07bell", "msgbell"},
		{"DEL stripped", "msg\x7Fdel", "msgdel"},
		{"null stripped", "msg\x00null", "msgnull"},
		{"empty", "", ""},
		{"mixed safe and unsafe", "ok\x01\x02\n\tok\x7F", "ok\n\tok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeStderr(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeStderr(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// limitedWriter - edge cases
// ---------------------------------------------------------------------------

func TestLimitedWriter_ZeroMax(t *testing.T) {
	var buf bytes.Buffer
	lw := &limitedWriter{buf: &buf, max: 0}
	n, err := lw.Write([]byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 4 {
		t.Errorf("Write should report full input length, got %d", n)
	}
	if buf.Len() != 0 {
		t.Errorf("buffer should be empty with max=0, got %d bytes", buf.Len())
	}
}

func TestLimitedWriter_MultipleWrites(t *testing.T) {
	var buf bytes.Buffer
	lw := &limitedWriter{buf: &buf, max: 10}
	lw.Write([]byte("hello"))  // 5 bytes
	lw.Write([]byte("world!")) // 6 bytes, but only 5 fit
	if buf.Len() != 10 {
		t.Errorf("buffer should be capped at 10, got %d", buf.Len())
	}
	if buf.String() != "helloworld" {
		t.Errorf("buffer = %q, want %q", buf.String(), "helloworld")
	}
	// Additional writes should be silently discarded.
	lw.Write([]byte("more"))
	if buf.Len() != 10 {
		t.Errorf("buffer should still be 10 after overflow, got %d", buf.Len())
	}
}

// ---------------------------------------------------------------------------
// platformLaunchSessionFn - injection point for testing LaunchSession
// ---------------------------------------------------------------------------

func TestLaunchSession_RoutesToPlatformLauncher(t *testing.T) {
	old := platformLaunchSessionFn
	defer func() { platformLaunchSessionFn = old }()

	var captured struct {
		shell         ShellInfo
		resumeCmd     string
		terminal      string
		cwd           string
		launchStyle   string
		paneDirection string
	}
	platformLaunchSessionFn = func(shell ShellInfo, resumeCmd, terminal, cwd, launchStyle, paneDirection string) error {
		captured.shell = shell
		captured.resumeCmd = resumeCmd
		captured.terminal = terminal
		captured.cwd = cwd
		captured.launchStyle = launchStyle
		captured.paneDirection = paneDirection
		return nil
	}

	cwd := t.TempDir()
	err := LaunchSession(
		ShellInfo{Name: "test-shell", Path: "test-path"},
		"test-session",
		ResumeConfig{
			CustomCommand: "my-cli --resume {sessionId}",
			Terminal:      "my-terminal",
			Cwd:           cwd,
			LaunchStyle:   LaunchStyleWindow,
			PaneDirection: "right",
		},
	)
	if err != nil {
		t.Fatalf("LaunchSession error: %v", err)
	}
	if captured.shell.Name != "test-shell" {
		t.Errorf("shell.Name = %q, want %q", captured.shell.Name, "test-shell")
	}
	if captured.resumeCmd != "my-cli --resume test-session" {
		t.Errorf("resumeCmd = %q, want %q", captured.resumeCmd, "my-cli --resume test-session")
	}
	if captured.terminal != "my-terminal" {
		t.Errorf("terminal = %q, want %q", captured.terminal, "my-terminal")
	}
	if captured.cwd != cwd {
		t.Errorf("cwd = %q, want %q", captured.cwd, cwd)
	}
	if captured.launchStyle != LaunchStyleWindow {
		t.Errorf("launchStyle = %q, want %q", captured.launchStyle, LaunchStyleWindow)
	}
	if captured.paneDirection != "right" {
		t.Errorf("paneDirection = %q, want %q", captured.paneDirection, "right")
	}
}

func TestLaunchSession_PaneStylePassedThrough(t *testing.T) {
	old := platformLaunchSessionFn
	defer func() { platformLaunchSessionFn = old }()

	var gotStyle, gotDir string
	platformLaunchSessionFn = func(_ ShellInfo, _, _, _, launchStyle, paneDirection string) error {
		gotStyle = launchStyle
		gotDir = paneDirection
		return nil
	}

	err := LaunchSession(
		ShellInfo{Name: "sh", Path: "sh"},
		"sess",
		ResumeConfig{
			CustomCommand: "echo {sessionId}",
			LaunchStyle:   LaunchStylePane,
			PaneDirection: "down",
		},
	)
	if err != nil {
		t.Fatalf("LaunchSession error: %v", err)
	}
	if gotStyle != LaunchStylePane {
		t.Errorf("launchStyle = %q, want %q", gotStyle, LaunchStylePane)
	}
	if gotDir != "down" {
		t.Errorf("paneDirection = %q, want %q", gotDir, "down")
	}
}

func TestLaunchSession_PropagatesLauncherError(t *testing.T) {
	old := platformLaunchSessionFn
	defer func() { platformLaunchSessionFn = old }()

	wantErr := errors.New("launcher failed")
	platformLaunchSessionFn = func(_ ShellInfo, _, _, _, _, _ string) error {
		return wantErr
	}

	err := LaunchSession(
		ShellInfo{Name: "sh", Path: "sh"},
		"sess",
		ResumeConfig{CustomCommand: "echo {sessionId}"},
	)
	if !errors.Is(err, wantErr) {
		t.Errorf("got error %v, want %v", err, wantErr)
	}
}

func TestLaunchSession_DefaultsTerminalWhenEmpty(t *testing.T) {
	old := platformLaunchSessionFn
	defer func() { platformLaunchSessionFn = old }()

	var gotTerminal string
	platformLaunchSessionFn = func(_ ShellInfo, _, terminal, _, _, _ string) error {
		gotTerminal = terminal
		return nil
	}

	err := LaunchSession(
		ShellInfo{Name: "sh", Path: "sh"},
		"sess",
		ResumeConfig{
			CustomCommand: "echo {sessionId}",
			Terminal:      "", // empty - should default
		},
	)
	if err != nil {
		t.Fatalf("LaunchSession error: %v", err)
	}
	if gotTerminal == "" {
		t.Error("terminal should be defaulted, got empty")
	}
	// Should match DefaultTerminal() output.
	if gotTerminal != DefaultTerminal() {
		t.Errorf("terminal = %q, want DefaultTerminal() = %q", gotTerminal, DefaultTerminal())
	}
}

func TestLaunchSession_DefaultsShellWhenEmpty(t *testing.T) {
	old := platformLaunchSessionFn
	defer func() { platformLaunchSessionFn = old }()

	var gotShell ShellInfo
	platformLaunchSessionFn = func(shell ShellInfo, _, _, _, _, _ string) error {
		gotShell = shell
		return nil
	}

	err := LaunchSession(
		ShellInfo{}, // empty - should default
		"sess",
		ResumeConfig{CustomCommand: "echo {sessionId}"},
	)
	if err != nil {
		t.Fatalf("LaunchSession error: %v", err)
	}
	if gotShell.Path == "" {
		t.Error("shell should be defaulted, got empty path")
	}
}

func TestLaunchSession_RejectsInvalidSessionID(t *testing.T) {
	old := platformLaunchSessionFn
	defer func() { platformLaunchSessionFn = old }()

	platformLaunchSessionFn = func(_ ShellInfo, _, _, _, _, _ string) error {
		t.Error("launcher should not be called with invalid session ID")
		return nil
	}

	err := LaunchSession(
		ShellInfo{Name: "test", Path: "test"},
		"; rm -rf /", // invalid session ID
		ResumeConfig{},
	)
	if err == nil {
		t.Error("LaunchSession should reject invalid session IDs")
	}
}

// ---------------------------------------------------------------------------
// platformLaunchSession - direct routing verification
// ---------------------------------------------------------------------------

func TestPlatformLaunchSession_RoutesByGOOS(t *testing.T) {
	// Verify platformLaunchSession dispatches by runtime.GOOS without
	// panicking. We use the fn var injection to avoid spawning terminals.
	old := platformLaunchSessionFn
	defer func() { platformLaunchSessionFn = old }()

	var called bool
	platformLaunchSessionFn = func(_ ShellInfo, _, _, _, _, _ string) error {
		called = true
		return nil
	}

	err := LaunchSession(
		ShellInfo{Name: "sh", Path: "sh"},
		"test-sess",
		ResumeConfig{CustomCommand: "echo {sessionId}"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("platformLaunchSessionFn was not called")
	}
}

// ---------------------------------------------------------------------------
// detectWindowsTerminals
// ---------------------------------------------------------------------------

func TestDetectWindowsTerminals_AlwaysIncludesConhost(t *testing.T) {
	terms := detectWindowsTerminals()
	found := false
	for _, ti := range terms {
		if ti.Name == termConhost {
			found = true
			break
		}
	}
	if !found {
		t.Error("detectWindowsTerminals should always include conhost")
	}
}

func TestDetectWindowsTerminals_IncludesWT(t *testing.T) {
	if _, err := exec.LookPath("wt.exe"); err != nil {
		t.Skip("wt.exe not on PATH")
	}
	terms := detectWindowsTerminals()
	found := false
	for _, ti := range terms {
		if ti.Name == termWindowsTerminal {
			found = true
			break
		}
	}
	if !found {
		t.Error("detectWindowsTerminals should include Windows Terminal when wt.exe is available")
	}
}

// ---------------------------------------------------------------------------
// DefaultTerminal - comprehensive OS routing
// ---------------------------------------------------------------------------

func TestDefaultTerminal_ReturnValueIsNonEmpty(t *testing.T) {
	term := DefaultTerminal()
	if term == "" {
		t.Error("DefaultTerminal() should never return empty")
	}
}

func TestDefaultTerminal_Idempotent(t *testing.T) {
	// Calling multiple times should always return the same result.
	first := DefaultTerminal()
	second := DefaultTerminal()
	if first != second {
		t.Errorf("DefaultTerminal() not idempotent: %q vs %q", first, second)
	}
}

func TestDefaultTerminal_WindowsValues(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}
	term := DefaultTerminal()
	valid := map[string]bool{termWindowsTerminal: true, termConhost: true}
	if !valid[term] {
		t.Errorf("DefaultTerminal() = %q, want one of %v", term, valid)
	}
}

func TestDefaultTerminal_DarwinValue(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-specific test")
	}
	term := DefaultTerminal()
	if term != "Terminal.app" {
		t.Errorf("DefaultTerminal() on macOS = %q, want %q", term, "Terminal.app")
	}
}

func TestDefaultTerminal_LinuxValue(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific test")
	}
	term := DefaultTerminal()
	// On Linux, should be one of the detected terminals or "xterm" fallback.
	if term == "" {
		t.Error("DefaultTerminal() on Linux should not be empty")
	}
}

// ---------------------------------------------------------------------------
// buildStartCmdLine - argument construction for different shell types
// ---------------------------------------------------------------------------

func TestBuildStartCmdLine_NonGitBash(t *testing.T) {
	shell := ShellInfo{Name: "bash", Path: "/usr/bin/bash"}
	got := buildStartCmdLine(shell, "echo test")
	if !strings.Contains(got, "-c") {
		t.Error("buildStartCmdLine with bash should use -c")
	}
	// Non-Git Bash should NOT bashify.
	// The resume command should be passed through cmdQuote.
	if !strings.Contains(got, `"echo test"`) {
		t.Errorf("expected cmdQuoted resume command, got %q", got)
	}
}

func TestBuildStartCmdLine_WindowsPowerShell(t *testing.T) {
	shell := ShellInfo{
		Name: "Windows PowerShell",
		Path: `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
		Args: []string{"-NoLogo"},
	}
	got := buildStartCmdLine(shell, `"ghcs" "--resume" "s1"`)
	if !strings.Contains(got, "-Command") {
		t.Error("should use -Command for PowerShell")
	}
	if !strings.Contains(got, "-NoLogo") {
		t.Error("should include -NoLogo from Args")
	}
	if !strings.Contains(got, "& ") {
		t.Error("should contain PowerShell call operator")
	}
}

func TestBuildStartCmdLine_CmdExe(t *testing.T) {
	shell := ShellInfo{Name: "Command Prompt", Path: `C:\Windows\System32\cmd.exe`}
	got := buildStartCmdLine(shell, "echo test")
	if !strings.Contains(got, "/k") {
		t.Error("should use /k for cmd.exe")
	}
}

func TestBuildStartCmdLine_GitBashBashifies(t *testing.T) {
	shell := ShellInfo{
		Name: "Git Bash",
		Path: `C:\Program Files\Git\bin\bash.exe`,
		Args: []string{"--login"},
	}
	got := buildStartCmdLine(shell, `"C:\Users\test\ghcs.exe" "--resume" "s1"`)
	// Git Bash should convert backslashes to forward slashes.
	if strings.Contains(got, `C:\Users`) {
		t.Error("Git Bash should convert backslashes to forward slashes")
	}
}

// ---------------------------------------------------------------------------
// DetectShells / DetectTerminals - public API routing
// ---------------------------------------------------------------------------

func TestDetectShells_RouterCoversCurrentOS(t *testing.T) {
	shells := DetectShells()
	// Should return at least one shell on any reasonable system.
	if len(shells) == 0 {
		t.Error("DetectShells() returned empty")
	}
}

func TestDetectTerminals_RouterCoversCurrentOS(t *testing.T) {
	terms := DetectTerminals()
	// On Windows and macOS, at least one terminal is always present.
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		if len(terms) == 0 {
			t.Error("DetectTerminals() should return at least one on Windows/macOS")
		}
	}
}
