//go:build windows

package platform

import (
	"io"
	"os"
)

// dialTSCode opens the Windows named pipe for tscode center IPC.
// Named pipes on Windows appear under \\.\pipe\ and can be opened as
// regular files, yielding a bidirectional stream suitable for JSON
// line-protocol communication.
func dialTSCode() (io.ReadWriteCloser, error) {
	return os.OpenFile(tscodePipePath(), os.O_RDWR, 0)
}
