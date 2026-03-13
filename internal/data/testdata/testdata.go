// Package testdata provides access to the fake session store database
// used for integration tests and screenshot demos.
package testdata

import (
	"path/filepath"
	"runtime"
)

// Path returns the absolute path to the fake_sessions.db SQLite database.
// The path is resolved relative to this source file, so it works regardless
// of the caller's working directory.
func Path() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "fake_sessions.db")
}
