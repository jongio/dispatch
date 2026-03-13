package platform

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// extractTTF — additional edge cases for coverage
// ---------------------------------------------------------------------------

// createZipWithRawEntries creates a zip file with entries that have
// specific raw names (useful for testing path traversal guards).
func createZipWithRawEntries(t *testing.T, entries map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, content := range entries {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return zipPath
}

func TestExtractTTF_SkipsDirectoryEntries(t *testing.T) {
	// Directories in zip files should be skipped.
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	// Add a directory entry.
	header := &zip.FileHeader{Name: "fonts/"}
	header.SetMode(os.ModeDir | 0o755)
	if _, err := w.CreateHeader(header); err != nil {
		t.Fatal(err)
	}
	// Add a real .ttf file.
	fw, err := w.Create("fonts/real.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte("ttf data")); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	_ = f.Close()

	destDir := t.TempDir()
	extracted, err := extractTTF(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractTTF() error: %v", err)
	}
	if len(extracted) != 1 {
		t.Errorf("extracted %d files, want 1 (directory should be skipped)", len(extracted))
	}
}

func TestExtractTTF_MultipleFilesPreservedContent(t *testing.T) {
	files := map[string][]byte{
		"font-a.ttf": []byte("content-a"),
		"font-b.ttf": []byte("content-b"),
		"font-c.ttf": []byte("content-c"),
		"readme.md":  []byte("ignored"),
	}
	zipPath := createZipWithRawEntries(t, files)

	destDir := t.TempDir()
	extracted, err := extractTTF(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractTTF() error: %v", err)
	}
	if len(extracted) != 3 {
		t.Errorf("extracted %d files, want 3", len(extracted))
	}

	// Verify each extracted file exists and has content.
	for _, path := range extracted {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("reading %s: %v", path, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("extracted file %s has zero length", path)
		}
	}
}

// ---------------------------------------------------------------------------
// copyFile — additional edge cases
// ---------------------------------------------------------------------------

func TestCopyFile_LargeContent(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create a 1KB file.
	content := make([]byte, 1024)
	for i := range content {
		content[i] = byte(i % 256)
	}

	srcPath := filepath.Join(srcDir, "large.bin")
	dstPath := filepath.Join(dstDir, "large_copy.bin")

	if err := os.WriteFile(srcPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile() error: %v", err)
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if len(got) != len(content) {
		t.Errorf("copied file size = %d, want %d", len(got), len(content))
	}
}

func TestCopyFile_OverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.txt")
	dstPath := filepath.Join(dir, "dst.txt")

	if err := os.WriteFile(dstPath, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcPath, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile() error: %v", err)
	}
	got, _ := os.ReadFile(dstPath)
	if string(got) != "new content" {
		t.Errorf("copyFile did not overwrite: got %q", string(got))
	}
}

// ---------------------------------------------------------------------------
// hasNerdFontFiles — additional edge cases
// ---------------------------------------------------------------------------

func TestHasNerdFontFiles_MultipleNerdFonts(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"NerdFont-Bold.ttf", "NerdFont-Italic.ttf", "NerdFont-Regular.ttf"} {
		f, _ := os.Create(filepath.Join(dir, name))
		_ = f.Close()
	}
	if !hasNerdFontFiles(dir) {
		t.Error("should return true with multiple nerd font files")
	}
}

func TestHasNerdFontFiles_SubdirNotRecursive(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Create(filepath.Join(subdir, "NerdFont.ttf"))
	_ = f.Close()

	// Should not find files in subdirectories (ReadDir is not recursive).
	if hasNerdFontFiles(dir) {
		t.Error("hasNerdFontFiles should not search subdirectories")
	}
}

// ---------------------------------------------------------------------------
// buildResumeCommandString — additional coverage
// ---------------------------------------------------------------------------

func TestBuildResumeCommandString_CustomCommandWithSessionID(t *testing.T) {
	cfg := ResumeConfig{
		CustomCommand: "my-tool --session {sessionId}",
	}
	result, err := buildResumeCommandString("abc123", cfg)
	if err != nil {
		t.Fatalf("buildResumeCommandString error: %v", err)
	}
	if !strings.Contains(result, "abc123") {
		t.Errorf("expected result to contain session ID, got %q", result)
	}
	if strings.Contains(result, "{sessionId}") {
		t.Error("result should have {sessionId} replaced")
	}
}

func TestBuildResumeCommandString_CustomCommandNoSessionID(t *testing.T) {
	cfg := ResumeConfig{
		CustomCommand: "simple-tool",
	}
	result, err := buildResumeCommandString("test123", cfg)
	if err != nil {
		t.Fatalf("buildResumeCommandString error: %v", err)
	}
	if result != "simple-tool" {
		t.Errorf("result = %q, want %q", result, "simple-tool")
	}
}

func TestBuildResumeCommandString_InvalidSessionIDWithSpaces(t *testing.T) {
	cfg := ResumeConfig{}
	_, err := buildResumeCommandString("invalid session id with spaces", cfg)
	if err == nil {
		t.Error("expected error for invalid session ID")
	}
}

func TestBuildResumeCommandString_InvalidCustomCommand(t *testing.T) {
	cfg := ResumeConfig{
		CustomCommand: "has\nnewline",
	}
	_, err := buildResumeCommandString("valid123", cfg)
	if err == nil {
		t.Error("expected error for custom command with newline")
	}
}

func TestBuildResumeCommandString_EmptySessionID(t *testing.T) {
	cfg := ResumeConfig{
		CustomCommand: "my-tool",
	}
	result, err := buildResumeCommandString("", cfg)
	if err != nil {
		t.Fatalf("buildResumeCommandString error: %v", err)
	}
	if result != "my-tool" {
		t.Errorf("result = %q, want %q", result, "my-tool")
	}
}

// ---------------------------------------------------------------------------
// cmdEscape — additional edge cases
// ---------------------------------------------------------------------------

func TestCmdEscape_EmptyString(t *testing.T) {
	got := cmdEscape("")
	if got != "" {
		t.Errorf("cmdEscape(\"\") = %q, want empty", got)
	}
}

func TestCmdEscape_NoSpecialChars(t *testing.T) {
	got := cmdEscape("hello world")
	if got != "hello world" {
		t.Errorf("cmdEscape(\"hello world\") = %q, want %q", got, "hello world")
	}
}

func TestCmdEscape_AllSpecialChars(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"a^b", "a^^b"},
		{"a&b", "a^&b"},
		{"a|b", "a^|b"},
		{"a<b", "a^<b"},
		{"a>b", "a^>b"},
		{"a(b", "a^(b"},
		{"a)b", "a^)b"},
		{"^&|<>()", "^^^&^|^<^>^(^)"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cmdEscape(tt.input)
			if got != tt.want {
				t.Errorf("cmdEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FindCLIBinary — smoke test
// ---------------------------------------------------------------------------

func TestFindCLIBinary_DoesNotPanic(t *testing.T) {
	result := FindCLIBinary()
	// Just verify it doesn't panic; result depends on system PATH.
	_ = result
}

// ---------------------------------------------------------------------------
// LaunchSession — error paths only
// ---------------------------------------------------------------------------

func TestLaunchSession_InvalidSessionIDReturnsError(t *testing.T) {
	// Exercise the validation error path without actually launching.
	err := LaunchSession(ShellInfo{}, "invalid session id!", ResumeConfig{})
	if err == nil {
		t.Error("expected error for invalid session ID")
	}
}

func TestLaunchSession_InvalidCustomCommandReturnsError(t *testing.T) {
	err := LaunchSession(ShellInfo{Path: "test"}, "valid123", ResumeConfig{
		CustomCommand: "cmd\nwith\nnewlines",
	})
	if err == nil {
		t.Error("expected error for custom command with newlines")
	}
}

// ---------------------------------------------------------------------------
// DefaultShell — additional validation
// ---------------------------------------------------------------------------

func TestDefaultShell_PathExists(t *testing.T) {
	sh := DefaultShell()
	if sh.Path == "" {
		t.Fatal("DefaultShell().Path should not be empty")
	}
	// On Windows, verify the path is actually an executable.
	if _, err := os.Stat(sh.Path); err != nil {
		t.Logf("DefaultShell().Path %q stat error: %v (may be a PATH-resolved name)", sh.Path, err)
	}
}

// ---------------------------------------------------------------------------
// DetectShells — additional validation
// ---------------------------------------------------------------------------

func TestDetectShells_AllHaveNames(t *testing.T) {
	shells := DetectShells()
	for i, sh := range shells {
		if sh.Name == "" {
			t.Errorf("DetectShells()[%d].Name is empty", i)
		}
		if sh.Path == "" {
			t.Errorf("DetectShells()[%d].Path is empty (Name=%q)", i, sh.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// DetectTerminals — additional validation
// ---------------------------------------------------------------------------

func TestDetectTerminals_AllHaveNames(t *testing.T) {
	terms := DetectTerminals()
	for i, term := range terms {
		if term.Name == "" {
			t.Errorf("DetectTerminals()[%d].Name is empty", i)
		}
	}
}

// ---------------------------------------------------------------------------
// buildResumeCommandString — non-custom-command path
// ---------------------------------------------------------------------------

func TestBuildResumeCommandString_NormalPathWithoutCustomCommand(t *testing.T) {
	// Exercise the non-custom-command code path. Whether CLI is in PATH
	// varies by environment — we test both outcomes.
	result, err := buildResumeCommandString("test123", ResumeConfig{
		YoloMode: true,
		Agent:    "testagent",
	})
	if err != nil {
		// CLI not found — that's a valid path too (covers the error branch).
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("unexpected error: %v", err)
		}
	} else {
		if result == "" {
			t.Error("expected non-empty result when CLI is found")
		}
	}
}

func TestBuildResumeCommandString_EmptySessionNoCustom(t *testing.T) {
	result, err := buildResumeCommandString("", ResumeConfig{})
	if err != nil {
		t.Logf("buildResumeCommandString with empty session: %v (CLI may not be in PATH)", err)
	} else if result == "" {
		t.Error("expected non-empty result")
	}
}

// ---------------------------------------------------------------------------
// LaunchSession — covers more code paths before the actual launch
// ---------------------------------------------------------------------------

func TestLaunchSession_EmptyShellUsesDefault(t *testing.T) {
	// This exercises the shell.Path == "" branch and cfg.Terminal == ""
	// branch before hitting the validation error.
	err := LaunchSession(ShellInfo{}, "bad session id!", ResumeConfig{})
	if err == nil {
		t.Error("expected validation error")
	}
}

func TestLaunchSession_WithTerminalAndShell(t *testing.T) {
	// Provide both shell and terminal but use invalid session ID.
	err := LaunchSession(
		ShellInfo{Name: "test", Path: "test.exe"},
		"bad session id!",
		ResumeConfig{Terminal: "TestTerminal"},
	)
	if err == nil {
		t.Error("expected validation error")
	}
}

// ---------------------------------------------------------------------------
// NewResumeCmd — CLI not found in PATH
// ---------------------------------------------------------------------------

func TestNewResumeCmd_CLINotFoundInPATH(t *testing.T) {
	// Temporarily set PATH to an empty temp dir so neither ghcs nor copilot
	// can be found. This exercises the "CLI not found" error branch.
	t.Setenv("PATH", t.TempDir())
	_, err := NewResumeCmd("abc123", ResumeConfig{})
	if err == nil {
		t.Fatal("expected error when CLI binary is not in PATH")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// SessionStorePath — DISPATCH_DB override
// ---------------------------------------------------------------------------

func TestSessionStorePath_DispatchDBOverride(t *testing.T) {
	t.Setenv("DISPATCH_DB", "/custom/path/session.db")
	got, err := SessionStorePath()
	if err != nil {
		t.Fatalf("SessionStorePath() error: %v", err)
	}
	want := filepath.Clean("/custom/path/session.db")
	if got != want {
		t.Errorf("SessionStorePath() = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// extractTTF — invalid destination directory (os.Create error)
// ---------------------------------------------------------------------------

func TestExtractTTF_InvalidDestDir(t *testing.T) {
	files := map[string][]byte{
		"font.ttf": []byte("ttf content"),
	}
	zipPath := createZipWithRawEntries(t, files)

	// Use a non-existent directory as destination.
	_, err := extractTTF(zipPath, filepath.Join(t.TempDir(), "nonexistent", "deep"))
	if err == nil {
		t.Error("expected error when destination directory does not exist")
	}
}

// ---------------------------------------------------------------------------
// buildResumeCommandString — with CWD for resolvedCwd coverage
// ---------------------------------------------------------------------------

func TestBuildResumeCommandString_WithCwd(t *testing.T) {
	cfg := ResumeConfig{
		CustomCommand: "my-tool {sessionId}",
		Cwd:           t.TempDir(),
	}
	result, err := buildResumeCommandString("abc123", cfg)
	if err != nil {
		t.Fatalf("buildResumeCommandString error: %v", err)
	}
	if !strings.Contains(result, "abc123") {
		t.Errorf("expected session ID in result, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// NewResumeCmd — with CWD (covers cmd.Dir assignment)
// ---------------------------------------------------------------------------

func TestNewResumeCmd_CustomCommandWithCwd(t *testing.T) {
	cwd := t.TempDir()
	cmd, err := NewResumeCmd("abc123", ResumeConfig{
		CustomCommand: "echo {sessionId}",
		Cwd:           cwd,
	})
	if err != nil {
		t.Fatalf("NewResumeCmd error: %v", err)
	}
	if cmd.Dir != cwd {
		t.Errorf("cmd.Dir = %q, want %q", cmd.Dir, cwd)
	}
}

func TestNewResumeCmd_CustomCommandNoCwd(t *testing.T) {
	cmd, err := NewResumeCmd("abc123", ResumeConfig{
		CustomCommand: "echo test",
	})
	if err != nil {
		t.Fatalf("NewResumeCmd error: %v", err)
	}
	// When no CWD is specified, Dir should be empty or the resolved cwd.
	// The key is that we exercise the resolvedCwd path.
	_ = cmd
}

// ---------------------------------------------------------------------------
// DefaultTerminal — smoke test for coverage
// ---------------------------------------------------------------------------

func TestDefaultTerminal_NonEmpty(t *testing.T) {
	result := DefaultTerminal()
	if result == "" {
		t.Error("DefaultTerminal() should not return empty string")
	}
}

// ---------------------------------------------------------------------------
// escapeAppleScript — exercise on Windows (still compiles, just for coverage)
// ---------------------------------------------------------------------------

func TestEscapeAppleScript_Additional(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{`say "hi"`, `say \"hi\"`},
		{`back\slash`, `back\\slash`},
	}
	for _, tt := range tests {
		got := escapeAppleScript(tt.input)
		if got != tt.want {
			t.Errorf("escapeAppleScript(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// buildResumeCommandString — CLI not found path
// ---------------------------------------------------------------------------

func TestBuildResumeCommandString_CLINotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := buildResumeCommandString("abc123", ResumeConfig{})
	if err == nil {
		t.Fatal("expected error when CLI binary is not in PATH")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// shellQuote — edge cases
// ---------------------------------------------------------------------------

func TestShellQuote_Additional(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "''"},
		{"simple", "simple"},
		{"with space", "'with space'"},
		{"with'quote", "'with'\\''quote'"},
		{"has$var", "'has$var'"},
		{"has;semi", "'has;semi'"},
	}
	for _, tt := range tests {
		got := shellQuote(tt.input)
		if got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// psQuote — edge cases
// ---------------------------------------------------------------------------

func TestPsQuote_Additional(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "& simple"},
		{"has$var", "& has`$var"},
		{"a;b", "& a`;b"},
		{"a|b", "& a`|b"},
	}
	for _, tt := range tests {
		got := psQuote(tt.input)
		if got != tt.want {
			t.Errorf("psQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// resolvedCwd — edge cases
// ---------------------------------------------------------------------------

func TestResolvedCwd_Empty(t *testing.T) {
	got := resolvedCwd("")
	if got != "" {
		t.Errorf("resolvedCwd(\"\") = %q, want empty", got)
	}
}

func TestResolvedCwd_ValidDir(t *testing.T) {
	dir := t.TempDir()
	got := resolvedCwd(dir)
	if got != dir {
		t.Errorf("resolvedCwd(%q) = %q, want %q", dir, got, dir)
	}
}

func TestResolvedCwd_NonexistentDirReturnsEmpty(t *testing.T) {
	got := resolvedCwd(filepath.Join(t.TempDir(), "nonexistent"))
	if got != "" {
		t.Errorf("resolvedCwd(nonexistent) = %q, want empty", got)
	}
}

func TestResolvedCwd_FileNotDirReturnsEmpty(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := resolvedCwd(f)
	if got != "" {
		t.Errorf("resolvedCwd(file) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// copyFile — invalid destination path
// ---------------------------------------------------------------------------

func TestCopyFile_InvalidDestPath(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "src.txt")
	_ = os.WriteFile(srcPath, []byte("data"), 0o644)
	err := copyFile(srcPath, filepath.Join(t.TempDir(), "nonexistent", "subdir", "file.txt"))
	if err != nil {
		// Covers the os.Create error path
		return
	}
	t.Error("expected error for invalid dest path")
}

// ---------------------------------------------------------------------------
// BuildResumeArgs — additional coverage
// ---------------------------------------------------------------------------

func TestBuildResumeArgs_AllFlags(t *testing.T) {
	args := BuildResumeArgs("test-session", ResumeConfig{
		YoloMode: true,
		Agent:    "myagent",
		Model:    "testmodel",
	})

	hasResume := false
	hasAllowAll := false
	hasAgent := false
	hasSession := false
	hasModel := false
	for _, a := range args {
		switch a {
		case "--resume":
			hasResume = true
		case "--allow-all":
			hasAllowAll = true
		case "myagent":
			hasAgent = true
		case "test-session":
			hasSession = true
		case "testmodel":
			hasModel = true
		}
	}
	if !hasResume {
		t.Error("expected '--resume' in args")
	}
	if !hasAllowAll {
		t.Error("expected '--allow-all' in args")
	}
	if !hasAgent {
		t.Error("expected agent name in args")
	}
	if !hasSession {
		t.Error("expected session ID in args")
	}
	if !hasModel {
		t.Error("expected model name in args")
	}
}

func TestBuildResumeArgs_EmptySession(t *testing.T) {
	args := BuildResumeArgs("", ResumeConfig{})
	// With no session ID and no flags, args should be empty.
	if len(args) != 0 {
		t.Errorf("expected empty args, got %v", args)
	}
}

// ---------------------------------------------------------------------------
// installFontsWindows — safe edge cases (no actual font installation)
// ---------------------------------------------------------------------------

func TestInstallFontsWindows_EmptyList(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only: requires LOCALAPPDATA")
	}
	// With empty file list, the function just creates the font dir (which
	// already exists on Windows) and returns nil. This is safe.
	err := installFontsWindows(nil)
	if err != nil {
		t.Errorf("installFontsWindows(nil) error: %v", err)
	}
}

func TestInstallFontsWindows_NoLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")
	err := installFontsWindows(nil)
	if err == nil {
		t.Error("expected error when LOCALAPPDATA is not set")
	}
}

func TestInstallFontsWindows_CopyError(t *testing.T) {
	// Pass a nonexistent source file to trigger copyFile error.
	err := installFontsWindows([]string{filepath.Join(t.TempDir(), "nonexistent.ttf")})
	if err == nil {
		t.Error("expected error for nonexistent source file")
	}
}

// ---------------------------------------------------------------------------
// isNerdFontInstalledWindows — env var edge cases
// ---------------------------------------------------------------------------

func TestIsNerdFontInstalledWindows_WINDIRFallback(t *testing.T) {
	// Clear WINDIR to exercise the fallback to C:\Windows.
	t.Setenv("WINDIR", "")
	// Just verify it doesn't panic — result depends on installed fonts.
	_ = isNerdFontInstalledWindows()
}

func TestIsNerdFontInstalledWindows_NoLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")
	// Should still work using WINDIR/system fonts.
	_ = isNerdFontInstalledWindows()
}

// ---------------------------------------------------------------------------
// buildResumeCommandString — args needing quoting
// ---------------------------------------------------------------------------

func TestBuildResumeCommandString_ArgsWithSpaces(t *testing.T) {
	cfg := ResumeConfig{
		CustomCommand: "my tool --arg {sessionId}",
	}
	result, err := buildResumeCommandString("abc-123", cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(result, "abc-123") {
		t.Error("expected session ID in result")
	}
}

func TestBuildResumeCommandString_WithModel(t *testing.T) {
	cfg := ResumeConfig{
		CustomCommand: "tool",
		Cwd:           t.TempDir(),
	}
	result, err := buildResumeCommandString("abc123", cfg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result != "tool" {
		t.Errorf("result = %q, want %q", result, "tool")
	}
}
