package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestMain isolates the OS user-config directory for the entire tui test
// binary. Many tests here build a Model (NewModel, NewModelWithQuery, or
// newTestModel) and persist preferences through config.Save — for example
// recordLaunch stamps a frecency entry on every session launch. Without this
// redirect those writes land in the developer's real config file, so running
// "go test ./..." (as "mage install" does) silently overwrites the user's
// %APPDATA%\dispatch\config.json (or ~/.config/dispatch/config.json) with
// defaults plus test data. Pointing the config-dir env var at a throwaway temp
// directory guarantees that no test — present or future — can clobber the
// developer's real settings.
func TestMain(m *testing.M) {
	os.Exit(runIsolated(m))
}

// runIsolated redirects the config directory to a temp dir, runs the package
// tests, and returns the exit code. It is a separate function so the temp dir
// cleanup runs before os.Exit (which would skip deferred calls).
func runIsolated(m *testing.M) int {
	tmp, err := os.MkdirTemp("", "dispatch-tui-config-*")
	if err != nil {
		panic("dispatch tui tests: creating temp config dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	// Redirect the same environment variable os.UserConfigDir() reads on each
	// platform so config.Save/Load resolve inside tmp instead of the real user
	// configuration directory.
	switch runtime.GOOS {
	case "windows":
		os.Setenv("APPDATA", tmp)
	case "darwin":
		// os.UserConfigDir() derives from $HOME on macOS.
		os.Setenv("HOME", tmp)
		os.MkdirAll(filepath.Join(tmp, "Library", "Application Support"), 0o755)
	default:
		os.Setenv("XDG_CONFIG_HOME", tmp)
	}

	return m.Run()
}
