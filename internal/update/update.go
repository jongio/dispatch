package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/jongio/dispatch/internal/platform"
)

const (
	// downloadTimeout limits the overall duration for downloading
	// release assets.
	downloadTimeout = 120 * time.Second

	// maxDownloadSize limits the size of downloaded release archives to
	// prevent resource exhaustion from unexpectedly large responses.
	maxDownloadSize int64 = 256 << 20 // 256 MiB

	// maxRedirects caps the number of HTTP redirects followed during
	// asset download.
	maxRedirects = 10

	// downloadBaseURL is the base URL for GitHub release asset downloads.
	downloadBaseURL = "https://github.com/jongio/dispatch/releases/download"

	// checksumFileName is the name of the SHA-256 checksum file in each
	// release.
	checksumFileName = "dispatch_checksums.txt"

	// binaryName is the base name of the dispatch executable.
	binaryName = "dispatch"

	// aliasName is the short alias for the dispatch executable.
	aliasName = "disp"

	// oldBinarySuffix is appended to the current binary on Windows before
	// replacement, since a running executable cannot be deleted.
	oldBinarySuffix = ".old"

	// newBinaryPerm is the file permission for the extracted binary.
	newBinaryPerm = 0o755

	// maxChecksumSize limits the checksums file read to 1 MiB.
	maxChecksumSize int64 = 1 << 20
)

var (
	updateHTTPTransport = http.DefaultTransport
	versionPattern      = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
)

func validateVersion(v string) error {
	if !versionPattern.MatchString(v) {
		return fmt.Errorf("invalid version %q: expected semantic version in major.minor.patch format", v)
	}
	return nil
}

// RunUpdate downloads and installs the latest version of dispatch. It
// prints progress to stderr and returns an error on failure.
func RunUpdate(currentVersion string) error {
	if isDevVersion(currentVersion) {
		return errors.New("cannot update a development build — install a release build first")
	}

	configDir, err := platform.ConfigDir()
	if err != nil {
		return fmt.Errorf("resolving config directory: %w", err)
	}
	if err := os.MkdirAll(configDir, configDirPerm); err != nil {
		return fmt.Errorf("ensuring config directory: %w", err)
	}

	lockPath := filepath.Join(configDir, lockFileName)
	lock, err := acquireUpdateLock(lockPath)
	if err != nil {
		return fmt.Errorf("another update is already in progress: %w", err)
	}
	defer releaseUpdateLock(lock)

	latest, err := fetchLatestVersion()
	if err != nil {
		return fmt.Errorf("checking latest version: %w", err)
	}
	if err := validateVersion(latest); err != nil {
		return fmt.Errorf("invalid latest version: %w", err)
	}

	if CompareVersions(latest, currentVersion) <= 0 {
		fmt.Fprintf(os.Stderr, "dispatch is already up to date (v%s)\n", currentVersion)
		return nil
	}

	fmt.Fprintf(os.Stderr, "Downloading dispatch v%s...\n", latest)

	// Create a temporary directory for the download and extraction.
	tmpDir, err := os.MkdirTemp("", "dispatch-update-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // best-effort cleanup

	// Download the release archive.
	asset := AssetName(latest)
	assetURL := fmt.Sprintf("%s/v%s/%s", downloadBaseURL, latest, asset)
	archivePath := filepath.Join(tmpDir, asset)
	if err := downloadAsset(archivePath, assetURL); err != nil {
		return fmt.Errorf("downloading %s: %w", asset, err)
	}

	// Download and verify checksum.
	checksumURL := fmt.Sprintf("%s/v%s/%s", downloadBaseURL, latest, checksumFileName)
	if err := verifyChecksum(archivePath, checksumURL, asset); err != nil {
		return fmt.Errorf("checksum verification: %w", err)
	}

	// Extract the binary from the archive.
	binPath, err := extractBinary(archivePath, tmpDir)
	if err != nil {
		return fmt.Errorf("extracting binary: %w", err)
	}

	// Replace the running binary.
	if err := replaceBinary(binPath); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}

	// Update the cache to reflect the new version.
	writeCache("", &updateCache{
		CheckedAt:      time.Now(),
		LatestVersion:  latest,
		CurrentVersion: latest,
	})

	fmt.Fprintf(os.Stderr, "Updated dispatch: v%s → v%s\n", currentVersion, latest)
	return nil
}

// AssetName returns the expected release archive filename for the current
// platform and architecture.
func AssetName(version string) string {
	return assetNameForPlatform(version, runtime.GOOS, runtime.GOARCH)
}

// assetNameForPlatform returns the archive filename for a given platform.
func assetNameForPlatform(version, goos, goarch string) string {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("dispatch_%s_%s_%s.%s", version, goos, goarch, ext)
}

func httpsOnlyCheckRedirect(req *http.Request, via []*http.Request) error {
	if req.URL.Scheme != "https" {
		return fmt.Errorf("refusing redirect to non-HTTPS URL: %s", req.URL)
	}
	if len(via) >= maxRedirects {
		return errors.New("too many redirects")
	}
	return nil
}

func newSecureClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		Transport:     updateHTTPTransport,
		CheckRedirect: httpsOnlyCheckRedirect,
	}
}

// downloadAsset downloads a URL to a local file path with HTTPS-only
// redirect enforcement and size limits.
func downloadAsset(dst, url string) error {
	client := newSecureClient(downloadTimeout)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request for %s: %w", url, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort cleanup

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	if resp.ContentLength > maxDownloadSize {
		return fmt.Errorf("download exceeds %d bytes", maxDownloadSize)
	}

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}

	if err := copyWithLimit(out, resp.Body, maxDownloadSize); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("writing %s: %w", dst, err)
	}

	return out.Close()
}

// verifyChecksum downloads the checksum file and verifies the SHA-256
// hash of the downloaded archive.
func verifyChecksum(archivePath, checksumURL, archiveName string) error {
	client := newSecureClient(apiTimeout)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, checksumURL, nil)
	if err != nil {
		return fmt.Errorf("creating checksum request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort cleanup

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d fetching checksums", resp.StatusCode)
	}
	if resp.ContentLength > maxChecksumSize {
		return fmt.Errorf("checksums file exceeds %d bytes", maxChecksumSize)
	}

	checksumData, err := io.ReadAll(io.LimitReader(resp.Body, maxChecksumSize+1))
	if err != nil {
		return fmt.Errorf("reading checksums: %w", err)
	}
	if int64(len(checksumData)) > maxChecksumSize {
		return fmt.Errorf("checksums file exceeds %d bytes", maxChecksumSize)
	}

	// Find the expected hash for our archive.
	expectedHash, err := ParseChecksum(string(checksumData), archiveName)
	if err != nil {
		return err
	}

	// Compute the actual hash of the downloaded file.
	actualHash, err := SHA256File(archivePath)
	if err != nil {
		return fmt.Errorf("computing checksum: %w", err)
	}

	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// ParseChecksum extracts the SHA-256 hash for a specific file from a
// checksums.txt file in "hash  filename\n" format.
func ParseChecksum(content, filename string) (string, error) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "<hash>  <filename>" (two spaces) or "<hash> <filename>"
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == filename {
			return strings.ToLower(parts[0]), nil
		}
	}
	return "", fmt.Errorf("no checksum found for %s", filename)
}

// SHA256File computes the SHA-256 hash of a file and returns it as a
// lowercase hex string.
func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // read-only

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyWithLimit(dst io.Writer, src io.Reader, maxBytes int64) error {
	limited := &io.LimitedReader{R: src, N: maxBytes + 1}
	written, err := io.Copy(dst, limited)
	if err != nil {
		return err
	}
	if written > maxBytes {
		return fmt.Errorf("payload exceeds %d bytes", maxBytes)
	}
	return nil
}

func normalizeArchiveEntryName(name string) string {
	normalized := strings.ReplaceAll(name, "\\", "/")
	normalized = strings.TrimPrefix(normalized, "./")
	return path.Clean(normalized)
}

func matchArchiveTarget(name, target string) (bool, error) {
	normalized := normalizeArchiveEntryName(name)
	if normalized == target {
		return true, nil
	}
	if path.Base(normalized) == target {
		return false, fmt.Errorf("unsafe archive entry path %q", name)
	}
	return false, nil
}

// extractBinary extracts the dispatch binary from a release archive
// (tar.gz or zip) and returns the path to the extracted file.
func extractBinary(archivePath, destDir string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractFromZip(archivePath, destDir)
	}
	return extractFromTarGz(archivePath, destDir)
}

// extractFromTarGz extracts the dispatch binary from a .tar.gz archive.
func extractFromTarGz(archivePath, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("opening archive: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only

	gr, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("decompressing archive: %w", err)
	}
	defer gr.Close() //nolint:errcheck // read-only

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading tar entry: %w", err)
		}

		match, err := matchArchiveTarget(header.Name, binaryName)
		if err != nil {
			return "", err
		}
		if !match {
			continue
		}
		if header.Typeflag != tar.TypeReg {
			return "", fmt.Errorf("unsupported tar entry type for %s", header.Name)
		}

		dst := filepath.Join(destDir, binaryName)
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, newBinaryPerm)
		if err != nil {
			return "", fmt.Errorf("creating %s: %w", dst, err)
		}

		if err := copyWithLimit(out, tr, maxDownloadSize); err != nil {
			_ = out.Close()
			_ = os.Remove(dst)
			return "", fmt.Errorf("extracting %s: %w", binaryName, err)
		}
		if err := out.Close(); err != nil {
			return "", fmt.Errorf("closing %s: %w", dst, err)
		}
		return dst, nil
	}
	return "", errors.New("dispatch binary not found in archive")
}

// extractFromZip extracts the dispatch binary from a .zip archive.
func extractFromZip(archivePath, destDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close() //nolint:errcheck // read-only

	target := binaryName + ".exe"
	for _, f := range r.File {
		match, err := matchArchiveTarget(f.Name, target)
		if err != nil {
			return "", err
		}
		if !match {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("opening %s in zip: %w", f.Name, err)
		}

		dst := filepath.Join(destDir, target)
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, newBinaryPerm)
		if err != nil {
			_ = rc.Close()
			return "", fmt.Errorf("creating %s: %w", dst, err)
		}

		if err := copyWithLimit(out, rc, maxDownloadSize); err != nil {
			_ = out.Close()
			_ = rc.Close()
			_ = os.Remove(dst)
			return "", fmt.Errorf("extracting %s: %w", target, err)
		}

		_ = rc.Close()
		if err := out.Close(); err != nil {
			return "", fmt.Errorf("closing %s: %w", dst, err)
		}
		return dst, nil
	}

	return "", errors.New("dispatch.exe not found in archive")
}

// replaceBinary replaces the currently running dispatch binary with a
// new version. On Unix, it uses an atomic rename. On Windows, it renames
// the running executable to .old before replacing.
func replaceBinary(newBinaryPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	exeDir := filepath.Dir(exePath)
	exeName := filepath.Base(exePath)

	if runtime.GOOS == "windows" {
		return replaceWindows(newBinaryPath, exeDir, exeName)
	}
	return replaceUnix(newBinaryPath, exePath)
}

// replaceUnix atomically replaces the binary via rename.
func replaceUnix(newBinaryPath, exePath string) error {
	// Write to a temp file in the same directory (same filesystem) to
	// ensure os.Rename is atomic.
	tmpPath := exePath + ".new"

	src, err := os.Open(newBinaryPath)
	if err != nil {
		return fmt.Errorf("opening new binary: %w", err)
	}
	defer src.Close() //nolint:errcheck // read-only

	dst, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, newBinaryPerm)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := dst.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, exePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replacing binary: %w", err)
	}

	return nil
}

// replaceWindows renames the running exe to .old, then copies the new
// binary and updates the disp.exe alias if present.
func replaceWindows(newBinaryPath, exeDir, exeName string) error {
	exePath := filepath.Join(exeDir, exeName)
	oldPath := exePath + oldBinarySuffix

	// Remove any previous .old file.
	_ = os.Remove(oldPath)

	// Rename running exe so we can write the new one.
	if err := os.Rename(exePath, oldPath); err != nil {
		return fmt.Errorf("renaming current binary: %w", err)
	}

	if err := copyFile(newBinaryPath, exePath); err != nil {
		rollbackErr := os.Rename(oldPath, exePath)
		if rollbackErr != nil {
			return fmt.Errorf("installing new binary: %w", errors.Join(err, fmt.Errorf("restoring original binary: %w", rollbackErr)))
		}
		return fmt.Errorf("installing new binary: %w", err)
	}

	// Best-effort cleanup of old binary.
	_ = os.Remove(oldPath)

	// Update disp.exe alias if it exists.
	aliasPath := filepath.Join(exeDir, aliasName+".exe")
	if _, err := os.Stat(aliasPath); err == nil {
		oldAlias := aliasPath + oldBinarySuffix
		_ = os.Remove(oldAlias)
		_ = os.Rename(aliasPath, oldAlias)
		_ = copyFile(exePath, aliasPath)
		_ = os.Remove(oldAlias)
	}

	return nil
}

// copyFile copies a file from src to dst, preserving executable
// permissions.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // read-only

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, newBinaryPerm)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}

	return out.Close()
}
