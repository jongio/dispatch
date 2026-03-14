package data

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jongio/dispatch/internal/platform"
	_ "modernc.org/sqlite"
)

// ErrIndexBusy is returned when the session store database is locked
// by another process (e.g. Copilot CLI reindexing).
var ErrIndexBusy = errors.New("session store is busy — Copilot CLI may be reindexing, try again shortly")

const (
	// defaultQueryLimit is the maximum number of sessions returned when no
	// explicit limit is provided to List/Search queries.
	defaultQueryLimit = 100

	// defaultGroupLimit caps the total rows returned when grouping sessions
	// by pivot field.  A high default is used because grouping typically
	// returns many small groups.
	defaultGroupLimit = 10_000

	// maxIDsPerQuery caps the number of IDs passed to ListSessionsByIDs to
	// stay within SQLite's variable limit and prevent resource exhaustion.
	maxIDsPerQuery = 500

	// limitClause is the SQL fragment appended to queries with a row cap.
	limitClause = " LIMIT ?"

	// coalesceCwd normalizes NULL session directories to empty strings.
	coalesceCwd = "COALESCE(s.cwd, '')"
)

// Store provides read-only access to the Copilot CLI session store.
type Store struct {
	db *sql.DB
}

// Open opens the session store at the default platform path.
func Open() (*Store, error) {
	path, err := platform.SessionStorePath()
	if err != nil {
		return nil, fmt.Errorf("resolving session store path: %w", err)
	}
	return OpenPath(path)
}

// OpenPath opens the session store at the given file path. The database is
// opened in read-only mode.
func OpenPath(path string) (*Store, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("session store not found at %s: %w", path, err)
	}
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("opening session store: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connecting to session store: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// ---------------------------------------------------------------------------
// Maintenance
// ---------------------------------------------------------------------------

// Maintain opens a temporary read-write connection to the session store,
// checkpoints the WAL, rebuilds and optimises the FTS5 search index, then
// closes the connection. This does NOT modify session data — only index
// and journal maintenance. Safe to call while the read-only Store is open.
func Maintain() error {
	path, err := platform.SessionStorePath()
	if err != nil {
		return fmt.Errorf("resolving session store path: %w", err)
	}
	if _, err := os.Stat(path); err != nil {
		return nil // no store yet — nothing to maintain
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("opening store for maintenance: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Checkpoint WAL — consolidates write-ahead log into the main db.
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		if isDBBusy(err) {
			return ErrIndexBusy
		}
		return fmt.Errorf("WAL checkpoint: %w", err)
	}

	// Rebuild FTS5 search index from source data.
	// FTS5 table may not exist in older stores — only ignore "no such table".
	if _, err := db.Exec("INSERT INTO search_index(search_index) VALUES('rebuild')"); err != nil {
		if !strings.Contains(err.Error(), "no such table") {
			if isDBBusy(err) {
				return ErrIndexBusy
			}
			return fmt.Errorf("FTS5 rebuild: %w", err)
		}
	}

	// Optimise FTS5 index (merge internal b-tree segments).
	if _, err := db.Exec("INSERT INTO search_index(search_index) VALUES('optimize')"); err != nil {
		if !strings.Contains(err.Error(), "no such table") {
			if isDBBusy(err) {
				return ErrIndexBusy
			}
			return fmt.Errorf("FTS5 optimize: %w", err)
		}
	}

	return nil
}

// LastReindexTime returns the modification time of the session store
// database file, which is updated whenever a reindex writes to it.
// Returns the zero time if the file cannot be found.
func LastReindexTime() time.Time {
	path, err := platform.SessionStorePath()
	if err != nil {
		return time.Time{}
	}
	// Check WAL first — active writes go there before checkpoint.
	if info, err := os.Stat(path + "-wal"); err == nil && info.Size() > 0 {
		return info.ModTime()
	}
	if info, err := os.Stat(path); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

// isDBBusy returns true if the error indicates the database is locked
// or busy — typically because the Copilot CLI is actively reindexing.
func isDBBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "busy") ||
		strings.Contains(msg, "locked")
}

// ---------------------------------------------------------------------------
// Query helpers
// ---------------------------------------------------------------------------

// escapeLIKE escapes SQL LIKE wildcard characters so that user input
// is treated as literal text in LIKE patterns.
func escapeLIKE(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// lastActiveExpr is the SQL expression that computes a session's true
// "last active" time: the latest turn timestamp, falling back to
// updated_at then created_at.  Use this in WHERE/GROUP BY clauses
// where column aliases are not available.
const lastActiveExpr = `COALESCE(
	(SELECT MAX(t.timestamp) FROM turns t WHERE t.session_id = s.id),
	s.updated_at,
	s.created_at
)`

// filterBuilder accumulates JOIN and WHERE clauses with parameterised args.
type filterBuilder struct {
	joins  []string
	wheres []string
	args   []any
}

func (fb *filterBuilder) apply(f FilterOptions) {
	// Always exclude sessions with zero turns.
	fb.wheres = append(fb.wheres, "EXISTS (SELECT 1 FROM turns t WHERE t.session_id = s.id)")

	if f.Query != "" {
		pattern := "%" + escapeLIKE(f.Query) + "%"
		if f.DeepSearch {
			// Deep mode: search session fields + related tables.
			fb.wheres = append(fb.wheres,
				`(s.summary LIKE ? ESCAPE '\' OR s.branch LIKE ? ESCAPE '\' OR s.repository LIKE ? ESCAPE '\' OR s.cwd LIKE ? ESCAPE '\'`+
					` OR EXISTS (SELECT 1 FROM turns t2 WHERE t2.session_id = s.id AND t2.user_message LIKE ? ESCAPE '\')`+
					` OR EXISTS (SELECT 1 FROM checkpoints cp WHERE cp.session_id = s.id AND (cp.title LIKE ? ESCAPE '\' OR cp.overview LIKE ? ESCAPE '\'))`+
					` OR EXISTS (SELECT 1 FROM session_files sf2 WHERE sf2.session_id = s.id AND sf2.file_path LIKE ? ESCAPE '\')`+
					` OR EXISTS (SELECT 1 FROM session_refs sr2 WHERE sr2.session_id = s.id AND sr2.ref_value LIKE ? ESCAPE '\'))`)
			fb.args = append(fb.args, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern)
		} else {
			// Quick mode: search only session-level fields (no JOINs).
			fb.wheres = append(fb.wheres,
				`(s.summary LIKE ? ESCAPE '\' OR s.branch LIKE ? ESCAPE '\' OR s.repository LIKE ? ESCAPE '\' OR s.cwd LIKE ? ESCAPE '\')`)
			fb.args = append(fb.args, pattern, pattern, pattern, pattern)
		}
	}
	if f.Folder != "" {
		fb.wheres = append(fb.wheres, `s.cwd LIKE ? ESCAPE '\'`)
		fb.args = append(fb.args, escapeLIKE(f.Folder)+"%")
	}
	if f.Repository != "" {
		fb.wheres = append(fb.wheres, "s.repository = ?")
		fb.args = append(fb.args, f.Repository)
	}
	if f.Branch != "" {
		fb.wheres = append(fb.wheres, "s.branch = ?")
		fb.args = append(fb.args, f.Branch)
	}
	if f.Since != nil {
		fb.wheres = append(fb.wheres, lastActiveExpr+" >= ?")
		fb.args = append(fb.args, f.Since.Format(time.RFC3339))
	}
	if f.Until != nil {
		fb.wheres = append(fb.wheres, lastActiveExpr+" <= ?")
		fb.args = append(fb.args, f.Until.Format(time.RFC3339))
	}
	if f.HasRefs {
		fb.wheres = append(fb.wheres, "EXISTS (SELECT 1 FROM session_refs sr WHERE sr.session_id = s.id)")
	}
	if len(f.ExcludedDirs) > 0 {
		for _, dir := range f.ExcludedDirs {
			fb.wheres = append(fb.wheres, `s.cwd NOT LIKE ? ESCAPE '\'`)
			fb.args = append(fb.args, escapeLIKE(dir)+"%")
		}
	}
}

func (fb *filterBuilder) joinSQL() string {
	if len(fb.joins) == 0 {
		return ""
	}
	return " " + strings.Join(fb.joins, " ")
}

func (fb *filterBuilder) whereSQL() string {
	if len(fb.wheres) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(fb.wheres, " AND ")
}

// sortColumn returns a safe SQL column expression for the given SortField.
func sortColumn(f SortField) string {
	switch f {
	case SortByCreated:
		return "s.created_at"
	case SortByTurns:
		return "turn_count"
	case SortByName:
		return "s.summary"
	case SortByFolder:
		return "s.cwd"
	default: // SortByUpdated and any unknown value
		return lastActiveExpr
	}
}

// sortDir returns a safe SQL direction keyword for the given SortOrder.
func sortDir(o SortOrder) string {
	if o == Ascending {
		return string(Ascending)
	}
	return string(Descending)
}

// pivotExpr returns the SQL expression used to compute the group label for
// a given PivotField.
func pivotExpr(p PivotField) string {
	switch p {
	case PivotByRepo:
		return "COALESCE(s.repository, '')"
	case PivotByBranch:
		return "COALESCE(s.branch, '')"
	case PivotByDate:
		return "SUBSTR(" + lastActiveExpr + ", 1, 10)"
	default: // PivotByFolder and any unknown value
		return coalesceCwd
	}
}

// sessionColumns is the shared SELECT list used by session queries.
// last_active_at is computed as the most recent turn timestamp, falling
// back to updated_at then created_at so that reindex-clobbered dates
// don't make old sessions appear recent.
var sessionColumns = `s.id, ` + coalesceCwd + `, COALESCE(s.repository,''), COALESCE(s.branch,''),
	COALESCE(s.summary,''), COALESCE(s.created_at,''), COALESCE(s.updated_at,''),
	` + lastActiveExpr + ` AS last_active_at,
	(SELECT COUNT(*) FROM turns t WHERE t.session_id = s.id) AS turn_count,
	(SELECT COUNT(DISTINCT sf.file_path) FROM session_files sf WHERE sf.session_id = s.id) AS file_count`

// scanner is the subset of *sql.Row and *sql.Rows used to read columns.
type scanner interface{ Scan(dest ...any) error }

func scanSession(sc scanner) (Session, error) {
	var sess Session
	err := sc.Scan(
		&sess.ID, &sess.Cwd, &sess.Repository, &sess.Branch,
		&sess.Summary, &sess.CreatedAt, &sess.UpdatedAt,
		&sess.LastActiveAt, &sess.TurnCount, &sess.FileCount,
	)
	return sess, err
}

// ---------------------------------------------------------------------------
// Public query methods
// ---------------------------------------------------------------------------

// ListSessions returns sessions matching the filter, ordered and limited as
// specified. TurnCount and FileCount are computed via subqueries.
func (s *Store) ListSessions(filter FilterOptions, sort SortOptions, limit int) ([]Session, error) {
	var fb filterBuilder
	fb.apply(filter)

	q := "SELECT " + sessionColumns + " FROM sessions s" + fb.joinSQL() + fb.whereSQL()
	q += fmt.Sprintf(" ORDER BY %s %s", sortColumn(sort.Field), sortDir(sort.Order))

	if limit <= 0 {
		limit = defaultQueryLimit
	}
	q += limitClause
	fb.args = append(fb.args, limit)

	rows, err := s.db.Query(q, fb.args...)
	if err != nil {
		return nil, fmt.Errorf("querying sessions: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows read-only

	var sessions []Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning session row: %w", err)
		}
		sessions = append(sessions, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating session rows: %w", err)
	}
	return sessions, nil
}

// ListSessionsByIDs returns sessions matching the given IDs, preserving the
// input order. IDs not found in the database are silently skipped.
func (s *Store) ListSessionsByIDs(ids []string) ([]Session, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	// Cap to prevent oversized IN clauses that could exceed SQLite's
	// variable limit or cause resource exhaustion.
	if len(ids) > maxIDsPerQuery {
		ids = ids[:maxIDsPerQuery]
	}
	// Build "WHERE s.id IN (?,?,...)" with one placeholder per ID.
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	q := "SELECT " + sessionColumns + " FROM sessions s WHERE s.id IN (" +
		strings.Join(placeholders, ",") + ")"

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying sessions by IDs: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows read-only

	// Index results by ID for order-preserving assembly.
	byID := make(map[string]Session, len(ids))
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning session row: %w", err)
		}
		byID[sess.ID] = sess
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating session rows: %w", err)
	}

	// Assemble result in input order, skipping missing IDs.
	result := make([]Session, 0, len(byID))
	for _, id := range ids {
		if sess, ok := byID[id]; ok {
			result = append(result, sess)
		}
	}
	return result, nil
}

// GetSession loads a single session and all of its related turns,
// checkpoints, files, and refs.
func (s *Store) GetSession(id string) (*SessionDetail, error) {
	row := s.db.QueryRow("SELECT "+sessionColumns+" FROM sessions s WHERE s.id = ?", id)
	sess, err := scanSession(row)
	if err != nil {
		return nil, fmt.Errorf("loading session %s: %w", id, err)
	}

	detail := &SessionDetail{Session: sess}

	// Turns
	tRows, err := s.db.Query(
		`SELECT session_id, turn_index, COALESCE(user_message,''), COALESCE(assistant_response,''), COALESCE(timestamp,'')
		 FROM turns WHERE session_id = ? ORDER BY turn_index`, id)
	if err != nil {
		return nil, fmt.Errorf("loading turns for session %s: %w", id, err)
	}
	defer tRows.Close() //nolint:errcheck // rows read-only
	for tRows.Next() {
		var t Turn
		if err := tRows.Scan(&t.SessionID, &t.TurnIndex, &t.UserMessage, &t.AssistantResponse, &t.Timestamp); err != nil {
			return nil, fmt.Errorf("scanning turn row: %w", err)
		}
		detail.Turns = append(detail.Turns, t)
	}
	if err := tRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating turn rows: %w", err)
	}

	// Checkpoints
	cRows, err := s.db.Query(
		`SELECT session_id, checkpoint_number, COALESCE(title,''), COALESCE(overview,''),
		        COALESCE(history,''), COALESCE(work_done,''), COALESCE(technical_details,''),
		        COALESCE(important_files,''), COALESCE(next_steps,'')
		 FROM checkpoints WHERE session_id = ? ORDER BY checkpoint_number`, id)
	if err != nil {
		return nil, fmt.Errorf("loading checkpoints for session %s: %w", id, err)
	}
	defer cRows.Close() //nolint:errcheck // rows read-only
	for cRows.Next() {
		var c Checkpoint
		if err := cRows.Scan(&c.SessionID, &c.CheckpointNumber, &c.Title, &c.Overview,
			&c.History, &c.WorkDone, &c.TechnicalDetails, &c.ImportantFiles, &c.NextSteps); err != nil {
			return nil, fmt.Errorf("scanning checkpoint row: %w", err)
		}
		detail.Checkpoints = append(detail.Checkpoints, c)
	}
	if err := cRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating checkpoint rows: %w", err)
	}

	// Files
	fRows, err := s.db.Query(
		`SELECT session_id, COALESCE(file_path,''), COALESCE(tool_name,''), turn_index, COALESCE(first_seen_at,'')
		 FROM session_files WHERE session_id = ? ORDER BY turn_index, file_path`, id)
	if err != nil {
		return nil, fmt.Errorf("loading files for session %s: %w", id, err)
	}
	defer fRows.Close() //nolint:errcheck // rows read-only
	for fRows.Next() {
		var f SessionFile
		if err := fRows.Scan(&f.SessionID, &f.FilePath, &f.ToolName, &f.TurnIndex, &f.FirstSeenAt); err != nil {
			return nil, fmt.Errorf("scanning file row: %w", err)
		}
		detail.Files = append(detail.Files, f)
	}
	if err := fRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating file rows: %w", err)
	}

	// Refs
	rRows, err := s.db.Query(
		`SELECT session_id, COALESCE(ref_type,''), COALESCE(ref_value,''), turn_index, COALESCE(created_at,'')
		 FROM session_refs WHERE session_id = ? ORDER BY turn_index`, id)
	if err != nil {
		return nil, fmt.Errorf("loading refs for session %s: %w", id, err)
	}
	defer rRows.Close() //nolint:errcheck // rows read-only
	for rRows.Next() {
		var r SessionRef
		if err := rRows.Scan(&r.SessionID, &r.RefType, &r.RefValue, &r.TurnIndex, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning ref row: %w", err)
		}
		detail.Refs = append(detail.Refs, r)
	}
	if err := rRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating ref rows: %w", err)
	}

	return detail, nil
}

// SearchSessions performs a fuzzy substring search across session metadata
// and turn content, returning matches ranked by source type. Sessions with
// zero turns are excluded.
func (s *Store) SearchSessions(query string, limit int) ([]SearchResult, error) {
	pattern := "%" + escapeLIKE(query) + "%"
	q := `SELECT content, session_id, source_type, source_id, 0 AS rank FROM (
		SELECT COALESCE(s2.summary,'') AS content, s2.id AS session_id,
			'session' AS source_type, '' AS source_id
		FROM sessions s2
		WHERE (s2.summary LIKE ? ESCAPE '\' OR s2.branch LIKE ? ESCAPE '\' OR s2.repository LIKE ? ESCAPE '\')
			AND EXISTS (SELECT 1 FROM turns t WHERE t.session_id = s2.id)
		UNION ALL
		SELECT COALESCE(t.user_message,'') AS content, t.session_id,
			'turn' AS source_type, CAST(t.turn_index AS TEXT) AS source_id
		FROM turns t
		WHERE t.user_message LIKE ? ESCAPE '\'
			AND EXISTS (SELECT 1 FROM turns t2 WHERE t2.session_id = t.session_id)
	) sub`
	args := []any{pattern, pattern, pattern, pattern} //nolint:prealloc // literal init is clearer than make+append

	if limit <= 0 {
		limit = defaultQueryLimit
	}
	q += limitClause
	args = append(args, limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("searching sessions: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows read-only

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.Content, &r.SessionID, &r.SourceType, &r.SourceID, &r.Rank); err != nil {
			return nil, fmt.Errorf("scanning search result: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating search results: %w", err)
	}
	return results, nil
}

// ListFolders returns the distinct cwd values across all sessions, sorted
// alphabetically.
func (s *Store) ListFolders() ([]string, error) {
	return s.distinctStrings("SELECT DISTINCT COALESCE(cwd,'') FROM sessions WHERE cwd IS NOT NULL AND cwd != '' ORDER BY cwd")
}

// ListRepositories returns the distinct non-empty repository values across
// all sessions, sorted alphabetically.
func (s *Store) ListRepositories() ([]string, error) {
	return s.distinctStrings("SELECT DISTINCT repository FROM sessions WHERE repository IS NOT NULL AND repository != '' ORDER BY repository")
}

// ListBranches returns distinct branch values. If repository is non-empty,
// results are filtered to that repository.
func (s *Store) ListBranches(repository string) ([]string, error) {
	if repository != "" {
		return s.distinctStrings(
			"SELECT DISTINCT branch FROM sessions WHERE branch IS NOT NULL AND branch != '' AND repository = ? ORDER BY branch",
			repository)
	}
	return s.distinctStrings("SELECT DISTINCT branch FROM sessions WHERE branch IS NOT NULL AND branch != '' ORDER BY branch")
}

// GroupSessions groups sessions by the specified pivot field, applying the
// given filter and sort order within each group.
func (s *Store) GroupSessions(pivot PivotField, filter FilterOptions, sort SortOptions, limit int) ([]SessionGroup, error) {
	var fb filterBuilder
	fb.apply(filter)

	expr := pivotExpr(pivot)
	q := fmt.Sprintf("SELECT %s AS pivot_label, %s FROM sessions s%s%s ORDER BY pivot_label, %s %s",
		expr, sessionColumns, fb.joinSQL(), fb.whereSQL(), sortColumn(sort.Field), sortDir(sort.Order))

	if limit <= 0 {
		limit = defaultGroupLimit
	}
	q += limitClause
	fb.args = append(fb.args, limit)

	rows, err := s.db.Query(q, fb.args...)
	if err != nil {
		return nil, fmt.Errorf("querying grouped sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	groupMap := make(map[string]*SessionGroup)
	var order []string

	for rows.Next() {
		var label string
		var sess Session
		if err := rows.Scan(&label,
			&sess.ID, &sess.Cwd, &sess.Repository, &sess.Branch,
			&sess.Summary, &sess.CreatedAt, &sess.UpdatedAt,
			&sess.LastActiveAt, &sess.TurnCount, &sess.FileCount); err != nil {
			return nil, fmt.Errorf("scanning grouped session row: %w", err)
		}
		g, ok := groupMap[label]
		if !ok {
			g = &SessionGroup{Label: label}
			groupMap[label] = g
			order = append(order, label)
		}
		g.Sessions = append(g.Sessions, sess)
		g.Count = len(g.Sessions)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating grouped session rows: %w", err)
	}

	result := make([]SessionGroup, 0, len(order))
	for _, label := range order {
		result = append(result, *groupMap[label])
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (s *Store) distinctStrings(query string, args ...any) ([]string, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying distinct values: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var vals []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scanning distinct value: %w", err)
		}
		vals = append(vals, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating distinct values: %w", err)
	}
	return vals, nil
}
