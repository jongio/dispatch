package platform

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// downloadTimeout limits the overall HTTP request duration for font downloads.
	downloadTimeout = 60 * time.Second
	// maxRedirects caps the number of HTTP redirects followed during download.
	maxRedirects = 10

	// defaultMaxFileSize limits each individual file extracted from a font zip.
	defaultMaxFileSize int64 = 50 << 20 // 50 MiB
	// defaultMaxTotalSize limits the cumulative bytes extracted from a font zip.
	defaultMaxTotalSize int64 = 500 << 20 // 500 MiB

	// nerdFontZipURL is the stable download URL for the JetBrainsMono Nerd
	// Font from the official nerd-fonts GitHub releases.
	//
	// Security: The URL uses HTTPS to GitHub Releases, which mitigates
	// man-in-the-middle attacks via TLS certificate validation. The
	// download client rejects redirects to non-HTTPS URLs to prevent
	// downgrade attacks. The download is additionally size-limited
	// (see maxDownloadSize in downloadFile) to prevent resource
	// exhaustion from unexpectedly large responses.
	nerdFontZipURL = "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/JetBrainsMono.zip"
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

// InstallNerdFont downloads the JetBrainsMono Nerd Font archive, extracts
// .ttf files, and installs them to the user-level font directory. The
// function is safe to call from a goroutine.
func InstallNerdFont() error {
	tmpDir, err := os.MkdirTemp("", "nerd-font-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // best-effort cleanup

	zipPath := filepath.Join(tmpDir, "JetBrainsMono.zip")
	if err := downloadFile(zipPath, nerdFontZipURL); err != nil {
		return fmt.Errorf("downloading font: %w", err)
	}

	ttfFiles, err := extractTTF(zipPath, tmpDir)
	if err != nil {
		return fmt.Errorf("extracting font: %w", err)
	}
	if len(ttfFiles) == 0 {
		return errors.New("no .ttf files found in archive")
	}

	switch runtime.GOOS {
	case "windows":
		return installFontsWindows(ttfFiles)
	case "darwin":
		return installFontsDarwin(ttfFiles)
	default:
		return installFontsLinux(ttfFiles)
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

// ---------------------------------------------------------------------------
// Installation helpers
// ---------------------------------------------------------------------------

func installFontsWindows(ttfFiles []string) error {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return errors.New("LOCALAPPDATA environment variable not set")
	}
	fontDir := filepath.Join(localAppData, "Microsoft", "Windows", "Fonts")
	if err := os.MkdirAll(fontDir, 0o755); err != nil {
		return fmt.Errorf("creating font directory: %w", err)
	}

	for _, src := range ttfFiles {
		dst := filepath.Join(fontDir, filepath.Base(src))
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copying font %s: %w", filepath.Base(src), err)
		}
		// Register the font in the per-user registry so Windows discovers it
		// without requiring admin privileges or a reboot.
		fontName := strings.TrimSuffix(filepath.Base(src), ".ttf") + " (TrueType)"
		_ = exec.Command("reg", "add",
			`HKCU\Software\Microsoft\Windows NT\CurrentVersion\Fonts`,
			"/v", fontName, "/t", "REG_SZ", "/d", dst, "/f",
		).Run()
	}
	return nil
}

func installFontsDarwin(ttfFiles []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolving home directory: %w", err)
	}
	fontDir := filepath.Join(home, "Library", "Fonts")
	if err := os.MkdirAll(fontDir, 0o755); err != nil {
		return fmt.Errorf("creating font directory: %w", err)
	}
	for _, src := range ttfFiles {
		dst := filepath.Join(fontDir, filepath.Base(src))
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copying font %s: %w", filepath.Base(src), err)
		}
	}
	return nil
}

func installFontsLinux(ttfFiles []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolving home directory: %w", err)
	}
	fontDir := filepath.Join(home, ".local", "share", "fonts")
	if err := os.MkdirAll(fontDir, 0o755); err != nil {
		return fmt.Errorf("creating font directory: %w", err)
	}
	for _, src := range ttfFiles {
		dst := filepath.Join(fontDir, filepath.Base(src))
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copying font %s: %w", filepath.Base(src), err)
		}
	}
	// Refresh the font cache so fc-match/applications discover the new fonts.
	_ = exec.Command("fc-cache", "-f").Run()
	return nil
}

// ---------------------------------------------------------------------------
// File utilities
// ---------------------------------------------------------------------------

func downloadFile(dst, url string) error {
	client := &http.Client{
		Timeout: downloadTimeout,
		// Reject redirects to non-HTTPS URLs to prevent cleartext
		// transmission of the download (CWE-319).
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" {
				return fmt.Errorf("refusing redirect to non-HTTPS URL: %s", req.URL)
			}
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
	resp, err := client.Get(url) //nolint:gosec // URL is a compile-time constant
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort cleanup

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	// Limit download to 256 MiB to prevent resource exhaustion from
	// an unexpectedly large or malicious HTTP response.
	const maxDownloadSize = 256 << 20
	if _, err = io.Copy(out, io.LimitReader(resp.Body, maxDownloadSize)); err != nil {
		_ = out.Close()
		return err
	}

	return out.Close()
}

// extractTTF extracts .ttf files from a zip using production size limits.
func extractTTF(zipPath, destDir string) ([]string, error) {
	return extractTTFWithLimits(zipPath, destDir, defaultMaxFileSize, defaultMaxTotalSize)
}

// extractTTFWithLimits extracts .ttf files from a zip with configurable size
// limits. maxFile limits each individual file; maxTotal limits cumulative
// extracted bytes.
func extractTTFWithLimits(zipPath, destDir string, maxFile, maxTotal int64) ([]string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close() //nolint:errcheck // best-effort cleanup

	var extracted []string
	var totalExtracted int64
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(f.Name)
		if !strings.HasSuffix(strings.ToLower(name), ".ttf") {
			continue
		}
		// Guard against path traversal: reject names that are empty, a
		// current/parent directory reference, or contain separators after
		// Base() (should never happen, but defence-in-depth).
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
			continue
		}

		dst := filepath.Join(destDir, name)
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("opening %s in zip: %w", f.Name, err)
		}

		out, err := os.Create(dst)
		if err != nil {
			_ = rc.Close()
			return nil, fmt.Errorf("creating %s: %w", dst, err)
		}

		n, err := io.Copy(out, io.LimitReader(rc, maxFile+1))
		if err != nil {
			_ = out.Close()
			_ = rc.Close()
			return nil, fmt.Errorf("extracting %s: %w", f.Name, err)
		}
		if n > maxFile {
			_ = out.Close()
			_ = rc.Close()
			return nil, fmt.Errorf("file %s exceeds maximum size of %d bytes", f.Name, maxFile)
		}
		totalExtracted += n
		if totalExtracted > maxTotal {
			_ = out.Close()
			_ = rc.Close()
			return nil, fmt.Errorf("total extracted size exceeds maximum of %d bytes", maxTotal)
		}
		_ = out.Close()
		_ = rc.Close()
		extracted = append(extracted, dst)
	}
	return extracted, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // best-effort cleanup

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err = io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}

	return out.Close()
}
