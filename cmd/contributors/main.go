// Package main provides a CLI tool for extracting contributor information
// from git history and generating CONTRIBUTORS.md or release notes.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jongio/dispatch/internal/contributors"
)

const usage = `Usage: go run ./cmd/contributors/ [command] [flags]

Commands:
  --all                        Generate CONTRIBUTORS.md from full git history
  --release <fromTag> <toTag>  Generate release contributor notes
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	repoDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	mode, fromTag, toTag, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	handled, err := runMode(repoDir, mode, fromTag, toTag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if !handled {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}
}

func parseArgs(args []string) (mode, fromTag, toTag string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--all":
			mode = "all"
		case "--release":
			if i+2 >= len(args) {
				return "", "", "", fmt.Errorf("error: --release requires <fromTag> <toTag>")
			}
			mode = "release"
			fromTag = args[i+1]
			toTag = args[i+2]
			i += 2
		default:
			return "", "", "", fmt.Errorf("unknown argument: %s", args[i])
		}
	}
	return mode, fromTag, toTag, nil
}

func runMode(repoDir, mode, fromTag, toTag string) (bool, error) {
	switch mode {
	case "all":
		return true, runAll(repoDir)
	case "release":
		return true, runRelease(repoDir, fromTag, toTag)
	default:
		return false, nil
	}
}

func runAll(repoDir string) error {
	contribs, err := contributors.ExtractAllContributors(repoDir)
	if err != nil {
		return fmt.Errorf("extracting contributors: %w", err)
	}

	content := contributors.FormatContributorsFile(contribs)
	outPath := filepath.Join(repoDir, "CONTRIBUTORS.md")
	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing CONTRIBUTORS.md: %w", err)
	}

	fmt.Printf("CONTRIBUTORS.md updated (%d contributors)\n", len(contribs))
	return nil
}

func runRelease(repoDir, fromTag, toTag string) error {
	release, err := contributors.ExtractContributors(repoDir, fromTag, toTag)
	if err != nil {
		return fmt.Errorf("extracting release contributors: %w", err)
	}

	var firstTimers []contributors.Contributor
	if fromTag != "" {
		// Build historical baseline: all contributors reachable from fromTag
		// (i.e., everyone who contributed before this release).
		baseline, err := contributors.ExtractContributorsUpTo(repoDir, fromTag)
		if err != nil {
			return fmt.Errorf("extracting historical contributors: %w", err)
		}
		firstTimers = contributors.DetectFirstTime(baseline, release)
	} else {
		// First release: everyone is a first-time contributor.
		firstTimers = release
	}

	md := contributors.FormatMarkdown(release, firstTimers)
	fmt.Print(md)
	return nil
}
