//go:build windows

package data

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/UserExistsError/conpty"
)

// ---------------------------------------------------------------------------
// findCopilotBinary — APPDATA candidate path
// ---------------------------------------------------------------------------

func TestFindCopilotBinary_APPDATACandidate(t *testing.T) {
	// Create a temp dir to act as APPDATA and plant a fake copilot.exe at
	// the expected path so findCopilotBinary discovers it.
	tmp := t.TempDir()

	exePath := filepath.Join(tmp, "npm", "node_modules",
		"@github", "copilot", "node_modules", "@github", "copilot-win32-x64", "copilot.exe")

	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("creating candidate dir: %v", err)
	}
	if err := os.WriteFile(exePath, []byte("fake"), 0o755); err != nil {
		t.Fatalf("writing fake binary: %v", err)
	}

	// Override APPDATA and clear ProgramFiles + PATH so the APPDATA
	// candidate is the one that gets matched.
	t.Setenv("APPDATA", tmp)
	t.Setenv("ProgramFiles", filepath.Join(tmp, "nonexistent-progfiles"))
	t.Setenv("PATH", "")

	got := findCopilotBinary()
	if got != exePath {
		t.Errorf("findCopilotBinary() = %q, want %q", got, exePath)
	}
}

func TestFindCopilotBinary_NotFound(t *testing.T) {
	tmp := t.TempDir()

	// Point all env vars at empty dirs so no candidate is found.
	t.Setenv("APPDATA", tmp)
	t.Setenv("ProgramFiles", tmp)
	t.Setenv("PATH", "")

	got := findCopilotBinary()
	if got != "" {
		t.Errorf("findCopilotBinary() = %q, want empty string", got)
	}
}

// ---------------------------------------------------------------------------
// ptyHandle.Close — idempotent (second close must not panic)
// ---------------------------------------------------------------------------

func TestPtyHandle_CloseIdempotent(t *testing.T) {
	// Start a real (but short-lived) ConPTY process.
	cpty, err := conpty.Start("cmd /c echo test",
		conpty.ConPtyDimensions(ptyDimCols, ptyDimRows))
	if err != nil {
		t.Fatalf("conpty.Start: %v", err)
	}

	h := &ptyHandle{cpty: cpty}

	// First close should succeed.
	if err := h.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}

	// Second close must not panic and should return the same error.
	err2 := h.Close()
	if err2 != nil {
		t.Errorf("second Close: %v (expected nil, same as first)", err2)
	}
}
