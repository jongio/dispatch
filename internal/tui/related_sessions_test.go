package tui

import (
	"testing"
	"time"

	"github.com/jongio/dispatch/internal/data"
)

func TestRankRelatedSessions_TierOrderAndTieBreaks(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	current := RelatedSession{
		Session: data.Session{
			ID:         "current",
			Cwd:        `C:\work\dispatch`,
			Repository: "jongio/dispatch",
			Branch:     "main",
		},
		Refs: []data.SessionRef{{RefType: "issue", RefValue: "183"}},
	}
	candidates := []RelatedSession{
		{Session: data.Session{ID: "recent-only", LastActiveAt: "2026-08-01T11:55:00Z"}},
		{Session: data.Session{ID: "same-cwd", Cwd: `C:\work\dispatch`, LastActiveAt: "2026-07-31T12:00:00Z"}},
		{Session: data.Session{ID: "same-repo", Repository: "jongio/dispatch", LastActiveAt: "2026-07-30T12:00:00Z"}},
		{Session: data.Session{ID: "same-repo-branch", Repository: "jongio/dispatch", Branch: "main", LastActiveAt: "2026-07-29T12:00:00Z"}},
		{
			Session: data.Session{ID: "shared-ref-old", LastActiveAt: "2026-07-01T12:00:00Z"},
			Refs:    []data.SessionRef{{RefType: "issue", RefValue: "183"}},
		},
		{
			Session: data.Session{ID: "shared-ref-new", LastActiveAt: "2026-07-02T12:00:00Z"},
			Refs:    []data.SessionRef{{RefType: "issue", RefValue: "183"}},
		},
		{Session: data.Session{ID: "current", LastActiveAt: "2026-08-01T12:00:00Z"}},
	}

	got := RankRelatedSessions(current, candidates, now)
	wantIDs := []string{"shared-ref-new", "shared-ref-old", "same-repo-branch", "same-repo", "same-cwd"}
	if len(got) != len(wantIDs) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(wantIDs), idsOfRelated(got))
	}
	for i, want := range wantIDs {
		if got[i].Session.ID != want {
			t.Fatalf("rank %d = %s, want %s (all %v)", i, got[i].Session.ID, want, idsOfRelated(got))
		}
	}
}

func TestRankRelatedSessions_UsesDisplayRepositoryAndDeterministicID(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	current := RelatedSession{
		Session:           data.Session{ID: "current", Repository: "stored/current"},
		DisplayRepository: "live/repo",
	}
	candidates := []RelatedSession{
		{Session: data.Session{ID: "b", Repository: "stored/other", LastActiveAt: "2026-08-01T10:00:00Z"}, DisplayRepository: "live/repo"},
		{Session: data.Session{ID: "a", Repository: "stored/other", LastActiveAt: "2026-08-01T10:00:00Z"}, DisplayRepository: "live/repo"},
		{Session: data.Session{ID: "newer", Repository: "stored/other", LastActiveAt: "2026-08-01T11:00:00Z"}},
	}

	got := RankRelatedSessions(current, candidates, now)
	wantIDs := []string{"a", "b", "newer"}
	for i, want := range wantIDs {
		if got[i].Session.ID != want {
			t.Fatalf("rank %d = %s, want %s (all %v)", i, got[i].Session.ID, want, idsOfRelated(got))
		}
	}
}

func idsOfRelated(sessions []RelatedSession) []string {
	ids := make([]string, len(sessions))
	for i, s := range sessions {
		ids[i] = s.Session.ID
	}
	return ids
}
