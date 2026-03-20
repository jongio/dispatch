// Package data — plans.go provides functions to detect and read Copilot CLI
// plan.md files from session-state directories.
package data

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// maxPlanFileSize caps the number of bytes read from a plan.md file to
// prevent memory exhaustion from adversarially large files.
const maxPlanFileSize = 64 * 1024 // 64 KB

// sessionIDPattern matches safe session ID values (UUIDs, hex strings, and
// similar tokens). Replicates the pattern from platform/shell.go to avoid
// an import cycle.
var sessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)

// ScanPlans checks which of the given session IDs have a plan.md file in
// their session-state directory. Returns a map of session ID → true for
// sessions that have a plan. Sessions without a plan are omitted from the
// map (not set to false) for efficiency.
func ScanPlans(sessionIDs []string) map[string]bool {
	stateDir := sessionStatePath()
	if stateDir == "" {
		return nil
	}

	result := make(map[string]bool, len(sessionIDs)/4) // expect ~25% to have plans
	for _, id := range sessionIDs {
		if !sessionIDPattern.MatchString(id) {
			continue
		}
		planPath := filepath.Join(stateDir, id, "plan.md")
		info, err := os.Lstat(planPath)
		if err != nil {
			continue
		}
		// Only accept regular files (no symlinks, devices, etc.).
		if info.Mode().IsRegular() && info.Size() > 0 {
			result[id] = true
		}
	}
	return result
}

// ScanAllPlans reads the session-state directory and returns a map of
// session ID → true for every session that has a non-empty plan.md file.
// Unlike ScanPlans, this does not require a pre-filtered list of IDs —
// it discovers sessions from the filesystem directly, matching the
// pattern used by ScanAttention.
func ScanAllPlans() map[string]bool {
	stateDir := sessionStatePath()
	if stateDir == "" {
		return nil
	}

	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return nil
	}

	result := make(map[string]bool, len(entries)/4)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		if !sessionIDPattern.MatchString(id) {
			continue
		}
		planPath := filepath.Join(stateDir, id, "plan.md")
		info, err := os.Lstat(planPath)
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() && info.Size() > 0 {
			result[id] = true
		}
	}
	return result
}

// ReadPlanContent reads the plan.md file for the given session ID.
// Returns the plan content as a string, capped at maxPlanFileSize bytes.
// Returns an empty string and an error if the file cannot be read or the
// session ID is invalid.
func ReadPlanContent(sessionID string) (string, error) {
	path, err := PlanFilePath(sessionID)
	if err != nil {
		return "", err
	}

	// Use Lstat to reject symlinks before reading.
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", &os.PathError{Op: "read", Path: path, Err: os.ErrInvalid}
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Read up to maxPlanFileSize bytes using LimitReader to handle
	// short reads correctly (f.Read is not guaranteed to fill the buffer).
	content, err := io.ReadAll(io.LimitReader(f, maxPlanFileSize))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// PlanFilePath returns the absolute path to the plan.md file for the
// given session ID. Returns an error if the session ID is invalid or
// the session-state directory cannot be resolved.
func PlanFilePath(sessionID string) (string, error) {
	if !sessionIDPattern.MatchString(sessionID) {
		return "", &os.PathError{Op: "open", Path: sessionID, Err: os.ErrInvalid}
	}

	stateDir := sessionStatePath()
	if stateDir == "" {
		return "", &os.PathError{Op: "open", Path: sessionID, Err: os.ErrNotExist}
	}

	path := filepath.Join(stateDir, sessionID, "plan.md")

	// Verify the resolved path is still under the session-state directory
	// to prevent path traversal via crafted session IDs like "a/../../../etc".
	cleaned := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(strings.ToLower(cleaned), strings.ToLower(stateDir+string(filepath.Separator))) {
			return "", &os.PathError{Op: "open", Path: sessionID, Err: os.ErrPermission}
		}
	} else {
		if !strings.HasPrefix(cleaned, stateDir+string(filepath.Separator)) {
			return "", &os.PathError{Op: "open", Path: sessionID, Err: os.ErrPermission}
		}
	}

	return path, nil
}
