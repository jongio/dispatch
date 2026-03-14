//go:build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// staleBinaryThreshold is the maximum acceptable age for a freshly-built
// binary.  If the binary on disk is older than this it is considered stale.
const staleBinaryThreshold = 30 * time.Second

// deadcodeAllowlist contains functions reported by deadcode that are not
// genuinely dead: build-tag stubs, interface implementations, and functions
// called only from files with non-default build tags.
var deadcodeAllowlist = []string{
	"launchInPlaceUnix",            // build-tag stub (launch_windows.go)
	"colorSchemeRef.UnmarshalJSON", // Windows-only (wttheme.go)
	"colorSchemeRef.resolve",       // Windows-only (wttheme.go)
	"DetectWTColorScheme",          // Windows-only (wttheme.go)
	"wtSettingsPaths",              // Windows-only (wttheme.go)
	"parseWTSettings",              // Windows-only (wttheme.go)
	"parseWTSettingsData",          // Windows-only (wttheme.go)
	"keyMap.ShortHelp",             // key.Map interface impl
	"keyMap.FullHelp",              // key.Map interface impl
	"CurrentTheme",                 // called from screenshot.go (//go:build screenshots)
}

const (
	// coverProfile is the file name for the raw coverage data.
	coverProfile = "coverage.out"
	// coverHTML is the file name for the generated HTML coverage report.
	coverHTML = "coverage.html"
)

var (
	binName    = "dispatch-dev"
	mainPkg    = "./cmd/dispatch/"
	versionVar = "github.com/jongio/dispatch/internal/tui.Version"
)

// Default target when running `mage` with no args.
var Default = Install

// Install runs tests, kills stale processes, builds the dev binary, and ensures it's in PATH.
func Install() error {
	if err := Test(); err != nil {
		return err
	}
	killStale()
	if err := Build(); err != nil {
		return err
	}
	if err := ensurePath(); err != nil {
		return err
	}
	return verify()
}

// Test runs all unit tests with race detection and shuffled order.
func Test() error {
	fmt.Println("\n=== Running tests ===")
	args := []string{"test"}
	if raceDetectorAvailable() {
		os.Setenv("CGO_ENABLED", "1")
		args = append(args, "-race")
		fmt.Println("   Race detector: enabled")
	} else {
		fmt.Println("   Race detector: skipped (requires gcc/CGO on Windows)")
	}
	args = append(args, "-shuffle=on", "./...", "-count=1")
	return run("go", args...)
}

// TestWSL runs tests under WSL Linux to exercise Unix-specific code paths.
func TestWSL() error {
	fmt.Println("\n=== Running tests in WSL ===")
	if _, err := exec.LookPath("wsl"); err != nil {
		fmt.Println("   Skipped (WSL not available)")
		return nil
	}
	wslPath, err := windowsToWSLPath(projectDir())
	if err != nil {
		return fmt.Errorf("converting path for WSL: %w", err)
	}
	cmd := fmt.Sprintf("cd %s && go test ./... -count=1", wslPath)
	return run("wsl", "bash", "-c", cmd)
}

// CoverageReport generates an HTML coverage report.
func CoverageReport() error {
	fmt.Println("\n=== Generating coverage report ===")
	if err := run("go", "test", "./internal/...", "-coverprofile="+coverProfile, "-covermode=atomic"); err != nil {
		return fmt.Errorf("coverage run: %w", err)
	}
	if err := run("go", "tool", "cover", "-html="+coverProfile, "-o", coverHTML); err != nil {
		return fmt.Errorf("coverage report: %w", err)
	}
	fmt.Printf("   Coverage report: %s\n", coverHTML)
	return nil
}

// Vet runs go vet on all packages.
func Vet() error {
	fmt.Println("\n=== Running vet ===")
	return run("go", "vet", "./...")
}

// Build compiles the dev binary with version info into bin/.
func Build() error {
	fmt.Println("\n=== Building binary ===")
	binDir := filepath.Join(projectDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("creating bin directory: %w", err)
	}

	version := devVersion()
	ldflags := fmt.Sprintf("-X %s=%s", versionVar, version)
	outPath := filepath.Join(binDir, binaryName())

	if err := run("go", "build", "-ldflags", ldflags, "-o", outPath, mainPkg); err != nil {
		return err
	}
	fmt.Printf("   Version: %s\n", version)
	return nil
}

// Preflight runs all pre-commit checks: format, tidy, vet, lint, build, test,
// race detection, WSL tests, vulnerability scan, strict formatting, dead code
// detection, and install verification. If preflight passes, CI will pass.
func Preflight() error {
	fmt.Println("\n=== 1/12 Formatting ===")
	if err := fmtSources(); err != nil {
		return fmt.Errorf("format: %w", err)
	}

	fmt.Println("\n=== 2/12 Tidying modules ===")
	if err := run("go", "mod", "tidy"); err != nil {
		return fmt.Errorf("mod tidy: %w", err)
	}

	fmt.Println("\n=== 3/12 Vetting ===")
	if err := run("go", "vet", "./..."); err != nil {
		return fmt.Errorf("vet: %w", err)
	}

	fmt.Println("\n=== 4/12 Linting ===")
	if _, err := exec.LookPath("golangci-lint"); err == nil {
		if out, err := cmdOutput("golangci-lint", "version"); err == nil {
			if !strings.Contains(out, "golangci-lint has version 2.") {
				fmt.Printf("   WARNING: golangci-lint v2 expected, got: %s\n", strings.TrimSpace(out))
			}
		}
		if err := run("golangci-lint", "run"); err != nil {
			return fmt.Errorf("lint: %w", err)
		}
	} else {
		fmt.Println("   Skipped (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)")
	}

	fmt.Println("\n=== 5/12 Building ===")
	if err := run("go", "build", "./..."); err != nil {
		return fmt.Errorf("build: %w", err)
	}

	fmt.Println("\n=== 6/12 Testing ===")
	if err := run("go", "test", "./...", "-count=1"); err != nil {
		return fmt.Errorf("test: %w", err)
	}

	fmt.Println("\n=== 7/12 Testing (race detector) ===")
	if err := run("go", "test", "-race", "./...", "-count=1"); err != nil {
		return fmt.Errorf("race test: %w", err)
	}

	fmt.Println("\n=== 8/12 Testing (WSL) ===")
	if err := TestWSL(); err != nil {
		return fmt.Errorf("WSL test: %w", err)
	}

	fmt.Println("\n=== 9/12 Vulnerability scan ===")
	if _, err := exec.LookPath("govulncheck"); err == nil {
		if err := run("govulncheck", "./..."); err != nil {
			return fmt.Errorf("vulncheck: %w", err)
		}
	} else {
		fmt.Println("   Skipped (install: go install golang.org/x/vuln/cmd/govulncheck@latest)")
	}

	fmt.Println("\n=== 10/12 Strict formatting (gofumpt) ===")
	if _, err := exec.LookPath("gofumpt"); err == nil {
		out, _ := cmdOutput("gofumpt", "-l", ".")
		if files := strings.TrimSpace(out); files != "" {
			return fmt.Errorf("gofumpt: files need formatting:\n%s", files)
		}
	} else {
		fmt.Println("   Skipped (install: go install mvdan.cc/gofumpt@latest)")
	}

	fmt.Println("\n=== 11/12 Dead code detection ===")
	if _, err := exec.LookPath("deadcode"); err == nil {
		if err := runDeadcode(); err != nil {
			return err
		}
	} else {
		fmt.Println("   Skipped (install: go install golang.org/x/tools/cmd/deadcode@latest)")
	}

	fmt.Println("\n=== 12/12 Install verification ===")
	if err := Install(); err != nil {
		return fmt.Errorf("install: %w", err)
	}

	fmt.Println("\n=== All 12/12 preflight checks passed — ready to commit ===")
	return nil
}

// Fmt formats all Go source files.
func Fmt() error {
	fmt.Println("=== Formatting ===")
	return fmtSources()
}

// Lint runs golangci-lint if available, otherwise falls back to go vet.
func Lint() error {
	fmt.Println("\n=== Linting ===")
	if _, err := exec.LookPath("golangci-lint"); err == nil {
		return run("golangci-lint", "run")
	}
	fmt.Println("   golangci-lint not found, using go vet")
	return run("go", "vet", "./...")
}

// Clean removes the bin/ directory.
func Clean() error {
	fmt.Println("=== Cleaning ===")
	return os.RemoveAll(filepath.Join(projectDir(), "bin"))
}

// --- helpers ---

func fmtSources() error {
	out, _ := cmdOutput("gofmt", "-l", ".")
	files := strings.TrimSpace(out)
	if files == "" {
		fmt.Println("   All files formatted")
		return nil
	}
	var unformatted []string
	for _, f := range strings.Split(files, "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			unformatted = append(unformatted, f)
		}
	}
	fmt.Printf("   Formatting %d file(s):\n", len(unformatted))
	for _, f := range unformatted {
		fmt.Printf("     %s\n", f)
	}
	return run("gofmt", "-w", ".")
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return binName + ".exe"
	}
	return binName
}

func projectDir() string {
	dir, _ := os.Getwd()
	return dir
}

func devVersion() string {
	hash, _ := cmdOutput("git", "rev-parse", "--short", "HEAD")
	ts := time.Now().Format("20060102-150405")
	return fmt.Sprintf("dev-%s-%s", strings.TrimSpace(hash), ts)
}

func killStale() {
	fmt.Println("\n=== Killing stale processes ===")
	if runtime.GOOS == "windows" {
		script := fmt.Sprintf(`Get-Process %s -ErrorAction SilentlyContinue | Stop-Process -Force`, binName)
		exec.Command("powershell", "-NoProfile", "-Command", script).Run()
	} else {
		exec.Command("pkill", "-f", binaryName()).Run()
	}
	time.Sleep(500 * time.Millisecond)
}

func ensurePath() error {
	binDir := filepath.Join(projectDir(), "bin")

	if runtime.GOOS != "windows" {
		path := os.Getenv("PATH")
		if !strings.Contains(path, binDir) {
			fmt.Printf("NOTE: Add %s to your PATH:\n  export PATH=\"%s:$PATH\"\n", binDir, binDir)
		}
		return nil
	}

	// Windows: ensure bin/ is in persistent PATH
	machinePath, _ := cmdOutput("powershell", "-NoProfile", "-Command",
		`[Environment]::GetEnvironmentVariable('Path','Machine')`)
	machinePath = strings.TrimSpace(machinePath)

	if containsPath(machinePath, binDir) {
		// Already registered; just make sure the current session has it
		ensureSessionPath(binDir)
		return nil
	}

	fmt.Printf("\n=== Adding %s to system PATH ===\n", binDir)
	newPath := binDir + ";" + machinePath
	err := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`[Environment]::SetEnvironmentVariable('Path','%s','Machine')`, newPath)).Run()
	if err != nil {
		fmt.Println("   Machine PATH failed (need admin), trying User PATH...")
		userPath, _ := cmdOutput("powershell", "-NoProfile", "-Command",
			`[Environment]::GetEnvironmentVariable('Path','User')`)
		userPath = strings.TrimSpace(userPath)
		if !containsPath(userPath, binDir) {
			exec.Command("powershell", "-NoProfile", "-Command",
				fmt.Sprintf(`[Environment]::SetEnvironmentVariable('Path','%s;%s','User')`, binDir, userPath)).Run()
		}
	}
	ensureSessionPath(binDir)
	return nil
}

func containsPath(pathList, dir string) bool {
	return strings.Contains(strings.ToLower(pathList), strings.ToLower(dir))
}

func ensureSessionPath(binDir string) {
	current := os.Getenv("Path")
	if !containsPath(current, binDir) {
		os.Setenv("Path", binDir+";"+current)
	}
}

func verify() error {
	outPath := filepath.Join(projectDir(), "bin", binaryName())
	info, err := os.Stat(outPath)
	if err != nil {
		return fmt.Errorf("binary not found after build at %s: %w", outPath, err)
	}

	age := time.Since(info.ModTime())
	if age > staleBinaryThreshold {
		return fmt.Errorf("%s seems stale (built %s, %.0fs ago)", binaryName(), info.ModTime().Format(time.DateTime), age.Seconds())
	}

	resolved, err := exec.LookPath(binaryName())
	if err != nil {
		return fmt.Errorf("%s not found in PATH: %w", binaryName(), err)
	}
	resolvedAbs, err := filepath.Abs(resolved)
	if err != nil {
		return fmt.Errorf("resolving actual binary path: %w", err)
	}
	expectedAbs, err := filepath.Abs(outPath)
	if err != nil {
		return fmt.Errorf("resolving expected binary path: %w", err)
	}
	if !strings.EqualFold(resolvedAbs, expectedAbs) {
		return fmt.Errorf("PATH resolves to %s, expected %s — another binary may be shadowing", resolvedAbs, expectedAbs)
	}

	fmt.Printf("\n✅ %s installed\n", binaryName())
	fmt.Printf("   Path:  %s\n", outPath)
	fmt.Printf("   Built: %s\n", info.ModTime().Format(time.DateTime))
	return nil
}

// runDeadcode executes `deadcode ./...` and filters the output against
// deadcodeAllowlist.  Only genuinely dead functions cause a failure.
func runDeadcode() error {
	// deadcode writes findings to stdout and exits 0 regardless.
	out, err := cmdOutput("deadcode", "./...")
	if err != nil {
		return fmt.Errorf("deadcode: %w", err)
	}

	var genuine []string
	allowed := 0
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isAllowlisted(line) {
			allowed++
			continue
		}
		genuine = append(genuine, line)
	}

	if len(genuine) > 0 {
		fmt.Println("   Unexpected dead code found:")
		for _, g := range genuine {
			fmt.Printf("     %s\n", g)
		}
		return fmt.Errorf("deadcode: %d genuine finding(s) (update deadcodeAllowlist if false positive)", len(genuine))
	}

	fmt.Printf("   OK (%d known exclusions)\n", allowed)
	return nil
}

// isAllowlisted reports whether a deadcode output line matches a function in
// the deadcodeAllowlist.  Each deadcode line ends with "unreachable func: <name>".
func isAllowlisted(line string) bool {
	for _, name := range deadcodeAllowlist {
		if strings.HasSuffix(line, ": "+name) {
			return true
		}
	}
	return false
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = projectDir()
	return cmd.Run()
}

func cmdOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = projectDir()
	out, err := cmd.Output()
	return string(out), err
}

func raceDetectorAvailable() bool {
	if runtime.GOOS != "windows" {
		return true
	}
	_, err := exec.LookPath("gcc")
	return err == nil
}

func windowsToWSLPath(winPath string) (string, error) {
	if len(winPath) < 2 || winPath[1] != ':' {
		return "", fmt.Errorf("unexpected Windows path format: %s", winPath)
	}
	drive := strings.ToLower(string(winPath[0]))
	rest := strings.ReplaceAll(winPath[2:], "\\", "/")
	return fmt.Sprintf("/mnt/%s%s", drive, rest), nil
}
