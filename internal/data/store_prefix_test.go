package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// seedID inserts a bare session with just an ID; the other columns are not
// relevant to prefix resolution.
func seedID(t *testing.T, s *Store, id string) {
	t.Helper()
	seedSession(t, s.db, id, "", "", "", "", "2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
}

func TestResolveIDPrefix_ExactMatch(t *testing.T) {
	s := newTestStore(t)
	defer s.Close() //nolint:errcheck // test cleanup
	seedID(t, s, "abcdef123456")

	got, err := s.ResolveIDPrefix(context.Background(), "abcdef123456")
	if err != nil {
		t.Fatalf("ResolveIDPrefix returned error: %v", err)
	}
	if got != "abcdef123456" {
		t.Errorf("got %q, want %q", got, "abcdef123456")
	}
}

func TestResolveIDPrefix_UniquePrefix(t *testing.T) {
	s := newTestStore(t)
	defer s.Close() //nolint:errcheck // test cleanup
	seedID(t, s, "abcdef123456")
	seedID(t, s, "zzz999")

	got, err := s.ResolveIDPrefix(context.Background(), "abc")
	if err != nil {
		t.Fatalf("ResolveIDPrefix returned error: %v", err)
	}
	if got != "abcdef123456" {
		t.Errorf("got %q, want %q", got, "abcdef123456")
	}
}

// TestResolveIDPrefix_ExactWinsOverPrefix verifies git-style behavior: a value
// that is both a full ID and a prefix of longer IDs resolves to the exact ID.
func TestResolveIDPrefix_ExactWinsOverPrefix(t *testing.T) {
	s := newTestStore(t)
	defer s.Close() //nolint:errcheck // test cleanup
	seedID(t, s, "abc")
	seedID(t, s, "abcdef")

	got, err := s.ResolveIDPrefix(context.Background(), "abc")
	if err != nil {
		t.Fatalf("ResolveIDPrefix returned error: %v", err)
	}
	if got != "abc" {
		t.Errorf("got %q, want exact match %q", got, "abc")
	}
}

func TestResolveIDPrefix_Ambiguous(t *testing.T) {
	s := newTestStore(t)
	defer s.Close() //nolint:errcheck // test cleanup
	seedID(t, s, "abcdef")
	seedID(t, s, "abcxyz")

	_, err := s.ResolveIDPrefix(context.Background(), "abc")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
	var ambErr *AmbiguousIDPrefixError
	if !errors.As(err, &ambErr) {
		t.Fatalf("expected *AmbiguousIDPrefixError, got %T: %v", err, err)
	}
	if ambErr.Prefix != "abc" {
		t.Errorf("Prefix = %q, want %q", ambErr.Prefix, "abc")
	}
	if len(ambErr.Candidates) != 2 {
		t.Fatalf("Candidates = %v, want 2 entries", ambErr.Candidates)
	}
	// Candidates are sorted by ID.
	if ambErr.Candidates[0] != "abcdef" || ambErr.Candidates[1] != "abcxyz" {
		t.Errorf("Candidates = %v, want [abcdef abcxyz]", ambErr.Candidates)
	}
	// The message should name the prefix and both candidates.
	msg := ambErr.Error()
	for _, want := range []string{"abc", "abcdef", "abcxyz"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message %q should contain %q", msg, want)
		}
	}
}

func TestResolveIDPrefix_NoMatch(t *testing.T) {
	s := newTestStore(t)
	defer s.Close() //nolint:errcheck // test cleanup
	seedID(t, s, "abcdef")

	_, err := s.ResolveIDPrefix(context.Background(), "zzz")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestResolveIDPrefix_Empty(t *testing.T) {
	s := newTestStore(t)
	defer s.Close() //nolint:errcheck // test cleanup

	_, err := s.ResolveIDPrefix(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty prefix, got nil")
	}
	if errors.Is(err, sql.ErrNoRows) {
		t.Errorf("empty prefix should not report ErrNoRows, got %v", err)
	}
}

// TestResolveIDPrefix_LiteralWildcards verifies that LIKE wildcard characters
// in the prefix are matched literally rather than as patterns.
func TestResolveIDPrefix_LiteralWildcards(t *testing.T) {
	s := newTestStore(t)
	defer s.Close() //nolint:errcheck // test cleanup
	seedID(t, s, "a_b123")
	seedID(t, s, "axb456")

	// "a_" must match only "a_b123"; if the underscore were treated as a LIKE
	// wildcard it would also match "axb456" and report an ambiguity.
	got, err := s.ResolveIDPrefix(context.Background(), "a_")
	if err != nil {
		t.Fatalf("ResolveIDPrefix returned error: %v", err)
	}
	if got != "a_b123" {
		t.Errorf("got %q, want %q", got, "a_b123")
	}
}

// TestResolveIDPrefix_CandidateCap verifies the ambiguity candidate list is
// bounded so error messages stay readable with many matches.
func TestResolveIDPrefix_CandidateCap(t *testing.T) {
	s := newTestStore(t)
	defer s.Close() //nolint:errcheck // test cleanup
	for i := 0; i < resolveIDPrefixLimit+5; i++ {
		seedID(t, s, fmt.Sprintf("z%03d", i))
	}

	_, err := s.ResolveIDPrefix(context.Background(), "z")
	var ambErr *AmbiguousIDPrefixError
	if !errors.As(err, &ambErr) {
		t.Fatalf("expected *AmbiguousIDPrefixError, got %T: %v", err, err)
	}
	if len(ambErr.Candidates) != resolveIDPrefixLimit+1 {
		t.Errorf("Candidates length = %d, want %d", len(ambErr.Candidates), resolveIDPrefixLimit+1)
	}
}
