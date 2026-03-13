// Package platform provides cross-platform path resolution, shell detection,
// and session launching for dispatch.
package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	appName         = "dispatch"
	sessionStoreRel = ".copilot/session-store.db"
)

// SessionStorePath returns the absolute path to the Copilot CLI session store
// SQLite database (~/.copilot/session-store.db).
//
// If the DISPATCH_DB environment variable is set, its value is returned
// instead. This allows tests and demo mode to point at a custom database.
func SessionStorePath() (string, error) {
	if override := os.Getenv("DISPATCH_DB"); override != "" {
		p := filepath.Clean(override)
		// Reject UNC paths on Windows to prevent outbound SMB auth.
		if runtime.GOOS == "windows" && strings.HasPrefix(p, `\\`) {
			return "", fmt.Errorf("DISPATCH_DB must be a local path (UNC paths are not allowed)")
		}
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, sessionStoreRel), nil
}

// ConfigDir returns the OS-appropriate configuration directory for
// dispatch:
//   - Windows: %APPDATA%\dispatch
//   - macOS:   ~/Library/Application Support/dispatch
//   - Linux:   ~/.config/dispatch
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving config directory: %w", err)
	}
	return filepath.Join(base, appName), nil
}
