package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// openCommand builds the platform command used to open a path with the
// default handler: explorer.exe on Windows (which avoids cmd.exe
// metacharacter injection), "open" on macOS, and "xdg-open" elsewhere.
func openCommand(ctx context.Context, path string) *exec.Cmd {
	switch runtime.GOOS {
	case "windows":
		return exec.CommandContext(ctx, "explorer", path)
	case "darwin":
		return exec.CommandContext(ctx, "open", path)
	default:
		return exec.CommandContext(ctx, "xdg-open", path)
	}
}

// OpenFile opens the given file path using the platform default application.
// On Windows it uses explorer.exe (avoids cmd.exe metacharacter injection),
// on macOS "open", and on Linux "xdg-open".
func OpenFile(path string) error {
	return openCommand(context.Background(), path).Start()
}

// OpenDir opens the given directory in the platform file manager. It returns
// an error if the path is empty or is not an existing directory, so callers
// can surface a clear message instead of spawning against a bad path.
func OpenDir(path string) error {
	if path == "" {
		return fmt.Errorf("no directory to open")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("directory not found: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}
	return openCommand(context.Background(), path).Start()
}
