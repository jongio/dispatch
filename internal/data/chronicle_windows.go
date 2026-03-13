//go:build windows

package data

import (
	"io"
	"os"
	"path/filepath"

	"github.com/UserExistsError/conpty"
)

// ptyDimCols and ptyDimRows control the pseudo-terminal size used when
// launching the Copilot CLI for reindexing.
const (
	ptyDimCols = 120
	ptyDimRows = 40
)

// ptyHandle wraps a Windows ConPTY pseudo-console.
type ptyHandle struct {
	cpty *conpty.ConPty
}

func (p *ptyHandle) Read(buf []byte) (int, error)  { return p.cpty.Read(buf) }
func (p *ptyHandle) Write(buf []byte) (int, error) { return p.cpty.Write(buf) }
func (p *ptyHandle) Close() error                  { return p.cpty.Close() }

// startPTY launches the Copilot CLI binary inside a Windows ConPTY
// pseudo-console so it believes it has an interactive terminal.
func startPTY(binary string) (io.ReadWriteCloser, error) {
	args := `"` + binary + `" --no-auto-update --no-color --no-custom-instructions`
	cpty, err := conpty.Start(args, conpty.ConPtyDimensions(ptyDimCols, ptyDimRows))
	if err != nil {
		return nil, err
	}
	return &ptyHandle{cpty: cpty}, nil
}

// findCopilotBinary returns the path to copilot.exe on Windows.
func findCopilotBinary() string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "nodejs", "node_modules",
			"@github", "copilot", "node_modules", "@github", "copilot-win32-x64", "copilot.exe"),
	}
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		candidates = append(candidates,
			filepath.Join(appdata, "npm", "node_modules",
				"@github", "copilot", "node_modules", "@github", "copilot-win32-x64", "copilot.exe"),
		)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
