package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestMain isolates the OS user-config directory for the entire config test
// binary. Save and Reset write to and delete the real config file, and this
// package's tests exercise both. Individual tests already redirect the config
// dir with helpers (withTempConfigDir, setupTempConfig), but this package-level
// baseline guarantees that even a test which forgets that guard can never touch
// the developer's real config file — the failure mode that let "go test ./..."
// (via "mage install") wipe the user's saved settings. Per-test t.Setenv calls
// still override this baseline (and restore back to it), so path and error-path
// tests keep working unchanged.
func TestMain(m *testing.M) {
	os.Exit(runWithIsolatedConfigDir(m))
}

// runWithIsolatedConfigDir redirects the config directory to a temp dir, runs
// the package tests, and returns the exit code. It is a separate function so
// the temp dir cleanup runs before os.Exit (which would skip deferred calls).
func runWithIsolatedConfigDir(m *testing.M) int {
	tmp, err := os.MkdirTemp("", "dispatch-config-test-*")
	if err != nil {
		panic("dispatch config tests: creating temp config dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	// Redirect the same environment variable os.UserConfigDir() reads on each
	// platform so configPath() resolves inside tmp instead of the real user
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
