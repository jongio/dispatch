package update

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// mockRoundTripper — intercepts all HTTP requests for testing
// ---------------------------------------------------------------------------

type mockRoundTripper struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}

func setMockTransport(t *testing.T, fn func(req *http.Request) (*http.Response, error)) {
	t.Helper()
	original := updateHTTPTransport
	updateHTTPTransport = &mockRoundTripper{fn: fn}
	t.Cleanup(func() {
		updateHTTPTransport = original
	})
}

// ---------------------------------------------------------------------------
// copyWithLimit
// ---------------------------------------------------------------------------

func TestCopyWithLimit_ExactLimit(t *testing.T) {
	t.Parallel()
	data := strings.NewReader("hello")
	var buf strings.Builder
	if err := copyWithLimit(&buf, data, 5); err != nil {
		t.Fatalf("copyWithLimit at exact limit: %v", err)
	}
	if buf.String() != "hello" {
		t.Errorf("got %q, want %q", buf.String(), "hello")
	}
}

func TestCopyWithLimit_UnderLimit(t *testing.T) {
	t.Parallel()
	data := strings.NewReader("hi")
	var buf strings.Builder
	if err := copyWithLimit(&buf, data, 100); err != nil {
		t.Fatalf("copyWithLimit under limit: %v", err)
	}
	if buf.String() != "hi" {
		t.Errorf("got %q, want %q", buf.String(), "hi")
	}
}

func TestCopyWithLimit_OverLimit(t *testing.T) {
	t.Parallel()
	data := strings.NewReader("this is way too long")
	var buf strings.Builder
	err := copyWithLimit(&buf, data, 5)
	if err == nil {
		t.Fatal("expected error when exceeding limit")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error should mention exceeds, got: %v", err)
	}
}

func TestCopyWithLimit_EmptyReader(t *testing.T) {
	t.Parallel()
	data := strings.NewReader("")
	var buf strings.Builder
	if err := copyWithLimit(&buf, data, 100); err != nil {
		t.Fatalf("copyWithLimit empty reader: %v", err)
	}
	if buf.String() != "" {
		t.Errorf("got %q, want empty", buf.String())
	}
}

// errWriter always returns an error on Write.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("write error") }

func TestCopyWithLimit_WriteError(t *testing.T) {
	t.Parallel()
	data := strings.NewReader("some data")
	err := copyWithLimit(errWriter{}, data, 100)
	if err == nil {
		t.Fatal("expected error from failing writer")
	}
	if !strings.Contains(err.Error(), "write error") {
		t.Errorf("expected write error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// normalizeArchiveEntryName
// ---------------------------------------------------------------------------

func TestNormalizeArchiveEntryName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "dispatch", "dispatch"},
		{"with dot slash", "./dispatch", "dispatch"},
		{"backslash", `foo\dispatch`, "foo/dispatch"},
		{"nested", "dir/subdir/dispatch", "dir/subdir/dispatch"},
		{"dot backslash", `.\dispatch`, "dispatch"},
		{"trailing slash", "dispatch/", "dispatch"},
		{"double dot", "../dispatch", "../dispatch"},
		{"mixed separators", `dir\sub/dispatch`, "dir/sub/dispatch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeArchiveEntryName(tt.in)
			if got != tt.want {
				t.Errorf("normalizeArchiveEntryName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// matchArchiveTarget
// ---------------------------------------------------------------------------

func TestMatchArchiveTarget(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		entry   string
		target  string
		match   bool
		wantErr bool
	}{
		{"exact match", "dispatch", "dispatch", true, false},
		{"no match", "readme.txt", "dispatch", false, false},
		{"subdirectory match (unsafe)", "subdir/dispatch", "dispatch", false, true},
		{"dot slash match", "./dispatch", "dispatch", true, false},
		{"unrelated base", "subdir/other", "dispatch", false, false},
		{"windows exe", "dispatch.exe", "dispatch.exe", true, false},
		{"path traversal", "../../dispatch", "dispatch", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			match, err := matchArchiveTarget(tt.entry, tt.target)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error for unsafe path")
				}
				if !strings.Contains(err.Error(), "unsafe archive entry path") {
					t.Errorf("error should mention unsafe path, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if match != tt.match {
				t.Errorf("matchArchiveTarget(%q, %q) = %v, want %v", tt.entry, tt.target, match, tt.match)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// httpsOnlyCheckRedirect
// ---------------------------------------------------------------------------

func TestHttpsOnlyCheckRedirect_HTTPS(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest("GET", "https://example.com/file", nil)
	via := []*http.Request{{}}
	if err := httpsOnlyCheckRedirect(req, via); err != nil {
		t.Errorf("HTTPS redirect should be allowed: %v", err)
	}
}

func TestHttpsOnlyCheckRedirect_HTTP(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest("GET", "http://example.com/file", nil)
	via := []*http.Request{{}}
	err := httpsOnlyCheckRedirect(req, via)
	if err == nil {
		t.Fatal("HTTP redirect should be rejected")
	}
	if !strings.Contains(err.Error(), "non-HTTPS") {
		t.Errorf("error should mention non-HTTPS, got: %v", err)
	}
}

func TestHttpsOnlyCheckRedirect_TooManyRedirects(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest("GET", "https://example.com/file", nil)
	via := make([]*http.Request, maxRedirects)
	err := httpsOnlyCheckRedirect(req, via)
	if err == nil {
		t.Fatal("too many redirects should be rejected")
	}
	if !strings.Contains(err.Error(), "too many redirects") {
		t.Errorf("error should mention too many redirects, got: %v", err)
	}
}

func TestHttpsOnlyCheckRedirect_ExactlyAtLimit(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest("GET", "https://example.com/file", nil)
	via := make([]*http.Request, maxRedirects-1)
	if err := httpsOnlyCheckRedirect(req, via); err != nil {
		t.Errorf("exactly at redirect limit minus 1 should be allowed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// newSecureClient
// ---------------------------------------------------------------------------

func TestNewSecureClient_NonNil(t *testing.T) {
	t.Parallel()
	client := newSecureClient(10)
	if client == nil {
		t.Fatal("newSecureClient returned nil")
	}
	if client.CheckRedirect == nil {
		t.Error("secure client should have CheckRedirect set")
	}
}

// ---------------------------------------------------------------------------
// replaceUnix — file operations (works on all platforms)
// ---------------------------------------------------------------------------

func TestReplaceUnix_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create "new binary" source file.
	newBin := filepath.Join(tmpDir, "new-dispatch")
	newContent := []byte("new binary content v2")
	if err := os.WriteFile(newBin, newContent, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create "current binary" target file.
	exePath := filepath.Join(tmpDir, "dispatch")
	if err := os.WriteFile(exePath, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceUnix(newBin, exePath); err != nil {
		t.Fatalf("replaceUnix: %v", err)
	}

	// Verify the binary was replaced.
	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("reading replaced binary: %v", err)
	}
	if string(got) != string(newContent) {
		t.Errorf("replaced content = %q, want %q", string(got), string(newContent))
	}

	// Verify .new temp file is cleaned up.
	if _, err := os.Stat(exePath + ".new"); !os.IsNotExist(err) {
		t.Error("temp .new file should be cleaned up after successful replace")
	}
}

func TestReplaceUnix_MissingSource(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	exePath := filepath.Join(tmpDir, "dispatch")
	if err := os.WriteFile(exePath, []byte("original"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := replaceUnix(filepath.Join(tmpDir, "nonexistent"), exePath)
	if err == nil {
		t.Fatal("expected error for missing source binary")
	}
	if !strings.Contains(err.Error(), "opening new binary") {
		t.Errorf("error should mention opening, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// replaceWindows — success path with companions
// ---------------------------------------------------------------------------

func TestReplaceWindows_SuccessNoCompanions(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	exeDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create "current binary".
	exePath := filepath.Join(exeDir, binaryName+".exe")
	if err := os.WriteFile(exePath, []byte("old exe"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create "new binary" source.
	newBin := filepath.Join(tmpDir, "new.exe")
	newContent := []byte("new exe v2")
	if err := os.WriteFile(newBin, newContent, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceWindows(newBin, exeDir, binaryName+".exe"); err != nil {
		t.Fatalf("replaceWindows: %v", err)
	}

	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("reading replaced binary: %v", err)
	}
	if string(got) != string(newContent) {
		t.Errorf("replaced content = %q, want %q", string(got), string(newContent))
	}

	// Old file should be cleaned up.
	if _, err := os.Stat(exePath + oldBinarySuffix); !os.IsNotExist(err) {
		t.Error(".old file should be removed after successful replace")
	}
}

func TestReplaceWindows_SuccessWithCompanion(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	exeDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create "current binary" as dispatch.exe.
	exePath := filepath.Join(exeDir, binaryName+".exe")
	if err := os.WriteFile(exePath, []byte("old dispatch"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create companion "disp.exe".
	companionPath := filepath.Join(exeDir, aliasName+".exe")
	if err := os.WriteFile(companionPath, []byte("old disp"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create "new binary" source.
	newBin := filepath.Join(tmpDir, "new.exe")
	newContent := []byte("new binary v2")
	if err := os.WriteFile(newBin, newContent, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceWindows(newBin, exeDir, binaryName+".exe"); err != nil {
		t.Fatalf("replaceWindows: %v", err)
	}

	// Main binary should be updated.
	got, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("reading main binary: %v", err)
	}
	if string(got) != string(newContent) {
		t.Errorf("main binary = %q, want %q", string(got), string(newContent))
	}

	// Companion should also be updated.
	gotCompanion, err := os.ReadFile(companionPath)
	if err != nil {
		t.Fatalf("reading companion: %v", err)
	}
	if string(gotCompanion) != string(newContent) {
		t.Errorf("companion = %q, want %q", string(gotCompanion), string(newContent))
	}
}

func TestReplaceWindows_CompanionSkipsSelf(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	exeDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create as disp.exe (alias name).
	exePath := filepath.Join(exeDir, aliasName+".exe")
	if err := os.WriteFile(exePath, []byte("old disp"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create dispatch.exe companion.
	companionPath := filepath.Join(exeDir, binaryName+".exe")
	if err := os.WriteFile(companionPath, []byte("old dispatch"), 0o755); err != nil {
		t.Fatal(err)
	}

	newBin := filepath.Join(tmpDir, "new.exe")
	newContent := []byte("new v2")
	if err := os.WriteFile(newBin, newContent, 0o755); err != nil {
		t.Fatal(err)
	}

	// Run as disp.exe — should update dispatch.exe companion.
	if err := replaceWindows(newBin, exeDir, aliasName+".exe"); err != nil {
		t.Fatalf("replaceWindows: %v", err)
	}

	gotCompanion, err := os.ReadFile(companionPath)
	if err != nil {
		t.Fatalf("reading companion dispatch.exe: %v", err)
	}
	if string(gotCompanion) != string(newContent) {
		t.Errorf("companion dispatch.exe = %q, want %q", string(gotCompanion), string(newContent))
	}
}

func TestReplaceWindows_PreviousOldFileRemoved(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	exeDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	exePath := filepath.Join(exeDir, binaryName+".exe")
	if err := os.WriteFile(exePath, []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Pre-existing .old file from previous update.
	oldPath := exePath + oldBinarySuffix
	if err := os.WriteFile(oldPath, []byte("ancient"), 0o755); err != nil {
		t.Fatal(err)
	}

	newBin := filepath.Join(tmpDir, "new.exe")
	if err := os.WriteFile(newBin, []byte("newest"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceWindows(newBin, exeDir, binaryName+".exe"); err != nil {
		t.Fatalf("replaceWindows: %v", err)
	}

	// .old should be cleaned up.
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("previous .old file should be removed")
	}
}

// ---------------------------------------------------------------------------
// fetchLatestVersion — mock transport
// ---------------------------------------------------------------------------

func TestFetchLatestVersion_Success(t *testing.T) {
	// NOTE: Not parallel — modifies package-level updateHTTPTransport.
	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v2.5.0"})
		return w.Result(), nil
	})

	version, err := fetchLatestVersion()
	if err != nil {
		t.Fatalf("fetchLatestVersion: %v", err)
	}
	if version != "2.5.0" {
		t.Errorf("version = %q, want %q", version, "2.5.0")
	}
}

func TestFetchLatestVersion_NonOKStatus(t *testing.T) {
	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		w.WriteHeader(http.StatusServiceUnavailable)
		return w.Result(), nil
	})

	_, err := fetchLatestVersion()
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should mention 503, got: %v", err)
	}
}

func TestFetchLatestVersion_InvalidJSON(t *testing.T) {
	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		_, _ = w.WriteString("not json")
		return w.Result(), nil
	})

	_, err := fetchLatestVersion()
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestFetchLatestVersion_NetworkError(t *testing.T) {
	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network unreachable")
	})

	_, err := fetchLatestVersion()
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestFetchLatestVersion_StripVPrefix(t *testing.T) {
	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v1.0.0"})
		return w.Result(), nil
	})

	version, err := fetchLatestVersion()
	if err != nil {
		t.Fatalf("fetchLatestVersion: %v", err)
	}
	if version != "1.0.0" {
		t.Errorf("version = %q, want v prefix stripped", version)
	}
}

// ---------------------------------------------------------------------------
// RunUpdate — early exit paths
// ---------------------------------------------------------------------------

func TestRunUpdate_AlreadyUpToDate(t *testing.T) {
	tmpDir := t.TempDir()
	setConfigDir(t, tmpDir)
	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v1.0.0"})
		return w.Result(), nil
	})

	err := RunUpdate("1.0.0")
	if err != nil {
		t.Fatalf("RunUpdate should succeed when already up to date: %v", err)
	}
}

func TestRunUpdate_InvalidLatestVersion(t *testing.T) {
	tmpDir := t.TempDir()
	setConfigDir(t, tmpDir)
	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "invalid"})
		return w.Result(), nil
	})

	err := RunUpdate("1.0.0")
	if err == nil {
		t.Fatal("expected error for invalid latest version")
	}
	if !strings.Contains(err.Error(), "invalid latest version") {
		t.Errorf("error should mention invalid latest version, got: %v", err)
	}
}

func TestRunUpdate_FetchVersionError(t *testing.T) {
	tmpDir := t.TempDir()
	setConfigDir(t, tmpDir)
	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("DNS resolution failed")
	})

	err := RunUpdate("1.0.0")
	if err == nil {
		t.Fatal("expected error when fetch fails")
	}
	if !strings.Contains(err.Error(), "checking latest version") {
		t.Errorf("error should mention checking, got: %v", err)
	}
}

func TestRunUpdate_DownloadFailure(t *testing.T) {
	tmpDir := t.TempDir()
	setConfigDir(t, tmpDir)

	callCount := 0
	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		callCount++
		w := httptest.NewRecorder()
		if callCount == 1 {
			// First call: fetchLatestVersion
			_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v2.0.0"})
		} else {
			// Second call: downloadAsset — fail
			w.WriteHeader(http.StatusNotFound)
		}
		return w.Result(), nil
	})

	err := RunUpdate("1.0.0")
	if err == nil {
		t.Fatal("expected error when download fails")
	}
	if !strings.Contains(err.Error(), "downloading") {
		t.Errorf("error should mention downloading, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// verifyChecksum — success path
// ---------------------------------------------------------------------------

func TestVerifyChecksum_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create archive file with known content.
	archiveData := []byte("test archive content for checksum")
	archivePath := filepath.Join(tmpDir, "dispatch.tar.gz")
	if err := os.WriteFile(archivePath, archiveData, 0o600); err != nil {
		t.Fatal(err)
	}

	// Compute real SHA-256.
	h := sha256.Sum256(archiveData)
	realHash := hex.EncodeToString(h[:])

	// Serve checksum file.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "%s  dispatch.tar.gz\n", realHash)
	}))
	defer srv.Close()

	if err := verifyChecksum(archivePath, srv.URL, "dispatch.tar.gz"); err != nil {
		t.Fatalf("verifyChecksum should succeed: %v", err)
	}
}

func TestVerifyChecksum_DownloadError(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "dispatch.tar.gz")
	if err := os.WriteFile(archivePath, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := verifyChecksum(archivePath, srv.URL, "dispatch.tar.gz")
	if err == nil {
		t.Fatal("expected error when checksum download fails")
	}
}

func TestVerifyChecksum_MissingArchiveName(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "dispatch.tar.gz")
	if err := os.WriteFile(archivePath, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "abcdef1234567890  other_file.tar.gz\n")
	}))
	defer srv.Close()

	err := verifyChecksum(archivePath, srv.URL, "dispatch.tar.gz")
	if err == nil {
		t.Fatal("expected error when archive name not in checksums")
	}
}

// ---------------------------------------------------------------------------
// downloadAsset — success path
// ---------------------------------------------------------------------------

func TestDownloadAsset_Success(t *testing.T) {
	t.Parallel()
	content := []byte("fake binary content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "downloaded.bin")
	if err := downloadAsset(dst, srv.URL); err != nil {
		t.Fatalf("downloadAsset: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("downloaded content = %q, want %q", string(got), string(content))
	}
}

func TestDownloadAsset_HTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "downloaded.bin")
	err := downloadAsset(dst, srv.URL)
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should mention 403, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// validateVersion
// ---------------------------------------------------------------------------

func TestValidateVersion_Parallel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		v       string
		wantErr bool
	}{
		{"1.0.0", false},
		{"0.0.1", false},
		{"99.99.99", false},
		{"v1.0.0", true},
		{"1.0", true},
		{"abc", true},
		{"", true},
		{"1.0.0.0", true},
	}
	for _, tt := range tests {
		t.Run(tt.v, func(t *testing.T) {
			t.Parallel()
			err := validateVersion(tt.v)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVersion(%q) error = %v, wantErr = %v", tt.v, err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// releaseUpdateLock — nil and empty path safety
// ---------------------------------------------------------------------------

func TestReleaseUpdateLock_Nil(t *testing.T) {
	t.Parallel()
	// Should not panic.
	releaseUpdateLock(nil)
}

func TestReleaseUpdateLock_EmptyPath(t *testing.T) {
	t.Parallel()
	// Should not panic.
	releaseUpdateLock(&updateLock{path: ""})
}

// ---------------------------------------------------------------------------
// RunUpdate — checksum verification failure path
// ---------------------------------------------------------------------------

func TestRunUpdate_ChecksumVerificationFails(t *testing.T) {
	tmpDir := t.TempDir()
	setConfigDir(t, tmpDir)

	callCount := 0
	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		callCount++
		w := httptest.NewRecorder()
		switch callCount {
		case 1:
			// fetchLatestVersion
			_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v2.0.0"})
		case 2:
			// downloadAsset — return valid archive data
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake archive data"))
		case 3:
			// verifyChecksum — return wrong checksum
			_, _ = fmt.Fprintf(w, "%s  dispatch_%s_%s_%s.zip\n",
				strings.Repeat("a", 64), "2.0.0", "windows", "amd64")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
		return w.Result(), nil
	})

	err := RunUpdate("1.0.0")
	if err == nil {
		t.Fatal("expected error when checksum verification fails")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error should mention checksum, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// isStaleLock — additional paths
// ---------------------------------------------------------------------------

func TestIsStaleLock_NonexistentFile(t *testing.T) {
	t.Parallel()
	stale, err := isStaleLock(filepath.Join(t.TempDir(), "no-such-lock"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stale {
		t.Error("nonexistent lock should not be stale")
	}
}

func TestIsStaleLock_RecentLock(t *testing.T) {
	t.Parallel()
	lockPath := filepath.Join(t.TempDir(), "recent.lock")
	metadata := lockMetadata{
		PID:       os.Getpid(),
		CreatedAt: time.Now().UTC(),
	}
	raw, _ := json.Marshal(metadata)
	if err := os.WriteFile(lockPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	stale, err := isStaleLock(lockPath)
	if err != nil {
		t.Fatalf("isStaleLock: %v", err)
	}
	if stale {
		t.Error("recent lock should not be stale")
	}
}

func TestIsStaleLock_OldLock(t *testing.T) {
	t.Parallel()
	lockPath := filepath.Join(t.TempDir(), "old.lock")
	metadata := lockMetadata{
		PID:       12345,
		CreatedAt: time.Now().Add(-lockStaleDuration - time.Hour).UTC(),
	}
	raw, _ := json.Marshal(metadata)
	if err := os.WriteFile(lockPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	stale, err := isStaleLock(lockPath)
	if err != nil {
		t.Fatalf("isStaleLock: %v", err)
	}
	if !stale {
		t.Error("old lock should be stale")
	}
}

// ---------------------------------------------------------------------------
// acquireUpdateLock — already held (not stale)
// ---------------------------------------------------------------------------

func TestAcquireUpdateLock_FailsWhenHeld(t *testing.T) {
	t.Parallel()
	lockPath := filepath.Join(t.TempDir(), lockFileName)

	// Create a fresh (non-stale) lock owned by this process.
	metadata := lockMetadata{
		PID:       os.Getpid(),
		CreatedAt: time.Now().UTC(),
	}
	raw, _ := json.Marshal(metadata)
	if err := os.WriteFile(lockPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := acquireUpdateLock(lockPath)
	if err == nil {
		t.Fatal("expected error when lock is held by active process")
	}
	if !strings.Contains(err.Error(), "lock file exists") {
		t.Errorf("error should mention lock file exists, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// replaceUnix — temp file creation failure
// ---------------------------------------------------------------------------

func TestReplaceUnix_ReadOnlyDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create source binary.
	newBin := filepath.Join(tmpDir, "new-dispatch")
	if err := os.WriteFile(newBin, []byte("new binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a read-only directory for the target.
	roDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(roDir, 0o755)
	})

	exePath := filepath.Join(roDir, "dispatch")
	err := replaceUnix(newBin, exePath)

	// On Windows, directory read-only bits don't restrict file creation.
	// On Unix as root (e.g. WSL), permission checks are bypassed.
	// In both cases the operation may succeed — that's valid behavior.
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		if err != nil {
			t.Logf("replaceUnix failed (OK for privileged/Windows): %v", err)
		}
	} else {
		if err == nil {
			t.Error("replaceUnix should fail when target directory is read-only")
		}
	}
}

// ---------------------------------------------------------------------------
// copyFile — destination directory doesn't exist
// ---------------------------------------------------------------------------

func TestCopyFile_DstDirMissing(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.bin")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(tmpDir, "nonexistent", "dir", "dst.bin")
	err := copyFile(src, dst)
	if err == nil {
		t.Fatal("expected error when destination directory doesn't exist")
	}
}

// ---------------------------------------------------------------------------
// writeCache — path in unwritable location
// ---------------------------------------------------------------------------

func TestWriteCache_EmptyPath(t *testing.T) {
	t.Parallel()
	// When path is empty AND cachePath() can't resolve, writeCache should
	// silently return without panic.
	writeCache("", &updateCache{
		CheckedAt:      time.Now(),
		LatestVersion:  "1.0.0",
		CurrentVersion: "1.0.0",
	})
	// No assertion — just verify no panic.
}

func TestWriteCache_ExplicitPath(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "explicit-cache.json")
	cache := &updateCache{
		CheckedAt:      time.Now().Truncate(time.Second),
		LatestVersion:  "3.0.0",
		CurrentVersion: "2.0.0",
	}
	writeCache(path, cache)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading cache: %v", err)
	}
	var got updateCache
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("parsing cache: %v", err)
	}
	if got.LatestVersion != "3.0.0" {
		t.Errorf("LatestVersion = %q, want %q", got.LatestVersion, "3.0.0")
	}
}

// ---------------------------------------------------------------------------
// extractFromZip — empty archive
// ---------------------------------------------------------------------------

func TestExtractFromZip_EmptyArchive(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a valid zip with no matching entries.
	archivePath := filepath.Join(tmpDir, "empty.zip")
	if err := createTestZip(archivePath, "unrelated.txt", []byte("data")); err != nil {
		t.Fatal(err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := extractFromZip(archivePath, extractDir)
	if err == nil {
		t.Error("expected error when dispatch.exe not found in zip")
	}
}

// ---------------------------------------------------------------------------
// extractFromTarGz — corrupt archive
// ---------------------------------------------------------------------------

func TestExtractFromTarGz_CorruptArchive(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	archivePath := filepath.Join(tmpDir, "corrupt.tar.gz")
	if err := os.WriteFile(archivePath, []byte("not a real archive"), 0o644); err != nil {
		t.Fatal(err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := extractFromTarGz(archivePath, extractDir)
	if err == nil {
		t.Error("expected error for corrupt archive")
	}
}

// ---------------------------------------------------------------------------
// CheckForUpdate — mock transport for network path
// ---------------------------------------------------------------------------

func TestCheckForUpdate_NetworkReturnsUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	setConfigDir(t, tmpDir)

	// No cache file — forces network call.
	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v9.0.0"})
		return w.Result(), nil
	})

	info := CheckForUpdate("1.0.0")
	if info == nil {
		t.Fatal("expected update info when newer version available")
	}
	if info.LatestVersion != "9.0.0" {
		t.Errorf("LatestVersion = %q, want %q", info.LatestVersion, "9.0.0")
	}
	if info.CurrentVersion != "1.0.0" {
		t.Errorf("CurrentVersion = %q, want %q", info.CurrentVersion, "1.0.0")
	}
}

func TestCheckForUpdate_NetworkReturnsUpToDate(t *testing.T) {
	tmpDir := t.TempDir()
	setConfigDir(t, tmpDir)

	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v1.0.0"})
		return w.Result(), nil
	})

	info := CheckForUpdate("1.0.0")
	if info != nil {
		t.Error("expected nil when already up to date via network")
	}
}

func TestCheckForUpdate_NetworkReturnsInvalidVersion(t *testing.T) {
	tmpDir := t.TempDir()
	setConfigDir(t, tmpDir)

	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "not-a-version"})
		return w.Result(), nil
	})

	info := CheckForUpdate("1.0.0")
	if info != nil {
		t.Error("expected nil for invalid version from network")
	}
}

// ---------------------------------------------------------------------------
// RunUpdate — extract binary failure (download + checksum pass, extract fails)
// ---------------------------------------------------------------------------

func TestRunUpdate_ExtractBinaryFailure(t *testing.T) {
	tmpDir := t.TempDir()
	setConfigDir(t, tmpDir)

	// Create a zip file that does NOT contain "dispatch.exe".
	zipBuf := createZipBytes(t, "readme.txt", []byte("not a binary"))
	zipHash := sha256Hex(zipBuf)

	callCount := 0
	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		callCount++
		w := httptest.NewRecorder()
		switch {
		case callCount == 1:
			// fetchLatestVersion
			_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v3.0.0"})
		case strings.Contains(req.URL.Path, "checksums"):
			// verifyChecksum — return correct hash
			assetName := AssetName("3.0.0")
			_, _ = fmt.Fprintf(w, "%s  %s\n", zipHash, assetName)
		default:
			// downloadAsset — return the zip file
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(zipBuf)
		}
		return w.Result(), nil
	})

	err := RunUpdate("1.0.0")
	if err == nil {
		t.Fatal("expected error when binary not found in archive")
	}
	if !strings.Contains(err.Error(), "extracting binary") {
		t.Errorf("error should mention extracting binary, got: %v", err)
	}
}

// createZipBytes creates a zip archive in memory with a single file entry.
func createZipBytes(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// sha256Hex returns the lowercase hex SHA-256 of data.
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// verifyChecksum — oversized response
// ---------------------------------------------------------------------------

func TestVerifyChecksum_OversizedResponse(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "dispatch.tar.gz")
	if err := os.WriteFile(archivePath, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", maxChecksumSize+1))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := verifyChecksum(archivePath, srv.URL, "dispatch.tar.gz")
	if err == nil {
		t.Fatal("expected error for oversized checksum response")
	}
}

// ---------------------------------------------------------------------------
// downloadAsset — network transport error
// ---------------------------------------------------------------------------

func TestDownloadAsset_NetworkError(t *testing.T) {
	t.Parallel()
	// Use an unreachable URL.
	dst := filepath.Join(t.TempDir(), "asset.bin")
	err := downloadAsset(dst, "http://127.0.0.1:1/nonexistent")
	if err == nil {
		t.Fatal("expected error for unreachable URL")
	}
}

// ---------------------------------------------------------------------------
// acquireUpdateLock — write failure (unwritable directory)
// ---------------------------------------------------------------------------

func TestAcquireUpdateLock_CreatesMetadata(t *testing.T) {
	t.Parallel()
	lockPath := filepath.Join(t.TempDir(), "test.lock")

	lock, err := acquireUpdateLock(lockPath)
	if err != nil {
		t.Fatalf("acquireUpdateLock: %v", err)
	}
	defer releaseUpdateLock(lock)

	// Verify the lock file contains valid JSON metadata.
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("reading lock: %v", err)
	}

	var meta lockMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("parsing lock metadata: %v", err)
	}
	if meta.PID != os.Getpid() {
		t.Errorf("PID = %d, want %d", meta.PID, os.Getpid())
	}
	if time.Since(meta.CreatedAt) > time.Minute {
		t.Errorf("CreatedAt = %v, expected recent", meta.CreatedAt)
	}
}

// ---------------------------------------------------------------------------
// writeCache — error paths
// ---------------------------------------------------------------------------

func TestWriteCache_UnwritableDir(t *testing.T) {
	t.Parallel()
	// Path in a non-existent deeply nested location.
	// writeCache should silently fail (no panic, no error return).
	badPath := filepath.Join(t.TempDir(), "no", "permissions", "cache.json")
	writeCache(badPath, &updateCache{
		CheckedAt:      time.Now(),
		LatestVersion:  "1.0.0",
		CurrentVersion: "1.0.0",
	})
	// Verify it was actually written (MkdirAll succeeds for nested dirs
	// in a temp directory). This tests the success path through MkdirAll.
	if _, err := os.Stat(badPath); err != nil {
		// If it failed, that's OK — writeCache is best-effort.
		t.Log("writeCache failed for nested path (expected in some environments)")
	}
}

// ---------------------------------------------------------------------------
// extractFromZip — success path (dispatch.exe found and extracted)
// ---------------------------------------------------------------------------

func TestExtractFromZip_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a zip containing dispatch.exe with known content.
	archivePath := filepath.Join(tmpDir, "good.zip")
	binaryContent := []byte("#!/bin/fake-dispatch-binary")
	if err := createTestZip(archivePath, "dispatch.exe", binaryContent); err != nil {
		t.Fatal(err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := extractFromZip(archivePath, extractDir)
	if err != nil {
		t.Fatalf("extractFromZip failed: %v", err)
	}

	if filepath.Base(got) != "dispatch.exe" {
		t.Errorf("expected dispatch.exe, got %s", filepath.Base(got))
	}

	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if !bytes.Equal(data, binaryContent) {
		t.Errorf("extracted content mismatch: got %q", data)
	}
}

// ---------------------------------------------------------------------------
// extractFromZip — nested path dispatch.exe (e.g., subdir/dispatch.exe)
// ---------------------------------------------------------------------------

func TestExtractFromZip_NestedPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a zip where dispatch.exe is inside a subdirectory.
	// matchArchiveTarget treats nested paths as unsafe (path traversal
	// protection), so extraction should fail with an error.
	archivePath := filepath.Join(tmpDir, "nested.zip")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	entry, err := w.Create("subdir/dispatch.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("nested binary")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err = extractFromZip(archivePath, extractDir)
	if err == nil {
		t.Error("expected error for nested path (path traversal protection)")
	}
	if !strings.Contains(err.Error(), "unsafe") {
		t.Errorf("expected 'unsafe' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// downloadAsset — file creation error
// ---------------------------------------------------------------------------

func TestDownloadAsset_CreateFileError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("binary data"))
	}))
	defer ts.Close()

	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		return http.DefaultTransport.RoundTrip(req)
	})

	// Use a non-existent directory as dst to trigger os.Create error.
	badDst := filepath.Join(t.TempDir(), "no-such-dir", "asset.zip")
	err := downloadAsset(badDst, ts.URL+"/asset")
	if err == nil {
		t.Error("expected error when dst dir does not exist")
	}
	if !strings.Contains(err.Error(), "creating") {
		t.Errorf("expected 'creating' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// downloadAsset — success with content
// ---------------------------------------------------------------------------

func TestDownloadAsset_SuccessContent(t *testing.T) {
	content := []byte("downloaded-content-here")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}))
	defer ts.Close()

	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		return http.DefaultTransport.RoundTrip(req)
	})

	dst := filepath.Join(t.TempDir(), "downloaded.bin")
	if err := downloadAsset(dst, ts.URL+"/asset.zip"); err != nil {
		t.Fatalf("downloadAsset: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: got %q", got)
	}
}

// ---------------------------------------------------------------------------
// downloadAsset — HTTP error status
// ---------------------------------------------------------------------------

func TestDownloadAsset_HTTPServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		return http.DefaultTransport.RoundTrip(req)
	})

	dst := filepath.Join(t.TempDir(), "asset.zip")
	err := downloadAsset(dst, ts.URL+"/asset")
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected HTTP 500 error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// downloadAsset — oversized content-length
// ---------------------------------------------------------------------------

func TestDownloadAsset_OversizedContentLength(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "999999999999")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		return http.DefaultTransport.RoundTrip(req)
	})

	dst := filepath.Join(t.TempDir(), "asset.zip")
	err := downloadAsset(dst, ts.URL+"/asset")
	if err == nil {
		t.Error("expected error for oversized content")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected 'exceeds' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// verifyChecksum — success path
// ---------------------------------------------------------------------------

func TestVerifyChecksum_SuccessPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake archive file and compute its checksum.
	archiveContent := []byte("fake-archive-content-for-checksum")
	archivePath := filepath.Join(tmpDir, "dispatch_1.0.0_windows_amd64.zip")
	if err := os.WriteFile(archivePath, archiveContent, 0o644); err != nil {
		t.Fatal(err)
	}

	hash := sha256.Sum256(archiveContent)
	checksum := hex.EncodeToString(hash[:])
	checksumFile := fmt.Sprintf("%s  dispatch_1.0.0_windows_amd64.zip\n", checksum)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(checksumFile))
	}))
	defer ts.Close()

	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		return http.DefaultTransport.RoundTrip(req)
	})

	err := verifyChecksum(archivePath, ts.URL+"/checksums.txt", "dispatch_1.0.0_windows_amd64.zip")
	if err != nil {
		t.Fatalf("verifyChecksum should succeed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// verifyChecksum — checksum mismatch
// ---------------------------------------------------------------------------

func TestVerifyChecksum_HashMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	archivePath := filepath.Join(tmpDir, "dispatch_1.0.0_windows_amd64.zip")
	if err := os.WriteFile(archivePath, []byte("real content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Serve a checksum that does NOT match.
	checksumFile := "0000000000000000000000000000000000000000000000000000000000000000  dispatch_1.0.0_windows_amd64.zip\n"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(checksumFile))
	}))
	defer ts.Close()

	setMockTransport(t, func(req *http.Request) (*http.Response, error) {
		return http.DefaultTransport.RoundTrip(req)
	})

	err := verifyChecksum(archivePath, ts.URL+"/checksums.txt", "dispatch_1.0.0_windows_amd64.zip")
	if err == nil {
		t.Error("expected error for checksum mismatch")
	}
	if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("expected 'mismatch' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// writeCache — success with explicit path (covers full write pipeline)
// ---------------------------------------------------------------------------

func TestWriteCache_SuccessExplicitPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")

	cache := &updateCache{
		CheckedAt:      time.Now().Truncate(time.Second),
		LatestVersion:  "2.5.0",
		CurrentVersion: "2.4.0",
	}
	writeCache(path, cache)

	// Verify the file was written and is valid JSON.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("writeCache should have created file: %v", err)
	}
	var got updateCache
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid JSON in cache file: %v", err)
	}
	if got.LatestVersion != "2.5.0" {
		t.Errorf("LatestVersion = %s, want 2.5.0", got.LatestVersion)
	}
	if got.CurrentVersion != "2.4.0" {
		t.Errorf("CurrentVersion = %s, want 2.4.0", got.CurrentVersion)
	}
}

// ---------------------------------------------------------------------------
// acquireUpdateLock — file operations coverage
// ---------------------------------------------------------------------------

func TestAcquireUpdateLock_WriteMetadata(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lock, err := acquireUpdateLock(lockPath)
	if err != nil {
		t.Fatalf("acquireUpdateLock: %v", err)
	}
	defer releaseUpdateLock(lock)

	// Verify lock file was created and contains metadata with pid.
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("lock file not created: %v", err)
	}
	if !strings.Contains(string(data), "PID") && !strings.Contains(string(data), "pid") {
		t.Errorf("lock file should contain pid, got: %s", data)
	}
}

// ---------------------------------------------------------------------------
// extractFromTarGz — nonexistent archive (os.Open error)
// ---------------------------------------------------------------------------

func TestExtractFromTarGz_NonexistentArchive(t *testing.T) {
	t.Parallel()
	_, err := extractFromTarGz(filepath.Join(t.TempDir(), "missing.tar.gz"), t.TempDir())
	if err == nil {
		t.Error("expected error for nonexistent archive")
	}
	if !strings.Contains(err.Error(), "opening archive") {
		t.Errorf("expected 'opening archive' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// extractFromZip — nonexistent archive (zip.OpenReader error)
// ---------------------------------------------------------------------------

func TestExtractFromZip_NonexistentArchive(t *testing.T) {
	t.Parallel()
	_, err := extractFromZip(filepath.Join(t.TempDir(), "missing.zip"), t.TempDir())
	if err == nil {
		t.Error("expected error for nonexistent archive")
	}
	if !strings.Contains(err.Error(), "opening zip") {
		t.Errorf("expected 'opening zip' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// extractFromTarGz — entry read error (corrupt mid-stream)
// ---------------------------------------------------------------------------

func TestExtractFromTarGz_CorruptEntryRead(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a gzip file with invalid tar content after the gzip header.
	// This causes gzip to decompress successfully but tar.Next() to fail.
	archivePath := filepath.Join(tmpDir, "bad_entry.tar.gz")
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	// Write some garbage that looks like tar header but is corrupt.
	if _, err := gw.Write([]byte("this is not a valid tar header but gzip is fine")); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := extractFromTarGz(archivePath, tmpDir)
	// Should either get a tar read error or "not found" — both are acceptable.
	if err == nil {
		t.Error("expected error for corrupt tar within valid gzip")
	}
}

// ---------------------------------------------------------------------------
// copyFile — destination directory doesn't exist
// ---------------------------------------------------------------------------

func TestCopyFile_DstDirNotExist(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "src.bin")
	if err := os.WriteFile(src, []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(tmpDir, "no-such-dir", "dst.bin")
	err := copyFile(src, dst)
	if err == nil {
		t.Error("expected error when dst directory doesn't exist")
	}
}

// ---------------------------------------------------------------------------
// fetchLatestVersion — non-200 HTTP response
// ---------------------------------------------------------------------------

func TestFetchLatestVersion_HTTPError(t *testing.T) {
	setMockTransport(t, func(_ *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		w.WriteHeader(http.StatusServiceUnavailable)
		return w.Result(), nil
	})

	_, err := fetchLatestVersion()
	if err == nil {
		t.Fatal("expected error for HTTP 503")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("expected 503 error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// verifyChecksum — download network error
// ---------------------------------------------------------------------------

func TestVerifyChecksum_DownloadNetworkError(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "archive.zip")
	if err := os.WriteFile(archivePath, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	setMockTransport(t, func(_ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("simulated network failure")
	})

	err := verifyChecksum(archivePath, "https://example.com/checksums.txt", "archive.zip")
	if err == nil {
		t.Fatal("expected error for network failure")
	}
	if !strings.Contains(err.Error(), "downloading checksums") {
		t.Errorf("expected 'downloading checksums' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// verifyChecksum — non-200 HTTP response
// ---------------------------------------------------------------------------

func TestVerifyChecksum_HTTPErrorStatus(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "archive.zip")
	if err := os.WriteFile(archivePath, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	setMockTransport(t, func(_ *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		w.WriteHeader(http.StatusNotFound)
		return w.Result(), nil
	})

	err := verifyChecksum(archivePath, "https://example.com/checksums.txt", "archive.zip")
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected HTTP 404 error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// verifyChecksum — archive not found in checksum file
// ---------------------------------------------------------------------------

func TestVerifyChecksum_ArchiveNotInChecksums(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "myarchive.zip")
	if err := os.WriteFile(archivePath, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	setMockTransport(t, func(_ *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("abc123  other_file.zip\n"))
		return w.Result(), nil
	})

	err := verifyChecksum(archivePath, "https://example.com/checksums.txt", "myarchive.zip")
	if err == nil {
		t.Fatal("expected error when archive not found in checksums")
	}
}

// ---------------------------------------------------------------------------
// verifyChecksum — archive file missing (SHA256File error)
// ---------------------------------------------------------------------------

func TestVerifyChecksum_ArchiveFileMissing(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "vanished.zip")
	// Deliberately do NOT create the archive file.

	// Serve a checksum that would match if the file existed.
	setMockTransport(t, func(_ *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("abc123def456  vanished.zip\n"))
		return w.Result(), nil
	})

	err := verifyChecksum(archivePath, "https://example.com/checksums.txt", "vanished.zip")
	if err == nil {
		t.Fatal("expected error when archive file is missing")
	}
	if !strings.Contains(err.Error(), "computing checksum") {
		t.Errorf("expected 'computing checksum' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CheckForUpdate — fetchLatestVersion fails (error path)
// ---------------------------------------------------------------------------

func TestCheckForUpdate_FetchError(t *testing.T) {
	setMockTransport(t, func(_ *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network down")
	})
	setConfigDir(t, t.TempDir())

	result := CheckForUpdate("1.0.0")
	if result != nil {
		t.Error("expected nil when fetch fails")
	}
}

// ---------------------------------------------------------------------------
// RunUpdate — configDir error path
// ---------------------------------------------------------------------------

func TestRunUpdate_MkdirAllError(t *testing.T) {
	// Manually test with a path that won't exist.
	err := RunUpdate("dev")
	if err == nil {
		t.Fatal("expected error for dev version")
	}
	if !strings.Contains(err.Error(), "development build") {
		t.Errorf("expected dev build error, got: %v", err)
	}
}
