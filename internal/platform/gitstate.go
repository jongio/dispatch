// Package platform — gitstate.go provides lightweight Git workspace state
// detection for session directories. It runs bounded git commands with context
// timeouts to avoid blocking the TUI.
package platform

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// GitState represents the workspace state of a Git checkout.
type GitState int

const (
	// GitStateUnknown means the state has not been determined yet.
	GitStateUnknown GitState = iota
	// GitStateClean means the working tree is clean with no pending changes.
	GitStateClean
	// GitStateDirty means tracked files have uncommitted modifications.
	GitStateDirty
	// GitStateUntracked means untracked files exist in the working tree.
	GitStateUntracked
	// GitStateAhead means the local branch is ahead of its upstream.
	GitStateAhead
	// GitStateBehind means the local branch is behind its upstream.
	GitStateBehind
	// GitStateMissing means the session directory no longer exists on disk.
	GitStateMissing
)

// Git state label constants returned by [GitState.String].
const (
	gitLabelUnknown   = "unknown"
	gitLabelClean     = "clean"
	gitLabelDirty     = "dirty"
	gitLabelUntracked = "untracked"
	gitLabelAhead     = "ahead"
	gitLabelBehind    = "behind"
	gitLabelMissing   = "missing"
)

// String returns a human-readable label for the git state.
func (g GitState) String() string {
	switch g {
	case GitStateClean:
		return gitLabelClean
	case GitStateDirty:
		return gitLabelDirty
	case GitStateUntracked:
		return gitLabelUntracked
	case GitStateAhead:
		return gitLabelAhead
	case GitStateBehind:
		return gitLabelBehind
	case GitStateMissing:
		return gitLabelMissing
	default:
		return gitLabelUnknown
	}
}

// gitCommandTimeout is the maximum time allowed for a single git command.
const gitCommandTimeout = 2 * time.Second

// DetectGitState checks the Git workspace state of a directory. It returns
// GitStateMissing if the path does not exist, GitStateUnknown if git is not
// available or the directory is not a git repository, and the appropriate
// state otherwise.
//
// The function uses context-bounded exec calls to ensure it never blocks
// longer than gitCommandTimeout per command.
func DetectGitState(dir string) GitState {
	// Check if directory exists.
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return GitStateMissing
	}

	// Verify this is a git repository by running git rev-parse.
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return GitStateUnknown
	}

	// Run git status --porcelain to detect dirty/untracked state.
	state := gitStatusPorcelain(dir)
	if state != GitStateClean {
		return state
	}

	// Check ahead/behind relative to upstream.
	ahead, behind := gitAheadBehind(dir)
	if behind > 0 {
		return GitStateBehind
	}
	if ahead > 0 {
		return GitStateAhead
	}

	return GitStateClean
}

// gitStatusPorcelain runs `git status --porcelain` and returns the workspace
// state based on the output. Returns GitStateClean if no output.
func gitStatusPorcelain(dir string) GitState {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return GitStateUnknown
	}

	if len(out) == 0 {
		return GitStateClean
	}

	// Parse output: lines starting with "?" indicate untracked files only.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	hasModified := false
	hasUntracked := false
	for _, line := range lines {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "??") {
			hasUntracked = true
		} else {
			hasModified = true
		}
	}

	if hasModified {
		return GitStateDirty
	}
	if hasUntracked {
		return GitStateUntracked
	}

	return GitStateClean
}

// gitAheadBehind runs `git rev-list --left-right --count HEAD...@{u}` and
// returns the ahead/behind counts. Returns (0, 0) on any error (no upstream,
// timeout, etc.).
func gitAheadBehind(dir string) (ahead, behind int) {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-list", "--left-right", "--count", "HEAD...@{u}")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) != 2 {
		return 0, 0
	}

	a, errA := strconv.Atoi(parts[0])
	b, errB := strconv.Atoi(parts[1])
	if errA != nil || errB != nil {
		return 0, 0
	}
	return a, b
}

// ScanGitStates runs DetectGitState for each session directory in the provided
// map (session ID to directory path) and returns the results. This is designed
// to be called as a background command during refresh cycles.
func ScanGitStates(sessions map[string]string) map[string]GitState {
	results := make(map[string]GitState, len(sessions))
	for id, dir := range sessions {
		results[id] = DetectGitState(dir)
	}
	return results
}
