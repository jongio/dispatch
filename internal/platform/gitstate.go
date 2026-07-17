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

// gitSafeArgs prepends configuration flags that neutralize executable Git
// config before the given git subcommand and its arguments. These commands run
// with cmd.Dir set to a session's working directory, which is untrusted input
// (it originates from the local session store and may point at an
// attacker-crafted repository). A repository's local .git/config can set
// core.fsmonitor to an arbitrary program, which `git status` executes while
// collecting state — arbitrary code execution in the user's context
// (CWE-829). Forcing core.fsmonitor=false (and clearing core.hooksPath as
// defense in depth) prevents Git from invoking any repository-supplied program.
func gitSafeArgs(args ...string) []string {
	out := make([]string, 0, 4+len(args))
	out = append(out, "-c", "core.fsmonitor=false", "-c", "core.hooksPath=")
	return append(out, args...)
}

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
	cmd := exec.CommandContext(ctx, "git", gitSafeArgs("rev-parse", "--git-dir")...)
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

	cmd := exec.CommandContext(ctx, "git", gitSafeArgs("status", "--porcelain")...)
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

	cmd := exec.CommandContext(ctx, "git", gitSafeArgs("rev-list", "--left-right", "--count", "HEAD...@{u}")...)
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

// ---------------------------------------------------------------------------
// Detailed git status (push/pull stats + working-tree counts)
// ---------------------------------------------------------------------------

// maxGitStatusFiles caps the number of changed-file entries retained by
// DetectGitStatus so a pathologically large working tree cannot exhaust memory
// or overwhelm the overlay. Counts remain complete; only the file list is
// truncated (GitStatus.Truncated reports when this happens).
const maxGitStatusFiles = 500

// detachedHeadLabel is the branch.head value git reports for a detached HEAD.
const detachedHeadLabel = "(detached)"

// GitFileStatus is a single changed path paired with its short two-character
// status code as shown by `git status --short` (e.g. " M", "??", "A ", "UU").
type GitFileStatus struct {
	Code string
	Path string
}

// GitStatus is a detailed snapshot of a directory's Git workspace. Unlike
// GitState (a single collapsed enum used for the list badge), it carries the
// standard push/pull counts and per-category working-tree counts needed for a
// full status view.
type GitStatus struct {
	Dir         string
	Exists      bool // directory exists on disk
	IsRepo      bool // directory is inside a Git work tree
	Branch      string
	Detached    bool
	Upstream    string // upstream ref, empty when there is none
	HasUpstream bool
	Ahead       int // commits ahead of upstream (to push)
	Behind      int // commits behind upstream (to pull)
	Staged      int // entries with an index (staged) change
	Modified    int // entries modified in the work tree (not staged)
	Untracked   int // untracked files
	Deleted     int // entries deleted in the work tree (not staged)
	Conflicts   int // unmerged (conflicted) entries

	Files     []GitFileStatus // changed entries, capped at maxGitStatusFiles
	Truncated bool            // Files was capped
}

// Clean reports whether the working tree has no changes of any category.
func (s GitStatus) Clean() bool {
	return s.Staged == 0 && s.Modified == 0 && s.Untracked == 0 &&
		s.Deleted == 0 && s.Conflicts == 0
}

// DetectGitStatus gathers a detailed Git status for dir. It returns a GitStatus
// with Exists=false when the path is missing, IsRepo=false when the path is not
// a Git repository (or git is unavailable / times out), and the fully populated
// status otherwise.
//
// It runs a single bounded `git status --porcelain=v2 --branch` command so the
// branch headers, ahead/behind counts, and changed entries come from one
// consistent snapshot and the call never blocks longer than gitCommandTimeout.
func DetectGitStatus(dir string) GitStatus {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return GitStatus{Dir: dir, Exists: false}
	}

	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	// core.quotepath=false keeps non-ASCII paths readable instead of octal-escaped.
	// gitSafeArgs neutralizes executable config (core.fsmonitor) since dir is untrusted.
	cmd := exec.CommandContext(ctx, "git",
		gitSafeArgs("-c", "core.quotepath=false", "status", "--porcelain=v2", "--branch")...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// Not a repository, git missing, or timeout.
		return GitStatus{Dir: dir, Exists: true, IsRepo: false}
	}

	s := parseGitStatusV2(string(out))
	s.Dir = dir
	s.Exists = true
	s.IsRepo = true
	return s
}

// parseGitStatusV2 parses the output of `git status --porcelain=v2 --branch`
// into a GitStatus. It is a pure function (no I/O) so it can be unit tested
// without a live repository. It does not set Dir/Exists/IsRepo — DetectGitStatus
// fills those from the on-disk check.
func parseGitStatusV2(output string) GitStatus {
	var s GitStatus
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case '#':
			parseBranchHeader(line, &s)
		case '1', '2':
			parseTrackedEntry(line, &s)
		case 'u':
			s.Conflicts++
			s.addFile(unmergedEntry(line))
		case '?':
			s.Untracked++
			s.addFile("??", strings.TrimPrefix(line, "? "))
		case '!':
			// Ignored files — not reported.
		}
	}
	return s
}

// parseBranchHeader parses a `# branch.*` header line into s.
func parseBranchHeader(line string, s *GitStatus) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return
	}
	switch fields[1] {
	case "branch.head":
		if fields[2] == detachedHeadLabel {
			s.Detached = true
			s.Branch = detachedHeadLabel
		} else {
			s.Branch = fields[2]
		}
	case "branch.upstream":
		s.Upstream = fields[2]
		s.HasUpstream = true
	case "branch.ab":
		// Format: "# branch.ab +<ahead> -<behind>".
		if len(fields) >= 4 {
			s.Ahead = atoiSign(fields[2])
			s.Behind = atoiSign(fields[3])
		}
	}
}

// parseTrackedEntry parses an ordinary ('1') or rename/copy ('2') changed entry
// and updates the per-category counts plus the file list.
func parseTrackedEntry(line string, s *GitStatus) {
	// XY is always the second space-delimited token.
	fields := strings.SplitN(line, " ", 3)
	if len(fields) < 2 || len(fields[1]) < 2 {
		return
	}
	xy := fields[1]
	x, y := xy[0], xy[1]

	if x != '.' {
		s.Staged++
	}
	switch y {
	case 'M', 'T':
		s.Modified++
	case 'D':
		s.Deleted++
	}

	s.addFile(shortCode(xy), pathFromTracked(line))
}

// pathFromTracked extracts the display path from a '1' or '2' porcelain-v2
// record. The path is always the final space-delimited token, so splitting on
// spaces up to the fixed field count keeps paths that contain spaces intact.
// For rename/copy ('2') records the field is "<new>\t<orig>"; only the new path
// is shown.
func pathFromTracked(line string) string {
	// Ordinary entries have 8 fixed fields before the path; rename/copy have 9.
	fixed := 9 // SplitN count for '1': 8 fields + path
	if line[0] == '2' {
		fixed = 10 // '2' adds the <Xscore> field before the path
	}
	parts := strings.SplitN(line, " ", fixed)
	if len(parts) < fixed {
		return ""
	}
	path := parts[fixed-1]
	if line[0] == '2' {
		if tab := strings.IndexByte(path, '\t'); tab >= 0 {
			path = path[:tab]
		}
	}
	return path
}

// unmergedEntry returns the short code and path for an unmerged ('u') entry.
// Unmerged records carry a two-char XY at token index 1 and the path as the
// final token, with 10 fixed fields preceding it.
func unmergedEntry(line string) (code, path string) {
	fields := strings.SplitN(line, " ", 3)
	code = "UU"
	if len(fields) >= 2 && len(fields[1]) >= 2 {
		code = shortCode(fields[1])
	}
	parts := strings.SplitN(line, " ", 11)
	if len(parts) == 11 {
		path = parts[10]
	}
	return code, path
}

// addFile appends a changed-file entry, capping the slice at maxGitStatusFiles
// and flagging truncation once the cap is reached.
func (s *GitStatus) addFile(code, path string) {
	if path == "" {
		return
	}
	if len(s.Files) >= maxGitStatusFiles {
		s.Truncated = true
		return
	}
	s.Files = append(s.Files, GitFileStatus{Code: code, Path: path})
}

// shortCode converts a porcelain-v2 XY status into the two-character short
// form used by `git status --short`, rendering unchanged positions ('.') as
// spaces (e.g. "M." -> "M ", ".M" -> " M").
func shortCode(xy string) string {
	b := []byte(xy[:2])
	for i := range b {
		if b[i] == '.' {
			b[i] = ' '
		}
	}
	return string(b)
}

// atoiSign parses a signed count token like "+2" or "-3" into its magnitude.
// Invalid input yields 0.
func atoiSign(tok string) int {
	tok = strings.TrimLeft(tok, "+-")
	n, err := strconv.Atoi(tok)
	if err != nil {
		return 0
	}
	return n
}
