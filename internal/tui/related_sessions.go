package tui

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/jongio/dispatch/internal/config"
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
func RankRelatedSessions(current RelatedSession, candidates []RelatedSession, now time.Time) []RelatedSession {
	out := make([]RelatedSession, 0, len(candidates))
	for _, c := range candidates {
		if c.Session.ID == "" || c.Session.ID == current.Session.ID {
			continue
		}
		out = append(out, c)
	}
	slices.SortStableFunc(out, func(a, b RelatedSession) int {
		ta := relatedTier(current, a)
		tb := relatedTier(current, b)
		if ta != tb {
			return cmp.Compare(tb, ta)
		}
		ra := relatedRecencyScore(a.Session, now)
		rb := relatedRecencyScore(b.Session, now)
		if ra != rb {
			return cmp.Compare(rb, ra)
		}
		return cmp.Compare(a.Session.ID, b.Session.ID)
	})
	if len(out) > maxRelatedSessions {
		out = out[:maxRelatedSessions]
	}
	return out
}

func relatedTier(current, candidate RelatedSession) int {
	if sharesSessionRef(current.Refs, candidate.Refs) {
		return 4
	}
	currentRepo := relatedRepo(current)
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

func sharesSessionRef(a, b []data.SessionRef) bool {
	seen := make(map[string]struct{}, len(a))
	for _, ref := range a {
		key := normalizedRefKey(ref)
		if key != "" {
			seen[key] = struct{}{}
		}
	}
	for _, ref := range b {
		if _, ok := seen[normalizedRefKey(ref)]; ok {
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
	return config.FrecencyScore(config.SessionLaunch{Count: 1, Last: t.Unix()}, now)
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
