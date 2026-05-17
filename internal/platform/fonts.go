package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// IsNerdFontInstalled checks whether any Nerd Font .ttf file is present in
// the OS-appropriate user or system font directories.
func IsNerdFontInstalled() bool {
	switch runtime.GOOS {
	case "windows":
		return isNerdFontInstalledWindows()
	case "darwin":
		return isNerdFontInstalledDarwin()
	default:
		return isNerdFontInstalledLinux()
	}
}

// ---------------------------------------------------------------------------
// Detection helpers
// ---------------------------------------------------------------------------

// checkNerdFontDirs returns true if any directory in dirs contains a Nerd Font
// file. Empty strings are silently skipped.
func checkNerdFontDirs(dirs []string) bool {
	for _, d := range dirs {
		if d != "" && hasNerdFontFiles(d) {
			return true
		}
	}
	return false
}

func isNerdFontInstalledWindows() bool {
	var dirs []string

	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		dirs = append(dirs, filepath.Join(localAppData, "Microsoft", "Windows", "Fonts"))
	}

	winDir := os.Getenv("WINDIR")
	if winDir == "" {
		winDir = `C:\Windows`
	}
	dirs = append(dirs, filepath.Join(winDir, "Fonts"))

	return checkNerdFontDirs(dirs)
}

func isNerdFontInstalledDarwin() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	return checkNerdFontDirs([]string{
		filepath.Join(home, "Library", "Fonts"),
		"/Library/Fonts",
	})
}

func isNerdFontInstalledLinux() bool {
	home, _ := os.UserHomeDir()

	dirs := []string{
		filepath.Join(home, ".local", "share", "fonts"),
		"/usr/share/fonts",
		"/usr/local/share/fonts",
	}
	if checkNerdFontDirs(dirs) {
		return true
	}

	// Try fc-list as a fallback.
	out, err := exec.Command("fc-list").Output()
	if err == nil && strings.Contains(string(out), "Nerd") {
		return true
	}
	return false
}

// hasNerdFontFiles returns true if the directory contains any .ttf file with
// "nerd" in its name (case-insensitive).
func hasNerdFontFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.Contains(name, "nerd") && strings.HasSuffix(name, ".ttf") {
			return true
		}
	}
	return false
}
