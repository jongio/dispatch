//go:build !windows

package platform

import (
	"os"
	"strings"
	"syscall"
)

// dangerousEnvPrefixes lists environment variable names that should be
// stripped before passing to child processes to prevent library injection.
var dangerousEnvPrefixes = []string{
	"LD_PRELOAD",
	"LD_LIBRARY_PATH",
	"LD_AUDIT",
	"DYLD_INSERT_LIBRARIES",
	"DYLD_LIBRARY_PATH",
	"DYLD_FRAMEWORK_PATH",
}

// filterEnv returns a copy of env with dangerous variables removed.
func filterEnv(env []string) []string {
	result := make([]string, 0, len(env))
	for _, e := range env {
		name := e
		if idx := strings.IndexByte(e, '='); idx >= 0 {
			name = e[:idx]
		}
		dangerous := false
		for _, prefix := range dangerousEnvPrefixes {
			if strings.EqualFold(name, prefix) {
				dangerous = true
				break
			}
		}
		if !dangerous {
			result = append(result, e)
		}
	}
	return result
}

func launchInPlaceUnix(shell ShellInfo, resumeCmd string, cwd string) error {
	if cwd != "" {
		// Best-effort: change directory before exec replaces the process.
		// If chdir fails we still launch from the current directory.
		_ = os.Chdir(cwd)
	}

	argv := []string{shell.Path}
	argv = append(argv, shell.Args...)
	argv = append(argv, "-c", resumeCmd)

	return syscall.Exec(shell.Path, argv, filterEnv(os.Environ()))
}
