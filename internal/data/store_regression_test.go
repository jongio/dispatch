package data

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
)

// ---------------------------------------------------------------------------
// Grouped-query limit semantics
//
// GroupSessions caps the number of rows it reads. The cap must keep the most
// relevant sessions according to the requested sort, exactly like
// ListSessions does, so that every group the user is looking at is
// represented. Truncating in pivot-label order instead collapses the result
// to whichever label happens to sort first, which looks to the user like
// "grouping shows one bucket of N sessions".
// ---------------------------------------------------------------------------

// seedGroupedFixture inserts sessions across three repositories. Repository
// names are chosen so that "aaa/old" sorts before "zzz/new" as a pivot label
// while its sessions are the least recently active.
func seedGroupedFixture(t *testing.T, s *Store) {
	t.Helper()
	// Oldest sessions live in the alphabetically first repository.
	for i := range 5 {
		id := fmt.Sprintf("old-%d", i)
		ts := fmt.Sprintf("2026-01-%02dT10:00:00Z", i+1)
		seedSession(t, s.db, id, "/work/old", "aaa/old", "main", "old work", ts, ts)
		seedTurn(t, s.db, id, 0, "question", "answer", ts)
	}
	// Newest sessions live in the alphabetically last repository.
	for i := range 3 {
		id := fmt.Sprintf("new-%d", i)
		ts := fmt.Sprintf("2026-08-%02dT10:00:00Z", i+1)
		seedSession(t, s.db, id, "/work/new", "zzz/new", "main", "new work", ts, ts)
		seedTurn(t, s.db, id, 0, "question", "answer", ts)
	}
}

// groupedSessionIDs flattens every session in every group.
func groupedSessionIDs(groups []SessionGroup) []string {
	var ids []string
	for _, g := range groups {
		for _, sess := range g.Sessions {
			ids = append(ids, sess.ID)
		}
	}
	return ids
}

func groupLabels(groups []SessionGroup) []string {
	labels := make([]string, 0, len(groups))
	for _, g := range groups {
		labels = append(labels, g.Label)
	}
	return labels
}

func TestGroupSessionsLimitKeepsMostRecentAcrossGroups(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	seedGroupedFixture(t, s)

	groups, err := s.GroupSessions(context.Background(), PivotByRepo, FilterOptions{},
		SortOptions{Field: SortByUpdated, Order: Descending}, 3)
	if err != nil {
		t.Fatalf("GroupSessions: %v", err)
	}

	got := groupedSessionIDs(groups)
	want := []string{"new-2", "new-1", "new-0"}
	if len(got) != len(want) {
		t.Fatalf("returned %d sessions %v, want %d %v", len(got), got, len(want), want)
	}
	for i, id := range want {
		if got[i] != id {
			t.Fatalf("session IDs = %v, want %v (limit must keep the most recently active sessions)", got, want)
		}
	}
}

func TestGroupSessionsLimitCoversEveryPivot(t *testing.T) {
	pivots := []PivotField{PivotByRepo, PivotByBranch, PivotByHost, PivotByFolder}
	for _, pivot := range pivots {
		t.Run(string(pivot), func(t *testing.T) {
			s := newTestStore(t)
			defer func() { _ = s.Close() }()

			// Two distinct pivot values: the first-sorting one holds only
			// stale sessions, the second holds the freshest session.
			seedSession(t, s.db, "stale-a", "/aaa", "aaa/repo", "aaa-branch", "stale", "2026-01-01T10:00:00Z", "2026-01-01T10:00:00Z")
			seedTurn(t, s.db, "stale-a", 0, "q", "a", "2026-01-01T10:00:00Z")
			seedSession(t, s.db, "stale-b", "/aaa", "aaa/repo", "aaa-branch", "stale", "2026-01-02T10:00:00Z", "2026-01-02T10:00:00Z")
			seedTurn(t, s.db, "stale-b", 0, "q", "a", "2026-01-02T10:00:00Z")
			seedSession(t, s.db, "fresh", "/zzz", "zzz/repo", "zzz-branch", "fresh", "2026-08-01T10:00:00Z", "2026-08-01T10:00:00Z")
			seedTurn(t, s.db, "fresh", 0, "q", "a", "2026-08-01T10:00:00Z")
			if _, err := s.db.Exec(`UPDATE sessions SET host_type = 'aaa-host' WHERE id LIKE 'stale%'`); err != nil {
				t.Fatalf("setting host_type: %v", err)
			}
			if _, err := s.db.Exec(`UPDATE sessions SET host_type = 'zzz-host' WHERE id = 'fresh'`); err != nil {
				t.Fatalf("setting host_type: %v", err)
			}

			groups, err := s.GroupSessions(context.Background(), pivot, FilterOptions{},
				SortOptions{Field: SortByUpdated, Order: Descending}, 1)
			if err != nil {
				t.Fatalf("GroupSessions(%s): %v", pivot, err)
			}
			got := groupedSessionIDs(groups)
			if len(got) != 1 || got[0] != "fresh" {
				t.Fatalf("GroupSessions(%s) with limit 1 = %v, want [fresh]", pivot, got)
			}
		})
	}
}

func TestGroupSessionsByDateLimitKeepsNewestDays(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()

	// Five sessions, one per day, spanning January to August.
	days := []string{
		"2026-01-29T12:00:00Z",
		"2026-01-30T12:00:00Z",
		"2026-02-02T12:00:00Z",
		"2026-08-19T12:00:00Z",
		"2026-08-20T12:00:00Z",
	}
	for i, ts := range days {
		id := fmt.Sprintf("day-%d", i)
		seedSession(t, s.db, id, "/work", "owner/repo", "main", "work", ts, ts)
		seedTurn(t, s.db, id, 0, "q", "a", ts)
	}

	groups, err := s.GroupSessions(context.Background(), PivotByDate, FilterOptions{},
		SortOptions{Field: SortByUpdated, Order: Descending}, 2)
	if err != nil {
		t.Fatalf("GroupSessions: %v", err)
	}

	got := groupedSessionIDs(groups)
	if len(got) != 2 {
		t.Fatalf("returned %d sessions %v, want 2", len(got), got)
	}
	for _, id := range got {
		if id != "day-3" && id != "day-4" {
			t.Fatalf("date pivot with limit 2 returned %v, want the two newest days (day-3, day-4); "+
				"truncating oldest-first hides recent activity", got)
		}
	}
}

func TestGroupSessionsOrdersGroupsByLabel(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	seedGroupedFixture(t, s)

	groups, err := s.GroupSessions(context.Background(), PivotByRepo, FilterOptions{},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("GroupSessions: %v", err)
	}
	want := []string{"aaa/old", "zzz/new"}
	got := groupLabels(groups)
	if len(got) != len(want) {
		t.Fatalf("group labels = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("group labels = %v, want %v (groups must stay label-ordered)", got, want)
		}
	}
}

func TestGroupSessionsSortsSessionsWithinGroup(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	seedGroupedFixture(t, s)

	groups, err := s.GroupSessions(context.Background(), PivotByRepo, FilterOptions{},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("GroupSessions: %v", err)
	}
	for _, g := range groups {
		for i := 1; i < len(g.Sessions); i++ {
			if g.Sessions[i-1].LastActiveAt < g.Sessions[i].LastActiveAt {
				t.Fatalf("group %q not sorted descending by last active: %v", g.Label, g.Sessions)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Excluded-word coverage
//
// Deep search matches a session when the term appears in any indexed content
// source. The excluded-word filter is the inverse operation and must cover
// the same sources, otherwise a session the user asked to hide stays visible
// (and is still discoverable through search).
// ---------------------------------------------------------------------------

// seedExcludedWordSources inserts one session per content source, each holding
// the token "quarantine" in exactly one place.
func seedExcludedWordSources(t *testing.T, s *Store) {
	t.Helper()
	const ts = "2026-05-01T10:00:00Z"
	mk := func(id, summary string) {
		seedSession(t, s.db, id, "/work/"+id, "owner/repo", "main", summary, ts, ts)
		seedTurn(t, s.db, id, 0, "unrelated", "unrelated", ts)
	}
	mk("by-summary", "quarantine this session")
	mk("by-user-message", "clean summary")
	mk("by-assistant-response", "clean summary")
	mk("by-checkpoint-title", "clean summary")
	mk("by-checkpoint-overview", "clean summary")
	mk("by-checkpoint-body", "clean summary")
	mk("keep-me", "clean summary")

	seedTurn(t, s.db, "by-user-message", 1, "please quarantine it", "sure", ts)
	seedTurn(t, s.db, "by-assistant-response", 1, "go on", "running quarantine now", ts)
	seedCheckpoint(t, s.db, "by-checkpoint-title", 1, "quarantine plan", "nothing here")
	seedCheckpoint(t, s.db, "by-checkpoint-overview", 1, "plan", "quarantine the flaky job")
	seedCheckpointBody(t, s.db, "by-checkpoint-body", 1, "quarantine")
}

// seedCheckpointBody inserts a checkpoint whose only match is in the
// long-form fields (history, work_done, technical_details, next_steps).
func seedCheckpointBody(t *testing.T, db *sql.DB, sessionID string, num int, token string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO checkpoints (session_id, checkpoint_number, title, overview, history, work_done, technical_details, important_files, next_steps)
		 VALUES (?, ?, 'plan', 'nothing', ?, ?, ?, '', ?)`,
		sessionID, num, token, token, token, token,
	)
	if err != nil {
		t.Fatalf("seeding checkpoint body: %v", err)
	}
}

func TestExcludedWordsCoverSameSourcesAsDeepSearch(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	seedExcludedWordSources(t, s)

	ctx := context.Background()
	sort := SortOptions{Field: SortByUpdated, Order: Descending}

	// Every session deep search can find by the token must be hidden when
	// the same token is excluded.
	found, err := s.ListSessions(ctx, FilterOptions{Query: "quarantine", DeepSearch: true}, sort, 0)
	if err != nil {
		t.Fatalf("deep search: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("deep search found no sessions; fixture is wrong")
	}

	remaining, err := s.ListSessions(ctx, FilterOptions{ExcludedWords: []string{"quarantine"}}, sort, 0)
	if err != nil {
		t.Fatalf("ListSessions with ExcludedWords: %v", err)
	}
	visible := make(map[string]bool, len(remaining))
	for _, sess := range remaining {
		visible[sess.ID] = true
	}
	for _, sess := range found {
		if visible[sess.ID] {
			t.Errorf("session %q matches deep search for %q but survives the excluded-word filter",
				sess.ID, "quarantine")
		}
	}
	if !visible["keep-me"] {
		t.Error("session without the excluded word should stay visible")
	}
}

func TestExcludedWordsAreCaseInsensitiveInEverySource(t *testing.T) {
	const ts = "2026-05-01T10:00:00Z"
	cases := []struct {
		name string
		// seed places the mixed-case token in one content source.
		seed func(t *testing.T, s *Store, id string)
	}{
		{
			name: "summary",
			seed: func(t *testing.T, s *Store, id string) {
				seedSession(t, s.db, id, "/work", "owner/repo", "main", "Refactor QuarantineRunner", ts, ts)
				seedTurn(t, s.db, id, 0, "unrelated", "unrelated", ts)
			},
		},
		{
			name: "user_message",
			seed: func(t *testing.T, s *Store, id string) {
				seedSession(t, s.db, id, "/work", "owner/repo", "main", "clean", ts, ts)
				seedTurn(t, s.db, id, 0, "Please QUARANTINE the job", "sure", ts)
			},
		},
		{
			name: "assistant_response",
			seed: func(t *testing.T, s *Store, id string) {
				seedSession(t, s.db, id, "/work", "owner/repo", "main", "clean", ts, ts)
				seedTurn(t, s.db, id, 0, "go on", "Running Quarantine now", ts)
			},
		},
		{
			name: "checkpoint_title",
			seed: func(t *testing.T, s *Store, id string) {
				seedSession(t, s.db, id, "/work", "owner/repo", "main", "clean", ts, ts)
				seedTurn(t, s.db, id, 0, "unrelated", "unrelated", ts)
				seedCheckpoint(t, s.db, id, 1, "QuArAnTiNe plan", "nothing")
			},
		},
		{
			name: "checkpoint_body",
			seed: func(t *testing.T, s *Store, id string) {
				seedSession(t, s.db, id, "/work", "owner/repo", "main", "clean", ts, ts)
				seedTurn(t, s.db, id, 0, "unrelated", "unrelated", ts)
				seedCheckpointBody(t, s.db, id, 1, "QUARANTINE")
			},
		},
	}

	// The configured word and the stored text differ in case in both
	// directions, so neither side may be relied on to be pre-folded.
	words := []string{"quarantine", "QUARANTINE", "QuArAnTiNe"}

	for _, tc := range cases {
		for _, word := range words {
			t.Run(tc.name+"/"+word, func(t *testing.T) {
				s := newTestStore(t)
				defer func() { _ = s.Close() }()
				tc.seed(t, s, "hide-me")

				sessions, err := s.ListSessions(context.Background(),
					FilterOptions{ExcludedWords: []string{word}},
					SortOptions{Field: SortByUpdated, Order: Descending}, 0)
				if err != nil {
					t.Fatalf("ListSessions: %v", err)
				}
				if len(sessions) != 0 {
					t.Fatalf("excluding %q left %d sessions visible; the %s match must be case-insensitive",
						word, len(sessions), tc.name)
				}
			})
		}
	}
}

func TestExcludedWordsCaseInsensitiveKeepsNonMatches(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	const ts = "2026-05-01T10:00:00Z"
	seedSession(t, s.db, "keep-me", "/work", "owner/repo", "main", "Quarterly report", ts, ts)
	seedTurn(t, s.db, "keep-me", 0, "unrelated", "unrelated", ts)

	sessions, err := s.ListSessions(context.Background(),
		FilterOptions{ExcludedWords: []string{"QUARANTINE"}},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	requireSessionIDs(t, sessions, "keep-me")
}

func TestExcludedWordsFoldNonASCIICase(t *testing.T) {
	const ts = "2026-05-01T10:00:00Z"
	cases := []struct {
		name   string
		stored string
		word   string
	}{
		{"upper-content-lower-word", "Réparer le CAFÉ", "café"},
		{"lower-content-upper-word", "Réparer le café", "CAFÉ"},
		{"umlaut", "ÜBER pipeline", "über"},
		{"a-umlaut", "Ärger report", "ärger"},
		{"cyrillic", "ПРИВЕТ мир", "привет"},
		{"turkish-dotted-i", "İSTANBUL trip", "İstanbul"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)
			defer func() { _ = s.Close() }()
			seedSession(t, s.db, "hide-me", "/work", "owner/repo", "main", tc.stored, ts, ts)
			seedTurn(t, s.db, "hide-me", 0, "unrelated", "unrelated", ts)

			sessions, err := s.ListSessions(context.Background(),
				FilterOptions{ExcludedWords: []string{tc.word}},
				SortOptions{Field: SortByUpdated, Order: Descending}, 0)
			if err != nil {
				t.Fatalf("ListSessions: %v", err)
			}
			if len(sessions) != 0 {
				t.Fatalf("excluding %q left the session with summary %q visible; "+
					"case folding must cover non-ASCII text", tc.word, tc.stored)
			}
		})
	}
}

func TestExcludedWordsFoldNonASCIIInTurnsAndCheckpoints(t *testing.T) {
	const ts = "2026-05-01T10:00:00Z"
	cases := []struct {
		name string
		seed func(t *testing.T, s *Store)
	}{
		{
			name: "turn",
			seed: func(t *testing.T, s *Store) {
				seedSession(t, s.db, "hide-me", "/work", "owner/repo", "main", "clean", ts, ts)
				seedTurn(t, s.db, "hide-me", 0, "check the CAFÉ order", "ok", ts)
			},
		},
		{
			name: "checkpoint",
			seed: func(t *testing.T, s *Store) {
				seedSession(t, s.db, "hide-me", "/work", "owner/repo", "main", "clean", ts, ts)
				seedTurn(t, s.db, "hide-me", 0, "unrelated", "unrelated", ts)
				seedCheckpoint(t, s.db, "hide-me", 1, "CAFÉ rollout", "nothing")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)
			defer func() { _ = s.Close() }()
			tc.seed(t, s)

			sessions, err := s.ListSessions(context.Background(),
				FilterOptions{ExcludedWords: []string{"café"}},
				SortOptions{Field: SortByUpdated, Order: Descending}, 0)
			if err != nil {
				t.Fatalf("ListSessions: %v", err)
			}
			if len(sessions) != 0 {
				t.Fatalf("excluding %q left the %s match visible", "café", tc.name)
			}
		})
	}
}

// Search and exclusion share one clause list, so case folding has to behave
// the same on both sides.
func TestDeepSearchFoldsNonASCIICase(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	const ts = "2026-05-01T10:00:00Z"
	seedSession(t, s.db, "find-me", "/work", "owner/repo", "main", "Réparer le CAFÉ", ts, ts)
	seedTurn(t, s.db, "find-me", 0, "unrelated", "unrelated", ts)

	for _, query := range []string{"café", "CAFÉ", "Café"} {
		found, err := s.ListSessions(context.Background(),
			FilterOptions{Query: query, DeepSearch: true},
			SortOptions{Field: SortByUpdated, Order: Descending}, 0)
		if err != nil {
			t.Fatalf("deep search %q: %v", query, err)
		}
		if len(found) != 1 {
			t.Errorf("deep search for %q found %d sessions, want 1", query, len(found))
		}
	}
}

func TestQuickSearchFoldsNonASCIICase(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	const ts = "2026-05-01T10:00:00Z"
	seedSession(t, s.db, "find-me", "/work", "owner/repo", "main", "Réparer le CAFÉ", ts, ts)
	seedTurn(t, s.db, "find-me", 0, "unrelated", "unrelated", ts)

	found, err := s.ListSessions(context.Background(),
		FilterOptions{Query: "café"},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("quick search: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("quick search for %q found %d sessions, want 1", "café", len(found))
	}
}

// Normalization runs a per-row callback, so it is only enabled when the term
// needs it: non-ASCII text (which SQLite's LIKE cannot fold) or whitespace
// (which the list collapses for display). This pins that trigger, and with it
// the one case the filter cannot fold: a single-word ASCII term whose only
// case-variant in the stored text is a non-ASCII character (Turkish "İ").
func TestNeedsNormalization(t *testing.T) {
	if !normReady {
		t.Skip("match normalization function not registered")
	}
	cases := []struct {
		term string
		want bool
	}{
		{"invoke", false},
		{"", false},
		{"devx-frontend-learn", false},
		{"Task Invoke", true},
		{"Task\tInvoke", true},
		{"café", true},
		{"ÜBER", true},
		{"привет", true},
		{"İstanbul", true},
	}
	for _, tc := range cases {
		if got := needsNormalization(tc.term); got != tc.want {
			t.Errorf("needsNormalization(%q) = %v, want %v", tc.term, got, tc.want)
		}
	}
}

func TestNormalizeMatchTextMirrorsDisplay(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"## Task\n\nInvoke the skill", "## task invoke the skill"},
		{"  leading and   trailing  ", "leading and trailing"},
		{"CAFÉ", "café"},
		{"tabs\tand\nnewlines", "tabs and newlines"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeMatchText(tc.in); got != tc.want {
			t.Errorf("normalizeMatchText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLikePatternNormalizesOnlyWhenAsked(t *testing.T) {
	if got, want := likePattern("Task\n\nInvoke", true), "%task invoke%"; got != want {
		t.Errorf("likePattern(normalized) = %q, want %q", got, want)
	}
	// Unnormalized patterns keep their case because SQLite's LIKE folds ASCII
	// on both sides already.
	if got, want := likePattern("Invoke", false), "%Invoke%"; got != want {
		t.Errorf("likePattern(raw) = %q, want %q", got, want)
	}
	// Wildcards stay literal in both modes.
	if got, want := likePattern("100%", false), `%100\%%`; got != want {
		t.Errorf("likePattern(wildcard) = %q, want %q", got, want)
	}
	if got, want := likePattern("50% off", true), `%50\% off%`; got != want {
		t.Errorf("likePattern(normalized wildcard) = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Displayed text vs stored text
//
// The session list renders summaries through CleanSummary, which collapses
// every whitespace run to a single space. A summary stored as
// "## Task\n\nInvoke the skill" is shown as "## Task Invoke the skill", so a
// user excluding the phrase they can see types "Task Invoke". Matching against
// the raw stored text never finds it.
// ---------------------------------------------------------------------------

// storedTaskSummary reproduces the real shape of an agent sub-task summary:
// a markdown heading, a blank line, then the body.
const storedTaskSummary = "## Task\n\nInvoke the devx-frontend-learn skill via the skill tool. You have the contract."

func TestExcludedWordsMatchDisplayedWhitespace(t *testing.T) {
	const ts = "2026-05-01T10:00:00Z"
	cases := []struct {
		name string
		word string
	}{
		{"phrase-as-displayed", "Task Invoke"},
		{"phrase-lowercase", "task invoke"},
		{"phrase-extra-spaces", "Task   Invoke"},
		{"phrase-spanning-body", "Invoke the devx-frontend-learn skill"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)
			defer func() { _ = s.Close() }()
			seedSession(t, s.db, "hide-me", "/work", "owner/repo", "main", storedTaskSummary, ts, ts)
			seedTurn(t, s.db, "hide-me", 0, "unrelated", "unrelated", ts)

			sessions, err := s.ListSessions(context.Background(),
				FilterOptions{ExcludedWords: []string{tc.word}},
				SortOptions{Field: SortByUpdated, Order: Descending}, 0)
			if err != nil {
				t.Fatalf("ListSessions: %v", err)
			}
			if len(sessions) != 0 {
				t.Fatalf("excluding %q left the session visible; the summary renders as "+
					"%q in the list, so the phrase the user sees must match", tc.word, "## Task Invoke ...")
			}
		})
	}
}

func TestExcludedWordsWhitespaceInTurnsAndCheckpoints(t *testing.T) {
	const ts = "2026-05-01T10:00:00Z"
	cases := []struct {
		name string
		seed func(t *testing.T, s *Store)
	}{
		{
			name: "turn",
			seed: func(t *testing.T, s *Store) {
				seedSession(t, s.db, "hide-me", "/work", "owner/repo", "main", "clean", ts, ts)
				seedTurn(t, s.db, "hide-me", 0, storedTaskSummary, "ok", ts)
			},
		},
		{
			name: "checkpoint",
			seed: func(t *testing.T, s *Store) {
				seedSession(t, s.db, "hide-me", "/work", "owner/repo", "main", "clean", ts, ts)
				seedTurn(t, s.db, "hide-me", 0, "unrelated", "unrelated", ts)
				seedCheckpoint(t, s.db, "hide-me", 1, "plan", storedTaskSummary)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)
			defer func() { _ = s.Close() }()
			tc.seed(t, s)

			sessions, err := s.ListSessions(context.Background(),
				FilterOptions{ExcludedWords: []string{"Task Invoke"}},
				SortOptions{Field: SortByUpdated, Order: Descending}, 0)
			if err != nil {
				t.Fatalf("ListSessions: %v", err)
			}
			if len(sessions) != 0 {
				t.Fatalf("excluding %q left the %s match visible", "Task Invoke", tc.name)
			}
		})
	}
}

// Search and exclusion share one clause list, so searching the phrase a user
// can see has to find the same sessions exclusion hides.
func TestSearchMatchesDisplayedWhitespace(t *testing.T) {
	const ts = "2026-05-01T10:00:00Z"
	for _, deep := range []bool{false, true} {
		name := "quick"
		if deep {
			name = "deep"
		}
		t.Run(name, func(t *testing.T) {
			s := newTestStore(t)
			defer func() { _ = s.Close() }()
			seedSession(t, s.db, "find-me", "/work", "owner/repo", "main", storedTaskSummary, ts, ts)
			seedTurn(t, s.db, "find-me", 0, "unrelated", "unrelated", ts)

			found, err := s.ListSessions(context.Background(),
				FilterOptions{Query: "Task Invoke", DeepSearch: deep},
				SortOptions{Field: SortByUpdated, Order: Descending}, 0)
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			if len(found) != 1 {
				t.Fatalf("%s search for %q found %d sessions, want 1", name, "Task Invoke", len(found))
			}
		})
	}
}

// A phrase must still be a phrase: collapsing whitespace must not turn the
// match into "these words appear somewhere in any order".
func TestExcludedWordsPhraseStaysContiguous(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	const ts = "2026-05-01T10:00:00Z"
	seedSession(t, s.db, "keep-me", "/work", "owner/repo", "main",
		"Task queue cleanup, then invoke the runner", ts, ts)
	seedTurn(t, s.db, "keep-me", 0, "unrelated", "unrelated", ts)

	sessions, err := s.ListSessions(context.Background(),
		FilterOptions{ExcludedWords: []string{"Task Invoke"}},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	requireSessionIDs(t, sessions, "keep-me")
}

func TestGroupSessionsExcludedWordsCoverCheckpoints(t *testing.T) {
	s := newTestStore(t)
	defer func() { _ = s.Close() }()
	seedExcludedWordSources(t, s)

	groups, err := s.GroupSessions(context.Background(), PivotByRepo,
		FilterOptions{ExcludedWords: []string{"quarantine"}},
		SortOptions{Field: SortByUpdated, Order: Descending}, 0)
	if err != nil {
		t.Fatalf("GroupSessions: %v", err)
	}
	for _, id := range groupedSessionIDs(groups) {
		if id != "keep-me" {
			t.Errorf("grouped list still shows %q after excluding %q", id, "quarantine")
		}
	}
}
