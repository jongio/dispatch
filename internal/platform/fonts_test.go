package platform

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// hasNerdFontFiles
// ---------------------------------------------------------------------------

func TestHasNerdFontFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	if hasNerdFontFiles(dir) {
		t.Error("hasNerdFontFiles should return false for empty directory")
	}
}

func TestHasNerdFontFiles_NonExistentDir(t *testing.T) {
	if hasNerdFontFiles("/nonexistent/path/xyz123") {
		t.Error("hasNerdFontFiles should return false for non-existent directory")
	}
}

func TestHasNerdFontFiles_WithNerdFont(t *testing.T) {
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
// IsNerdFontInstalled (smoke test)
// ---------------------------------------------------------------------------

func TestIsNerdFontInstalled_ReturnsBool(t *testing.T) {
	// Result depends on system; verify no crash and log result for visibility.
	installed := IsNerdFontInstalled()
	t.Logf("IsNerdFontInstalled() = %v", installed)
}
