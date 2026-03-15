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

func isNerdFontInstalledWindows() bool {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		userFonts := filepath.Join(localAppData, "Microsoft", "Windows", "Fonts")
		if hasNerdFontFiles(userFonts) {
			return true
		}
	}
	winDir := os.Getenv("WINDIR")
	if winDir == "" {
		winDir = `C:\Windows`
	}
	return hasNerdFontFiles(filepath.Join(winDir, "Fonts"))
}

func isNerdFontInstalledDarwin() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	if hasNerdFontFiles(filepath.Join(home, "Library", "Fonts")) {
		return true
	}
	return hasNerdFontFiles("/Library/Fonts")
}

func isNerdFontInstalledLinux() bool {
	home, err := os.UserHomeDir()
	if err == nil {
		if hasNerdFontFiles(filepath.Join(home, ".local", "share", "fonts")) {
			return true
		}
	}
	for _, dir := range []string{"/usr/share/fonts", "/usr/local/share/fonts"} {
		if hasNerdFontFiles(dir) {
			return true
		}
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
