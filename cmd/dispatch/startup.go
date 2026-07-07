package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jongio/dispatch/internal/data"
)

// startupOptions holds the initial TUI state parsed from the command line: an
// optional free-text search query plus structured filters that seed the first
// session query. A zero value behaves like a plain "dispatch" launch.
type startupOptions struct {
	Query      string
	Repository string
	Branch     string
	Folder     string
}

// SeedQuery renders the options as a search-bar query string using the same
// token grammar the TUI parses (repo:, branch:, folder:, plus free text). The
// caller feeds the result to the model so the filters seed the first query and
// show up in the search header. Values containing spaces are quoted so the
// tokenizer keeps them intact. An empty startupOptions yields "".
func (o startupOptions) SeedQuery() string {
	var parts []string
	if o.Repository != "" {
		parts = append(parts, "repo:"+quoteTokenValue(o.Repository))
	}
	if o.Branch != "" {
		parts = append(parts, "branch:"+quoteTokenValue(o.Branch))
	}
	if o.Folder != "" {
		parts = append(parts, "folder:"+quoteTokenValue(o.Folder))
	}
	if o.Query != "" {
		parts = append(parts, o.Query)
	}
	return strings.Join(parts, " ")
}

// quoteTokenValue wraps a token value in double quotes when it contains
// whitespace so the search tokenizer reads it as a single value.
func quoteTokenValue(v string) string {
	if strings.ContainsAny(v, " \t") {
		return `"` + v + `"`
	}
	return v
}

// startupFlags is the raw parse of the startup filter flags before any git
// detection or path validation runs.
type startupFlags struct {
	repo    string
	branch  string
	cwd     string
	query   string
	current bool
	// queryParts collects bare non-flag tokens (dispatch "some query").
	queryParts []string
}

// detectGitRepoFn resolves the owner/repo slug and branch for a directory. It
// is a seam so tests can substitute detection without a real git checkout.
var detectGitRepoFn = detectGitRepo

// resolveStartupOptions turns parsed startup flags into concrete startup
// options. It validates a --cwd path, and when --current is set it detects the
// git repository and branch from the base directory. It returns a clear error
// (with no side effects) when a path is invalid or a directory is not a git
// repository, so the caller can exit without starting the TUI.
func resolveStartupOptions(f startupFlags) (startupOptions, error) {
	opts := startupOptions{
		Repository: f.repo,
		Branch:     f.branch,
	}

	// A free-text query can come from --query and/or bare tokens; join both.
	queryWords := make([]string, 0, len(f.queryParts)+1)
	if f.query != "" {
		queryWords = append(queryWords, f.query)
	}
	queryWords = append(queryWords, f.queryParts...)
	opts.Query = strings.TrimSpace(strings.Join(queryWords, " "))

	// The base directory for --cwd/--current defaults to the process working
	// directory when --cwd is not given.
	baseDir := f.cwd
	if baseDir == "" {
		if wd, err := os.Getwd(); err == nil {
			baseDir = wd
		}
	}

	if f.cwd != "" {
		cleaned := filepath.Clean(f.cwd)
		info, err := os.Stat(cleaned)
		if err != nil {
			return startupOptions{}, fmt.Errorf("invalid --cwd path: %s", f.cwd)
		}
		if !info.IsDir() {
			return startupOptions{}, fmt.Errorf("--cwd is not a directory: %s", f.cwd)
		}
		opts.Folder = cleaned
	}

	if f.current {
		repo, branch, err := detectGitRepoFn(baseDir)
		if err != nil {
			return startupOptions{}, err
		}
		if repo == "" && branch == "" {
			return startupOptions{}, fmt.Errorf("could not detect a repository or branch from %s", baseDir)
		}
		// Explicit --repo/--branch flags win over detected values.
		if opts.Repository == "" {
			opts.Repository = repo
		}
		if opts.Branch == "" {
			opts.Branch = branch
		}
	}

	return opts, nil
}

// detectGitRepo resolves the owner/repo slug and branch for the git repository
// containing dir. repo is "" when no github remote is configured; branch is ""
// for a detached HEAD. It returns an error when dir is not inside a git work
// tree or when git is unavailable.
func detectGitRepo(dir string) (repo, branch string, err error) {
	if dir == "" {
		return "", "", fmt.Errorf("no working directory to detect a git repository")
	}
	if out, gitErr := runGit(dir, "rev-parse", "--is-inside-work-tree"); gitErr != nil || strings.TrimSpace(out) != "true" {
		return "", "", fmt.Errorf("not a git repository: %s", dir)
	}

	if b, gitErr := runGit(dir, "rev-parse", "--abbrev-ref", "HEAD"); gitErr == nil {
		b = strings.TrimSpace(b)
		if b != "HEAD" { // "HEAD" indicates a detached checkout.
			branch = b
		}
	}

	if remote, gitErr := runGit(dir, "config", "--get", "remote.origin.url"); gitErr == nil {
		repo = data.NormalizeRepoSlug(strings.TrimSpace(remote))
	}

	return repo, branch, nil
}

// runGit runs a git command in dir and returns its stdout. It applies a short
// timeout so a misbehaving git never blocks startup.
func runGit(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}
