// Package main provides a CLI tool for extracting contributor information
// from git history and generating CONTRIBUTORS.md or release notes.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jongio/dispatch/internal/contributors"
)

const usage = `Usage: go run ./cmd/contributors/ [command] [flags]

Commands:
  --all                        Generate CONTRIBUTORS.md from full git history
  --release <fromTag> <toTag>  Generate release contributor notes

Flags:
  --format=md|changelog        Output format for --release (default: md)
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

	var (
		mode    string // "all" or "release"
		fromTag string
		toTag   string
		format  = "md"
	)

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--all":
			mode = "all"

		case args[i] == "--release":
			mode = "release"
			if i+2 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --release requires <fromTag> <toTag>")
				fmt.Fprint(os.Stderr, usage)
				os.Exit(1)
			}
			fromTag = args[i+1]
			toTag = args[i+2]
			i += 2

		case strings.HasPrefix(args[i], "--format="):
			format = strings.TrimPrefix(args[i], "--format=")

		default:
			fmt.Fprintf(os.Stderr, "unknown argument: %s\n", args[i])
			fmt.Fprint(os.Stderr, usage)
			os.Exit(1)
		}
	}

	switch mode {
	case "all":
		if err := runAll(repoDir); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "release":
		if err := runRelease(repoDir, fromTag, toTag, format); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
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

func runRelease(repoDir, fromTag, toTag, format string) error {
	switch format {
	case "md", "changelog":
		// valid formats
	default:
		return fmt.Errorf("unknown format: %s (expected md or changelog)", format)
	}

	release, err := contributors.ExtractContributors(repoDir, fromTag, toTag)
	if err != nil {
		return fmt.Errorf("extracting release contributors: %w", err)
	}

	// Build historical baseline: all contributors reachable from fromTag
	// (i.e., everyone who contributed before this release).
	baseline, err := contributors.ExtractContributorsUpTo(repoDir, fromTag)
	if err != nil {
		return fmt.Errorf("extracting historical contributors: %w", err)
	}

	firstTimers := contributors.DetectFirstTime(baseline, release)
	md := contributors.FormatMarkdown(release, firstTimers)
	fmt.Print(md)
	return nil
}
