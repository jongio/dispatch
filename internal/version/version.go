// Package version holds the application version string, set at build time
// via ldflags.  Placing it in its own package avoids coupling application
// metadata to any particular layer (TUI, CLI, etc.).
package version

// Version is set at build time via ldflags. Defaults to "dev" for
// development builds.
var Version = "dev"
