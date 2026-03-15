//go:build windows

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// BuildResumeArgs — combinations
// ---------------------------------------------------------------------------

func TestCovBuildResumeArgs_AllCombinations(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		cfg       ResumeConfig
		wantLen   int
		noResume  bool
	}{
		{"empty/empty", "", ResumeConfig{}, 0, true},
		{"id only", "s1", ResumeConfig{}, 2, false},
		{"yolo only", "", ResumeConfig{YoloMode: true}, 1, true},
		{"agent only", "", ResumeConfig{Agent: "a"}, 2, true},
		{"model only", "", ResumeConfig{Model: "m"}, 2, true},
		{"id+yolo", "s1", ResumeConfig{YoloMode: true}, 3, false},
		{"id+agent", "s1", ResumeConfig{Agent: "a"}, 4, false},
		{"id+model", "s1", ResumeConfig{Model: "m"}, 4, false},
		{"all flags with id", "s1", ResumeConfig{YoloMode: true, Agent: "a", Model: "m"}, 7, false},
		{"all flags no id", "", ResumeConfig{YoloMode: true, Agent: "a", Model: "m"}, 5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := BuildResumeArgs(tt.sessionID, tt.cfg)
			if len(args) != tt.wantLen {
				t.Errorf("len=%d want %d; args=%v", len(args), tt.wantLen, args)
			}
			hasResume := false
			for _, a := range args {
				if a == "--resume" {
					hasResume = true
				}
			}
			if tt.noResume && hasResume {
				t.Error("should not have --resume flag")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateSessionID — table driven
// ---------------------------------------------------------------------------

func TestCovValidateSessionID_TableDriven(t *testing.T) {
	valid := []string{
		"a", "Z", "0", "abc-123", "sess.id", "a1b2c3",
		"A_B_C", "test.session.2024", strings.Repeat("x", 128),
	}
	for _, id := range valid {
		if err := validateSessionID(id); err != nil {
			t.Errorf("validateSessionID(%q) unexpected error: %v", id, err)
		}
	}

	invalid := []string{
		"", " ", "-abc", ".abc", "a b", "a;b", "a|b", "a&b",
		"a/b", `a\b`, "a\"b", "a'b", "$(cmd)", "`cmd`",
		"a\nb", "a\x00b", strings.Repeat("a", 129),
	}
	for _, id := range invalid {
		if err := validateSessionID(id); err == nil {
			t.Errorf("validateSessionID(%q) should error", id)
		}
	}
}

// ---------------------------------------------------------------------------
// validateCustomCommand — table driven
// ---------------------------------------------------------------------------

func TestCovValidateCustomCommand_TableDriven(t *testing.T) {
	valid := []string{
		"echo hello",
		"my-cli --resume {sessionId}",
		"cmd\targ",
		strings.Repeat("a", 1000),
	}
	for _, cmd := range valid {
		if err := validateCustomCommand(cmd); err != nil {
			t.Errorf("validateCustomCommand(%q) unexpected error: %v", cmd, err)
		}
	}

	invalid := []string{"", "   ", "\t", "cmd\nflag", "cmd\rflag", "cmd\r\nflag"}
	for _, cmd := range invalid {
		if err := validateCustomCommand(cmd); err == nil {
			t.Errorf("validateCustomCommand(%q) should error", cmd)
		}
	}
}

// ---------------------------------------------------------------------------
// buildCustomCmd — empty-after-expansion branch
// ---------------------------------------------------------------------------

func TestCovBuildCustomCmd_EmptyAfterExpansion(t *testing.T) {
	// "{sessionId}" passes validation, but replacing with "" yields ""
	_, err := buildCustomCmd("", "{sessionId}")
	if err == nil {
		t.Error("expected error when command is empty after expansion")
	}
	if err != nil && !strings.Contains(err.Error(), "empty after expansion") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCovBuildCustomCmd_SessionIdReplacement(t *testing.T) {
	cmd, err := buildCustomCmd("abc-123", "start {sessionId} end")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cmd.Args) != 3 || cmd.Args[1] != "abc-123" {
		t.Errorf("args = %v, want [start abc-123 end]", cmd.Args)
	}
}

func TestCovBuildCustomCmd_NoPlaceholder(t *testing.T) {
	cmd, err := buildCustomCmd("sess", "echo hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cmd.Args) != 3 {
		t.Errorf("args = %v, want 3 args", cmd.Args)
	}
}

// ---------------------------------------------------------------------------
// buildResumeCommandString — additional branches
// ---------------------------------------------------------------------------

func TestCovBuildResumeCommandString_CustomCommandEmptyAfterExpand(t *testing.T) {
	// Template "{sessionId}" with empty sessionID: passes validation,
	// but expanded is empty → error
	_, err := buildResumeCommandString("", ResumeConfig{
		CustomCommand: "  {sessionId}  ",
	})
	if err == nil {
		t.Error("expected error for command empty after expansion")
	}
}

func TestCovBuildResumeCommandString_NoCLIBinary(t *testing.T) {
	// Without custom command, depends on CLI binary presence
	_, err := buildResumeCommandString("test-session", ResumeConfig{})
	// May succeed or fail depending on PATH
	t.Logf("buildResumeCommandString (no custom cmd): err=%v", err)
}

func TestCovBuildResumeCommandString_NoCLIBinaryNoSession(t *testing.T) {
	_, err := buildResumeCommandString("", ResumeConfig{})
	t.Logf("buildResumeCommandString (no session, no custom cmd): err=%v", err)
}

// ---------------------------------------------------------------------------
// shellQuote — edge cases
// ---------------------------------------------------------------------------

func TestCovShellQuote_NullByte(t *testing.T) {
	got := shellQuote("abc\x00def")
	if strings.Contains(got, "\x00") {
		t.Error("should strip null bytes")
	}
	if got != "abcdef" {
		t.Errorf("got %q, want %q", got, "abcdef")
	}
}

func TestCovShellQuote_NullByteOnly(t *testing.T) {
	if got := shellQuote("\x00"); got != "''" {
		t.Errorf("got %q, want \"''\"", got)
	}
}

func TestCovShellQuote_SimpleAndQuoted(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "''"},
		{"simple", "simple"},
		{"with space", "'with space'"},
		{"it's", "'it'\\''s'"},
		{"a;b", "'a;b'"},
		{"/usr/bin/test", "/usr/bin/test"},
	}
	for _, tt := range tests {
		if got := shellQuote(tt.in); got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// psQuote — edge cases
// ---------------------------------------------------------------------------

func TestCovPsQuote_AllMetachars(t *testing.T) {
	input := "`$;|()"
	got := psQuote(input)
	for _, pair := range []struct{ in, expect string }{
		{"`", "``"}, {"$", "`$"}, {";", "`;"}, {"|", "`|"}, {"(", "`("}, {")", "`)"},
	} {
		if !strings.Contains(got, pair.expect) {
			t.Errorf("psQuote(%q): expected %q in result %q", input, pair.expect, got)
		}
	}
	if !strings.HasPrefix(got, "& ") {
		t.Errorf("psQuote should start with '& ', got %q", got)
	}
}

func TestCovPsQuote_NullByte(t *testing.T) {
	got := psQuote("cmd\x00arg")
	if strings.Contains(got, "\x00") {
		t.Error("should strip null bytes")
	}
}

func TestCovPsQuote_DoubleQuotesToSingle(t *testing.T) {
	got := psQuote(`"C:\path" --flag`)
	if strings.Contains(got, `"`) {
		t.Errorf("should replace double quotes with single: %q", got)
	}
}

// ---------------------------------------------------------------------------
// cmdEscape — edge cases
// ---------------------------------------------------------------------------

func TestCovCmdEscape_IndividualChars(t *testing.T) {
	tests := []struct{ in, want string }{
		{"^", "^^"},
		{"&", "^&"},
		{"|", "^|"},
		{"<", "^<"},
		{">", "^>"},
		{"(", "^("},
		{")", "^)"},
		{"%", "%%"},
		{"!", "^!"},
		{"safe", "safe"},
	}
	for _, tt := range tests {
		if got := cmdEscape(tt.in); got != tt.want {
			t.Errorf("cmdEscape(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCovCmdEscape_NullByte(t *testing.T) {
	if got := cmdEscape("a\x00b"); got != "ab" {
		t.Errorf("got %q, want %q", got, "ab")
	}
}

// ---------------------------------------------------------------------------
// resolvedCwd
// ---------------------------------------------------------------------------

func TestCovResolvedCwd_Cases(t *testing.T) {
	if got := resolvedCwd(""); got != "" {
		t.Errorf("empty → %q, want empty", got)
	}

	dir := t.TempDir()
	if got := resolvedCwd(dir); got != dir {
		t.Errorf("valid dir → %q, want %q", got, dir)
	}

	if got := resolvedCwd("C:\\nonexistent\\path\\xyz"); got != "" {
		t.Errorf("nonexistent → %q, want empty", got)
	}

	// File (not dir)
	f, _ := os.CreateTemp("", "cov-test")
	_ = f.Close()
	defer os.Remove(f.Name()) //nolint:errcheck // test cleanup
	if got := resolvedCwd(f.Name()); got != "" {
		t.Errorf("file → %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// SessionStorePath
// ---------------------------------------------------------------------------

func TestCovSessionStorePath_Override(t *testing.T) {
	t.Setenv("DISPATCH_DB", "C:\\test\\custom.db")
	path, err := SessionStorePath()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if path != "C:\\test\\custom.db" {
		t.Errorf("got %q, want override path", path)
	}
}

func TestCovSessionStorePath_Default(t *testing.T) {
	t.Setenv("DISPATCH_DB", "")
	path, err := SessionStorePath()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Error("should be absolute")
	}
	if !strings.HasSuffix(path, filepath.Join(".copilot", "session-store.db")) {
		t.Errorf("unexpected suffix: %q", path)
	}
}

// ---------------------------------------------------------------------------
// ConfigDir
// ---------------------------------------------------------------------------

func TestCovConfigDir_Basic(t *testing.T) {
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if filepath.Base(dir) != "dispatch" {
		t.Errorf("base = %q, want 'dispatch'", filepath.Base(dir))
	}
	if !filepath.IsAbs(dir) {
		t.Error("should be absolute")
	}
}

// ---------------------------------------------------------------------------
// hasNerdFontFiles
// ---------------------------------------------------------------------------

func TestCovHasNerdFontFiles_Cases(t *testing.T) {
	// With nerd font
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "SomeNerdFont.ttf"), nil, 0o644)
	if !hasNerdFontFiles(dir) {
		t.Error("should find nerd font")
	}

	// Empty dir
	if hasNerdFontFiles(t.TempDir()) {
		t.Error("should not find in empty dir")
	}

	// Non-existent
	if hasNerdFontFiles("C:\\nonexistent\\dir\\xyz") {
		t.Error("should not find in non-existent dir")
	}

	// Non-nerd TTF
	dir2 := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir2, "Arial.ttf"), nil, 0o644)
	if hasNerdFontFiles(dir2) {
		t.Error("should not match non-nerd TTF")
	}

	// OTF (wrong extension)
	dir3 := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir3, "NerdFont.otf"), nil, 0o644)
	if hasNerdFontFiles(dir3) {
		t.Error("should not match .otf files")
	}
}

// ---------------------------------------------------------------------------
// FindCLIBinary
// ---------------------------------------------------------------------------

func TestCovFindCLIBinary_Smoke(t *testing.T) {
	result := FindCLIBinary()
	t.Logf("FindCLIBinary() = %q", result)
}

// ---------------------------------------------------------------------------
// DetectShells / DefaultShell
// ---------------------------------------------------------------------------

func TestCovDetectShells(t *testing.T) {
	shells := DetectShells()
	if len(shells) == 0 {
		t.Fatal("no shells detected on Windows")
	}
	for _, s := range shells {
		if s.Name == "" || s.Path == "" {
			t.Errorf("shell has empty fields: %+v", s)
		}
	}
}

func TestCovDefaultShell(t *testing.T) {
	sh := DefaultShell()
	if sh.Name == "" || sh.Path == "" {
		t.Error("should return non-empty Name and Path")
	}
}

// ---------------------------------------------------------------------------
// DetectTerminals / DefaultTerminal
// ---------------------------------------------------------------------------

func TestCovDetectTerminals(t *testing.T) {
	terms := DetectTerminals()
	if len(terms) == 0 {
		t.Fatal("should detect at least one terminal on Windows")
	}
}

func TestCovDefaultTerminal(t *testing.T) {
	term := DefaultTerminal()
	if term != "Windows Terminal" && term != "conhost" {
		t.Errorf("unexpected terminal: %q", term)
	}
}

// ---------------------------------------------------------------------------
// IsNerdFontInstalled
// ---------------------------------------------------------------------------

func TestCovIsNerdFontInstalled(t *testing.T) {
	_ = IsNerdFontInstalled()
}

// ---------------------------------------------------------------------------
// escapeAppleScript
// ---------------------------------------------------------------------------

func TestCovEscapeAppleScript_TableDriven(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"empty", "", ""},
		{"plain", "hello", "hello"},
		{"backslash", `a\b`, `a\\b`},
		{"double quote", `a"b`, `a\"b`},
		{"single quote", "a'b", `a'\''b`},
		{"control chars", "a\x01b\x7Fc", "abc"},
		{"newlines stripped", "a\nb\rc", "abc"},
		{"tabs stripped", "a\tb", "ab"},
		{"null stripped", "a\x00b", "ab"},
		{"mixed", `say "hi" at C:\p`, `say \"hi\" at C:\\p`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeAppleScript(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NewResumeCmd
// ---------------------------------------------------------------------------

func TestCovNewResumeCmd_InvalidSessionID(t *testing.T) {
	_, err := NewResumeCmd("; evil", ResumeConfig{})
	if err == nil {
		t.Error("expected error for invalid session ID")
	}
}

func TestCovNewResumeCmd_EmptySessionCustomCmd(t *testing.T) {
	cmd, err := NewResumeCmd("", ResumeConfig{CustomCommand: "echo hello"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
}

func TestCovNewResumeCmd_CustomCmdWithCwd(t *testing.T) {
	dir := t.TempDir()
	cmd, err := NewResumeCmd("valid-session", ResumeConfig{
		CustomCommand: "echo {sessionId}",
		Cwd:           dir,
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if cmd.Dir != dir {
		t.Errorf("Dir = %q, want %q", cmd.Dir, dir)
	}
}

func TestCovNewResumeCmd_CustomCmdInvalidCwd(t *testing.T) {
	cmd, err := NewResumeCmd("valid-session", ResumeConfig{
		CustomCommand: "echo {sessionId}",
		Cwd:           "C:\\nonexistent\\path",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if cmd.Dir != "" {
		t.Errorf("Dir = %q, want empty for invalid cwd", cmd.Dir)
	}
}

// ===========================================================================
// Windows-specific: isNerdFontInstalledWindows
// ===========================================================================

func TestCovIsNerdFontInstalledWindows_UserFontPresent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	fontDir := filepath.Join(dir, "Microsoft", "Windows", "Fonts")
	_ = os.MkdirAll(fontDir, 0o755)
	_ = os.WriteFile(filepath.Join(fontDir, "JetBrainsMonoNerdFont.ttf"), []byte("fake"), 0o644)

	if !isNerdFontInstalledWindows() {
		t.Error("should detect nerd font in user directory")
	}
}

func TestCovIsNerdFontInstalledWindows_EmptyLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")
	// Should skip user fonts and check system fonts
	_ = isNerdFontInstalledWindows()
}

func TestCovIsNerdFontInstalledWindows_EmptyWinDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir) // no fonts here
	t.Setenv("WINDIR", "")
	// Falls back to C:\Windows
	_ = isNerdFontInstalledWindows()
}

// ===========================================================================
// Windows-specific: wtSettingsPaths
// ===========================================================================

func TestCovWtSettingsPaths_EmptyLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")
	paths := wtSettingsPaths()
	if paths != nil {
		t.Error("expected nil when LOCALAPPDATA is empty")
	}
}

func TestCovWtSettingsPaths_WithLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "C:\\Users\\test\\AppData\\Local")
	paths := wtSettingsPaths()
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			t.Errorf("non-absolute path: %q", p)
		}
	}
}

// ===========================================================================
// Windows-specific: DetectWTColorScheme with fake settings
// ===========================================================================

func TestCovDetectWTColorScheme_FakeSettings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	settingsDir := filepath.Join(dir, "Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState")
	_ = os.MkdirAll(settingsDir, 0o755)

	settings := `{
		"defaultProfile": "{test-guid}",
		"profiles": {
			"defaults": {},
			"list": [{"guid": "{test-guid}", "colorScheme": "TestScheme"}]
		},
		"schemes": [{
			"name": "TestScheme",
			"foreground": "#FFFFFF", "background": "#000000",
			"black": "#000", "red": "#F00", "green": "#0F0", "yellow": "#FF0",
			"blue": "#00F", "purple": "#F0F", "cyan": "#0FF", "white": "#FFF",
			"brightBlack": "#888", "brightRed": "#F88", "brightGreen": "#8F8",
			"brightYellow": "#FF8", "brightBlue": "#88F", "brightPurple": "#F8F",
			"brightCyan": "#8FF", "brightWhite": "#FFF"
		}]
	}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settings), 0o644)

	scheme, err := DetectWTColorScheme()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if scheme == nil {
		t.Fatal("expected non-nil scheme")
	}
	if scheme.Name != "TestScheme" {
		t.Errorf("Name = %q, want 'TestScheme'", scheme.Name)
	}
}

func TestCovDetectWTColorScheme_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	settingsDir := filepath.Join(dir, "Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState")
	_ = os.MkdirAll(settingsDir, 0o755)
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte("{invalid}"), 0o644)

	// Should continue past invalid JSON (try next path)
	scheme, _ := DetectWTColorScheme()
	// May be nil since no valid settings found
	_ = scheme
}

func TestCovDetectWTColorScheme_NoSettings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	// No settings.json files at all
	scheme, err := DetectWTColorScheme()
	if err != nil {
		t.Logf("error: %v", err)
	}
	if scheme != nil {
		t.Error("expected nil scheme when no settings files exist")
	}
}

// ===========================================================================
// parseWTSettingsData — additional edge cases
// ===========================================================================

func TestCovParseWTSettingsData_DefaultsFallbackObjectScheme(t *testing.T) {
	data := []byte(`{
		"defaultProfile": "{abc}",
		"profiles": {
			"defaults": {"colorScheme": {"dark": "DarkFallback"}},
			"list": [{"guid": "{abc}"}]
		},
		"schemes": [{"name": "DarkFallback", "foreground": "#FFF", "background": "#000",
			"black":"#000","red":"#F00","green":"#0F0","yellow":"#FF0",
			"blue":"#00F","purple":"#F0F","cyan":"#0FF","white":"#FFF",
			"brightBlack":"#888","brightRed":"#F88","brightGreen":"#8F8",
			"brightYellow":"#FF8","brightBlue":"#88F","brightPurple":"#F8F",
			"brightCyan":"#8FF","brightWhite":"#FFF"}]
	}`)
	scheme, err := parseWTSettingsData(data)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if scheme == nil || scheme.Name != "DarkFallback" {
		t.Errorf("expected DarkFallback scheme, got %+v", scheme)
	}
}

func TestCovParseWTSettingsData_ProfileSchemeNotInSchemes(t *testing.T) {
	data := []byte(`{
		"defaultProfile": "{abc}",
		"profiles": {
			"defaults": {},
			"list": [{"guid": "{abc}", "colorScheme": "Missing"}]
		},
		"schemes": []
	}`)
	_, err := parseWTSettingsData(data)
	if err == nil {
		t.Error("expected error when scheme not found")
	}
}

func TestCovParseWTSettingsData_EmptyProfileList(t *testing.T) {
	data := []byte(`{
		"defaultProfile": "{abc}",
		"profiles": {"defaults": {}, "list": []},
		"schemes": []
	}`)
	scheme, err := parseWTSettingsData(data)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if scheme != nil {
		t.Error("expected nil scheme for empty profile list")
	}
}

func TestCovParseWTSettingsData_InvalidJSON(t *testing.T) {
	_, err := parseWTSettingsData([]byte("{bad json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ===========================================================================
// colorSchemeRef — UnmarshalJSON and resolve
// ===========================================================================

func TestCovColorSchemeRef_UnmarshalAndResolve(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    string
		wantErr bool
	}{
		{"plain string", `"Campbell"`, "Campbell", false},
		{"dark/light object", `{"dark":"D","light":"L"}`, "D", false},
		{"dark only", `{"dark":"DarkOnly"}`, "DarkOnly", false},
		{"light only", `{"light":"LightOnly"}`, "LightOnly", false},
		{"empty string", `""`, "", false},
		{"empty object", `{}`, "", false},
		{"invalid array", `[1,2]`, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ref colorSchemeRef
			err := ref.UnmarshalJSON([]byte(tt.json))
			if (err != nil) != tt.wantErr {
				t.Fatalf("UnmarshalJSON error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				got := ref.resolve()
				if got != tt.want {
					t.Errorf("resolve() = %q, want %q", got, tt.want)
				}
			}
		})
	}
}

func TestCovColorSchemeRef_Resolve_PlainPrecedence(t *testing.T) {
	ref := colorSchemeRef{plain: "Plain", dark: "Dark", light: "Light"}
	if got := ref.resolve(); got != "Plain" {
		t.Errorf("got %q, want 'Plain' (plain takes precedence)", got)
	}
}

// ===========================================================================
// defaultWindowsShell — smoke test
// ===========================================================================

func TestCovDefaultWindowsShell(t *testing.T) {
	sh := defaultWindowsShell()
	if sh.Path == "" {
		t.Error("should always find a shell on Windows")
	}
	switch sh.Name {
	case "PowerShell 7", "Windows PowerShell", "Command Prompt":
		// expected
	default:
		t.Errorf("unexpected shell: %q", sh.Name)
	}
}

// ===========================================================================
// detectWindowsShells — verify structure
// ===========================================================================

func TestCovDetectWindowsShells(t *testing.T) {
	shells := detectWindowsShells()
	if len(shells) == 0 {
		t.Fatal("should find at least one shell on Windows")
	}
	for _, s := range shells {
		if s.Name == "" || s.Path == "" {
			t.Errorf("invalid shell: %+v", s)
		}
	}
}

// ===========================================================================
// detectWindowsTerminals — verify structure
// ===========================================================================

func TestCovDetectWindowsTerminals(t *testing.T) {
	terms := detectWindowsTerminals()
	// Windows always has at least conhost
	if len(terms) == 0 {
		t.Fatal("should find at least conhost")
	}
	found := false
	for _, term := range terms {
		if term.Name == "conhost" {
			found = true
		}
	}
	if !found {
		t.Error("conhost should always be present")
	}
}

// ===========================================================================
// LaunchSession — error path test (no opening windows)
// ===========================================================================

func TestCovLaunchSession_InvalidSessionID(t *testing.T) {
	err := LaunchSession(ShellInfo{}, "; evil", ResumeConfig{
		CustomCommand: "echo {sessionId}",
	})
	if err == nil {
		t.Error("expected error for invalid session ID")
	}
}

func TestCovLaunchSession_EmptyCustomCommand(t *testing.T) {
	err := LaunchSession(ShellInfo{}, "valid-session", ResumeConfig{
		CustomCommand: "   ",
	})
	if err == nil {
		t.Error("expected error for empty custom command")
	}
}

// ===========================================================================
// defaultWindowsShell — hit different branches via PATH manipulation
// ===========================================================================

func TestCovDefaultWindowsShell_CmdFallback(t *testing.T) {
	// With empty PATH, no shells are found → falls through to cmd.exe
	t.Setenv("PATH", "")
	sh := defaultWindowsShell()
	// cmd.exe also won't be found, but the code still returns with Name="Command Prompt"
	if sh.Name != "Command Prompt" {
		t.Errorf("expected 'Command Prompt' fallback, got %q", sh.Name)
	}
}

func TestCovDefaultWindowsShell_WindowsPowerShell(t *testing.T) {
	// Set PATH to only include the Windows PowerShell directory,
	// so pwsh.exe is not found but powershell.exe is.
	sysRoot := os.Getenv("SYSTEMROOT")
	if sysRoot == "" {
		t.Skip("SYSTEMROOT not set")
	}
	psDir := filepath.Join(sysRoot, "System32", "WindowsPowerShell", "v1.0")
	if _, err := os.Stat(psDir); err != nil {
		t.Skip("WindowsPowerShell directory not found")
	}
	t.Setenv("PATH", psDir)
	sh := defaultWindowsShell()
	if sh.Name != "Windows PowerShell" {
		t.Logf("Expected 'Windows PowerShell', got %q (pwsh may be in same dir)", sh.Name)
	}
}

// ===========================================================================
// LaunchSession — successful setup path (error in buildResumeCommandString)
// ===========================================================================

func TestCovLaunchSession_SetupWithTerminalDefault(t *testing.T) {
	// Call with Terminal="" to trigger DefaultTerminal() assignment,
	// but use a custom command that fails validation to avoid opening windows.
	err := LaunchSession(ShellInfo{}, "valid-session", ResumeConfig{
		CustomCommand: "cmd\n--evil",
	})
	if err == nil {
		t.Error("expected error for newline in custom command")
	}
}

func TestCovLaunchSession_SetupWithShellDefault(t *testing.T) {
	// Shell.Path="" triggers DefaultShell(), Terminal="" triggers DefaultTerminal()
	err := LaunchSession(ShellInfo{}, "", ResumeConfig{
		CustomCommand: "   {sessionId}   ",
	})
	if err == nil {
		t.Error("expected error for empty command after expansion")
	}
}

// ===========================================================================
// IsNerdFontInstalled — cover Windows-specific branch
// ===========================================================================

func TestCovIsNerdFontInstalled_WindowsBranch(t *testing.T) {
	// Set up a controlled environment with a nerd font file
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	fontDir := filepath.Join(dir, "Microsoft", "Windows", "Fonts")
	_ = os.MkdirAll(fontDir, 0o755)
	_ = os.WriteFile(filepath.Join(fontDir, "TestNerdFontCov.ttf"), nil, 0o644)

	if !IsNerdFontInstalled() {
		t.Error("should detect nerd font via IsNerdFontInstalled")
	}
}

func TestCovIsNerdFontInstalled_NoFonts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("WINDIR", dir) // empty dir
	if IsNerdFontInstalled() {
		t.Error("should not find nerd fonts in empty dirs")
	}
}

// ===========================================================================
// paths.go error paths — trigger os.UserConfigDir / os.UserHomeDir failures
// ===========================================================================

func TestCovConfigDir_ErrorPath(t *testing.T) {
	// On Windows, os.UserConfigDir reads %APPDATA%. Empty ⇒ error.
	t.Setenv("APPDATA", "")
	_, err := ConfigDir()
	if err == nil {
		t.Error("expected error when APPDATA is empty")
	}
}

func TestCovSessionStorePath_Error(t *testing.T) {
	// SessionStorePath uses os.UserHomeDir which reads USERPROFILE on Windows.
	t.Setenv("DISPATCH_DB", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("HOME", "")
	_, err := SessionStorePath()
	if err == nil {
		t.Error("expected error from SessionStorePath when home dir unavailable")
	}
}

// ===========================================================================
// DefaultTerminal — cover both wt.exe and conhost paths
// ===========================================================================

func TestCovDefaultTerminal_Conhost(t *testing.T) {
	// Empty PATH → wt.exe not found → falls through to "conhost"
	t.Setenv("PATH", "")
	term := DefaultTerminal()
	if term != "conhost" {
		t.Errorf("got %q, want 'conhost'", term)
	}
}

func TestCovDefaultTerminal_WindowsTerminal(t *testing.T) {
	// If wt.exe is available, it should return "Windows Terminal"
	if p, err := exec.LookPath("wt.exe"); err == nil {
		t.Setenv("PATH", filepath.Dir(p))
		term := DefaultTerminal()
		if term != "Windows Terminal" {
			t.Errorf("got %q, want 'Windows Terminal'", term)
		}
	} else {
		t.Skip("wt.exe not available")
	}
}

// ===========================================================================
// LaunchSession — cover the resolvedCwd + switch "windows" path
// Uses cmd.exe as the shell with a harmless "exit 0" custom command
// so the spawned window closes immediately without error dialogs.
// ===========================================================================

func TestCovLaunchSession_FallbackPath(t *testing.T) {
	// Test the command-line builder for the cmd /c start fallback path.
	// We don't actually spawn cmd.exe (which would flash a visible window).
	tests := []struct {
		name   string
		shell  ShellInfo
		resume string
		want   string
	}{
		{
			name:   "cmd shell",
			shell:  ShellInfo{Path: `C:\WINDOWS\system32\cmd.exe`, Name: "Command Prompt"},
			resume: "echo hello",
			want:   `cmd.exe /c start "" "C:\WINDOWS\system32\cmd.exe" /k echo hello`,
		},
		{
			name:   "pwsh shell",
			shell:  ShellInfo{Path: `C:\Program Files\PowerShell\7\pwsh.exe`, Name: "PowerShell"},
			resume: "echo hello",
			want:   `cmd.exe /c start "" "C:\Program Files\PowerShell\7\pwsh.exe" -Command & echo hello`,
		},
		{
			name:   "bash shell with args",
			shell:  ShellInfo{Path: `/usr/bin/bash`, Name: "bash", Args: []string{"--login"}},
			resume: "echo hello",
			want:   `cmd.exe /c start "" "/usr/bin/bash" --login -c "echo hello"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildStartCmdLine(tt.shell, tt.resume)
			if got != tt.want {
				t.Errorf("buildStartCmdLine() =\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}


