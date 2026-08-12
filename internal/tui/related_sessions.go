package tui

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/jongio/dispatch/internal/data"
)

const maxRelatedSessions = 5

// RelatedSession is the rankable and renderable data for one nearby session.
type RelatedSession struct {
	Session           data.Session
	Refs              []data.SessionRef
	DisplayRepository string
}

// RankRelatedSessions returns up to five candidate sessions ranked by strongest
// relationship to current, then recency, then ID for deterministic ordering.
//
// Tier and recency are computed once per candidate rather than inside the
// comparator. Doing the work in the comparator re-derived the current
// session's ref set and re-parsed timestamps on every one of the O(n log n)
// comparisons, which is the hot path when a large store is browsed with the
// preview pane open.
func RankRelatedSessions(current RelatedSession, candidates []RelatedSession, now time.Time) []RelatedSession {
	currentRefs := refKeySet(current.Refs)
	currentRepo := relatedRepo(current)

	type scored struct {
		session RelatedSession
		tier    int
		recency float64
	}

	out := make([]scored, 0, len(candidates))
	for _, c := range candidates {
		if c.Session.ID == "" || c.Session.ID == current.Session.ID {
			continue
		}
		out = append(out, scored{
			session: c,
			tier:    relatedTierWith(current, currentRefs, currentRepo, c),
			recency: relatedRecencyScore(c.Session, now),
		})
	}
	slices.SortStableFunc(out, func(a, b scored) int {
		if a.tier != b.tier {
			return cmp.Compare(b.tier, a.tier)
		}
		if a.recency != b.recency {
			return cmp.Compare(b.recency, a.recency)
		}
		return cmp.Compare(a.session.Session.ID, b.session.Session.ID)
	})
	if len(out) > maxRelatedSessions {
		out = out[:maxRelatedSessions]
	}
	ranked := make([]RelatedSession, len(out))
	for i, s := range out {
		ranked[i] = s.session
	}
	return ranked
}

// relatedTierWith scores one candidate against the current session using a
// pre-computed ref key set and display repository, so callers ranking many
// candidates derive those once instead of per comparison.
func relatedTierWith(current RelatedSession, currentRefs map[string]struct{}, currentRepo string, candidate RelatedSession) int {
	if sharesRefKey(currentRefs, candidate.Refs) {
		return 4
	}
	candidateRepo := relatedRepo(candidate)
	if currentRepo != "" && candidateRepo != "" && strings.EqualFold(currentRepo, candidateRepo) {
		if current.Session.Branch != "" && candidate.Session.Branch != "" &&
			strings.EqualFold(current.Session.Branch, candidate.Session.Branch) {
			return 3
		}
		return 2
	}
	if current.Session.Cwd != "" && candidate.Session.Cwd != "" &&
		strings.EqualFold(current.Session.Cwd, candidate.Session.Cwd) {
		return 1
	}
	return 0
}

// refKeySet builds the set of normalized ref keys for one session.
func refKeySet(refs []data.SessionRef) map[string]struct{} {
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if key := normalizedRefKey(ref); key != "" {
			seen[key] = struct{}{}
		}
	}
	return seen
}

// sharesRefKey reports whether any ref in refs appears in the pre-built set.
func sharesRefKey(set map[string]struct{}, refs []data.SessionRef) bool {
	for _, ref := range refs {
		key := normalizedRefKey(ref)
		if key == "" {
			continue
		}
		if _, ok := set[key]; ok {
			return true
		}
	}
	return false
}

func normalizedRefKey(ref data.SessionRef) string {
	refType := strings.TrimSpace(strings.ToLower(ref.RefType))
	refValue := strings.TrimSpace(strings.ToLower(ref.RefValue))
	if refType == "" || refValue == "" {
		return ""
	}
	return refType + ":" + refValue
}

func relatedRepo(s RelatedSession) string {
	if s.DisplayRepository != "" {
		return s.DisplayRepository
	}
	return s.Session.Repository
}

func relatedRecencyScore(s data.Session, now time.Time) float64 {
	t := parseRelatedTimestamp(s.LastActiveAt)
	if t.IsZero() {
		t = parseRelatedTimestamp(s.UpdatedAt)
	}
	if t.IsZero() {
		t = parseRelatedTimestamp(s.CreatedAt)
	}
	if t.IsZero() {
		return 0
	}
	age := now.Sub(t)
	if age < 0 {
		age = 0
	}
	return -age.Seconds()
}

func parseRelatedTimestamp(timestamp string) time.Time {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, timestamp); err == nil {
			return t
		}
	}
	return time.Time{}
}
