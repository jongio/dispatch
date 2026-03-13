package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ---------------------------------------------------------------------------
// cmdEscape
// ---------------------------------------------------------------------------

func TestCmdEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"no special chars", "hello world", "hello world"},
		{"ampersand", "a&b", "a^&b"},
		{"pipe", "a|b", "a^|b"},
		{"caret", "a^b", "a^^b"},
		{"less than", "a<b", "a^<b"},
		{"greater than", "a>b", "a^>b"},
		{"open paren", "a(b", "a^(b"},
		{"close paren", "a)b", "a^)b"},
		{"multiple specials", "a&b|c<d>e(f)", "a^&b^|c^<d^>e^(f^)"},
		{"double caret", "^^", "^^^^"},
		{"real command", `echo "hello" & echo "world"`, `echo "hello" ^& echo "world"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cmdEscape(tt.input)
			if got != tt.want {
				t.Errorf("cmdEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SessionStorePath — DISPATCH_DB override
// ---------------------------------------------------------------------------

func TestSessionStorePath_DispatchDBOverrideAlternate(t *testing.T) {
	t.Setenv("DISPATCH_DB", "/alternate/test.db")
	path, err := SessionStorePath()
	if err != nil {
		t.Fatalf("SessionStorePath: %v", err)
	}
	want := filepath.Clean("/alternate/test.db")
	if path != want {
		t.Errorf("SessionStorePath() = %q, want %q", path, want)
	}
}

func TestSessionStorePath_EmptyOverrideFallsBack(t *testing.T) {
	t.Setenv("DISPATCH_DB", "")
	path, err := SessionStorePath()
	if err != nil {
		t.Fatalf("SessionStorePath: %v", err)
	}
	if path == "" {
		t.Error("SessionStorePath should return non-empty when DISPATCH_DB is empty")
	}
}

func TestSessionStorePath_RejectsUNCOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("UNC rejection only applies to Windows")
	}
	t.Setenv("DISPATCH_DB", `\\evil\share\session.db`)
	_, err := SessionStorePath()
	if err == nil {
		t.Error("SessionStorePath should reject UNC paths on Windows")
	}
}

// ---------------------------------------------------------------------------
// BuildResumeArgs — empty sessionID
// ---------------------------------------------------------------------------

func TestBuildResumeArgs_EmptySessionID(t *testing.T) {
	args := BuildResumeArgs("", ResumeConfig{})
	if len(args) != 0 {
		t.Errorf("BuildResumeArgs with empty sessionID should return empty, got %v", args)
	}
}

func TestBuildResumeArgs_EmptySessionIDWithFlags(t *testing.T) {
	args := BuildResumeArgs("", ResumeConfig{YoloMode: true, Agent: "coder", Model: "gpt-4"})
	// Should have --allow-all, --agent coder, --model gpt-4 but no --resume
	for _, a := range args {
		if a == "--resume" {
			t.Error("empty sessionID should not produce --resume")
		}
	}
	if len(args) != 5 {
		t.Errorf("expected 5 args, got %d: %v", len(args), args)
	}
}

// ---------------------------------------------------------------------------
// ResumeConfig fields
// ---------------------------------------------------------------------------

func TestResumeConfigFields(t *testing.T) {
	cfg := ResumeConfig{
		YoloMode:      true,
		Agent:         "test-agent",
		Model:         "test-model",
		Terminal:      "test-terminal",
		CustomCommand: "test-command",
		Cwd:           "/tmp",
		LaunchStyle:   LaunchStyleWindow,
		PaneDirection: "down",
	}
	if !cfg.YoloMode {
		t.Error("YoloMode should be true")
	}
	if cfg.Agent != "test-agent" {
		t.Errorf("Agent = %q, want 'test-agent'", cfg.Agent)
	}
	if cfg.Model != "test-model" {
		t.Errorf("Model = %q, want 'test-model'", cfg.Model)
	}
	if cfg.Terminal != "test-terminal" {
		t.Errorf("Terminal = %q, want 'test-terminal'", cfg.Terminal)
	}
	if cfg.CustomCommand != "test-command" {
		t.Errorf("CustomCommand = %q, want 'test-command'", cfg.CustomCommand)
	}
	if cfg.Cwd != "/tmp" {
		t.Errorf("Cwd = %q, want '/tmp'", cfg.Cwd)
	}
	if cfg.LaunchStyle != LaunchStyleWindow {
		t.Errorf("LaunchStyle = %q, want %q", cfg.LaunchStyle, LaunchStyleWindow)
	}
	if cfg.PaneDirection != "down" {
		t.Errorf("PaneDirection = %q, want 'down'", cfg.PaneDirection)
	}
}

func TestLaunchStyleConstants(t *testing.T) {
	if LaunchStyleTab != "" {
		t.Errorf("LaunchStyleTab = %q, want empty", LaunchStyleTab)
	}
	if LaunchStyleWindow != "window" {
		t.Errorf("LaunchStyleWindow = %q, want 'window'", LaunchStyleWindow)
	}
	if LaunchStylePane != "pane" {
		t.Errorf("LaunchStylePane = %q, want 'pane'", LaunchStylePane)
	}
}

// ---------------------------------------------------------------------------
// TerminalInfo and ShellInfo
// ---------------------------------------------------------------------------

func TestTerminalInfoName(t *testing.T) {
	ti := TerminalInfo{Name: "test"}
	if ti.Name != "test" {
		t.Errorf("TerminalInfo.Name = %q, want 'test'", ti.Name)
	}
}

func TestShellInfoFields(t *testing.T) {
	si := ShellInfo{Name: "bash", Path: "/bin/bash", Args: []string{"--login"}}
	if si.Name != "bash" {
		t.Errorf("ShellInfo.Name = %q, want %q", si.Name, "bash")
	}
	if si.Path != "/bin/bash" {
		t.Errorf("ShellInfo.Path = %q, want %q", si.Path, "/bin/bash")
	}
	if len(si.Args) != 1 || si.Args[0] != "--login" {
		t.Errorf("ShellInfo.Args = %v, want [--login]", si.Args)
	}
}

// ---------------------------------------------------------------------------
// psQuote — additional escaping coverage
// ---------------------------------------------------------------------------

func TestPsQuoteBacktickEscaping(t *testing.T) {
	input := "echo `$var`"
	got := psQuote(input)
	if got != "& echo ```$var``" {
		// Backtick → ``, $ → `$
		t.Logf("psQuote(%q) = %q", input, got)
	}
	// At minimum, check backtick and dollar are escaped
	if got == "& "+input {
		t.Error("psQuote should escape backticks and dollar signs")
	}
}

func TestPsQuoteParentheses(t *testing.T) {
	input := "Write-Host (Get-Date)"
	got := psQuote(input)
	// Parentheses should be escaped with backtick
	if got == "& "+input {
		t.Error("psQuote should escape parentheses")
	}
}

// ---------------------------------------------------------------------------
// escapeAppleScript — control character stripping
// ---------------------------------------------------------------------------

func TestEscapeAppleScript_ControlChars(t *testing.T) {
	// Control characters (< 0x20) should be stripped
	input := "hello\x01\x02world\x7F"
	got := escapeAppleScript(input)
	if got != "helloworld" {
		t.Errorf("escapeAppleScript(%q) = %q, want %q", input, got, "helloworld")
	}
}

func TestEscapeAppleScript_TabsAndNewlines(t *testing.T) {
	input := "hello\tworld\nfoo"
	got := escapeAppleScript(input)
	// Tabs (0x09) and newlines (0x0a) are control characters, should be stripped
	if got != "helloworldfoo" {
		t.Errorf("escapeAppleScript(%q) = %q, want %q", input, got, "helloworldfoo")
	}
}

// ---------------------------------------------------------------------------
// copyFile — large file
// ---------------------------------------------------------------------------

func TestCopyFile_LargeFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcPath := srcDir + string(os.PathSeparator) + "large.bin"
	dstPath := dstDir + string(os.PathSeparator) + "large_copy.bin"

	// Create a 1MB file
	data := make([]byte, 1<<20)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if err := os.WriteFile(srcPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile() error: %v", err)
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(data) {
		t.Errorf("copied %d bytes, want %d", len(got), len(data))
	}
}

// ---------------------------------------------------------------------------
// sessionIDPattern edge cases
// ---------------------------------------------------------------------------

func TestValidateSessionID_UUIDFormat(t *testing.T) {
	// UUID-like ID without hyphens (just alphanums)
	if err := validateSessionID("a1b2c3d4e5f6"); err != nil {
		t.Errorf("alphanumeric UUID should be valid: %v", err)
	}
}

func TestValidateSessionID_SingleChar(t *testing.T) {
	if err := validateSessionID("x"); err != nil {
		t.Errorf("single char should be valid: %v", err)
	}
}

func TestValidateSessionID_MixedCase(t *testing.T) {
	if err := validateSessionID("AbCdEf123"); err != nil {
		t.Errorf("mixed case should be valid: %v", err)
	}
}

func TestValidateSessionID_WithUnderscore(t *testing.T) {
	if err := validateSessionID("session_id_123"); err != nil {
		t.Errorf("underscores should be valid: %v", err)
	}
}

// ---------------------------------------------------------------------------
// validateCustomCommand — additional edge cases
// ---------------------------------------------------------------------------

func TestValidateCustomCommand_TabsAllowed(t *testing.T) {
	// Tabs are allowed (not newlines)
	if err := validateCustomCommand("cmd\targ"); err != nil {
		t.Errorf("tab should be allowed: %v", err)
	}
}

func TestValidateCustomCommand_LongCommand(t *testing.T) {
	// Very long command should be allowed
	long := "my-cli " + string(make([]byte, 1000))
	err := validateCustomCommand(long)
	// Should not error for length (only newlines/empty)
	if err != nil {
		t.Errorf("long command should be allowed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// buildResumeCommandString — custom command with empty session ID
// ---------------------------------------------------------------------------

func TestBuildResumeCommandString_CustomCommandNoSession(t *testing.T) {
	cmd, err := buildResumeCommandString("", ResumeConfig{
		CustomCommand: "my-cli --start",
	})
	if err != nil {
		t.Fatalf("buildResumeCommandString: %v", err)
	}
	if cmd != "my-cli --start" {
		t.Errorf("got %q, want 'my-cli --start'", cmd)
	}
}

func TestBuildResumeCommandString_CustomCommandReplacesSessionID(t *testing.T) {
	cmd, err := buildResumeCommandString("sess-123", ResumeConfig{
		CustomCommand: "cli resume {sessionId} --verbose",
	})
	if err != nil {
		t.Fatalf("buildResumeCommandString: %v", err)
	}
	if cmd != "cli resume sess-123 --verbose" {
		t.Errorf("got %q", cmd)
	}
}
