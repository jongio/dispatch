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
//
// Inside WSL the Copilot CLI runs on the Windows side, so the session store
// lives under the Windows user profile (e.g. /mnt/c/Users/<user>). When the
// database is not found at the Linux home directory, we fall back to scanning
// the Windows user-profile directories exposed via the WSL mount.
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
	p := filepath.Join(home, sessionStoreRel)

	// Inside WSL the session store is created by Copilot CLI on the Windows
	// side. If the database does not exist at the Linux home, try the
	// Windows user profile directory via the WSL mount.
	if runtime.GOOS == "linux" {
		if _, statErr := os.Stat(p); statErr != nil && isWSL() {
			if winPath := findWindowsSessionStore(); winPath != "" {
				return winPath, nil
			}
		}
	}

	return p, nil
}

// wslMountRoot is the default directory where WSL mounts Windows drives.
const wslMountRoot = "/mnt/c/Users"

// isWSL reports whether the current process is running inside Windows
// Subsystem for Linux.
func isWSL() bool {
	// WSL2 (and recent WSL1) always set WSL_DISTRO_NAME.
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	// Older WSL1 may not set the env var; fall back to /proc/version.
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// findWindowsSessionStore scans Windows user-profile directories under the
// default WSL mount for a Copilot session store database.
func findWindowsSessionStore() string {
	entries, err := os.ReadDir(wslMountRoot)
	if err != nil {
		return ""
	}

	// Skip well-known non-user directories.
	skip := map[string]struct{}{
		"public":       {},
		"default":      {},
		"default user": {},
		"all users":    {},
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, ok := skip[strings.ToLower(e.Name())]; ok {
			continue
		}
		candidate := filepath.Join(wslMountRoot, e.Name(), sessionStoreRel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
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
