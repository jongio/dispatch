package platform

import (
	"os/exec"
	"runtime"
)

// OpenFile opens the given file path using the platform default application.
// On Windows it uses explorer.exe (avoids cmd.exe metacharacter injection),
// on macOS "open", and on Linux "xdg-open".
func OpenFile(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}
