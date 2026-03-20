//go:build !windows

package main

// checkTerminalCompat detects terminal environments where the TUI cannot
// render correctly. On non-Windows platforms this is a no-op — MSYS/MinTTY
// compatibility issues are Windows-specific.
func checkTerminalCompat() string { return "" }
