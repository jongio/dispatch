// Package contributors extracts contributor information from git history.
package contributors

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// Contributor represents a person who contributed to the project.
type Contributor struct {
	Name   string
	Email  string
	Handle string // GitHub username, extracted from noreply email if possible
	Count  int
}

// botSuffix identifies automated bot accounts in git history.
const botSuffix = "[bot]"

// noreplyPattern matches GitHub noreply email addresses and captures the username.
// Formats: "12345+username@users.noreply.github.com" or "username@users.noreply.github.com".
var noreplyPattern = regexp.MustCompile(`^(?:\d+\+)?([^@]+)@users\.noreply\.github\.com$`)

// coAuthorPattern matches "Name <email>" from Co-authored-by trailers.
var coAuthorPattern = regexp.MustCompile(`^(.+?)\s*<([^>]+)>$`)

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// ExtractContributors returns all unique contributors between two git tags.
func ExtractContributors(repoDir, fromTag, toTag string) ([]Contributor, error) {
	return extract(repoDir, fromTag+".."+toTag)
}

// ExtractAllContributors returns all-time contributors across the entire
// git history.
func ExtractAllContributors(repoDir string) ([]Contributor, error) {
	return extract(repoDir, "")
}

// ExtractContributorsUpTo returns all contributors reachable from the given
// ref (all ancestors including the ref itself). This is useful for building
// a historical baseline before a release tag.
func ExtractContributorsUpTo(repoDir, ref string) ([]Contributor, error) {
	return extract(repoDir, ref)
}

// DetectFirstTime returns contributors in release who don't appear in all
// (i.e., first-time contributors for this release).
func DetectFirstTime(all, release []Contributor) []Contributor {
	known := make(map[string]struct{}, len(all))
	for _, c := range all {
		known[strings.ToLower(c.Email)] = struct{}{}
	}

	var firstTimers []Contributor
	for _, c := range release {
		if _, exists := known[strings.ToLower(c.Email)]; !exists {
			firstTimers = append(firstTimers, c)
		}
	}
	return firstTimers
}

// FormatMarkdown formats a contributor section for release notes.
func FormatMarkdown(contributors []Contributor, firstTimers []Contributor) string {
	if len(contributors) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("### Contributors\n\n")
	b.WriteString("Thanks to the following people for their contributions to this release:\n\n")

	for _, c := range sortByName(contributors) {
		b.WriteString("- ")
		b.WriteString(formatEntry(c))
		b.WriteByte('\n')
	}

	if len(firstTimers) > 0 {
		b.WriteByte('\n')
		parts := make([]string, 0, len(firstTimers))
		for _, c := range firstTimers {
			parts = append(parts, formatEntry(c))
		}
		b.WriteString("New contributors: ")
		b.WriteString(strings.Join(parts, ", "))
		b.WriteString(" -- welcome!\n")
	}

	return b.String()
}

// FormatContributorsFile formats the full CONTRIBUTORS.md content.
func FormatContributorsFile(contributors []Contributor) string {
	var b strings.Builder
	b.WriteString("# Contributors\n\n")
	b.WriteString("Thank you to everyone who has contributed to dispatch!\n\n")
	b.WriteString("This file is auto-generated. Run `mage contributors` to update.\n\n")
	b.WriteString("## Contributors\n\n")

	if len(contributors) == 0 {
		return b.String()
	}

	// Sort by contribution count (descending), then name (ascending).
	sorted := make([]Contributor, len(contributors))
	copy(sorted, contributors)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
	})

	for _, c := range sorted {
		b.WriteString("- ")
		b.WriteString(formatEntry(c))
		if c.Count == 1 {
			b.WriteString(" -- 1 contribution\n")
		} else {
			fmt.Fprintf(&b, " -- %d contributions\n", c.Count)
		}
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func extract(repoDir, revRange string) ([]Contributor, error) {
	logArgs := []string{"log"}
	trailerArgs := []string{"log"}
	if revRange != "" {
		logArgs = append(logArgs, revRange)
		trailerArgs = append(trailerArgs, revRange)
	}
	logArgs = append(logArgs, "--format=%aN|%aE")
	trailerArgs = append(trailerArgs, "--format=%(trailers:key=Co-authored-by,valueonly)")

	logOutput, err := gitOutput(repoDir, logArgs...)
	if err != nil {
		return nil, fmt.Errorf("git log authors: %w", err)
	}

	trailerOutput, err := gitOutput(repoDir, trailerArgs...)
	if err != nil {
		return nil, fmt.Errorf("git log trailers: %w", err)
	}

	authors := parseGitLogOutput(logOutput)
	coAuthors := parseCoAuthoredBy(trailerOutput)
	merged := mergeContributors(authors, coAuthors)
	return filterBots(merged), nil
}

func gitOutput(repoDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// parseGitLogOutput parses output from git log --format='%aN|%aE'.
// Each non-empty line is expected to be "Name|email".
func parseGitLogOutput(output string) []Contributor {
	lines := strings.Split(output, "\n")
	result := make([]Contributor, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		email := strings.TrimSpace(parts[1])
		if name == "" || email == "" {
			continue
		}
		result = append(result, Contributor{
			Name:  name,
			Email: email,
			Count: 1,
		})
	}
	return result
}

// parseCoAuthoredBy parses output from
// git log --format='%(trailers:key=Co-authored-by,valueonly)'.
// Each non-empty line is expected to be "Name <email>".
func parseCoAuthoredBy(output string) []Contributor {
	lines := strings.Split(output, "\n")
	result := make([]Contributor, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := coAuthorPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := strings.TrimSpace(m[1])
		email := strings.TrimSpace(m[2])
		if name == "" || email == "" {
			continue
		}
		result = append(result, Contributor{
			Name:  name,
			Email: email,
			Count: 1,
		})
	}
	return result
}

// mergeContributors deduplicates contributors by email (case-insensitive),
// summing their contribution counts and extracting GitHub handles.
func mergeContributors(groups ...[]Contributor) []Contributor {
	type entry struct {
		name   string
		email  string
		handle string
		count  int
	}

	seen := make(map[string]*entry)
	var order []string

	for _, group := range groups {
		for _, c := range group {
			key := strings.ToLower(c.Email)
			if e, exists := seen[key]; exists {
				e.count += c.Count
			} else {
				seen[key] = &entry{
					name:   c.Name,
					email:  c.Email,
					handle: extractHandle(c.Email),
					count:  c.Count,
				}
				order = append(order, key)
			}
		}
	}

	result := make([]Contributor, 0, len(seen))
	for _, key := range order {
		e := seen[key]
		result = append(result, Contributor{
			Name:   e.name,
			Email:  e.email,
			Handle: e.handle,
			Count:  e.count,
		})
	}
	return result
}

// extractHandle extracts a GitHub username from a noreply email address.
// Returns empty string if the email doesn't match the noreply pattern.
func extractHandle(email string) string {
	m := noreplyPattern.FindStringSubmatch(email)
	if m == nil {
		return ""
	}
	return m[1]
}

// isBot reports whether a contributor is an automated bot account.
func isBot(c Contributor) bool {
	return strings.HasSuffix(c.Name, botSuffix)
}

// filterBots returns contributors with bot accounts removed.
func filterBots(contributors []Contributor) []Contributor {
	result := make([]Contributor, 0, len(contributors))
	for _, c := range contributors {
		if !isBot(c) {
			result = append(result, c)
		}
	}
	return result
}

// formatEntry formats a contributor as "**Name** (@handle)" or "**Name**".
func formatEntry(c Contributor) string {
	if c.Handle != "" {
		return "**" + c.Name + "** (@" + c.Handle + ")"
	}
	return "**" + c.Name + "**"
}

func sortByName(contributors []Contributor) []Contributor {
	sorted := make([]Contributor, len(contributors))
	copy(sorted, contributors)
	sort.Slice(sorted, func(i, j int) bool {
		return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
	})
	return sorted
}
