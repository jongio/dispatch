//go:build !windows

package data

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/creack/pty"
)

// ptyDimCols and ptyDimRows control the pseudo-terminal size used when
// launching the Copilot CLI for reindexing.
const (
	ptyDimCols = 120
	ptyDimRows = 40
)

// ptyHandle wraps a Unix PTY file descriptor. Close is idempotent:
// the underlying file is closed exactly once even if Close is called
// multiple times (e.g. explicit cancel close + deferred cleanup).
type ptyHandle struct {
	ptmx      *os.File
	closeOnce sync.Once
	closeErr  error
}

func (p *ptyHandle) Read(buf []byte) (int, error)  { return p.ptmx.Read(buf) }
func (p *ptyHandle) Write(buf []byte) (int, error) { return p.ptmx.Write(buf) }
func (p *ptyHandle) Close() error {
	p.closeOnce.Do(func() {
		p.closeErr = p.ptmx.Close()
	})
	return p.closeErr
}

// startPTY launches the Copilot CLI binary inside a Unix PTY so it
// believes it has an interactive terminal.
func startPTY(binary string) (io.ReadWriteCloser, error) {
	cmd := exec.Command(binary, "--no-auto-update", "--no-color", "--no-custom-instructions")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: ptyDimRows, Cols: ptyDimCols})
	if err != nil {
		return nil, err
	}
	return &ptyHandle{ptmx: ptmx}, nil
}

// findCopilotBinary returns the path to the copilot binary on Unix systems.
func findCopilotBinary() string {
	// Try PATH first.
	if p, err := exec.LookPath("copilot"); err == nil {
		return p
	}

	home, _ := os.UserHomeDir()

	// Determine platform-specific binary directory name.
	var binaryDir string
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			binaryDir = "copilot-darwin-arm64"
		} else {
			binaryDir = "copilot-darwin-x64"
		}
	case "linux":
		if runtime.GOARCH == "arm64" {
			binaryDir = "copilot-linux-arm64"
		} else {
			binaryDir = "copilot-linux-x64"
		}
	}

	if binaryDir == "" {
		return ""
	}

	// Check common npm global install locations.
	candidates := []string{
		filepath.Join(home, ".npm-global", "lib", "node_modules",
			"@github", "copilot", "node_modules", "@github", binaryDir, "copilot"),
		filepath.Join("/usr", "local", "lib", "node_modules",
			"@github", "copilot", "node_modules", "@github", binaryDir, "copilot"),
		filepath.Join("/usr", "lib", "node_modules",
			"@github", "copilot", "node_modules", "@github", binaryDir, "copilot"),
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
