package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
)

// pathDirExistsFn reports whether dir exists on disk and is a directory. It is
// a package variable so tests can exercise the missing-directory branch without
// touching the filesystem, matching the seam pattern used elsewhere in this
// package (see open.go).
var pathDirExistsFn = defaultPathDirExists

// runPath prints a single session's working directory and nothing else, so it
// drops straight into a subshell:
//
//	cd "$(dispatch path my-alias)"
//
// It resolves the session the same way `dispatch open` does (full ID, alias,
// short ID prefix, --last, or --current) by reusing open's resolver seams, so
// the matching rules stay identical. The absolute path is written to w with a
// trailing newline. A session with no recorded directory, or a directory that
// no longer exists on disk, is reported to the caller as an error so
// `cd "$(...)"` fails loudly instead of landing in the wrong place. args[0] is
// expected to be "path".
func runPath(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	id, last, current, err := parsePathArgs(args)
	if err != nil {
		return err
	}

	cfg, err := openLoadConfigFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	sess, err := resolvePathSession(cfg, id, last, current)
	if err != nil {
		return err
	}

	dir := strings.TrimSpace(sess.Cwd)
	if dir == "" {
		return fmt.Errorf("session %s has no recorded directory", shortID(sess.ID))
	}
	if !pathDirExistsFn(dir) {
		return fmt.Errorf("directory %q for session %s no longer exists", dir, shortID(sess.ID))
	}

	fmt.Fprintln(w, dir)
	return nil
}

// parsePathArgs extracts the session ID and the --last / --current selectors
// from the path subcommand arguments. Exactly one selector is required: a
// positional ID, --last, or --current. args[0] is expected to be "path".
func parsePathArgs(args []string) (id string, last, current bool, err error) {
	rest := args
	if len(rest) > 0 {
		rest = rest[1:] // drop the "path" token
	}

	var positionals []string
	for _, arg := range rest {
		switch {
		case arg == "--last" || arg == "-l":
			last = true
		case arg == "--current":
			current = true
		case strings.HasPrefix(arg, "-"):
			return "", false, false, fmt.Errorf("unknown flag: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}

	selectors := 0
	if last {
		selectors++
	}
	if current {
		selectors++
	}
	if len(positionals) > 0 {
		selectors++
	}

	switch {
	case selectors == 0:
		return "", false, false, errors.New("path requires a session ID (or use --last or --current)")
	case selectors > 1:
		return "", false, false, errors.New("path takes a single session ID, --last, or --current, not a combination")
	case len(positionals) > 1:
		return "", false, false, fmt.Errorf("path accepts a single session ID, got %d arguments", len(positionals))
	}

	if len(positionals) == 1 {
		id = positionals[0]
	}
	return id, last, current, nil
}

// resolvePathSession loads the session named by the parsed selectors, reusing
// the same store seams as open so ID, alias, prefix, --last, and --current all
// resolve identically.
func resolvePathSession(cfg *config.Config, id string, last, current bool) (*data.Session, error) {
	switch {
	case last:
		sess, err := openGetLastSessionFn()
		if err != nil {
			return nil, err
		}
		if sess == nil {
			return nil, errors.New("no sessions found")
		}
		return sess, nil

	case current:
		repo, branch, err := openDetectGitFn()
		if err != nil {
			return nil, fmt.Errorf("--current: %w", err)
		}
		if repo == "" && branch == "" {
			return nil, errors.New("--current: could not detect a repository or branch from the current directory")
		}
		sessions, err := openListSessionsFn(data.FilterOptions{Repository: repo, Branch: branch})
		if err != nil {
			return nil, err
		}
		if len(sessions) == 0 {
			return nil, errors.New("no sessions found matching the current repository and branch")
		}
		return &sessions[0], nil

	default:
		// Resolve the argument as an alias first, then fall back to treating it
		// as a session ID or prefix, mirroring open.
		if resolved := cfg.SessionIDForAlias(id); resolved != "" {
			id = resolved
		}
		sess, err := openGetSessionFn(id)
		if err != nil {
			return nil, err
		}
		if sess == nil {
			return nil, fmt.Errorf("session %q not found", id)
		}
		return sess, nil
	}
}

// defaultPathDirExists reports whether dir exists on disk and is a directory.
func defaultPathDirExists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}
