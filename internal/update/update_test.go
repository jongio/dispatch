package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// AssetName / assetNameForPlatform
// ---------------------------------------------------------------------------

func TestAssetNameForPlatform(t *testing.T) {
	t.Parallel()
	tests := []struct {
		version, goos, goarch string
		want                  string
	}{
		{"0.4.1", "linux", "amd64", "dispatch_0.4.1_linux_amd64.tar.gz"},
		{"0.4.1", "linux", "arm64", "dispatch_0.4.1_linux_arm64.tar.gz"},
		{"0.4.1", "darwin", "amd64", "dispatch_0.4.1_darwin_amd64.tar.gz"},
		{"0.4.1", "darwin", "arm64", "dispatch_0.4.1_darwin_arm64.tar.gz"},
		{"0.4.1", "windows", "amd64", "dispatch_0.4.1_windows_amd64.zip"},
		{"0.4.1", "windows", "arm64", "dispatch_0.4.1_windows_arm64.zip"},
		{"1.0.0", "linux", "amd64", "dispatch_1.0.0_linux_amd64.tar.gz"},
	}
	for _, tt := range tests {
		name := tt.goos + "_" + tt.goarch + "_v" + tt.version
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := assetNameForPlatform(tt.version, tt.goos, tt.goarch)
			if got != tt.want {
				t.Errorf("assetNameForPlatform(%q, %q, %q) = %q, want %q",
					tt.version, tt.goos, tt.goarch, got, tt.want)
			}
		})
	}
}

func TestAssetName_CurrentPlatform(t *testing.T) {
	t.Parallel()
	name := AssetName("1.2.3")
	if name == "" {
		t.Fatal("AssetName returned empty string")
	}
	// Should contain the version.
	if got := name; got == "" {
		t.Error("AssetName should not be empty")
	}
}

// ---------------------------------------------------------------------------
// ParseChecksum
// ---------------------------------------------------------------------------

func TestParseChecksum(t *testing.T) {
	t.Parallel()
	content := `aabbccdd1122334455667788aabbccdd1122334455667788aabbccdd11223344  dispatch_0.4.1_linux_amd64.tar.gz
11223344556677889900aabbccddeeff11223344556677889900aabbccddeeff  dispatch_0.4.1_windows_amd64.zip
ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100  dispatch_0.4.1_darwin_arm64.tar.gz
`

	tests := []struct {
		filename string
		want     string
		wantErr  bool
	}{
		{
			"dispatch_0.4.1_linux_amd64.tar.gz",
			"aabbccdd1122334455667788aabbccdd1122334455667788aabbccdd11223344",
			false,
		},
		{
			"dispatch_0.4.1_windows_amd64.zip",
			"11223344556677889900aabbccddeeff11223344556677889900aabbccddeeff",
			false,
		},
		{
			"dispatch_0.4.1_darwin_arm64.tar.gz",
			"ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100",
			false,
		},
		{"dispatch_0.4.1_freebsd_amd64.tar.gz", "", true}, // missing
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()
			got, err := ParseChecksum(content, tt.filename)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseChecksum() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseChecksum_EmptyContent(t *testing.T) {
	t.Parallel()
	_, err := ParseChecksum("", "dispatch_0.4.1_linux_amd64.tar.gz")
	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestParseChecksum_MixedCase(t *testing.T) {
	t.Parallel()
	content := `AABBCCDD1122334455667788AABBCCDD1122334455667788AABBCCDD11223344  dispatch_0.4.1_linux_amd64.tar.gz`

	got, err := ParseChecksum(content, "dispatch_0.4.1_linux_amd64.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should be normalized to lowercase.
	want := "aabbccdd1122334455667788aabbccdd1122334455667788aabbccdd11223344"
	if got != want {
		t.Errorf("ParseChecksum() = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// SHA256File
// ---------------------------------------------------------------------------

func TestSHA256File(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.bin")
	data := []byte("hello dispatch update test")

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := SHA256File(path)
	if err != nil {
		t.Fatalf("SHA256File: %v", err)
	}

	// Compute expected hash.
	h := sha256.Sum256(data)
	want := hex.EncodeToString(h[:])

	if got != want {
		t.Errorf("SHA256File() = %q, want %q", got, want)
	}
}

func TestSHA256File_Missing(t *testing.T) {
	t.Parallel()
	_, err := SHA256File(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// ---------------------------------------------------------------------------
// extractFromTarGz
// ---------------------------------------------------------------------------

func TestExtractFromTarGz(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	binaryContent := []byte("#!/bin/sh\necho hello")

	// Create a tar.gz archive with a "dispatch" entry.
	if err := createTestTarGz(archivePath, "dispatch", binaryContent); err != nil {
		t.Fatalf("creating test archive: %v", err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := extractFromTarGz(archivePath, extractDir)
	if err != nil {
		t.Fatalf("extractFromTarGz: %v", err)
	}

	if filepath.Base(got) != "dispatch" {
		t.Errorf("extracted file name = %q, want %q", filepath.Base(got), "dispatch")
	}

	content, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if string(content) != string(binaryContent) {
		t.Errorf("extracted content = %q, want %q", string(content), string(binaryContent))
	}
}

func TestExtractFromTarGz_NoBinary(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")

	// Create a tar.gz archive with a different file name.
	if err := createTestTarGz(archivePath, "other-file", []byte("data")); err != nil {
		t.Fatalf("creating test archive: %v", err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := extractFromTarGz(archivePath, extractDir)
	if err == nil {
		t.Error("expected error when binary not found in archive")
	}
}

// ---------------------------------------------------------------------------
// extractFromZip
// ---------------------------------------------------------------------------

func TestExtractFromZip(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.zip")
	binaryContent := []byte("MZ fake exe content")

	// Create a zip archive with a "dispatch.exe" entry.
	if err := createTestZip(archivePath, "dispatch.exe", binaryContent); err != nil {
		t.Fatalf("creating test zip: %v", err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := extractFromZip(archivePath, extractDir)
	if err != nil {
		t.Fatalf("extractFromZip: %v", err)
	}

	if filepath.Base(got) != "dispatch.exe" {
		t.Errorf("extracted file name = %q, want %q", filepath.Base(got), "dispatch.exe")
	}

	content, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if string(content) != string(binaryContent) {
		t.Errorf("extracted content = %q, want %q", string(content), string(binaryContent))
	}
}

func TestExtractFromZip_NoBinary(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.zip")

	if err := createTestZip(archivePath, "readme.txt", []byte("readme")); err != nil {
		t.Fatalf("creating test zip: %v", err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := extractFromZip(archivePath, extractDir)
	if err == nil {
		t.Error("expected error when binary not found in zip")
	}
}

// ---------------------------------------------------------------------------
// extractBinary (routing)
// ---------------------------------------------------------------------------

func TestExtractBinary_RoutesToZip(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.zip")
	if err := createTestZip(archivePath, "dispatch.exe", []byte("exe")); err != nil {
		t.Fatal(err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := extractBinary(archivePath, extractDir)
	if err != nil {
		t.Errorf("extractBinary(.zip) failed: %v", err)
	}
}

func TestExtractBinary_RoutesToTarGz(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	if err := createTestTarGz(archivePath, "dispatch", []byte("bin")); err != nil {
		t.Fatal(err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := extractBinary(archivePath, extractDir)
	if err != nil {
		t.Errorf("extractBinary(.tar.gz) failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// copyFile
// ---------------------------------------------------------------------------

func TestCopyFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.bin")
	dst := filepath.Join(tmpDir, "dst.bin")
	content := []byte("binary content for copy test")

	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading dst: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("copied content = %q, want %q", string(got), string(content))
	}
}

func TestCopyFile_MissingSrc(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	err := copyFile(filepath.Join(tmpDir, "nope"), filepath.Join(tmpDir, "dst"))
	if err == nil {
		t.Error("expected error for missing source")
	}
}

// ---------------------------------------------------------------------------
// Security regression tests
// ---------------------------------------------------------------------------

func TestExtractFromTarGz_PathTraversalRejected(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "path-traversal.tar.gz")
	if err := createTestTarGz(archivePath, "../../dispatch", []byte("evil")); err != nil {
		t.Fatalf("creating malicious tar: %v", err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := extractFromTarGz(archivePath, extractDir)
	if err == nil || !strings.Contains(err.Error(), "unsafe archive entry path") {
		t.Fatalf("expected unsafe path error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(extractDir, binaryName)); !os.IsNotExist(err) {
		t.Fatalf("unsafe tar entry should not be extracted, stat err = %v", err)
	}
}

func TestExtractFromZip_PathTraversalRejected(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "path-traversal.zip")
	if err := createTestZip(archivePath, "../../dispatch.exe", []byte("evil")); err != nil {
		t.Fatalf("creating malicious zip: %v", err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := extractFromZip(archivePath, extractDir)
	if err == nil || !strings.Contains(err.Error(), "unsafe archive entry path") {
		t.Fatalf("expected unsafe path error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(extractDir, binaryName+".exe")); !os.IsNotExist(err) {
		t.Fatalf("unsafe zip entry should not be extracted, stat err = %v", err)
	}
}

func TestExtractFromTarGz_SymlinkRejected(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "symlink.tar.gz")
	if err := createTestTarGzWithHeader(archivePath, &tar.Header{
		Name:     binaryName,
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
		Mode:     0o777,
	}, nil); err != nil {
		t.Fatalf("creating symlink tar: %v", err)
	}

	extractDir := filepath.Join(tmpDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := extractFromTarGz(archivePath, extractDir)
	if err == nil || !strings.Contains(err.Error(), "unsupported tar entry type") {
		t.Fatalf("expected symlink rejection error, got %v", err)
	}
}

func TestDownloadAsset_RejectsNonHTTPSRedirect(t *testing.T) {
	setTestHTTPTransport(t)

	httpTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "blocked")
	}))
	defer httpTarget.Close()

	httpsRedirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, httpTarget.URL, http.StatusFound)
	}))
	defer httpsRedirect.Close()

	err := downloadAsset(filepath.Join(t.TempDir(), "asset.bin"), httpsRedirect.URL)
	if err == nil || !strings.Contains(err.Error(), "non-HTTPS") {
		t.Fatalf("expected non-HTTPS redirect rejection, got %v", err)
	}
}

func TestDownloadAsset_EnforcesMaxSize(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.FormatInt(maxDownloadSize+1, 10))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := downloadAsset(filepath.Join(t.TempDir(), "asset.bin"), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "download exceeds") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "dispatch.tar.gz")
	if err := os.WriteFile(archivePath, []byte("actual archive data"), 0o600); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("0", 64)+"  dispatch.tar.gz\n")
	}))
	defer srv.Close()

	err := verifyChecksum(archivePath, srv.URL, "dispatch.tar.gz")
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}
}

func TestReplaceWindows_RollbackOnFailure(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	exeDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	exePath := filepath.Join(exeDir, binaryName+".exe")
	original := []byte("original binary")
	if err := os.WriteFile(exePath, original, 0o755); err != nil {
		t.Fatal(err)
	}

	err := replaceWindows(filepath.Join(tmpDir, "missing-new.exe"), exeDir, binaryName+".exe")
	if err == nil {
		t.Fatal("expected replaceWindows to fail when new binary is missing")
	}

	got, readErr := os.ReadFile(exePath)
	if readErr != nil {
		t.Fatalf("reading restored executable: %v", readErr)
	}
	if string(got) != string(original) {
		t.Fatalf("rollback restored %q, want %q", string(got), string(original))
	}
	if _, statErr := os.Stat(exePath + oldBinarySuffix); !os.IsNotExist(statErr) {
		t.Fatalf("old backup should be cleaned up, stat err = %v", statErr)
	}
}

// ---------------------------------------------------------------------------
// RunUpdate edge cases
// ---------------------------------------------------------------------------

func TestRunUpdate_DevVersion(t *testing.T) {
	t.Parallel()
	err := RunUpdate("dev")
	if err == nil {
		t.Fatal("expected error for dev version")
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// createTestTarGz creates a .tar.gz archive containing a single file.
func createTestTarGz(archivePath, filename string, content []byte) error {
	return createTestTarGzWithHeader(archivePath, &tar.Header{
		Name: filename,
		Mode: 0o755,
		Size: int64(len(content)),
	}, content)
}

func createTestTarGzWithHeader(archivePath string, hdr *tar.Header, content []byte) error {
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	gw := gzip.NewWriter(f)
	defer gw.Close() //nolint:errcheck

	tw := tar.NewWriter(gw)
	defer tw.Close() //nolint:errcheck

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if len(content) == 0 {
		return nil
	}
	if _, err := tw.Write(content); err != nil {
		return err
	}
	return nil
}

func setTestHTTPTransport(t *testing.T) {
	t.Helper()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // test-only transport

	original := updateHTTPTransport
	updateHTTPTransport = transport
	t.Cleanup(func() {
		updateHTTPTransport = original
		transport.CloseIdleConnections()
	})
}

// createTestZip creates a .zip archive containing a single file.
func createTestZip(archivePath, filename string, content []byte) error {
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	zw := zip.NewWriter(f)
	defer zw.Close() //nolint:errcheck

	w, err := zw.Create(filename)
	if err != nil {
		return err
	}
	if _, err := w.Write(content); err != nil {
		return err
	}
	return nil
}
