package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ---------------------------------------------------------------------------
// hasNerdFontFiles
// ---------------------------------------------------------------------------

func TestHasNerdFontFiles_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if hasNerdFontFiles(dir) {
		t.Error("hasNerdFontFiles should return false for empty directory")
	}
}

func TestHasNerdFontFiles_NonExistentDir(t *testing.T) {
	t.Parallel()
	if hasNerdFontFiles("/nonexistent/path/xyz123") {
		t.Error("hasNerdFontFiles should return false for non-existent directory")
	}
}

func TestHasNerdFontFiles_WithNerdFont(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "JetBrainsMonoNerdFont-Regular.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if !hasNerdFontFiles(dir) {
		t.Error("hasNerdFontFiles should return true when a nerd font .ttf exists")
	}
}

func TestHasNerdFontFiles_CaseInsensitive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "SomeNERDFont.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if !hasNerdFontFiles(dir) {
		t.Error("hasNerdFontFiles should be case-insensitive for 'nerd'")
	}
}

func TestHasNerdFontFiles_NonTTFIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "NerdFont.otf"))
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if hasNerdFontFiles(dir) {
		t.Error("hasNerdFontFiles should ignore non-.ttf files")
	}
}

func TestHasNerdFontFiles_NonNerdTTFIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "Arial.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if hasNerdFontFiles(dir) {
		t.Error("hasNerdFontFiles should ignore .ttf files without 'nerd' in name")
	}
}

func TestHasNerdFontFiles_TableDriven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		files    []string
		expected bool
	}{
		{"empty dir", nil, false},
		{"nerd ttf", []string{"MyNerdFont.ttf"}, true},
		{"nerd uppercase", []string{"NERD-FONT.TTF"}, true}, // ToLower makes .TTF match .ttf
		{"nerd lowercase ttf", []string{"nerd-mono.ttf"}, true},
		{"only otf", []string{"nerd.otf"}, false},
		{"mixed files with nerd ttf", []string{"readme.md", "font.otf", "JetBrainsNerdMono.ttf"}, true},
		{"ttf without nerd", []string{"Roboto.ttf", "Arial.ttf"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for _, name := range tt.files {
				f, err := os.Create(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				_ = f.Close()
			}
			got := hasNerdFontFiles(dir)
			if got != tt.expected {
				t.Errorf("hasNerdFontFiles() = %v, want %v (files: %v)", got, tt.expected, tt.files)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// IsNerdFontInstalled
// ---------------------------------------------------------------------------

func TestIsNerdFontInstalled_DetectsFontInUserDir(t *testing.T) {
	// Point the per-user font directory at a temp dir that contains a Nerd
	// Font so IsNerdFontInstalled has a deterministic answer regardless of
	// what is actually installed on the host. The dispatcher must route to
	// the OS-appropriate helper and report true.
	home := t.TempDir()
	var fontDir string
	switch runtime.GOOS {
	case "windows":
		t.Setenv("LOCALAPPDATA", home)
		fontDir = filepath.Join(home, "Microsoft", "Windows", "Fonts")
	case "darwin":
		t.Setenv("HOME", home)
		fontDir = filepath.Join(home, "Library", "Fonts")
	default:
		t.Setenv("HOME", home)
		fontDir = filepath.Join(home, ".local", "share", "fonts")
	}
	if err := os.MkdirAll(fontDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fontDir, "JetBrainsMonoNerdFont-Regular.ttf"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !IsNerdFontInstalled() {
		t.Error("IsNerdFontInstalled() = false, want true when a Nerd Font .ttf is present in the user font directory")
	}
}
