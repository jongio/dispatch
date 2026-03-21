// Package platform provides OS-specific shell, terminal, font, and path helpers.
package platform

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// ShellInfo describes a shell that can be used to launch Copilot CLI sessions.
type ShellInfo struct {
	Name string   // Human-readable name (e.g. "PowerShell", "bash").
	Path string   // Absolute path to the shell executable.
	Args []string // Default arguments used when launching the shell.
}

// LaunchStyle constants control how a session is opened externally.
const (
	// LaunchStyleTab opens a new tab in the current terminal window.
	LaunchStyleTab = ""
	// LaunchStyleWindow opens a brand-new terminal window.
	LaunchStyleWindow = "window"
	// LaunchStylePane opens a split pane in the current tab (Windows Terminal only).
	LaunchStylePane = "pane"

	// termWindowsTerminal is the human-readable name for Windows Terminal.
	termWindowsTerminal = "Windows Terminal"
	// termConhost is the human-readable name for the legacy Windows console host.
	termConhost = "conhost"
)

// ResumeConfig holds optional CLI flags appended when resuming a session.
type ResumeConfig struct {
	YoloMode      bool
	Agent         string
	Model         string
	Terminal      string // preferred terminal emulator name (empty = auto-detect)
	CustomCommand string // when set, replaces the entire copilot CLI command
	Cwd           string // working directory to launch the session in
	LaunchStyle   string // "", "window", or "pane" — controls tab vs window vs split pane
	PaneDirection string // "auto", "right", "down", "left", "up" — split direction for pane mode
}

// TerminalInfo describes a terminal emulator available on the system.
type TerminalInfo struct {
	Name string // Human-readable name (e.g. "Windows Terminal", "alacritty").
}

// FindCLIBinary returns the absolute path to the Copilot CLI binary,
// preferring "ghcs" and falling back to "copilot". Returns an empty
// string when neither is found on PATH.
func FindCLIBinary() string {
	if p, err := exec.LookPath("ghcs"); err == nil {
		return p
	}
	if p, err := exec.LookPath("copilot"); err == nil {
		return p
	}
	return ""
}

// sessionIDPattern matches safe session ID values (UUIDs, hex strings, and
// short alphanum identifiers used by Copilot CLI). Anything outside this
// pattern is rejected before it reaches os/exec.
var sessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)

// resolvedCwd returns dir if it exists as a directory on disk, otherwise "".
// This ensures we never try to launch into a stale or invalid path.
func resolvedCwd(dir string) string {
	if dir == "" {
		return ""
	}
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return ""
}

// validateSessionID returns an error if the session ID contains characters
// that could be interpreted as shell metacharacters or path components.
func validateSessionID(id string) error {
	if !sessionIDPattern.MatchString(id) {
		return errors.New("invalid session ID: contains disallowed characters")
	}
	return nil
}

// BuildResumeArgs constructs the argument list for resuming a session
// using "copilot --resume <sessionID>" with optional flags from cfg.
// If sessionID is empty, the --resume flag is omitted (starts a new session).
func BuildResumeArgs(sessionID string, cfg ResumeConfig) []string {
	var args []string
	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}
	if cfg.YoloMode {
		args = append(args, "--allow-all")
	}
	if cfg.Agent != "" {
		args = append(args, "--agent", cfg.Agent)
	}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	return args
}

// NewResumeCmd creates an *exec.Cmd for resuming a Copilot CLI session.
// The returned command has no Stdin/Stdout/Stderr configured; callers
// (or tea.ExecProcess) should attach them as needed.
//
// When cfg.CustomCommand is set, the custom command string (with
// {sessionId} replaced) is split on whitespace and executed directly,
// bypassing the copilot CLI binary lookup.
func NewResumeCmd(sessionID string, cfg ResumeConfig) (*exec.Cmd, error) {
	if sessionID != "" {
		if err := validateSessionID(sessionID); err != nil {
			return nil, err
		}
	}
	var cmd *exec.Cmd
	if cfg.CustomCommand != "" {
		c, err := buildCustomCmd(sessionID, cfg.CustomCommand)
		if err != nil {
			return nil, err
		}
		cmd = c
	} else {
		binary := FindCLIBinary()
		if binary == "" {
			return nil, errors.New("ghcs/copilot CLI not found in PATH")
		}
		args := BuildResumeArgs(sessionID, cfg)
		cmd = exec.Command(binary, args...)
	}
	if cwd := resolvedCwd(cfg.Cwd); cwd != "" {
		cmd.Dir = cwd
	}
	return cmd, nil
}

// validateCustomCommand rejects custom command templates containing dangerous
// characters. The custom_command comes from the user's own local config file,
// so this is defense-in-depth (not a remote attack vector). The argv-style
// exec path (strings.Fields) is inherently safer, but buildResumeCommandString
// passes the expanded command through a shell, so we guard against embedded
// newlines that could inject additional shell commands.
func validateCustomCommand(cmd string) error {
	if strings.TrimSpace(cmd) == "" {
		return errors.New("custom command is empty or whitespace-only")
	}
	if strings.ContainsAny(cmd, "\n\r") {
		return errors.New("custom command contains embedded newlines")
	}
	return nil
}

// buildCustomCmd parses a custom command template, replaces {sessionId}
// with the actual session ID, splits on whitespace, and returns an *exec.Cmd.
func buildCustomCmd(sessionID, template string) (*exec.Cmd, error) {
	if err := validateCustomCommand(template); err != nil {
		return nil, err
	}
	expanded := strings.ReplaceAll(template, "{sessionId}", sessionID)
	parts := strings.Fields(expanded)
	if len(parts) == 0 {
		return nil, errors.New("custom command is empty after expansion")
	}
	return exec.Command(parts[0], parts[1:]...), nil
}

// buildResumeCommandString returns the full command string for launching
// a session resume in a shell (used by new-window launchers). Arguments
// containing whitespace or shell metacharacters are quoted.
//
// On Windows, double-quote quoting (cmdQuote) is used because cmd.exe
// and PowerShell both understand double quotes, while POSIX single
// quotes cause misinterpretation on Windows (e.g., UNC-path errors).
//
// When cfg.CustomCommand is set, {sessionId} is replaced and the result
// is returned directly (the user is responsible for quoting within their
// custom command template).
func buildResumeCommandString(sessionID string, cfg ResumeConfig) (string, error) {
	if sessionID != "" {
		if err := validateSessionID(sessionID); err != nil {
			return "", err
		}
	}
	if cfg.CustomCommand != "" {
		if err := validateCustomCommand(cfg.CustomCommand); err != nil {
			return "", err
		}
		expanded := strings.ReplaceAll(cfg.CustomCommand, "{sessionId}", sessionID)
		if strings.TrimSpace(expanded) == "" {
			return "", errors.New("custom command is empty after expansion")
		}
		return expanded, nil
	}

	binary := FindCLIBinary()
	if binary == "" {
		return "", errors.New("ghcs/copilot CLI not found in PATH")
	}

	// Choose quoting style: Windows uses double quotes (understood by
	// cmd.exe and PowerShell), Unix uses POSIX single quotes.
	quote := shellQuote
	if runtime.GOOS == "windows" {
		quote = cmdQuote
	}

	args := BuildResumeArgs(sessionID, cfg)
	if len(args) == 0 {
		return quote(binary), nil
	}
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = quote(a)
	}
	return quote(binary) + " " + strings.Join(quoted, " "), nil
}

// shellQuote wraps s in POSIX single quotes if it contains whitespace or
// shell metacharacters. Single quotes suppress all shell interpretation;
// embedded single quotes are escaped with the POSIX end-escape-reopen idiom (end quote,
// escaped literal quote, restart quote).
func shellQuote(s string) string {
	// Strip null bytes — they can truncate strings in C-based shell
	// parsers and bypass validation logic (CWE-626).
	s = strings.ReplaceAll(s, "\x00", "")
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n\r\"'`$\\!;|&<>(){}") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// psQuote wraps the resume command for PowerShell's -Command flag.
// PowerShell requires the call operator (&) before a quoted executable
// path, otherwise it treats the string as output instead of a command.
// We replace double quotes with single quotes because single-quoted
// strings are literal in PowerShell and survive nested quoting through
// intermediate launchers (wt.exe, cmd.exe) where double-quote escaping
// gets mangled by the OS command-line parser. PowerShell metacharacters
// ($, ;, |, parentheses) are escaped with backticks to prevent injection.
func psQuote(resumeCmd string) string {
	// Strip null bytes — they can truncate strings in shell parsers
	// and bypass validation logic (CWE-626).
	resumeCmd = strings.ReplaceAll(resumeCmd, "\x00", "")
	r := strings.NewReplacer(
		"`", "``",
		"$", "`$",
		";", "`;",
		"|", "`|",
		"(", "`(",
		")", "`)",
		`"`, `'`,
	)
	return "& " + r.Replace(resumeCmd)
}

// cmdQuote wraps s in double quotes for cmd.exe. Interior double quotes
// are escaped with a backslash, which is how Windows CreateProcess and
// cmd.exe interpret them. Unlike shellQuote (POSIX single quotes), this
// produces quoting that cmd.exe actually understands.
func cmdQuote(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	if s == "" {
		return `""`
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// cmdEscape escapes cmd.exe metacharacters with caret (^) to prevent
// command injection when strings are passed via cmd.exe /k or /c.
func cmdEscape(s string) string {
	// Strip null bytes — they can truncate strings in C-based parsers
	// and bypass validation logic (CWE-626).
	s = strings.ReplaceAll(s, "\x00", "")
	r := strings.NewReplacer(
		"^", "^^",
		"&", "^&",
		"|", "^|",
		"<", "^<",
		">", "^>",
		"(", "^(",
		")", "^)",
		"%", "%%",
		"!", "^!",
	)
	return r.Replace(s)
}

// isGitBash reports whether the given shell is Git Bash (MSYS).
// Git Bash is an MSYS/MinGW bash that lives under the Git install directory.
func isGitBash(shell ShellInfo) bool {
	lower := strings.ToLower(shell.Path)
	return strings.Contains(lower, "bash") &&
		(strings.Contains(lower, "git") || strings.EqualFold(shell.Name, "Git Bash"))
}

// bashifyCmd converts a Windows-style resume command for use with bash -c.
// It performs three transformations:
//  1. Backslash → forward slash (MSYS path compatibility).
//  2. Escape existing single quotes with the POSIX end-escape-reopen
//     idiom so they survive inside the single-quoted tokens from step 3.
//  3. Double quote → single quote. Double quotes can be stripped by
//     intermediate launchers (cmd.exe, wt.exe) before bash sees them,
//     causing space-in-path breakage. Single quotes survive intact and
//     bash treats their contents as literal strings.
func bashifyCmd(resumeCmd string) string {
	s := strings.ReplaceAll(resumeCmd, `\`, `/`)
	// Escape any pre-existing single quotes BEFORE converting double
	// quotes, so paths like "Bob's Tools" become 'Bob'\''s Tools' —
	// the POSIX idiom for a literal single quote inside a single-quoted
	// string (end quote, escaped literal quote, restart quote).
	s = strings.ReplaceAll(s, `'`, `'\''`)
	s = strings.ReplaceAll(s, `"`, `'`)
	return s
}

// DetectShells returns the list of shells available on the current OS.
func DetectShells() []ShellInfo {
	if runtime.GOOS == "windows" {
		return detectWindowsShells()
	}
	return detectUnixShells()
}

// DefaultShell returns the user's preferred shell. On Unix systems this is
// derived from the $SHELL environment variable; on Windows it defaults to
// PowerShell.
func DefaultShell() ShellInfo {
	if runtime.GOOS == "windows" {
		return defaultWindowsShell()
	}
	return defaultUnixShell()
}

// DefaultTerminal returns the name of the default terminal emulator for the
// current OS. On Windows this is "Windows Terminal" when wt.exe is available,
// falling back to "conhost". On macOS it is "Terminal.app". On Linux it
// returns the first detected terminal from the standard candidate list, or
// "xterm" as a last resort.
func DefaultTerminal() string {
	switch runtime.GOOS {
	case "windows":
		if _, err := exec.LookPath("wt.exe"); err == nil {
			return termWindowsTerminal
		}
		return termConhost
	case "darwin":
		return "Terminal.app"
	default:
		// Inside WSL, prefer Windows Terminal when wt.exe is available
		// since it's the terminal the user is actually sitting in.
		if isWSL() {
			if _, err := exec.LookPath("wt.exe"); err == nil {
				return termWindowsTerminal
			}
		}
		if terms := detectLinuxTerminals(); len(terms) > 0 {
			return terms[0].Name
		}
		return "xterm"
	}
}

// maxStderrCapture is the maximum number of bytes captured from a launched
// process's stderr. This prevents unbounded memory growth if a subprocess
// floods stderr within the grace window (CWE-400).
const maxStderrCapture = 64 * 1024 // 64 KB

// limitedWriter wraps a bytes.Buffer and stops writing after max bytes.
type limitedWriter struct {
	buf *bytes.Buffer
	max int
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	n := len(p) // report full input as consumed
	remaining := lw.max - lw.buf.Len()
	if remaining <= 0 {
		return n, nil // discard silently
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	lw.buf.Write(p)
	return n, nil
}

// sanitizeStderr strips control characters from stderr output before it is
// displayed in the TUI. This prevents terminal escape sequence injection
// (CWE-150) from a malicious or buggy subprocess.
func sanitizeStderr(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\t' {
			return -1
		}
		if r == 0x7F {
			return -1
		}
		return r
	}, s)
}

// startAndWaitBriefly starts a command and waits briefly for it to exit.
// If the process exits with an error within the grace period, the error is
// returned so the caller can surface it to the user (e.g., broken custom
// commands, unreachable terminal emulators). If the process is still running
// after the grace period, it is assumed to have launched successfully.
func startAndWaitBriefly(cmd *exec.Cmd) error {
	var stderr bytes.Buffer
	if cmd.Stderr == nil {
		cmd.Stderr = &limitedWriter{buf: &stderr, max: maxStderrCapture}
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	const grace = 3 * time.Second
	select {
	case err := <-done:
		if err != nil {
			if msg := strings.TrimSpace(sanitizeStderr(stderr.String())); msg != "" {
				return fmt.Errorf("%w: %s", err, msg)
			}
			return err
		}
		return nil
	case <-time.After(grace):
		return nil
	}
}

// LaunchSession opens a new terminal window running the Copilot CLI session
// resume command for the given sessionID. The detected CLI binary ("ghcs" or
// "copilot") is used with "session resume <sessionID>" plus any flags from cfg.
// If cfg.Terminal is set, that terminal emulator is preferred.
//
// When shell has an empty Path, the platform default shell is used. When
// cfg.Terminal is empty, the platform default terminal is used. This allows
// callers to omit shell/terminal configuration and still get a working launch.
func LaunchSession(shell ShellInfo, sessionID string, cfg ResumeConfig) error {
	if shell.Path == "" {
		shell = DefaultShell()
	}
	if shell.Path == "" {
		return errors.New("no shell available: could not find any shell executable on this system")
	}
	if cfg.Terminal == "" {
		cfg.Terminal = DefaultTerminal()
	}

	resumeCmd, err := buildResumeCommandString(sessionID, cfg)
	if err != nil {
		return err
	}

	cwd := resolvedCwd(cfg.Cwd)

	switch runtime.GOOS {
	case "windows":
		return launchWindowsSession(shell, resumeCmd, cfg.Terminal, cwd, cfg.LaunchStyle, cfg.PaneDirection)
	case "darwin":
		return launchDarwinSession(shell, resumeCmd, cfg.Terminal, cwd, cfg.LaunchStyle == LaunchStyleWindow)
	default:
		return launchLinuxSession(shell, resumeCmd, cfg.Terminal, cwd, cfg.LaunchStyle, cfg.PaneDirection)
	}
}

// ---------------------------------------------------------------------------
// Windows helpers
// ---------------------------------------------------------------------------

func detectWindowsShells() []ShellInfo {
	var shells []ShellInfo

	// PowerShell 7+ (pwsh.exe)
	if p, err := exec.LookPath("pwsh.exe"); err == nil {
		shells = append(shells, ShellInfo{Name: "PowerShell 7", Path: p, Args: []string{"-NoLogo"}})
	}

	// Windows PowerShell (powershell.exe)
	if p, err := exec.LookPath("powershell.exe"); err == nil {
		shells = append(shells, ShellInfo{Name: "Windows PowerShell", Path: p, Args: []string{"-NoLogo"}})
	}

	// cmd.exe
	if p, err := exec.LookPath("cmd.exe"); err == nil {
		shells = append(shells, ShellInfo{Name: "Command Prompt", Path: p})
	}

	// Git Bash — check common install locations.
	gitBashCandidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Git", "bin", "bash.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Git", "bin", "bash.exe"),
	}
	for _, candidate := range gitBashCandidates {
		if _, err := os.Stat(candidate); err == nil {
			shells = append(shells, ShellInfo{Name: "Git Bash", Path: candidate, Args: []string{"--login", "-i"}})
			break
		}
	}

	// WSL bash
	if p, err := exec.LookPath("wsl.exe"); err == nil {
		shells = append(shells, ShellInfo{Name: "WSL", Path: p})
	}

	return shells
}

func defaultWindowsShell() ShellInfo {
	if p, err := exec.LookPath("pwsh.exe"); err == nil {
		return ShellInfo{Name: "PowerShell 7", Path: p, Args: []string{"-NoLogo"}}
	}
	if p, err := exec.LookPath("powershell.exe"); err == nil {
		return ShellInfo{Name: "Windows PowerShell", Path: p, Args: []string{"-NoLogo"}}
	}
	// Fallback to cmd.exe — always present on Windows.
	p, _ := exec.LookPath("cmd.exe")
	return ShellInfo{Name: "Command Prompt", Path: p}
}

// appendWTPaneDirFlags translates a dispatch pane direction string into
// the correct wt.exe split-pane flags.
//
// Windows Terminal's -H and -V flags control the split *orientation* (the
// direction the divider runs), not which side the new pane appears on.
// WT itself decides actual pane placement based on available space.
//
// Mapping:
//
//	"down"  → -H  horizontal split — divider runs horizontally, new pane below
//	"up"    → -H  horizontal split — WT picks closest available placement
//	"right" → -V  vertical split   — divider runs vertically, new pane to the right
//	"left"  → -V  vertical split   — WT picks closest available placement
//	"auto"  → (no flag) WT default behavior
//	""      → (no flag) WT default behavior
//	unknown → (no flag) WT default behavior
func appendWTPaneDirFlags(args []string, dir string) []string {
	switch dir {
	case "down", "up":
		return append(args, "-H")
	case "right", "left":
		return append(args, "-V")
	default:
		// "auto" or empty — let Windows Terminal choose.
		return args
	}
}

func launchWindowsSession(shell ShellInfo, resumeCmd string, terminal string, cwd string, launchStyle string, paneDirection string) error {
	// Use Windows Terminal when configured (or defaulted by LaunchSession).
	if terminal == termWindowsTerminal {
		if p, err := exec.LookPath("wt.exe"); err == nil {
			var args []string
			switch launchStyle {
			case LaunchStyleWindow:
				// Force a brand-new Windows Terminal window.
				args = append(args, "-w", "new", "new-tab")
			case LaunchStylePane:
				// Open a split pane in the current tab.
				args = append(args, "-w", "0", "split-pane")
				args = appendWTPaneDirFlags(args, paneDirection)
			default:
				// Open a new tab in the most recently used window.
				// Without -w 0, wt.exe opens a new window by default.
				args = append(args, "-w", "0", "new-tab")
			}
			if cwd != "" {
				args = append(args, "--startingDirectory", cwd)
			}
			switch {
			case strings.Contains(strings.ToLower(shell.Path), "pwsh"), strings.Contains(strings.ToLower(shell.Path), "powershell"):
				args = append(args, shell.Path, "-NoLogo", "-Command", psQuote(resumeCmd))
			case strings.Contains(strings.ToLower(shell.Path), "cmd"):
				args = append(args, shell.Path, "/k", cmdEscape(resumeCmd))
			default:
				bashCmd := resumeCmd
				if isGitBash(shell) {
					bashCmd = bashifyCmd(resumeCmd)
				}
				args = append(args, shell.Path, "-c", bashCmd)
			}
			cmd := exec.Command(p, args...)
			return startAndWaitBriefly(cmd)
		}
	}

	// Fallback: use cmd /c start to open a new console window.
	cmdLine := buildStartCmdLine(shell, resumeCmd)

	cmd := exec.Command("cmd.exe")
	setCmdLine(cmd, cmdLine)
	if cwd != "" {
		cmd.Dir = cwd
	}
	return startAndWaitBriefly(cmd)
}

// buildStartCmdLine constructs the raw command line for cmd /c start.
//
// We build the raw command line and use SysProcAttr.CmdLine to
// bypass Go's syscall.EscapeArg. EscapeArg converts "" to "\"\""
// using C runtime escaping rules, but cmd.exe treats \ as a
// literal character (not an escape), so it interprets the result
// as \\ (two backslashes = UNC path prefix). start then tries to
// execute \\ and shows "Windows cannot find '\\'" error dialog.
func buildStartCmdLine(shell ShellInfo, resumeCmd string) string {
	var cmdLine strings.Builder
	// Pass an empty title ("") first because start treats the first
	// quoted argument as the window title, not the executable.
	cmdLine.WriteString(`cmd.exe /c start ""`)
	cmdLine.WriteString(` "`)
	cmdLine.WriteString(shell.Path)
	cmdLine.WriteString(`"`)

	for _, a := range shell.Args {
		cmdLine.WriteString(` `)
		cmdLine.WriteString(a)
	}

	switch {
	case strings.Contains(strings.ToLower(shell.Path), "pwsh"), strings.Contains(strings.ToLower(shell.Path), "powershell"):
		cmdLine.WriteString(` -Command `)
		cmdLine.WriteString(psQuote(resumeCmd))
	case strings.Contains(strings.ToLower(shell.Path), "cmd"):
		cmdLine.WriteString(` /k `)
		cmdLine.WriteString(cmdEscape(resumeCmd))
	default:
		// For bash-like shells launched through cmd.exe /c start, wrap
		// resumeCmd in cmdQuote (double quotes) so cmd.exe passes it as
		// a single argument to the shell's -c flag. Without quoting,
		// cmd.exe would interpret metacharacters (&, |, >) in the
		// resume command before the shell receives them.
		bashCmd := resumeCmd
		if isGitBash(shell) {
			bashCmd = bashifyCmd(resumeCmd)
		}
		cmdLine.WriteString(` -c `)
		cmdLine.WriteString(cmdQuote(bashCmd))
	}

	return cmdLine.String()
}

// ---------------------------------------------------------------------------
// Unix helpers (macOS / Linux)
// ---------------------------------------------------------------------------

func detectUnixShells() []ShellInfo {
	var shells []ShellInfo
	seen := make(map[string]struct{})

	f, err := os.Open("/etc/shells")
	if err != nil {
		// Fallback: probe well-known paths.
		for _, name := range []string{"bash", "zsh", "fish", "sh"} {
			if p, err := exec.LookPath(name); err == nil {
				shells = append(shells, ShellInfo{Name: name, Path: p})
			}
		}
		return shells
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if _, err := os.Stat(line); err == nil {
			name := filepath.Base(line)
			// Deduplicate by name so the config panel cycle doesn't get
			// stuck when /etc/shells lists the same shell under multiple
			// paths (e.g. /bin/bash and /usr/bin/bash).
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			shells = append(shells, ShellInfo{Name: name, Path: line})
		}
	}
	// Ignore scanner errors — best-effort detection from /etc/shells.
	_ = scanner.Err()

	return shells
}

func defaultUnixShell() ShellInfo {
	if shellEnv := os.Getenv("SHELL"); shellEnv != "" {
		if filepath.IsAbs(shellEnv) {
			if info, err := os.Stat(shellEnv); err == nil && !info.IsDir() {
				return ShellInfo{
					Name: filepath.Base(shellEnv),
					Path: shellEnv,
				}
			}
		}
	}
	// Fallback.
	if p, err := exec.LookPath("bash"); err == nil {
		return ShellInfo{Name: "bash", Path: p}
	}
	return ShellInfo{Name: "sh", Path: "/bin/sh"}
}

func launchDarwinSession(shell ShellInfo, resumeCmd string, terminal string, cwd string, forceNewWindow bool) error {
	// Prepend a cd to the resume command so the session starts in the
	// original working directory. Double quotes are used for the path
	// because the -c argument is already single-quoted in the AppleScript
	// template; escapeAppleScript handles the AppleScript layer.
	if cwd != "" {
		resumeCmd = "cd " + shellQuote(cwd) + " && " + resumeCmd
	}

	switch terminal {
	case "iTerm2":
		escapedShell := escapeAppleScript(shell.Path)
		var script string
		if forceNewWindow {
			script = fmt.Sprintf(
				`tell application "iTerm2" to create window with default profile command "%s -c '%s'"`,
				escapedShell, escapeAppleScript(resumeCmd),
			)
		} else {
			// Open a tab in the current window (or a new window if none exist).
			script = fmt.Sprintf(
				`tell application "iTerm2"
					if (count of windows) > 0 then
						tell current window to create tab with default profile command "%s -c '%s'"
					else
						create window with default profile command "%s -c '%s'"
					end if
				end tell`,
				escapedShell, escapeAppleScript(resumeCmd),
				escapedShell, escapeAppleScript(resumeCmd),
			)
		}
		cmd := exec.Command("osascript", "-e", script)
		return startAndWaitBriefly(cmd)
	case "WezTerm":
		if p, err := exec.LookPath("wezterm"); err == nil {
			cmd := exec.Command(p, "start", "--", shell.Path, "-c", resumeCmd)
			if cwd != "" {
				cmd.Dir = cwd
			}
			return startAndWaitBriefly(cmd)
		}
	}
	// Default: Terminal.app
	escapedCmd := escapeAppleScript(resumeCmd)
	escapedShellPath := escapeAppleScript(shell.Path)
	var script string
	if forceNewWindow {
		script = fmt.Sprintf(
			`tell application "Terminal" to do script "%s -c '%s'"`,
			escapedShellPath, escapedCmd,
		)
	} else {
		// Open a tab in the frontmost window (falls back to new window if none open).
		script = fmt.Sprintf(
			`tell application "Terminal"
				activate
				if (count of windows) > 0 then
					do script "%s -c '%s'" in front window
				else
					do script "%s -c '%s'"
				end if
			end tell`,
			escapedShellPath, escapedCmd,
			escapedShellPath, escapedCmd,
		)
	}
	cmd := exec.Command("osascript", "-e", script)
	return startAndWaitBriefly(cmd)
}

// escapeAppleScript escapes a string for safe embedding within AppleScript
// double-quoted or single-quoted string literals. Control characters are
// stripped first to prevent breaking out of string literals, then backslashes
// and quotes are escaped to prevent injection.
func escapeAppleScript(s string) string {
	// Strip control characters that could break AppleScript string literals.
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 && r != 0x7F {
			b.WriteRune(r)
		}
	}
	s = b.String()
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `'`, `'\''`)
	return s
}

func launchLinuxSession(shell ShellInfo, resumeCmd string, terminal string, cwd string, launchStyle string, paneDirection string) error {
	// Inside WSL, route to Windows Terminal when configured.
	if terminal == termWindowsTerminal && isWSL() {
		return launchWSLWindowsTerminal(shell, resumeCmd, cwd, launchStyle, paneDirection)
	}

	// Supported terminal emulators and their argument patterns.
	terminals := []struct {
		name string
		args func() []string
	}{
		{"alacritty", func() []string { return []string{"-e", shell.Path, "-c", resumeCmd} }},
		{"kitty", func() []string { return []string{shell.Path, "-c", resumeCmd} }},
		{"wezterm", func() []string { return []string{"start", "--", shell.Path, "-c", resumeCmd} }},
		{"gnome-terminal", func() []string { return []string{"--", shell.Path, "-c", resumeCmd} }},
		{"konsole", func() []string { return []string{"-e", shell.Path, "-c", resumeCmd} }},
		{"xfce4-terminal", func() []string {
			escaped := strings.ReplaceAll(resumeCmd, "'", "'\\''")
			return []string{"-e", shell.Path + " -c '" + escaped + "'"}
		}},
		{"xterm", func() []string { return []string{"-e", shell.Path, "-c", resumeCmd} }},
	}

	// If a terminal is configured, try it first.
	if terminal != "" {
		for _, t := range terminals {
			if t.name == terminal {
				if p, err := exec.LookPath(t.name); err == nil {
					cmd := exec.Command(p, t.args()...)
					if cwd != "" {
						cmd.Dir = cwd
					}
					return startAndWaitBriefly(cmd)
				}
			}
		}
	}

	// Auto-detect: try Windows Terminal first if running under WSL.
	if isWSL() {
		if _, err := exec.LookPath("wt.exe"); err == nil {
			return launchWSLWindowsTerminal(shell, resumeCmd, cwd, launchStyle, paneDirection)
		}
	}

	// Auto-detect: try terminals in order of popularity.
	for _, t := range terminals {
		if p, err := exec.LookPath(t.name); err == nil {
			cmd := exec.Command(p, t.args()...)
			if cwd != "" {
				cmd.Dir = cwd
			}
			return startAndWaitBriefly(cmd)
		}
	}

	return errors.New("no supported terminal emulator found; tried alacritty, kitty, wezterm, gnome-terminal, konsole, xfce4-terminal, xterm")
}

// launchWSLWindowsTerminal launches a session via wt.exe from within WSL.
// It translates the working directory from a Linux path to a Windows path
// using wslpath and constructs wt.exe arguments for the requested launch
// style (tab, window, or pane).
func launchWSLWindowsTerminal(shell ShellInfo, resumeCmd string, cwd string, launchStyle string, paneDirection string) error {
	p, err := exec.LookPath("wt.exe")
	if err != nil {
		return fmt.Errorf("wt.exe not found on PATH: %w", err)
	}

	// Translate cwd to a Windows path (empty string on failure).
	var winCwd string
	if cwd != "" {
		winCwd, _ = wslToWindowsPath(cwd)
	}

	distro := os.Getenv("WSL_DISTRO_NAME")
	args := buildWSLWTArgs(shell, resumeCmd, winCwd, distro, launchStyle, paneDirection)

	cmd := exec.Command(p, args...)
	return startAndWaitBriefly(cmd)
}

// buildWSLWTArgs constructs the wt.exe argument list for launching a WSL
// session inside Windows Terminal. It is separated from launchWSLWindowsTerminal
// so it can be tested without requiring wt.exe or a WSL environment.
func buildWSLWTArgs(shell ShellInfo, resumeCmd, winCwd, distro, launchStyle, paneDirection string) []string {
	var args []string
	switch launchStyle {
	case LaunchStyleWindow:
		args = append(args, "-w", "new", "new-tab")
	case LaunchStylePane:
		args = append(args, "-w", "0", "split-pane")
		args = appendWTPaneDirFlags(args, paneDirection)
	default:
		args = append(args, "-w", "0", "new-tab")
	}

	if winCwd != "" {
		args = append(args, "--startingDirectory", winCwd)
	}

	// wt.exe wsl.exe -d <distro> -- <shell> -c <resumeCmd>
	if distro != "" {
		args = append(args, "wsl.exe", "-d", distro, "--", shell.Path, "-c", resumeCmd)
	} else {
		args = append(args, "wsl.exe", "--", shell.Path, "-c", resumeCmd)
	}

	return args
}

// wslToWindowsPath translates a Linux filesystem path to a Windows path
// using the wslpath utility available inside WSL.
func wslToWindowsPath(linuxPath string) (string, error) {
	cmd := exec.Command("wslpath", "-w", linuxPath)
	out, err := cmd.Output()
	if err != nil {
		// Include stderr from wslpath for better diagnostics.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("wslpath: %s: %w", strings.TrimSpace(string(exitErr.Stderr)), err)
		}
		return "", fmt.Errorf("wslpath: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ---------------------------------------------------------------------------
// Terminal detection
// ---------------------------------------------------------------------------

// DetectTerminals returns the list of terminal emulators available on the
// current OS.
func DetectTerminals() []TerminalInfo {
	switch runtime.GOOS {
	case "windows":
		return detectWindowsTerminals()
	case "darwin":
		return detectDarwinTerminals()
	default:
		return detectLinuxTerminals()
	}
}

func detectWindowsTerminals() []TerminalInfo {
	var terms []TerminalInfo
	if _, err := exec.LookPath("wt.exe"); err == nil {
		terms = append(terms, TerminalInfo{Name: termWindowsTerminal})
	}
	terms = append(terms, TerminalInfo{Name: termConhost})
	return terms
}

func detectDarwinTerminals() []TerminalInfo {
	terms := []TerminalInfo{{Name: "Terminal.app"}}
	if _, err := os.Stat("/Applications/iTerm.app"); err == nil {
		terms = append(terms, TerminalInfo{Name: "iTerm2"})
	}
	if _, err := exec.LookPath("wezterm"); err == nil {
		terms = append(terms, TerminalInfo{Name: "WezTerm"})
	}
	return terms
}

func detectLinuxTerminals() []TerminalInfo {
	var terms []TerminalInfo

	// Inside WSL, include Windows Terminal when wt.exe is available
	// via the WSL-Windows interop PATH.
	if isWSL() {
		if _, err := exec.LookPath("wt.exe"); err == nil {
			terms = append(terms, TerminalInfo{Name: termWindowsTerminal})
		}
	}

	candidates := []string{
		"alacritty", "kitty", "wezterm",
		"gnome-terminal", "konsole", "xfce4-terminal", "xterm",
	}
	for _, name := range candidates {
		if _, err := exec.LookPath(name); err == nil {
			terms = append(terms, TerminalInfo{Name: name})
		}
	}
	return terms
}
