//go:build windows

package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

type platformRecorderOutput struct {
	Args []string `json:"args"`
	Cwd  string   `json:"cwd"`
}

const platformRecorderProgram = `package main

import (
	"encoding/json"
	"os"
	"strconv"
)

type output struct {
	Args []string ` + "`json:\"args\"`" + `
	Cwd  string   ` + "`json:\"cwd\"`" + `
}

func main() {
	cwd, _ := os.Getwd()
	data, _ := json.Marshal(output{Args: os.Args[1:], Cwd: cwd})
	if path := os.Getenv("DISPATCH_PLATFORM_RECORD"); path != "" {
		_ = os.WriteFile(path, data, 0600)
	}
	_, _ = os.Stdout.WriteString(os.Getenv("DISPATCH_PLATFORM_STDOUT"))
	_, _ = os.Stderr.WriteString(os.Getenv("DISPATCH_PLATFORM_STDERR"))
	if code, err := strconv.Atoi(os.Getenv("DISPATCH_PLATFORM_EXIT")); err == nil && code != 0 {
		os.Exit(code)
	}
}
`

var (
	platformRecorderOnce sync.Once
	platformRecorderDir  string
	platformRecorderExe  string
	errPlatformRecorder  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if platformRecorderDir != "" {
		_ = os.RemoveAll(platformRecorderDir)
	}
	os.Exit(code)
}

func buildPlatformRecorder(t *testing.T) string {
	t.Helper()

	platformRecorderOnce.Do(func() {
		platformRecorderDir, errPlatformRecorder = os.MkdirTemp("", "dispatch-platform-recorder-*")
		if errPlatformRecorder != nil {
			return
		}
		sourcePath := filepath.Join(platformRecorderDir, "main.go")
		errPlatformRecorder = os.WriteFile(sourcePath, []byte(platformRecorderProgram), 0o600)
		if errPlatformRecorder != nil {
			return
		}

		platformRecorderExe = filepath.Join(platformRecorderDir, "recorder.exe")
		cmd := exec.Command("go", "build", "-o", platformRecorderExe, sourcePath)
		if output, err := cmd.CombinedOutput(); err != nil {
			errPlatformRecorder = fmt.Errorf("build recorder executable: %w\n%s", err, output)
		}
	})
	if errPlatformRecorder != nil {
		t.Fatal(errPlatformRecorder)
	}
	return platformRecorderExe
}

func platformRecorderPath(t *testing.T, recorder string, names ...string) string {
	t.Helper()

	dir := t.TempDir()
	data, err := os.ReadFile(recorder)
	if err != nil {
		t.Fatalf("read recorder executable: %v", err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o700); err != nil {
			t.Fatalf("install recorder as %s: %v", name, err)
		}
	}
	return dir
}

func readPlatformRecording(t *testing.T, path string) platformRecorderOutput {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			var output platformRecorderOutput
			if err := json.Unmarshal(data, &output); err != nil {
				t.Fatalf("decode platform recording: %v", err)
			}
			return output
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read platform recording: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for platform recording at %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func assertPlatformArgs(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("recorded args = %q, want %q", got, want)
	}
}

func setWSLDistroForTest(t *testing.T, distro string) {
	t.Helper()

	original, existed := os.LookupEnv("WSL_DISTRO_NAME")
	if distro == "" {
		if err := os.Unsetenv("WSL_DISTRO_NAME"); err != nil {
			t.Fatalf("clear WSL_DISTRO_NAME: %v", err)
		}
	} else if err := os.Setenv("WSL_DISTRO_NAME", distro); err != nil {
		t.Fatalf("set WSL_DISTRO_NAME: %v", err)
	}
	wslOnce = sync.Once{}
	wslCached = false

	t.Cleanup(func() {
		if existed {
			_ = os.Setenv("WSL_DISTRO_NAME", original)
		} else {
			_ = os.Unsetenv("WSL_DISTRO_NAME")
		}
		wslOnce = sync.Once{}
		wslCached = false
		_ = isWSL()
	})
}

func TestPlatformAPlusFindCLIAndResumeCommandConstruction(t *testing.T) {
	recorder := buildPlatformRecorder(t)

	t.Run("falls back to ghcs", func(t *testing.T) {
		pathDir := platformRecorderPath(t, recorder, "ghcs.exe")
		t.Setenv("PATH", pathDir)

		got := FindCLIBinary()
		want := filepath.Join(pathDir, "ghcs.exe")
		if !strings.EqualFold(got, want) {
			t.Fatalf("FindCLIBinary() = %q, want %q", got, want)
		}
	})

	t.Run("builds the exported resume command", func(t *testing.T) {
		pathDir := platformRecorderPath(t, recorder, "copilot.exe")
		t.Setenv("PATH", pathDir)

		got, err := BuildResumeCommandString("session-123", ResumeConfig{
			YoloMode: true,
			Agent:    "code reviewer",
			Model:    "model;safe",
		})
		if err != nil {
			t.Fatalf("BuildResumeCommandString() error: %v", err)
		}

		binary := filepath.Join(pathDir, "copilot.exe")
		want := strings.Join([]string{
			cmdQuote(binary),
			cmdQuote("--resume"),
			cmdQuote("session-123"),
			cmdQuote("--allow-all"),
			cmdQuote("--agent"),
			cmdQuote("code reviewer"),
			cmdQuote("--model"),
			cmdQuote("model;safe"),
		}, " ")
		if got != want {
			t.Fatalf("BuildResumeCommandString() = %q, want %q", got, want)
		}
	})
}

func TestPlatformAPlusNewSessionDefaultsAndSuccess(t *testing.T) {
	recorder := buildPlatformRecorder(t)
	pathDir := platformRecorderPath(t, recorder, "pwsh.exe", "wt.exe")
	t.Setenv("PATH", pathDir)

	originalNewSession := launchNewSessionPlatformFn
	t.Cleanup(func() { launchNewSessionPlatformFn = originalNewSession })

	var gotShell ShellInfo
	var gotTerminal string
	launchNewSessionPlatformFn = func(shell ShellInfo, command, terminal, cwd, style, direction string) (int, error) {
		gotShell = shell
		gotTerminal = terminal
		if command != defaultNewSessionCommand {
			t.Fatalf("command = %q, want %q", command, defaultNewSessionCommand)
		}
		return 73, nil
	}

	pid, err := LaunchNewSession(LaunchNewSessionConfig{})
	if err != nil {
		t.Fatalf("LaunchNewSession() error: %v", err)
	}
	if pid != 73 {
		t.Fatalf("PID = %d, want 73", pid)
	}
	if filepath.Base(gotShell.Path) != "pwsh.exe" {
		t.Fatalf("default shell path = %q, want pwsh.exe", gotShell.Path)
	}
	if gotTerminal != termWindowsTerminal {
		t.Fatalf("default terminal = %q, want %q", gotTerminal, termWindowsTerminal)
	}

	originalPlatform := platformLaunchSessionFn
	t.Cleanup(func() { platformLaunchSessionFn = originalPlatform })
	platformLaunchSessionFn = func(ShellInfo, string, string, string, string, string) error {
		return nil
	}
	pid, err = launchNewSessionPlatformImpl(
		ShellInfo{Path: "test-shell"},
		"copilot",
		"test-terminal",
		"",
		LaunchStyleTab,
		"",
	)
	if err != nil {
		t.Fatalf("launchNewSessionPlatformImpl() error: %v", err)
	}
	if pid != 0 {
		t.Fatalf("PID = %d, want 0 for watcher discovery", pid)
	}
}

func TestPlatformAPlusWindowsTerminalCommandConstruction(t *testing.T) {
	recorder := buildPlatformRecorder(t)
	pathDir := platformRecorderPath(t, recorder, "wt.exe")
	t.Setenv("PATH", pathDir)

	tests := []struct {
		name          string
		shell         ShellInfo
		resumeCommand string
		cwd           string
		style         string
		direction     string
		want          []string
	}{
		{
			name:          "PowerShell tab with working directory",
			shell:         ShellInfo{Name: "PowerShell 7", Path: `C:\Tools\pwsh.exe`},
			resumeCommand: `"C:\Program Files\copilot.exe" --resume session-1`,
			cwd:           t.TempDir(),
			style:         LaunchStyleTab,
		},
		{
			name:          "cmd in new window",
			shell:         ShellInfo{Name: "Command Prompt", Path: `C:\Windows\System32\cmd.exe`},
			resumeCommand: `echo safe & echo literal`,
			style:         LaunchStyleWindow,
		},
		{
			name:          "Git Bash in right pane",
			shell:         ShellInfo{Name: "Git Bash", Path: `C:\Program Files\Git\bin\bash.exe`},
			resumeCommand: `"C:\Users\O'Brien\copilot.exe" --resume session-2`,
			style:         LaunchStylePane,
			direction:     "right",
		},
		{
			name:          "generic shell in down pane",
			shell:         ShellInfo{Name: "bash", Path: `C:\Tools\bash.exe`},
			resumeCommand: `copilot --resume session-3`,
			style:         LaunchStylePane,
			direction:     "down",
		},
	}

	for i := range tests {
		tt := &tests[i]
		switch tt.name {
		case "PowerShell tab with working directory":
			tt.want = []string{
				"-w", "0", "new-tab",
				"--startingDirectory", tt.cwd,
				tt.shell.Path, "-NoLogo", "-Command", psQuote(tt.resumeCommand),
			}
		case "cmd in new window":
			tt.want = []string{
				"-w", "new", "new-tab",
				tt.shell.Path, "/k", cmdEscape(tt.resumeCommand),
			}
		case "Git Bash in right pane":
			tt.want = []string{
				"-w", "0", "split-pane", "-V",
				tt.shell.Path, "-c", bashifyCmd(tt.resumeCommand),
			}
		case "generic shell in down pane":
			tt.want = []string{
				"-w", "0", "split-pane", "-H",
				tt.shell.Path, "-c", tt.resumeCommand,
			}
		}

		t.Run(tt.name, func(t *testing.T) {
			recordPath := filepath.Join(t.TempDir(), "record.json")
			t.Setenv("DISPATCH_PLATFORM_RECORD", recordPath)

			err := launchWindowsSession(
				tt.shell,
				tt.resumeCommand,
				termWindowsTerminal,
				tt.cwd,
				tt.style,
				tt.direction,
			)
			if err != nil {
				t.Fatalf("launchWindowsSession() error: %v", err)
			}

			recording := readPlatformRecording(t, recordPath)
			assertPlatformArgs(t, recording.Args, tt.want)
		})
	}
}

func TestPlatformAPlusTmuxCommandConstruction(t *testing.T) {
	recorder := buildPlatformRecorder(t)

	t.Run("pane route invokes tmux with exact args", func(t *testing.T) {
		pathDir := platformRecorderPath(t, recorder, "tmux.exe")
		t.Setenv("PATH", pathDir)
		t.Setenv("TMUX", "active")
		recordPath := filepath.Join(t.TempDir(), "record.json")
		t.Setenv("DISPATCH_PLATFORM_RECORD", recordPath)
		cwd := t.TempDir()
		shell := ShellInfo{Name: "bash", Path: `C:\Tools\bash.exe`}

		err := platformLaunchSession(
			shell,
			"copilot --resume session-4",
			termWindowsTerminal,
			cwd,
			LaunchStylePane,
			"left",
		)
		if err != nil {
			t.Fatalf("platformLaunchSession() error: %v", err)
		}

		recording := readPlatformRecording(t, recordPath)
		assertPlatformArgs(t, recording.Args, []string{
			"split-window", "-h", "-c", cwd,
			shell.Path, "-c", "copilot --resume session-4",
		})
		if recording.Cwd != cwd {
			t.Fatalf("tmux process cwd = %q, want %q", recording.Cwd, cwd)
		}
	})

	t.Run("missing tmux returns a diagnostic error", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		t.Setenv("TMUX", "active")

		err := platformLaunchSession(
			ShellInfo{Path: "bash"},
			"copilot",
			"",
			"",
			LaunchStylePane,
			"",
		)
		if err == nil || !strings.Contains(err.Error(), "tmux binary was not found") {
			t.Fatalf("error = %v, want missing tmux diagnostic", err)
		}
	})
}

func TestPlatformAPlusLinuxLauncherBehavior(t *testing.T) {
	recorder := buildPlatformRecorder(t)
	setWSLDistroForTest(t, "")

	t.Run("configured kitty receives structured arguments and cwd", func(t *testing.T) {
		pathDir := platformRecorderPath(t, recorder, "kitty.exe")
		t.Setenv("PATH", pathDir)
		recordPath := filepath.Join(t.TempDir(), "record.json")
		t.Setenv("DISPATCH_PLATFORM_RECORD", recordPath)
		cwd := t.TempDir()

		err := launchLinuxSession(
			ShellInfo{Path: "/bin/bash"},
			"copilot --resume session-5",
			"kitty",
			cwd,
			LaunchStyleTab,
			"",
		)
		if err != nil {
			t.Fatalf("launchLinuxSession() error: %v", err)
		}

		recording := readPlatformRecording(t, recordPath)
		assertPlatformArgs(t, recording.Args, []string{
			"/bin/bash", "-c", "copilot --resume session-5",
		})
		if recording.Cwd != cwd {
			t.Fatalf("kitty process cwd = %q, want %q", recording.Cwd, cwd)
		}
	})

	t.Run("xfce command protects embedded single quotes", func(t *testing.T) {
		pathDir := platformRecorderPath(t, recorder, "xfce4-terminal.exe")
		t.Setenv("PATH", pathDir)
		recordPath := filepath.Join(t.TempDir(), "record.json")
		t.Setenv("DISPATCH_PLATFORM_RECORD", recordPath)

		err := launchLinuxSession(
			ShellInfo{Path: "/bin/bash"},
			"printf 'safe value'",
			"xfce4-terminal",
			"",
			LaunchStyleTab,
			"",
		)
		if err != nil {
			t.Fatalf("launchLinuxSession() error: %v", err)
		}

		recording := readPlatformRecording(t, recordPath)
		assertPlatformArgs(t, recording.Args, []string{
			"-e", `/bin/bash -c 'printf '\''safe value'\'''`,
		})
	})

	t.Run("auto detection selects the first available terminal", func(t *testing.T) {
		pathDir := platformRecorderPath(t, recorder, "alacritty.exe")
		t.Setenv("PATH", pathDir)
		recordPath := filepath.Join(t.TempDir(), "record.json")
		t.Setenv("DISPATCH_PLATFORM_RECORD", recordPath)

		err := launchLinuxSession(
			ShellInfo{Path: "/bin/zsh"},
			"copilot",
			"",
			"",
			LaunchStyleTab,
			"",
		)
		if err != nil {
			t.Fatalf("launchLinuxSession() error: %v", err)
		}

		recording := readPlatformRecording(t, recordPath)
		assertPlatformArgs(t, recording.Args, []string{
			"-e", "/bin/zsh", "-c", "copilot",
		})
	})

	t.Run("missing terminals returns the full fallback diagnostic", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		err := launchLinuxSession(
			ShellInfo{Path: "/bin/bash"},
			"copilot",
			"unknown-terminal",
			"",
			LaunchStyleTab,
			"",
		)
		if !errors.Is(err, ErrNoTerminalEmulator) {
			t.Fatalf("error = %v, want ErrNoTerminalEmulator", err)
		}
		if !strings.Contains(err.Error(), "tried alacritty, kitty, wezterm") {
			t.Fatalf("error = %q, want attempted terminal list", err)
		}
	})

	t.Run("Darwin iTerm new window constructs a protected AppleScript command", func(t *testing.T) {
		pathDir := platformRecorderPath(t, recorder, "osascript.exe")
		t.Setenv("PATH", pathDir)
		recordPath := filepath.Join(t.TempDir(), "record.json")
		t.Setenv("DISPATCH_PLATFORM_RECORD", recordPath)

		err := launchDarwinSession(
			ShellInfo{Path: "/bin/zsh"},
			`printf "safe"`,
			"iTerm2",
			"/Users/test/work dir",
			true,
		)
		if err != nil {
			t.Fatalf("launchDarwinSession() error: %v", err)
		}

		recording := readPlatformRecording(t, recordPath)
		if len(recording.Args) != 2 || recording.Args[0] != "-e" {
			t.Fatalf("osascript args = %q, want -e and one script", recording.Args)
		}
		script := recording.Args[1]
		for _, fragment := range []string{
			`tell application "iTerm2" to create window`,
			`/bin/zsh -c`,
			`/Users/test/work dir`,
			`printf \"safe\"`,
		} {
			if !strings.Contains(script, fragment) {
				t.Fatalf("AppleScript %q does not contain %q", script, fragment)
			}
		}
	})

	t.Run("Darwin iTerm tab uses the current window fallback script", func(t *testing.T) {
		pathDir := platformRecorderPath(t, recorder, "osascript.exe")
		t.Setenv("PATH", pathDir)
		recordPath := filepath.Join(t.TempDir(), "record.json")
		t.Setenv("DISPATCH_PLATFORM_RECORD", recordPath)

		err := launchDarwinSession(
			ShellInfo{Path: "/bin/bash"},
			"copilot --resume session-7",
			"iTerm2",
			"",
			false,
		)
		if err != nil {
			t.Fatalf("launchDarwinSession() error: %v", err)
		}

		recording := readPlatformRecording(t, recordPath)
		if len(recording.Args) != 2 || recording.Args[0] != "-e" {
			t.Fatalf("osascript args = %q, want -e and one script", recording.Args)
		}
		script := recording.Args[1]
		for _, fragment := range []string{
			`tell application "iTerm2"`,
			`if (count of windows) > 0 then`,
			`create tab with default profile command`,
			`else`,
			`create window with default profile command`,
			`/bin/bash -c 'copilot --resume session-7'`,
		} {
			if !strings.Contains(script, fragment) {
				t.Fatalf("AppleScript %q does not contain %q", script, fragment)
			}
		}
	})

	t.Run("Darwin Terminal new window constructs a do script command", func(t *testing.T) {
		pathDir := platformRecorderPath(t, recorder, "osascript.exe")
		t.Setenv("PATH", pathDir)
		recordPath := filepath.Join(t.TempDir(), "record.json")
		t.Setenv("DISPATCH_PLATFORM_RECORD", recordPath)

		err := launchDarwinSession(
			ShellInfo{Path: "/bin/fish"},
			"copilot",
			"Terminal.app",
			"",
			true,
		)
		if err != nil {
			t.Fatalf("launchDarwinSession() error: %v", err)
		}

		recording := readPlatformRecording(t, recordPath)
		assertPlatformArgs(t, recording.Args, []string{
			"-e",
			`tell application "Terminal" to do script "/bin/fish -c 'copilot'"`,
		})
	})

	t.Run("Darwin Terminal tab activates and reuses the front window", func(t *testing.T) {
		pathDir := platformRecorderPath(t, recorder, "osascript.exe")
		t.Setenv("PATH", pathDir)
		recordPath := filepath.Join(t.TempDir(), "record.json")
		t.Setenv("DISPATCH_PLATFORM_RECORD", recordPath)

		err := launchDarwinSession(
			ShellInfo{Path: "/bin/bash"},
			"copilot --resume session-8",
			"Terminal.app",
			"",
			false,
		)
		if err != nil {
			t.Fatalf("launchDarwinSession() error: %v", err)
		}

		recording := readPlatformRecording(t, recordPath)
		if len(recording.Args) != 2 || recording.Args[0] != "-e" {
			t.Fatalf("osascript args = %q, want -e and one script", recording.Args)
		}
		script := recording.Args[1]
		for _, fragment := range []string{
			`tell application "Terminal"`,
			`activate`,
			`if (count of windows) > 0 then`,
			`do script "/bin/bash -c 'copilot --resume session-8'" in front window`,
			`else`,
		} {
			if !strings.Contains(script, fragment) {
				t.Fatalf("AppleScript %q does not contain %q", script, fragment)
			}
		}
	})
}

func TestPlatformAPlusWSLWindowsTerminalBehavior(t *testing.T) {
	recorder := buildPlatformRecorder(t)
	pathDir := platformRecorderPath(t, recorder, "wt.exe", "wslpath.exe")
	t.Setenv("PATH", pathDir)
	setWSLDistroForTest(t, "Ubuntu-Test")
	recordPath := filepath.Join(t.TempDir(), "record.json")
	t.Setenv("DISPATCH_PLATFORM_RECORD", recordPath)
	t.Setenv("DISPATCH_PLATFORM_STDOUT", "C:\\Users\\test\\project\r\n")

	err := launchLinuxSession(
		ShellInfo{Path: "/bin/bash"},
		"copilot --resume session-6",
		termWindowsTerminal,
		"/home/test/project",
		LaunchStylePane,
		"right",
	)
	if err != nil {
		t.Fatalf("launchLinuxSession() WSL error: %v", err)
	}

	recording := readPlatformRecording(t, recordPath)
	assertPlatformArgs(t, recording.Args, []string{
		"-w", "0", "split-pane", "-V",
		"--startingDirectory", `C:\Users\test\project`,
		"wsl.exe", "-d", "Ubuntu-Test", "--",
		"/bin/bash", "-c", "copilot --resume session-6",
	})
}

func TestPlatformAPlusFontDetectionBehavior(t *testing.T) {
	recorder := buildPlatformRecorder(t)

	t.Run("Darwin user font directory detects a Nerd Font", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("USERPROFILE", home)
		fontDir := filepath.Join(home, "Library", "Fonts")
		if err := os.MkdirAll(fontDir, 0o755); err != nil {
			t.Fatalf("create Darwin font directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(fontDir, "FiraCodeNerdFont.ttf"), nil, 0o600); err != nil {
			t.Fatalf("write Darwin font fixture: %v", err)
		}

		if !isNerdFontInstalledDarwin() {
			t.Fatal("isNerdFontInstalledDarwin() = false, want true")
		}
	})

	t.Run("Linux user font directory detects a Nerd Font", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("USERPROFILE", home)
		fontDir := filepath.Join(home, ".local", "share", "fonts")
		if err := os.MkdirAll(fontDir, 0o755); err != nil {
			t.Fatalf("create Linux font directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(fontDir, "JetBrainsNerdFont.ttf"), nil, 0o600); err != nil {
			t.Fatalf("write Linux font fixture: %v", err)
		}

		if !isNerdFontInstalledLinux() {
			t.Fatal("isNerdFontInstalledLinux() = false, want true")
		}
	})

	t.Run("Linux falls back to fc-list output", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("USERPROFILE", home)
		pathDir := platformRecorderPath(t, recorder, "fc-list.exe")
		t.Setenv("PATH", pathDir)
		recordPath := filepath.Join(t.TempDir(), "record.json")
		t.Setenv("DISPATCH_PLATFORM_RECORD", recordPath)
		t.Setenv("DISPATCH_PLATFORM_STDOUT", "JetBrainsMono Nerd Font:style=Regular\n")

		if !isNerdFontInstalledLinux() {
			t.Fatal("isNerdFontInstalledLinux() = false, want fc-list fallback detection")
		}
		recording := readPlatformRecording(t, recordPath)
		assertPlatformArgs(t, recording.Args, nil)
	})

	t.Run("Linux reports false when directories and fc-list are unavailable", func(t *testing.T) {
		t.Setenv("USERPROFILE", t.TempDir())
		t.Setenv("PATH", t.TempDir())

		if isNerdFontInstalledLinux() {
			t.Fatal("isNerdFontInstalledLinux() = true, want false")
		}
	})
}

func TestPlatformAPlusOpenURLRejectsMalformedPercentEncoding(t *testing.T) {
	err := OpenURL("https://example.com/%zz")
	if err == nil || !strings.Contains(err.Error(), "invalid URL") {
		t.Fatalf("OpenURL() error = %v, want invalid URL", err)
	}
}

func TestPlatformAPlusWSLAndFocusErrorBehavior(t *testing.T) {
	t.Run("WSL environment detection uses the documented signal", func(t *testing.T) {
		setWSLDistroForTest(t, "Ubuntu-Test")
		if !isWSL() {
			t.Fatal("isWSL() = false with WSL_DISTRO_NAME set")
		}
	})

	t.Run("invalid focus PIDs are rejected before Windows API calls", func(t *testing.T) {
		for _, pid := range []int{0, -1} {
			err := FocusSessionWindow(pid)
			if err == nil || !strings.Contains(err.Error(), "invalid PID") {
				t.Fatalf("FocusSessionWindow(%d) error = %v, want invalid PID", pid, err)
			}
		}
	})

	t.Run("window enumeration returns no handle for an impossible PID", func(t *testing.T) {
		const impossiblePID uint32 = ^uint32(0)
		if hwnd := findWindowForPIDs(map[uint32]struct{}{impossiblePID: {}}); hwnd != 0 {
			t.Fatalf("findWindowForPIDs() = %v, want 0", hwnd)
		}
	})
}
