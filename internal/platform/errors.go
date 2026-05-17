package platform

import "errors"

// Sentinel errors returned by platform operations.
var (
	ErrCLINotFound         = errors.New("ghcs/copilot CLI not found in PATH")
	ErrEmptyAfterExpansion = errors.New("custom command is empty after expansion")
	ErrNoTerminalEmulator  = errors.New("no supported terminal emulator found")
)
