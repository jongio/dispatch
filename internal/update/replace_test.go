package update

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeFakeBin(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("fake-binary-content"), 0o755); err != nil {
		t.Fatalf("writeFakeBin %s: %v", path, err)
	}
}

func TestReplaceBinary_RoutesToPlatformFunction(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "newsrc")
	writeFakeBin(t, src)
	err := replaceBinary(src)
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "resolving executable path") {
		t.Fatalf("path resolution should succeed, got: %v", err)
	}
}

func TestReplaceBinary_MissingSourceFile(t *testing.T) {
	err := replaceBinary(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Fatal("expected error for missing source binary")
	}
}

func TestReplaceWindows_HappyPath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new.exe")
	writeFakeBin(t, src)
	exeName := "dispatch.exe"
	writeFakeBin(t, filepath.Join(dir, exeName))
	if err := replaceWindows(src, dir, exeName); err != nil {
		t.Fatalf("replaceWindows: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, exeName))
	if err != nil {
		t.Fatalf("reading replaced exe: %v", err)
	}
	if string(got) != "fake-binary-content" {
		t.Fatalf("replaced exe has wrong content: %q", got)
	}
}

func TestReplaceWindows_TargetExeNotFound(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new.exe")
	writeFakeBin(t, src)
	err := replaceWindows(src, dir, "dispatch.exe")
	if err == nil {
		t.Fatal("expected error when target exe missing")
	}
	if !strings.Contains(err.Error(), "renaming current binary") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReplaceWindows_InvokedAsAliasNoCompanion(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new.exe")
	writeFakeBin(t, src)
	exeName := "disp.exe"
	writeFakeBin(t, filepath.Join(dir, exeName))
	if err := replaceWindows(src, dir, exeName); err != nil {
		t.Fatalf("replaceWindows as alias: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, exeName))
	if string(got) != "fake-binary-content" {
		t.Fatalf("alias exe not updated: %q", got)
	}
}

func TestReplaceWindows_OldSuffixCleanup(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new.exe")
	writeFakeBin(t, src)
	exeName := "dispatch.exe"
	exePath := filepath.Join(dir, exeName)
	writeFakeBin(t, exePath)
	oldPath := exePath + ".old"
	writeFakeBin(t, oldPath)
	if err := replaceWindows(src, dir, exeName); err != nil {
		t.Fatalf("replaceWindows with pre-existing .old: %v", err)
	}
	if _, err := os.Stat(oldPath); err == nil {
		t.Fatal(".old file should have been cleaned up")
	}
}

func TestReplaceWindows_SequentialUpdates(t *testing.T) {
	dir := t.TempDir()
	exeName := "dispatch.exe"
	writeFakeBin(t, filepath.Join(dir, exeName))
	for i := 0; i < 2; i++ {
		src := filepath.Join(dir, "new.exe")
		content := []byte("version-" + string(rune('A'+i)))
		if err := os.WriteFile(src, content, 0o755); err != nil {
			t.Fatalf("write source %d: %v", i, err)
		}
		if err := replaceWindows(src, dir, exeName); err != nil {
			t.Fatalf("replaceWindows iteration %d: %v", i, err)
		}
	}
	got, _ := os.ReadFile(filepath.Join(dir, exeName))
	if string(got) != "version-B" {
		t.Fatalf("expected version-B, got %q", got)
	}
}

func TestReplaceWindows_CompanionUpdated(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new.exe")
	writeFakeBin(t, src)
	exeName := "dispatch.exe"
	writeFakeBin(t, filepath.Join(dir, exeName))
	writeFakeBin(t, filepath.Join(dir, "disp.exe"))
	if err := replaceWindows(src, dir, exeName); err != nil {
		t.Fatalf("replaceWindows: %v", err)
	}
	for _, name := range []string{"dispatch.exe", "disp.exe"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if string(got) != "fake-binary-content" {
			t.Fatalf("%s not updated: %q", name, got)
		}
	}
}

func TestReplaceUnix_HappyPath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "newsrc")
	writeFakeBin(t, src)
	exePath := filepath.Join(dir, "dispatch")
	writeFakeBin(t, exePath)
	if err := replaceUnix(src, exePath); err != nil {
		t.Fatalf("replaceUnix: %v", err)
	}
	got, _ := os.ReadFile(exePath)
	if string(got) != "fake-binary-content" {
		t.Fatalf("exe not updated: %q", got)
	}
	if _, err := os.Stat(exePath + ".new"); err == nil {
		t.Fatal(".new temp file should not remain")
	}
}

func TestReplaceUnix_SourceNotFound(t *testing.T) {
	dir := t.TempDir()
	exePath := filepath.Join(dir, "dispatch")
	writeFakeBin(t, exePath)
	err := replaceUnix(filepath.Join(dir, "nonexistent"), exePath)
	if err == nil {
		t.Fatal("expected error for missing source")
	}
	if !strings.Contains(err.Error(), "opening new binary") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReplaceUnix_TargetInNonexistentDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "newsrc")
	writeFakeBin(t, src)
	err := replaceUnix(src, filepath.Join(dir, "nodir", "dispatch"))
	if err == nil {
		t.Fatal("expected error for nonexistent target directory")
	}
	if !strings.Contains(err.Error(), "creating temp file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReplaceUnix_TempFileCleanedUp(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "newsrc")
	writeFakeBin(t, src)
	exePath := filepath.Join(dir, "dispatch")
	writeFakeBin(t, exePath)
	if err := replaceUnix(src, exePath); err != nil {
		t.Fatalf("replaceUnix: %v", err)
	}
	if _, err := os.Stat(exePath + ".new"); err == nil {
		t.Fatal(".new temp file should be cleaned up")
	}
}

func TestRenameWithRetry_HappyPath(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "a.txt")
	writeFakeBin(t, old)
	dst := filepath.Join(dir, "b.txt")
	if err := renameWithRetry(old, dst); err != nil {
		t.Fatalf("renameWithRetry: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatal("renamed file should exist")
	}
	if _, err := os.Stat(old); err == nil {
		t.Fatal("old path should not exist")
	}
}

func TestRenameWithRetry_NonexistentSource(t *testing.T) {
	dir := t.TempDir()
	err := renameWithRetry(filepath.Join(dir, "nope"), filepath.Join(dir, "dst"))
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestRemoveWithRetry_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "victim")
	writeFakeBin(t, f)
	if err := removeWithRetry(f); err != nil {
		t.Fatalf("removeWithRetry: %v", err)
	}
	if _, err := os.Stat(f); err == nil {
		t.Fatal("file should have been removed")
	}
}

func TestRemoveWithRetry_NonexistentFile(t *testing.T) {
	dir := t.TempDir()
	if err := removeWithRetry(filepath.Join(dir, "nope")); err != nil {
		t.Fatalf("removeWithRetry on absent file: %v", err)
	}
}

func TestCopyFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	writeFakeBin(t, src)
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "fake-binary-content" {
		t.Fatalf("wrong content: %q", got)
	}
}

func TestCopyFile_MissingSource(t *testing.T) {
	dir := t.TempDir()
	err := copyFile(filepath.Join(dir, "nope"), filepath.Join(dir, "dst"))
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestCopyFile_PreservesExecutablePerm(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permissions not meaningful on Windows")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	writeFakeBin(t, src)
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	perm := info.Mode().Perm()
	if perm&0o111 == 0 {
		t.Fatalf("expected executable permission, got %o", perm)
	}
}
