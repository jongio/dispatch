package data

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Benchmark helpers
// ---------------------------------------------------------------------------

// newBenchStore creates an in-memory SQLite store with the session-store
// schema applied. The caller should defer store.Close().
func newBenchStore(b *testing.B) *Store {
	b.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatalf("opening in-memory SQLite: %v", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		b.Fatalf("creating schema: %v", err)
	}
	return &Store{db: db}
}

// benchSeedSession inserts a session row.
func benchSeedSession(b *testing.B, db *sql.DB, id, cwd, repo, branch, summary, created, updated string) {
	b.Helper()
	_, err := db.Exec(
		`INSERT INTO sessions (id, cwd, repository, branch, summary, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, cwd, repo, branch, summary, created, updated,
	)
	if err != nil {
		b.Fatalf("seeding session %s: %v", id, err)
	}
}

// benchSeedTurn inserts a turn row.
func benchSeedTurn(b *testing.B, db *sql.DB, sessionID string, index int, userMsg, assistantResp, ts string) {
	b.Helper()
	_, err := db.Exec(
		`INSERT INTO turns (session_id, turn_index, user_message, assistant_response, timestamp)
		 VALUES (?, ?, ?, ?, ?)`,
		sessionID, index, userMsg, assistantResp, ts,
	)
	if err != nil {
		b.Fatalf("seeding turn %s/%d: %v", sessionID, index, err)
	}
}

// benchSeedFile inserts a session_files row.
func benchSeedFile(b *testing.B, db *sql.DB, sessionID, filePath, toolName string, turnIndex int, firstSeen string) {
	b.Helper()
	_, err := db.Exec(
		`INSERT INTO session_files (session_id, file_path, tool_name, turn_index, first_seen_at)
		 VALUES (?, ?, ?, ?, ?)`,
		sessionID, filePath, toolName, turnIndex, firstSeen,
	)
	if err != nil {
		b.Fatalf("seeding file %s/%s: %v", sessionID, filePath, err)
	}
}

// benchSeedRef inserts a session_refs row.
func benchSeedRef(b *testing.B, db *sql.DB, sessionID, refType, refValue string, turnIndex int, created string) {
	b.Helper()
	_, err := db.Exec(
		`INSERT INTO session_refs (session_id, ref_type, ref_value, turn_index, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		sessionID, refType, refValue, turnIndex, created,
	)
	if err != nil {
		b.Fatalf("seeding ref %s/%s: %v", sessionID, refType, err)
	}
}

// benchSeedCheckpoint inserts a checkpoints row.
func benchSeedCheckpoint(b *testing.B, db *sql.DB, sessionID string, num int, title, overview string) {
	b.Helper()
	_, err := db.Exec(
		`INSERT INTO checkpoints (session_id, checkpoint_number, title, overview)
		 VALUES (?, ?, ?, ?)`,
		sessionID, num, title, overview,
	)
	if err != nil {
		b.Fatalf("seeding checkpoint %s/%d: %v", sessionID, num, err)
	}
}

// populateBenchData seeds n sessions each with turns, files, refs, and
// checkpoints to create a realistic benchmark dataset.
func populateBenchData(b *testing.B, s *Store, n int) {
	b.Helper()
	db := s.db

	repos := []string{"owner/repo-a", "owner/repo-b", "org/service-x", "org/lib-y", ""}
	branches := []string{"main", "feature/search", "feature/api", "fix/bug-123", "develop"}
	folders := []string{
		"/home/user/project-a",
		"/home/user/project-b",
		"/home/user/work/service",
		"/tmp/scratch",
		"/home/user/oss/contrib",
	}
	summaries := []string{
		"Implement auth module",
		"Add search feature with fuzzy matching",
		"Build REST API endpoints",
		"Quick experiment with caching",
		"Fix database connection pooling",
		"Refactor middleware pipeline",
		"Add unit tests for parser",
		"Deploy to staging environment",
		"Debug memory leak in worker",
		"Update dependencies and security patches",
	}

	baseTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)

	for i := range n {
		sid := fmt.Sprintf("bench-sess-%04d", i)
		created := baseTime.Add(time.Duration(i) * time.Hour).Format(time.RFC3339)
		updated := baseTime.Add(time.Duration(i)*time.Hour + 2*time.Hour).Format(time.RFC3339)

		benchSeedSession(b, db, sid,
			folders[i%len(folders)],
			repos[i%len(repos)],
			branches[i%len(branches)],
			summaries[i%len(summaries)],
			created, updated,
		)

		// Each session gets 1-5 turns.
		turnCount := (i % 5) + 1
		for t := range turnCount {
			ts := baseTime.Add(time.Duration(i)*time.Hour + time.Duration(t)*10*time.Minute).Format(time.RFC3339)
			benchSeedTurn(b, db, sid, t,
				fmt.Sprintf("User message %d for session %d", t, i),
				fmt.Sprintf("Assistant response %d for session %d", t, i),
				ts,
			)
		}

		// Half the sessions get files.
		if i%2 == 0 {
			benchSeedFile(b, db, sid, fmt.Sprintf("src/module_%d.go", i), "edit", 0, created)
			benchSeedFile(b, db, sid, fmt.Sprintf("src/module_%d_test.go", i), "create", 1, updated)
		}

		// Every third session gets a ref.
		if i%3 == 0 {
			benchSeedRef(b, db, sid, "pr", fmt.Sprintf("%d", 100+i), 0, updated)
		}

		// Every fifth session gets a checkpoint.
		if i%5 == 0 {
			benchSeedCheckpoint(b, db, sid, 1,
				fmt.Sprintf("Checkpoint for session %d", i),
				fmt.Sprintf("Overview of work done in session %d", i),
			)
		}
	}
}

// ---------------------------------------------------------------------------
// ListSessions benchmarks
// ---------------------------------------------------------------------------

func BenchmarkListSessions(b *testing.B) {
	s := newBenchStore(b)
	defer s.Close() //nolint:errcheck
	populateBenchData(b, s, 200)

	filter := FilterOptions{}
	sort := SortOptions{Field: SortByUpdated, Order: Descending}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := s.ListSessions(filter, sort, 100); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkListSessionsWithSearch(b *testing.B) {
	s := newBenchStore(b)
	defer s.Close() //nolint:errcheck
	populateBenchData(b, s, 200)

	filter := FilterOptions{Query: "auth module"}
	sort := SortOptions{Field: SortByUpdated, Order: Descending}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := s.ListSessions(filter, sort, 100); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkListSessionsWithDateRange(b *testing.B) {
	s := newBenchStore(b)
	defer s.Close() //nolint:errcheck
	populateBenchData(b, s, 200)

	since := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC)
	filter := FilterOptions{Since: &since, Until: &until}
	sort := SortOptions{Field: SortByUpdated, Order: Descending}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := s.ListSessions(filter, sort, 100); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkListSessionsCombinedFilters(b *testing.B) {
	s := newBenchStore(b)
	defer s.Close() //nolint:errcheck
	populateBenchData(b, s, 200)

	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	filter := FilterOptions{
		Query:      "search",
		Repository: "owner/repo-b",
		Since:      &since,
	}
	sort := SortOptions{Field: SortByCreated, Order: Ascending}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := s.ListSessions(filter, sort, 50); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// GetSession benchmark
// ---------------------------------------------------------------------------

func BenchmarkGetSession(b *testing.B) {
	s := newBenchStore(b)
	defer s.Close() //nolint:errcheck
	populateBenchData(b, s, 200)

	// Pick a session that has turns, files, refs, and a checkpoint.
	// Session index 0: even (files), 0%3==0 (ref), 0%5==0 (checkpoint).
	const targetID = "bench-sess-0000"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := s.GetSession(targetID); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// SearchSessions benchmark
// ---------------------------------------------------------------------------

func BenchmarkSearchSessions(b *testing.B) {
	s := newBenchStore(b)
	defer s.Close() //nolint:errcheck
	populateBenchData(b, s, 200)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := s.SearchSessions("fuzzy matching", 50); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// GroupSessions benchmark
// ---------------------------------------------------------------------------

func BenchmarkGroupSessionsByFolder(b *testing.B) {
	s := newBenchStore(b)
	defer s.Close() //nolint:errcheck
	populateBenchData(b, s, 200)

	filter := FilterOptions{}
	sort := SortOptions{Field: SortByUpdated, Order: Descending}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := s.GroupSessions(PivotByFolder, filter, sort, 0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGroupSessionsByRepo(b *testing.B) {
	s := newBenchStore(b)
	defer s.Close() //nolint:errcheck
	populateBenchData(b, s, 200)

	filter := FilterOptions{}
	sort := SortOptions{Field: SortByUpdated, Order: Descending}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := s.GroupSessions(PivotByRepo, filter, sort, 0); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// ListSessions scaling sub-benchmarks
// ---------------------------------------------------------------------------

func BenchmarkListSessionsScaling(b *testing.B) {
	for _, n := range []int{10, 50, 200, 500} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			s := newBenchStore(b)
			defer s.Close() //nolint:errcheck
			populateBenchData(b, s, n)

			filter := FilterOptions{}
			sort := SortOptions{Field: SortByUpdated, Order: Descending}

			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := s.ListSessions(filter, sort, 100); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
