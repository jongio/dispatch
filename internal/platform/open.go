package platform

import (
	"context"
	"os/exec"
	"runtime"
)

// OpenFile opens the given file path using the platform default application.
// On Windows it uses explorer.exe (avoids cmd.exe metacharacter injection),
// on macOS "open", and on Linux "xdg-open".
func OpenFile(path string) error {
	ctx := context.Background()
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.CommandContext(ctx, "explorer", path)
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", path)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", path)
	}
	return cmd.Start()
}
