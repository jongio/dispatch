package data

import (
	"strings"
)

// refTypePriority ranks reference types when picking the single best ref to
// open for a session. A pull request is the most useful landing page, then the
// issue, then the commit.
var refTypePriority = map[string]int{
	"pr":     3,
	"issue":  2,
	"commit": 1,
}

// BestRef returns the most useful reference to open for a session, preferring a
// pull request over an issue over a commit. The second return value is false
// when the slice has no usable references.
func BestRef(refs []SessionRef) (SessionRef, bool) {
	best := SessionRef{}
	bestRank := 0
	for _, r := range refs {
		if strings.TrimSpace(r.RefValue) == "" {
			continue
		}
		rank := refTypePriority[strings.ToLower(r.RefType)]
		if rank == 0 {
			continue
		}
		if rank > bestRank {
			best = r
			bestRank = rank
		}
	}
	return best, bestRank > 0
}

// RefURL builds a github.com URL for a session reference. The repository may be
// an "owner/repo" slug or a git remote URL (https or ssh form); anything else
// yields ok=false. Pull request and issue values are reduced to their digits so
// stored forms like "#42" or "PR42" still resolve. Commit values are kept as-is
// after a basic hex-character check. It returns ok=false when a valid URL
// cannot be built.
func RefURL(repository, refType, refValue string) (url string, ok bool) {
	slug := normalizeRepoSlug(repository)
	if slug == "" {
		return "", false
	}
	value := strings.TrimSpace(refValue)
	if value == "" {
		return "", false
	}
	base := "https://github.com/" + slug
	switch strings.ToLower(refType) {
	case "pr":
		n := digitsOnly(value)
		if n == "" {
			return "", false
		}
		return base + "/pull/" + n, true
	case "issue":
		n := digitsOnly(value)
		if n == "" {
			return "", false
		}
		return base + "/issues/" + n, true
	case "commit":
		if !isHex(value) {
			return "", false
		}
		return base + "/commit/" + value, true
	default:
		return "", false
	}
}

// NormalizeRepoSlug reduces a repository identifier to its "owner/repo" form.
// It accepts a bare slug, an https URL, a scp-style ssh remote, or a
// github.com-prefixed path, and returns "" when it cannot extract owner/repo.
// It matches the form stored in the session repository column, so callers can
// use it to compare a live git remote against stored sessions.
func NormalizeRepoSlug(repository string) string {
	return normalizeRepoSlug(repository)
}

// normalizeRepoSlug reduces a repository identifier to its "owner/repo" form.
// It accepts a bare slug, an https URL, a scp-style ssh remote, or a
// github.com-prefixed path, and returns "" when it cannot extract owner/repo.
func normalizeRepoSlug(repository string) string {
	s := strings.TrimSpace(repository)
	if s == "" {
		return ""
	}
	s = strings.TrimSuffix(s, ".git")

	// scp-style: git@github.com:owner/repo
	if i := strings.Index(s, "@"); i >= 0 {
		if j := strings.Index(s[i:], ":"); j >= 0 {
			s = s[i+j+1:]
		}
	}

	// Strip a scheme like https:// or ssh://.
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}

	// Drop a leading host segment (github.com, github.com:443, etc.).
	if i := strings.Index(s, "github.com"); i >= 0 {
		s = s[i+len("github.com"):]
	}
	s = strings.TrimLeft(s, ":/")

	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return ""
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	if owner == "" || repo == "" {
		return ""
	}
	return owner + "/" + repo
}

// digitsOnly returns the digit runes of s, or "" if s has none.
func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isHex reports whether s is non-empty and made up only of hex digits, which is
// the shape of a git commit SHA.
func isHex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
