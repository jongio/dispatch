package platform

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// DefaultTerminal
// ---------------------------------------------------------------------------

func TestDefaultTerminalReturnsNonEmpty(t *testing.T) {
	term := DefaultTerminal()
	if term == "" {
		t.Fatal("DefaultTerminal() returned empty string")
	}
}

func TestDefaultTerminalReturnsExpectedPerOS(t *testing.T) {
	term := DefaultTerminal()
	switch runtime.GOOS {
	case "windows":
		// Must be either "Windows Terminal" or "conhost".
		if term != "Windows Terminal" && term != "conhost" {
			t.Errorf("on Windows, DefaultTerminal() = %q; want %q or %q", term, "Windows Terminal", "conhost")
		}
	case "darwin":
		if term != "Terminal.app" {
			t.Errorf("on macOS, DefaultTerminal() = %q; want %q", term, "Terminal.app")
		}
	default:
		// On Linux we accept any non-empty value (first detected or "xterm").
		// Just verify it's not empty (covered by TestDefaultTerminalReturnsNonEmpty).
	}
}

// ---------------------------------------------------------------------------
// DefaultShell
// ---------------------------------------------------------------------------

func TestDefaultShellReturnsValidShellInfo(t *testing.T) {
	sh := DefaultShell()
	if sh.Name == "" {
		t.Error("DefaultShell().Name is empty")
	}
	if sh.Path == "" {
		t.Error("DefaultShell().Path is empty")
	}
}

// ---------------------------------------------------------------------------
// DetectTerminals / DetectShells
// ---------------------------------------------------------------------------

func TestDetectTerminalsNonEmpty(t *testing.T) {
	terms := DetectTerminals()
	// On Windows and macOS at least one terminal is always present.
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		if len(terms) == 0 {
			t.Fatal("DetectTerminals() returned no terminals")
		}
	}
}

func TestDetectShellsNonEmpty(t *testing.T) {
	shells := DetectShells()
	if len(shells) == 0 {
		t.Fatal("DetectShells() returned no shells")
	}
}

// ---------------------------------------------------------------------------
// BuildResumeArgs
// ---------------------------------------------------------------------------

func TestBuildResumeArgs(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		cfg       ResumeConfig
		want      []string
	}{
		{
			name:      "basic resume",
			sessionID: "abc123",
			cfg:       ResumeConfig{},
			want:      []string{"--resume", "abc123"},
		},
		{
			name:      "with yolo mode",
			sessionID: "abc123",
			cfg:       ResumeConfig{YoloMode: true},
			want:      []string{"--resume", "abc123", "--allow-all"},
		},
		{
			name:      "with agent",
			sessionID: "abc123",
			cfg:       ResumeConfig{Agent: "coder"},
			want:      []string{"--resume", "abc123", "--agent", "coder"},
		},
		{
			name:      "with model",
			sessionID: "abc123",
			cfg:       ResumeConfig{Model: "gpt-4"},
			want:      []string{"--resume", "abc123", "--model", "gpt-4"},
		},
		{
			name:      "with all flags",
			sessionID: "abc123",
			cfg:       ResumeConfig{YoloMode: true, Agent: "coder", Model: "gpt-4"},
			want:      []string{"--resume", "abc123", "--allow-all", "--agent", "coder", "--model", "gpt-4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildResumeArgs(tt.sessionID, tt.cfg)
			if len(got) != len(tt.want) {
				t.Fatalf("BuildResumeArgs() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("BuildResumeArgs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// shellQuote
// ---------------------------------------------------------------------------

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "''"},
		{"simple", "simple"},
		{"path with spaces", "'path with spaces'"},
		{`path"with"quotes`, `'path"with"quotes'`},
		{"no-special-chars", "no-special-chars"},
		{"has;semicolon", "'has;semicolon'"},
		{"has|pipe", "'has|pipe'"},
		{"it's quoted", "'it'\\''s quoted'"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shellQuote(tt.input)
			if got != tt.want {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// cmdQuote
// ---------------------------------------------------------------------------

func TestCmdQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", `""`},
		{"simple", `"simple"`},
		{`C:\Windows\system32\cmd.exe`, `"C:\Windows\system32\cmd.exe"`},
		{`C:\Program Files\PowerShell\7\pwsh.exe`, `"C:\Program Files\PowerShell\7\pwsh.exe"`},
		{`path with "quotes"`, `"path with \"quotes\""`},
		{"no-special", `"no-special"`},
		{`%PATH%`, `"%%PATH%%"`},
		{`%USERPROFILE%\bin`, `"%%USERPROFILE%%\bin"`},
		{`100%`, `"100%%"`},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cmdQuote(tt.input)
			if got != tt.want {
				t.Errorf("cmdQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCmdQuoteStripsNullBytes(t *testing.T) {
	got := cmdQuote("abc\x00def")
	want := `"abcdef"`
	if got != want {
		t.Errorf("cmdQuote with null byte = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// psQuote
// ---------------------------------------------------------------------------

func TestPsQuote(t *testing.T) {
	got := psQuote(`"C:\Program Files\copilot.exe" --resume abc123`)
	want := `& 'C:\Program Files\copilot.exe' --resume abc123`
	if got != want {
		t.Errorf("psQuote() = %q, want %q", got, want)
	}
}

func TestPsQuoteNoDoubleQuotes(t *testing.T) {
	got := psQuote("simple-path --flag value")
	want := "& simple-path --flag value"
	if got != want {
		t.Errorf("psQuote() = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// validateSessionID
// ---------------------------------------------------------------------------

func TestValidateSessionID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid simple", "abc123", false},
		{"valid with dashes", "sess-abc-123", false},
		{"valid with dots", "sess.2024.01", false},
		{"valid with underscores", "A_B_C", false},
		{"valid single char", "a", false},
		{"valid max length", string(make([]byte, 128)), true}, // depends on content
		{"empty string", "", true},
		{"spaces", "abc def", true},
		{"shell semicolon", "abc;rm", true},
		{"shell pipe", "abc|cat", true},
		{"shell ampersand", "abc&&cat", true},
		{"dollar sign", "$(whoami)", true},
		{"backtick", "`id`", true},
		{"newline", "abc\ndef", true},
		{"null byte", "abc\x00def", true},
		{"slash", "abc/def", true},
		{"backslash", `abc\def`, true},
		{"double quote", `abc"def`, true},
		{"single quote", "abc'def", true},
		{"starts with dash", "-abc", true},
		{"starts with dot", ".abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSessionID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSessionID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSessionID_BoundaryLengths(t *testing.T) {
	// 128 characters should be valid
	validID := make([]byte, 128)
	for i := range validID {
		validID[i] = 'a'
	}
	if err := validateSessionID(string(validID)); err != nil {
		t.Errorf("128-char ID should be valid: %v", err)
	}

	// 129 characters should be invalid
	invalidID := make([]byte, 129)
	for i := range invalidID {
		invalidID[i] = 'a'
	}
	if err := validateSessionID(string(invalidID)); err == nil {
		t.Error("129-char ID should be invalid")
	}
}

// ---------------------------------------------------------------------------
// escapeAppleScript
// ---------------------------------------------------------------------------

func TestEscapeAppleScript(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"no special chars", "hello world", "hello world"},
		{"backslash", `path\to\file`, `path\\to\\file`},
		{"double quotes", `say "hello"`, `say \"hello\"`},
		{"single quotes", "it's a test", `it'\''s a test`},
		{"mixed", `say "hello" to O'Brien at C:\path`, `say \"hello\" to O'\''Brien at C:\\path`},
		{"multiple backslashes", `\\server\share`, `\\\\server\\share`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeAppleScript(tt.input)
			if got != tt.want {
				t.Errorf("escapeAppleScript(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildCustomCmd
// ---------------------------------------------------------------------------

func TestBuildCustomCmd(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		template  string
		wantArgs  int // expected number of args (including program name)
		wantErr   bool
	}{
		{
			name:      "simple replacement",
			sessionID: "abc-123",
			template:  "my-cli --resume {sessionId}",
			wantArgs:  3,
			wantErr:   false,
		},
		{
			name:      "multiple placeholders",
			sessionID: "abc-123",
			template:  "cli {sessionId} --id {sessionId}",
			wantArgs:  4,
			wantErr:   false,
		},
		{
			name:      "no placeholder",
			sessionID: "abc-123",
			template:  "my-cli --start",
			wantArgs:  2,
			wantErr:   false,
		},
		{
			name:      "empty after expansion",
			sessionID: "abc",
			template:  "   ",
			wantArgs:  0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := buildCustomCmd(tt.sessionID, tt.template)
			if (err != nil) != tt.wantErr {
				t.Fatalf("buildCustomCmd error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && len(cmd.Args) != tt.wantArgs {
				t.Errorf("args count = %d, want %d; args = %v", len(cmd.Args), tt.wantArgs, cmd.Args)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateCustomCommand
// ---------------------------------------------------------------------------

func TestValidateCustomCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		wantErr bool
	}{
		{"valid simple command", "my-cli --resume {sessionId}", false},
		{"valid no placeholder", "echo hello", false},
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"tab only", "\t", true},
		{"contains newline", "cmd\n--flag", true},
		{"contains carriage return", "cmd\r--flag", true},
		{"contains CRLF", "cmd\r\n--flag", true},
		{"newline at end", "cmd --flag\n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCustomCommand(tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCustomCommand(%q) error = %v, wantErr %v", tt.cmd, err, tt.wantErr)
			}
		})
	}
}

func TestBuildCustomCmd_RejectsNewlines(t *testing.T) {
	_, err := buildCustomCmd("abc-123", "my-cli\n--evil-flag")
	if err == nil {
		t.Fatal("buildCustomCmd should reject commands with embedded newlines")
	}
}

func TestBuildResumeCommandString_RejectsNewlinesInCustomCommand(t *testing.T) {
	_, err := buildResumeCommandString("valid-session", ResumeConfig{
		CustomCommand: "my-cli\n--evil-flag",
	})
	if err == nil {
		t.Fatal("buildResumeCommandString should reject custom commands with embedded newlines")
	}
}

// ---------------------------------------------------------------------------
// buildResumeCommandString — custom command path
// ---------------------------------------------------------------------------

func TestBuildResumeCommandString_CustomCommand(t *testing.T) {
	cmd, err := buildResumeCommandString("test-session", ResumeConfig{
		CustomCommand: "my-cli --resume {sessionId} --flag",
	})
	if err != nil {
		t.Fatalf("buildResumeCommandString: %v", err)
	}
	if cmd != "my-cli --resume test-session --flag" {
		t.Errorf("got %q, want 'my-cli --resume test-session --flag'", cmd)
	}
}

func TestBuildResumeCommandString_CustomCommandEmpty(t *testing.T) {
	_, err := buildResumeCommandString("test-session", ResumeConfig{
		CustomCommand: "   ",
	})
	if err == nil {
		t.Fatal("should error on empty custom command")
	}
}

func TestBuildResumeCommandString_InvalidSessionID(t *testing.T) {
	_, err := buildResumeCommandString("; rm -rf /", ResumeConfig{
		CustomCommand: "my-cli {sessionId}",
	})
	if err == nil {
		t.Fatal("should reject invalid session ID")
	}
}

// ---------------------------------------------------------------------------
// NewResumeCmd — custom command path
// ---------------------------------------------------------------------------

func TestNewResumeCmd_CustomCommand(t *testing.T) {
	cmd, err := NewResumeCmd("valid-session", ResumeConfig{
		CustomCommand: "echo {sessionId}",
	})
	if err != nil {
		t.Fatalf("NewResumeCmd: %v", err)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
}

func TestNewResumeCmd_EmptySessionIDStartsNewSession(t *testing.T) {
	if FindCLIBinary() == "" {
		t.Skip("Copilot CLI not installed")
	}
	cmd, err := NewResumeCmd("", ResumeConfig{})
	if err != nil {
		t.Fatalf("empty session ID should be allowed for new sessions: %v", err)
	}
	// Should not contain --resume flag.
	for _, arg := range cmd.Args {
		if arg == "--resume" {
			t.Fatal("empty session ID should not produce --resume flag")
		}
	}
}

func TestNewResumeCmd_CustomCommandEmptyTemplate(t *testing.T) {
	_, err := NewResumeCmd("valid-session", ResumeConfig{
		CustomCommand: "   ",
	})
	if err == nil {
		t.Fatal("should error on whitespace-only custom command")
	}
}

// ---------------------------------------------------------------------------
// resolvedCwd
// ---------------------------------------------------------------------------

func TestResolvedCwd_EmptyString(t *testing.T) {
	if got := resolvedCwd(""); got != "" {
		t.Errorf("resolvedCwd(\"\") = %q; want \"\"", got)
	}
}

func TestResolvedCwd_ExistingDir(t *testing.T) {
	dir := t.TempDir()
	if got := resolvedCwd(dir); got != dir {
		t.Errorf("resolvedCwd(%q) = %q; want %q", dir, got, dir)
	}
}

func TestResolvedCwd_NonexistentDir(t *testing.T) {
	if got := resolvedCwd("/no/such/path/here"); got != "" {
		t.Errorf("resolvedCwd returned %q for nonexistent path; want \"\"", got)
	}
}

func TestResolvedCwd_FileNotDir(t *testing.T) {
	f, err := os.CreateTemp("", "cwd-test")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	defer func() { _ = os.Remove(f.Name()) }()

	if got := resolvedCwd(f.Name()); got != "" {
		t.Errorf("resolvedCwd returned %q for a file; want \"\"", got)
	}
}

// ---------------------------------------------------------------------------
// NewResumeCmd with Cwd
// ---------------------------------------------------------------------------

func TestNewResumeCmd_SetsDirFromCwd(t *testing.T) {
	dir := t.TempDir()
	cmd, err := NewResumeCmd("valid-session", ResumeConfig{
		CustomCommand: "echo {sessionId}",
		Cwd:           dir,
	})
	if err != nil {
		t.Fatalf("NewResumeCmd: %v", err)
	}
	if cmd.Dir != dir {
		t.Errorf("cmd.Dir = %q; want %q", cmd.Dir, dir)
	}
}

func TestNewResumeCmd_IgnoresInvalidCwd(t *testing.T) {
	cmd, err := NewResumeCmd("valid-session", ResumeConfig{
		CustomCommand: "echo {sessionId}",
		Cwd:           "/no/such/path/here",
	})
	if err != nil {
		t.Fatalf("NewResumeCmd: %v", err)
	}
	if cmd.Dir != "" {
		t.Errorf("cmd.Dir = %q; want \"\" for invalid cwd", cmd.Dir)
	}
}

// ---------------------------------------------------------------------------
// FindCLIBinary
// ---------------------------------------------------------------------------

func TestFindCLIBinary_ReturnsStringOrEmpty(t *testing.T) {
	// FindCLIBinary should either find a binary or return empty.
	// We can't guarantee ghcs/copilot is installed, but we verify
	// it doesn't panic and returns a valid result.
	result := FindCLIBinary()
	// Result is either empty or a path.
	if result != "" {
		// If found, it should be an absolute path or at least non-empty.
		t.Logf("FindCLIBinary found: %s", result)
	}
}

// ---------------------------------------------------------------------------
// DetectShells — ensure returned shells have valid fields
// ---------------------------------------------------------------------------

func TestDetectShellsValidFields(t *testing.T) {
	shells := DetectShells()
	for _, sh := range shells {
		if sh.Name == "" {
			t.Error("shell Name should not be empty")
		}
		if sh.Path == "" {
			t.Error("shell Path should not be empty")
		}
	}
}

// ---------------------------------------------------------------------------
// DetectTerminals — validate structure
// ---------------------------------------------------------------------------

func TestDetectTerminalsValidFields(t *testing.T) {
	terms := DetectTerminals()
	for _, term := range terms {
		if term.Name == "" {
			t.Error("terminal Name should not be empty")
		}
	}
}

// ---------------------------------------------------------------------------
// DefaultShell — validate on Windows
// ---------------------------------------------------------------------------

func TestDefaultShellHasPath(t *testing.T) {
	sh := DefaultShell()
	if sh.Path == "" {
		t.Error("DefaultShell should always have a non-empty Path")
	}
	if sh.Name == "" {
		t.Error("DefaultShell should always have a non-empty Name")
	}
}

// ---------------------------------------------------------------------------
// shellQuote — additional edge cases
// ---------------------------------------------------------------------------

func TestShellQuoteAllMetachars(t *testing.T) {
	// Test every metacharacter individually
	metachars := ` 	` + "\n\r" + `"'` + "`$\\!;|&<>(){}"
	for _, ch := range metachars {
		input := "a" + string(ch) + "b"
		got := shellQuote(input)
		if got[0] != '\'' {
			t.Errorf("shellQuote(%q) = %q, expected single-quoted (metachar %q)", input, got, string(ch))
		}
	}
}

func TestShellQuotePreservesContent(t *testing.T) {
	input := `path with "quotes" and spaces`
	got := shellQuote(input)
	// Should be wrapped in single quotes with content intact (double quotes are literal in single quotes)
	if got != `'path with "quotes" and spaces'` {
		t.Errorf("shellQuote(%q) = %q", input, got)
	}
}

// ---------------------------------------------------------------------------
// psQuote — additional tests
// ---------------------------------------------------------------------------

func TestPsQuoteMultipleDoubleQuotes(t *testing.T) {
	input := `"one" "two" "three"`
	got := psQuote(input)
	want := `& 'one' 'two' 'three'`
	if got != want {
		t.Errorf("psQuote(%q) = %q, want %q", input, got, want)
	}
}

func TestPsQuoteEmptyInput(t *testing.T) {
	got := psQuote("")
	want := "& "
	if got != want {
		t.Errorf("psQuote(%q) = %q, want %q", "", got, want)
	}
}

func TestPsQuoteEscapesMetachars(t *testing.T) {
	// PowerShell metacharacters must be escaped with backtick.
	input := `$(Start-Process calc); echo "done" | Out-File`
	got := psQuote(input)
	// $ → `$, ( → `(, ) → `), ; → `;, | → `|, " → '
	if !strings.Contains(got, "`$") {
		t.Errorf("psQuote should escape $, got %q", got)
	}
	if !strings.Contains(got, "`;") {
		t.Errorf("psQuote should escape ;, got %q", got)
	}
	if !strings.Contains(got, "`|") {
		t.Errorf("psQuote should escape |, got %q", got)
	}
	if strings.Contains(got, `"`) {
		t.Errorf("psQuote should replace \" with ', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// LaunchSession — validation
// ---------------------------------------------------------------------------

func TestLaunchSession_EmptyShellPath_AfterDefault(t *testing.T) {
	// Stub the platform launcher so no real terminal is spawned (issue #28).
	old := platformLaunchSessionFn
	defer func() { platformLaunchSessionFn = old }()

	var gotShell ShellInfo
	var gotResumeCmd string
	platformLaunchSessionFn = func(shell ShellInfo, resumeCmd string, _ string, _ string, _ string, _ string) error {
		gotShell = shell
		gotResumeCmd = resumeCmd
		return nil
	}

	err := LaunchSession(ShellInfo{}, "test-session-id", ResumeConfig{})
	// On systems without ghcs/copilot CLI, buildResumeCommandString returns
	// an error. That is fine — the important thing is no process was spawned.
	if err != nil {
		// If the error is about CLI not found, the test still proves no
		// zombie was created (the stub was never reached).
		if strings.Contains(err.Error(), "not found") {
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	// When the CLI binary IS found, verify argument construction.
	if gotShell.Path == "" {
		t.Error("expected non-empty shell path after defaulting")
	}
	if gotResumeCmd == "" {
		t.Error("expected non-empty resume command")
	}
	if !strings.Contains(gotResumeCmd, "test-session-id") {
		t.Errorf("resume command %q does not contain session ID", gotResumeCmd)
	}
}

func TestLaunchSession_DefaultShell_AlwaysHasPath(t *testing.T) {
	sh := DefaultShell()
	if sh.Path == "" {
		t.Error("DefaultShell() returned empty Path — launch would fail")
	}
	if sh.Name == "" {
		t.Error("DefaultShell() returned empty Name — display would be unclear")
	}
}

// ---------------------------------------------------------------------------
// startAndWaitBriefly
// ---------------------------------------------------------------------------

// TestHelperProcess is a test helper process used by startAndWaitBriefly tests.
// It is invoked as a subprocess by the test and exits based on env vars.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_TEST_HELPER_PROCESS") != "1" {
		return
	}
	switch os.Getenv("GO_TEST_HELPER_MODE") {
	case "exit0":
		os.Exit(0)
	case "exit1":
		fmt.Fprintln(os.Stderr, "helper: something went wrong")
		os.Exit(1)
	case "exit1_escape":
		fmt.Fprintln(os.Stderr, "error\x1b[2Jhidden\x07bell")
		os.Exit(1)
	default:
		os.Exit(0)
	}
}

func helperCmd(mode string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	cmd.Env = append(os.Environ(),
		"GO_TEST_HELPER_PROCESS=1",
		"GO_TEST_HELPER_MODE="+mode,
	)
	return cmd
}

func TestStartAndWaitBriefly_SuccessfulExit(t *testing.T) {
	err := startAndWaitBriefly(helperCmd("exit0"))
	if err != nil {
		t.Errorf("expected nil error for exit 0, got %v", err)
	}
}

func TestStartAndWaitBriefly_ImmediateFailure(t *testing.T) {
	err := startAndWaitBriefly(helperCmd("exit1"))
	if err == nil {
		t.Fatal("expected error for exit 1, got nil")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("expected stderr in error message, got %q", err.Error())
	}
}

func TestStartAndWaitBriefly_StderrEscapeSequencesStripped(t *testing.T) {
	err := startAndWaitBriefly(helperCmd("exit1_escape"))
	if err == nil {
		t.Fatal("expected error for exit 1, got nil")
	}
	msg := err.Error()
	if strings.Contains(msg, "\x1b") || strings.Contains(msg, "\x07") {
		t.Errorf("stderr contains unstripped control chars: %q", msg)
	}
	if !strings.Contains(msg, "error") {
		t.Errorf("expected error content in message, got %q", msg)
	}
}

func TestStartAndWaitBriefly_StartFailure(t *testing.T) {
	cmd := exec.Command("/nonexistent/binary/that/does/not/exist")
	err := startAndWaitBriefly(cmd)
	if err == nil {
		t.Fatal("expected error for nonexistent binary, got nil")
	}
}

func TestStartAndWaitBriefly_PresetStderrNotOverwritten(t *testing.T) {
	cmd := helperCmd("exit1")
	var custom bytes.Buffer
	cmd.Stderr = &custom
	err := startAndWaitBriefly(cmd)
	if err == nil {
		t.Fatal("expected error for exit 1, got nil")
	}
	// stderr output should go to the custom writer, not the internal one.
	if custom.Len() == 0 {
		t.Error("custom stderr writer should have received output")
	}
}

// ---------------------------------------------------------------------------
// limitedWriter
// ---------------------------------------------------------------------------

func TestLimitedWriter_CapsOutput(t *testing.T) {
	var buf bytes.Buffer
	lw := &limitedWriter{buf: &buf, max: 10}
	input := []byte("hello world, this is too long")
	n, err := lw.Write(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(input) {
		t.Errorf("Write should report full input length %d, got %d", len(input), n)
	}
	if buf.Len() != 10 {
		t.Errorf("buffer should be capped at 10 bytes, got %d", buf.Len())
	}
}

func TestLimitedWriter_ExactFit(t *testing.T) {
	var buf bytes.Buffer
	lw := &limitedWriter{buf: &buf, max: 5}
	lw.Write([]byte("hello"))
	if buf.String() != "hello" {
		t.Errorf("expected 'hello', got %q", buf.String())
	}
	// Further writes should be discarded.
	lw.Write([]byte(" world"))
	if buf.String() != "hello" {
		t.Errorf("expected 'hello' after overflow, got %q", buf.String())
	}
}

// ---------------------------------------------------------------------------
// sanitizeStderr
// ---------------------------------------------------------------------------

func TestSanitizeStderr_StripsControlChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean", "normal error", "normal error"},
		{"with newline and tab", "line1\nline2\ttab", "line1\nline2\ttab"},
		{"escape sequences", "err\x1b[2Jhidden", "err[2Jhidden"},
		{"bell", "alert\x07end", "alertend"},
		{"null", "has\x00null", "hasnull"},
		{"DEL", "has\x7Fdel", "hasdel"},
		{"carriage return", "over\rwrite", "overwrite"},
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
