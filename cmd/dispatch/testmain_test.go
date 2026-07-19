package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestMain isolates the OS user-config directory for the entire cmd/dispatch
// test binary. Some tests exercise commands that read, write, or delete the
// real config file — for example TestHandleArgs_ClearCache runs the
// "--clear-cache" path, which calls config.Reset and deletes config.json.
// Without this redirect that runs against the developer's real config file, so
// "go test ./..." (as "mage install" does) silently wipes the user's
// %APPDATA%\dispatch\config.json (or ~/.config/dispatch/config.json). Pointing
// the config-dir env var at a throwaway temp directory guarantees that no test
// — present or future — can touch the user's real settings.
func TestMain(m *testing.M) {
	os.Exit(runWithIsolatedConfig(m))
}

// runWithIsolatedConfig redirects the config directory to a temp dir, runs the
// package tests, and returns the exit code. It is a separate function so the
// temp dir cleanup runs before os.Exit (which would skip deferred calls).
func runWithIsolatedConfig(m *testing.M) int {
	tmp, err := os.MkdirTemp("", "dispatch-cli-config-*")
	if err != nil {
		panic("dispatch cli tests: creating temp config dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	// Redirect the same environment variable os.UserConfigDir() reads on each
	// platform so config.Save/Load/Reset resolve inside tmp instead of the real
	// user configuration directory.
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
