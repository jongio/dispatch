package platform

import (
	"testing"
)

func TestOpenFile_NonExistentPath(t *testing.T) {
	t.Parallel()
	// OpenFile should not panic even for a non-existent path.
	// It will start the OS opener which may fail silently or report
	// an error through its own UI. We only verify no Go-level panic.
	err := OpenFile("/nonexistent/path/that/does/not/exist.txt")
	// On Windows cmd /c start returns immediately without error even for
	// missing paths; on Linux/macOS xdg-open/open may or may not error.
	// We just verify no panic occurred.
	_ = err
}
