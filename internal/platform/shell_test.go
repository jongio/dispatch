package platform

import (
	"os"
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
