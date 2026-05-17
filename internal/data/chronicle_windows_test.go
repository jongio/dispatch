//go:build windows

package data

import (
"testing"

"github.com/UserExistsError/conpty"
)

// ---------------------------------------------------------------------------
// ptyHandle.Close - idempotent (second close must not panic)
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