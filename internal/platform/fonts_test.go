package platform

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
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
// copyFile
// ---------------------------------------------------------------------------

func TestCopyFile_Basic(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "source.txt")
	dstPath := filepath.Join(dstDir, "dest.txt")

	content := []byte("hello, world!")
	if err := os.WriteFile(srcPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile() error: %v", err)
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("reading dest: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("copyFile content = %q, want %q", string(got), string(content))
	}
}

func TestCopyFile_EmptyFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "empty.txt")
	dstPath := filepath.Join(dstDir, "empty_copy.txt")

	if err := os.WriteFile(srcPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile() error on empty file: %v", err)
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("expected empty file, got size %d", info.Size())
	}
}

func TestCopyFile_NonExistentSource(t *testing.T) {
	dstDir := t.TempDir()
	err := copyFile("/nonexistent/file.txt", filepath.Join(dstDir, "out.txt"))
	if err == nil {
		t.Error("copyFile should fail for non-existent source")
	}
}

func TestCopyFile_InvalidDest(t *testing.T) {
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "source.txt")
	if err := os.WriteFile(srcPath, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := copyFile(srcPath, "/nonexistent/dir/out.txt")
	if err == nil {
		t.Error("copyFile should fail for invalid destination path")
	}
}

// ---------------------------------------------------------------------------
// extractTTF
// ---------------------------------------------------------------------------

func createTestZip(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return zipPath
}

func TestExtractTTF_ExtractsTTFOnly(t *testing.T) {
	zipPath := createTestZip(t, map[string][]byte{
		"font.ttf":    []byte("fake ttf data"),
		"readme.md":   []byte("readme content"),
		"font.otf":    []byte("otf data"),
		"another.ttf": []byte("another ttf"),
	})

	destDir := t.TempDir()
	extracted, err := extractTTF(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractTTF() error: %v", err)
	}
	if len(extracted) != 2 {
		t.Errorf("extracted %d files, want 2 (.ttf only)", len(extracted))
	}
}

func TestExtractTTF_EmptyZip(t *testing.T) {
	zipPath := createTestZip(t, map[string][]byte{})
	destDir := t.TempDir()
	extracted, err := extractTTF(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractTTF() error: %v", err)
	}
	if len(extracted) != 0 {
		t.Errorf("extracted %d files from empty zip, want 0", len(extracted))
	}
}

func TestExtractTTF_NoTTFFiles(t *testing.T) {
	zipPath := createTestZip(t, map[string][]byte{
		"readme.md": []byte("no fonts here"),
		"font.otf":  []byte("otf only"),
	})
	destDir := t.TempDir()
	extracted, err := extractTTF(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractTTF() error: %v", err)
	}
	if len(extracted) != 0 {
		t.Errorf("extracted %d files, want 0 (no .ttf)", len(extracted))
	}
}

func TestExtractTTF_CaseInsensitiveTTF(t *testing.T) {
	zipPath := createTestZip(t, map[string][]byte{
		"Font.TTF":  []byte("uppercase extension"),
		"other.Ttf": []byte("mixed case"),
	})
	destDir := t.TempDir()
	extracted, err := extractTTF(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractTTF() error: %v", err)
	}
	if len(extracted) != 2 {
		t.Errorf("extracted %d files, want 2 (case-insensitive .ttf)", len(extracted))
	}
}

func TestExtractTTF_InvalidZipPath(t *testing.T) {
	destDir := t.TempDir()
	_, err := extractTTF("/nonexistent/file.zip", destDir)
	if err == nil {
		t.Error("extractTTF should fail for non-existent zip")
	}
}

func TestExtractTTF_SubdirTTFExtracted(t *testing.T) {
	// Files in subdirectories should have their base name extracted.
	zipPath := createTestZip(t, map[string][]byte{
		"fonts/subfolder/myfont.ttf": []byte("nested font"),
	})
	destDir := t.TempDir()
	extracted, err := extractTTF(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractTTF() error: %v", err)
	}
	if len(extracted) != 1 {
		t.Fatalf("extracted %d files, want 1", len(extracted))
	}
	// Should be extracted as base name in destDir.
	base := filepath.Base(extracted[0])
	if base != "myfont.ttf" {
		t.Errorf("extracted file base = %q, want %q", base, "myfont.ttf")
	}
}

func TestExtractTTF_ContentPreserved(t *testing.T) {
	content := []byte("this is fake TTF binary data for testing")
	zipPath := createTestZip(t, map[string][]byte{
		"test.ttf": content,
	})
	destDir := t.TempDir()
	extracted, err := extractTTF(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractTTF() error: %v", err)
	}
	if len(extracted) != 1 {
		t.Fatalf("extracted %d files, want 1", len(extracted))
	}
	got, err := os.ReadFile(extracted[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("extracted content mismatch")
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

// ---------------------------------------------------------------------------
// extractTTF — Security: ZIP path traversal prevention
// ---------------------------------------------------------------------------

func TestExtractTTF_PathTraversal_DotDotSlash(t *testing.T) {
	// A zip entry like "../../evil.ttf" must not write outside destDir.
	// extractTTF uses filepath.Base() to strip directory components.
	zipPath := createTestZip(t, map[string][]byte{
		"../../evil.ttf": []byte("traversal attempt"),
	})
	destDir := t.TempDir()
	extracted, err := extractTTF(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractTTF: %v", err)
	}

	absDir, _ := filepath.Abs(destDir)
	for _, f := range extracted {
		abs, _ := filepath.Abs(f)
		if !strings.HasPrefix(abs, absDir) {
			t.Errorf("extracted file %q escapes destDir %q", abs, absDir)
		}
	}
	// The file should still be extracted (as "evil.ttf" in destDir).
	if len(extracted) != 1 {
		t.Fatalf("expected 1 file, got %d", len(extracted))
	}
	if filepath.Base(extracted[0]) != "evil.ttf" {
		t.Errorf("base = %q, want %q", filepath.Base(extracted[0]), "evil.ttf")
	}
}

func TestExtractTTF_PathTraversal_DeepTraversal(t *testing.T) {
	zipPath := createTestZip(t, map[string][]byte{
		"../../../../../tmp/evil.ttf": []byte("deep traversal"),
	})
	destDir := t.TempDir()
	extracted, err := extractTTF(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractTTF: %v", err)
	}

	absDir, _ := filepath.Abs(destDir)
	for _, f := range extracted {
		abs, _ := filepath.Abs(f)
		if !strings.HasPrefix(abs, absDir) {
			t.Errorf("extracted file %q escapes destDir %q", abs, absDir)
		}
	}
}

func TestExtractTTF_PathTraversal_AbsolutePath(t *testing.T) {
	zipPath := createTestZip(t, map[string][]byte{
		"/etc/fonts/evil.ttf": []byte("absolute path injection"),
	})
	destDir := t.TempDir()
	extracted, err := extractTTF(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractTTF: %v", err)
	}

	absDir, _ := filepath.Abs(destDir)
	for _, f := range extracted {
		abs, _ := filepath.Abs(f)
		if !strings.HasPrefix(abs, absDir) {
			t.Errorf("extracted file %q escapes destDir %q", abs, absDir)
		}
	}
}

func TestExtractTTF_PathTraversal_BackslashSeparator(t *testing.T) {
	// Windows-style path separators in zip entries.
	zipPath := createTestZip(t, map[string][]byte{
		`..\..\evil.ttf`: []byte("backslash traversal"),
	})
	destDir := t.TempDir()
	extracted, err := extractTTF(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractTTF: %v", err)
	}

	absDir, _ := filepath.Abs(destDir)
	for _, f := range extracted {
		abs, _ := filepath.Abs(f)
		if !strings.HasPrefix(abs, absDir) {
			t.Errorf("extracted file %q escapes destDir %q", abs, absDir)
		}
	}
}

// ---------------------------------------------------------------------------
// extractTTF — Security: special name handling
// ---------------------------------------------------------------------------

func TestExtractTTF_SkipsDotAndDotDotNames(t *testing.T) {
	// Entries whose base name is "." or ".." must be skipped entirely.
	// These are not valid TTF files and the guard in extractTTF should
	// catch them even if they somehow ended up with a .ttf suffix earlier.
	zipPath := createTestZip(t, map[string][]byte{
		".":  []byte("dot entry"),
		"..": []byte("dotdot entry"),
	})
	destDir := t.TempDir()
	extracted, err := extractTTF(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractTTF: %v", err)
	}
	if len(extracted) != 0 {
		t.Errorf("expected 0 extracted files for ./.. entries, got %d", len(extracted))
	}
}

func TestExtractTTF_SkipsEmptyName(t *testing.T) {
	// An entry with an empty name should be silently skipped.
	// We can't easily create a zip entry with a truly empty name via
	// the Go stdlib, but we can verify the guard logic works by testing
	// extractTTF with entries whose Base resolves to the directory guard.
	zipPath := createTestZip(t, map[string][]byte{
		"subdir/": {}, // directory entry
	})
	destDir := t.TempDir()
	extracted, err := extractTTF(zipPath, destDir)
	if err != nil {
		t.Fatalf("extractTTF: %v", err)
	}
	if len(extracted) != 0 {
		t.Errorf("expected 0 extracted files for directory entries, got %d", len(extracted))
	}
}

// ---------------------------------------------------------------------------
// extractTTF — Security: oversized file rejection
// ---------------------------------------------------------------------------

// createTestZipLargeFile creates a zip containing a single file of the given
// size. Uses zero-filled content which compresses efficiently for fast tests.
func createTestZipLargeFile(t *testing.T, name string, size int64) string {
	t.Helper()
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	fw, err := w.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	chunk := make([]byte, 64*1024)
	var written int64
	for written < size {
		remaining := size - written
		toWrite := chunk
		if remaining < int64(len(chunk)) {
			toWrite = chunk[:remaining]
		}
		n, err := fw.Write(toWrite)
		if err != nil {
			t.Fatal(err)
		}
		written += int64(n)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return zipPath
}

func TestExtractTTF_RejectsOversizedFile(t *testing.T) {
	// Use small limits so the test doesn't need tens of MiBs of disk space.
	const testMaxFile int64 = 1024 // 1 KiB
	zipPath := createTestZipLargeFile(t, "huge.ttf", testMaxFile+1)
	destDir := t.TempDir()

	_, err := extractTTFWithLimits(zipPath, destDir, testMaxFile, 10*testMaxFile)
	if err == nil {
		t.Fatal("extractTTFWithLimits should reject files exceeding the per-file limit")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Errorf("error should mention size limit, got: %v", err)
	}
}

func TestExtractTTF_AcceptsFileAtLimit(t *testing.T) {
	// A file exactly at the per-file limit should be accepted.
	const testMaxFile int64 = 1024 // 1 KiB
	zipPath := createTestZipLargeFile(t, "exact.ttf", testMaxFile)
	destDir := t.TempDir()

	extracted, err := extractTTFWithLimits(zipPath, destDir, testMaxFile, 10*testMaxFile)
	if err != nil {
		t.Fatalf("extractTTFWithLimits should accept file exactly at limit: %v", err)
	}
	if len(extracted) != 1 {
		t.Errorf("expected 1 extracted file, got %d", len(extracted))
	}
}

// ---------------------------------------------------------------------------
// extractTTF — Security: total size limit
// ---------------------------------------------------------------------------

// createTestZipMultiLargeFiles creates a zip with count files each of the
// given size. All names end in .ttf.
func createTestZipMultiLargeFiles(t *testing.T, count int, sizeEach int64) string {
	t.Helper()
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	chunk := make([]byte, 64*1024)

	for i := 0; i < count; i++ {
		name := filepath.Join("fonts", strings.ReplaceAll(
			strings.ReplaceAll(filepath.Base(t.Name()), "/", "_"),
			"\\", "_",
		)+string(rune('A'+i))+".ttf")
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		var written int64
		for written < sizeEach {
			remaining := sizeEach - written
			toWrite := chunk
			if remaining < int64(len(chunk)) {
				toWrite = chunk[:remaining]
			}
			n, err := fw.Write(toWrite)
			if err != nil {
				t.Fatal(err)
			}
			written += int64(n)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return zipPath
}

func TestExtractTTF_RejectsTotalSizeExceeded(t *testing.T) {
	// Use small limits: 11 files × 500 bytes = 5500 bytes exceeds the 5 KiB
	// total limit, without needing hundreds of MiBs of disk space.
	const testMaxFile int64 = 1024      // 1 KiB per file
	const testMaxTotal int64 = 5 * 1024 // 5 KiB total
	const perFile int64 = 500
	zipPath := createTestZipMultiLargeFiles(t, 11, perFile)
	destDir := t.TempDir()

	_, err := extractTTFWithLimits(zipPath, destDir, testMaxFile, testMaxTotal)
	if err == nil {
		t.Fatal("extractTTFWithLimits should reject when total extracted size exceeds limit")
	}
	if !strings.Contains(err.Error(), "total extracted size exceeds") {
		t.Errorf("error should mention total size limit, got: %v", err)
	}
}
