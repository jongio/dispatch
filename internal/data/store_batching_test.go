package data

import (
	"context"
	"fmt"
	"testing"
)

// Both ListSessionsByIDs and SessionRefsBySessionIDs used to silently truncate
// their input at maxIDsPerQuery, which callers could not distinguish from "no
// such session" / "this session has no refs". These tests seed more rows than
// the batch size so a regression to truncation fails loudly.

func TestListSessionsByIDs_BatchesBeyondMaxIDsPerQuery(t *testing.T) {
	s := newTestStore(t)
	defer s.Close() //nolint:errcheck // test cleanup

	const total = maxIDsPerQuery*2 + 37
	ids := make([]string, total)
	for i := range ids {
		id := fmt.Sprintf("sess-%05d", i)
		ids[i] = id
		seedSession(t, s.db, id, "/tmp/p", "owner/repo", "main", "summary",
			"2024-01-10T10:00:00Z", "2024-01-10T12:00:00Z")
	}

	got, err := s.ListSessionsByIDs(context.Background(), ids)
	if err != nil {
		t.Fatalf("ListSessionsByIDs(%d ids): %v", total, err)
	}
	if len(got) != total {
		t.Fatalf("got %d sessions, want %d — IDs past the batch size were dropped", len(got), total)
	}
	// Input order must be preserved across batch boundaries.
	for i, sess := range got {
		if sess.ID != ids[i] {
			t.Fatalf("result[%d] = %q, want %q — batching broke input ordering", i, sess.ID, ids[i])
		}
	}
}

func TestSessionRefsBySessionIDs_BatchesBeyondMaxIDsPerQuery(t *testing.T) {
	s := newTestStore(t)
	defer s.Close() //nolint:errcheck // test cleanup

	const total = maxIDsPerQuery*2 + 37
	ids := make([]string, total)
	for i := range ids {
		id := fmt.Sprintf("sess-%05d", i)
		ids[i] = id
		seedSession(t, s.db, id, "/tmp/p", "owner/repo", "main", "summary",
			"2024-01-10T10:00:00Z", "2024-01-10T12:00:00Z")
		seedRef(t, s.db, id, "pr", fmt.Sprintf("%d", i), 0, "2024-01-10T11:00:00Z")
	}

	refs, err := s.SessionRefsBySessionIDs(context.Background(), ids)
	if err != nil {
		t.Fatalf("SessionRefsBySessionIDs(%d ids): %v", total, err)
	}
	if len(refs) != total {
		t.Fatalf("got refs for %d sessions, want %d — IDs past the batch size were dropped", len(refs), total)
	}
	// Spot-check a session that only a later batch could have supplied.
	last := ids[total-1]
	got := refs[last]
	if len(got) != 1 || got[0].RefValue != fmt.Sprintf("%d", total-1) {
		t.Fatalf("refs[%s] = %+v, want a single pr ref %d", last, got, total-1)
	}
}
