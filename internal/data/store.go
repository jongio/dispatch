package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jongio/dispatch/internal/platform"
	"golang.org/x/sync/errgroup"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
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

	// maxIDsPerQuery caps the number of IDs bound in a single ID lookup
	// statement, to stay within SQLite's variable limit. Callers passing
	// more IDs are served by issuing multiple batched queries.
	maxIDsPerQuery = 500

	// limitClause is the SQL fragment appended to queries with a row cap.
	limitClause = " LIMIT ?"

	// coalesceCwd normalizes NULL session directories to empty strings.
	coalesceCwd = "COALESCE(s.cwd, '')"
)

// Store provides read-only access to the Copilot CLI session store.
type Store struct {
	db          *sql.DB
	hasHostType bool   // true if sessions table has host_type column (schema v3+)
	hasFTS5     bool   // true if FTS5 search_index virtual table is present
	tempDir     string // OS temp directory prefix, used to filter automated sessions
	homeDir     string // user home directory, used to filter hidden dotfolder sessions
}

// closeRows closes a rows object, intentionally discarding the error.
// sql.Rows.Close returns an error only if the rows were already closed
// or if there was a network error on the underlying connection - neither
// applies to local SQLite. This helper documents the intent and silences
// the linter in one place.
func closeRows(rows *sql.Rows) {
	_ = rows.Close()
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
	db, err := openReadOnlySQLite(path)
	if err != nil {
		return nil, fmt.Errorf("opening session store: %w", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connecting to session store: %w", err)
	}

	// Detect schema version: check if host_type column exists.
	hasHostType := false
	rows, err := db.QueryContext(context.Background(), "PRAGMA table_info(sessions)")
	if err == nil {
		defer closeRows(rows)
		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull int
			var dflt sql.NullString
			var pk int
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
				if name == "host_type" {
					hasHostType = true
					break
				}
			}
		}
	}

	// Detect FTS5 search_index table availability.
	hasFTS5 := false
	ftsRow := db.QueryRowContext(context.Background(), "SELECT name FROM sqlite_master WHERE type='table' AND name='search_index'")
	var ftsName string
	if ftsRow.Scan(&ftsName) == nil && ftsName == "search_index" {
		hasFTS5 = true
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("could not resolve home directory", "error", err)
	}
	return &Store{db: db, hasHostType: hasHostType, hasFTS5: hasFTS5, tempDir: os.TempDir(), homeDir: homeDir}, nil
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
func Maintain(ctx context.Context) error {
	path, err := platform.SessionStorePath()
	if err != nil {
		return fmt.Errorf("resolving session store path: %w", err)
	}
	if _, err := os.Stat(path); err != nil {
		return nil // no store yet — nothing to maintain
	}

	dsn, err := readWriteSQLiteDSN(path)
	if err != nil {
		return fmt.Errorf("building maintenance connection: %w", err)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("opening store for maintenance: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Checkpoint WAL — consolidates write-ahead log into the main db.
	var busy, logFrames, checkpointedFrames int
	if err := db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointedFrames); err != nil {
		if isDBBusy(err) {
			return ErrIndexBusy
		}
		return fmt.Errorf("WAL checkpoint: %w", err)
	}
	if busy != 0 {
		return ErrIndexBusy
	}

	// Rebuild FTS5 search index from source data.
	// FTS5 table may not exist in older stores — only ignore "no such table".
	if _, err := db.ExecContext(ctx, "INSERT INTO search_index(search_index) VALUES('rebuild')"); err != nil {
		if !isSQLiteNoSuchTable(err) {
			if isDBBusy(err) {
				return ErrIndexBusy
			}
			return fmt.Errorf("FTS5 rebuild: %w", err)
		}
	}

	// Optimise FTS5 index (merge internal b-tree segments).
	if _, err := db.ExecContext(ctx, "INSERT INTO search_index(search_index) VALUES('optimize')"); err != nil {
		if !isSQLiteNoSuchTable(err) {
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

// isDBBusy returns true if err indicates the SQLite database is busy or
// locked. It first tries errors.As to extract the typed *sqlite.Error and
// check the numeric error code (SQLITE_BUSY=5, SQLITE_LOCKED=6). This
// catches errors produced by the modernc.org/sqlite driver directly.
//
// A string-based fallback is retained because database/sql may rewrap driver
// errors in ways that strip the typed *sqlite.Error (e.g. via fmt.Errorf
// without %w in older call sites). The fallback ensures busy detection still
// works in those edge cases.
func isDBBusy(err error) bool {
	if err == nil {
		return false
	}

	// Prefer typed error code check via errors.As.
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		code := sqliteErr.Code()
		// Mask off extended error code bits to get the primary result code.
		primary := code & 0xFF
		if primary == sqlite3.SQLITE_BUSY || primary == sqlite3.SQLITE_LOCKED {
			return true
		}
	}

	// Fallback: string matching for errors that lost the typed wrapper.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "busy") ||
		strings.Contains(msg, "locked")
}

// isSQLiteNoSuchTable returns true if err indicates a "no such table"
// error from SQLite. It mirrors the isDBBusy pattern: typed error code
// check via errors.As first, with a string fallback for errors that lost
// the typed wrapper during rewrapping.
//
// SQLite reports "no such table" as SQLITE_ERROR (generic code 1) with the
// table name in the message — there is no dedicated error code for it.
func isSQLiteNoSuchTable(err error) bool {
	if err == nil {
		return false
	}

	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		primary := sqliteErr.Code() & 0xFF
		if primary == sqlite3.SQLITE_ERROR &&
			strings.Contains(strings.ToLower(sqliteErr.Error()), "no such table") {
			return true
		}
	}

	// Fallback: string matching for errors that lost the typed wrapper.
	return strings.Contains(strings.ToLower(err.Error()), "no such table")
}

// isFTS5QueryError returns true if err indicates an FTS5-specific query
// failure (e.g. malformed MATCH syntax). These errors are non-fatal for
// search — the caller should return an empty result set instead of
// propagating. Uses the same typed-first, string-fallback strategy as
// isDBBusy and isSQLiteNoSuchTable.
func isFTS5QueryError(err error) bool {
	if err == nil {
		return false
	}

	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		if strings.Contains(strings.ToLower(sqliteErr.Error()), "fts5") {
			return true
		}
	}

	// Fallback: string matching for errors that lost the typed wrapper.
	return strings.Contains(strings.ToLower(err.Error()), "fts5")
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
// "last active" time as the most recent signal across indexed turns,
// updated_at, and created_at.  Indexed turns may lag behind actual
// activity (reindex hasn't run), while updated_at may occasionally be
// noisy (metadata-only updates).  Taking the MAX of all three gives
// the best available estimate.
//
// Each value is COALESCE'd to empty-string so that SQLite's multi-arg MAX()
// never sees NULL (which would poison the result to NULL).
const lastActiveExpr = `MAX(
	COALESCE((SELECT MAX(t_last.timestamp) FROM turns t_last WHERE t_last.session_id = s.id), ''),
	COALESCE(s.updated_at, ''),
	COALESCE(s.created_at, '')
)`

const lastActiveJoinedExpr = `MAX(
	COALESCE(tc.last_turn_at, ''),
	COALESCE(s.updated_at, ''),
	COALESCE(s.created_at, '')
)`

// filterBuilder accumulates JOIN and WHERE clauses with parameterised args.
type filterBuilder struct {
	joins                []string
	wheres               []string
	args                 []any
	lastActiveExpr       string
	expandDatePredicates bool

	// hasFTS reports whether the FTS5 search_index table is available. When
	// true, deep search resolves turn and checkpoint text through the index
	// instead of scanning those tables with LIKE.
	hasFTS bool
}

func (fb *filterBuilder) lastActiveSQL() string {
	if fb.lastActiveExpr != "" {
		return fb.lastActiveExpr
	}
	return lastActiveExpr
}

func (fb *filterBuilder) apply(f FilterOptions) {
	// Exclude empty sessions: no turns AND no files, checkpoints, or refs.
	// Sessions with activity in any of these tables are kept even if turns
	// haven't been recorded (e.g. MCP / sub-agent work).
	fb.wheres = append(fb.wheres,
		`(EXISTS (SELECT 1 FROM turns t_activity WHERE t_activity.session_id = s.id)`+
			` OR EXISTS (SELECT 1 FROM session_files sf_activity WHERE sf_activity.session_id = s.id)`+
			` OR EXISTS (SELECT 1 FROM checkpoints cp2 WHERE cp2.session_id = s.id)`+
			` OR EXISTS (SELECT 1 FROM session_refs sr3 WHERE sr3.session_id = s.id))`)

	if f.Query != "" {
		pattern := "%" + escapeLIKE(f.Query) + "%"
		if f.DeepSearch {
			// Deep mode: search session fields + related tables.
			clauses := []string{
				`s.summary LIKE ? ESCAPE '\'`,
				`s.branch LIKE ? ESCAPE '\'`,
				`s.repository LIKE ? ESCAPE '\'`,
				`s.cwd LIKE ? ESCAPE '\'`,
				`EXISTS (SELECT 1 FROM turns t2 WHERE t2.session_id = s.id AND t2.user_message LIKE ? ESCAPE '\')`,
				`EXISTS (SELECT 1 FROM checkpoints cp WHERE cp.session_id = s.id AND (cp.title LIKE ? ESCAPE '\' OR cp.overview LIKE ? ESCAPE '\'))`,
				`EXISTS (SELECT 1 FROM session_files sf2 WHERE sf2.session_id = s.id AND sf2.file_path LIKE ? ESCAPE '\')`,
				`EXISTS (SELECT 1 FROM session_refs sr2 WHERE sr2.session_id = s.id AND sr2.ref_value LIKE ? ESCAPE '\')`,
			}
			fb.args = append(fb.args, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern, pattern)

			// The FTS5 index covers assistant replies and every checkpoint text
			// field, none of which the LIKE clauses above reach. It is additive
			// rather than a replacement: LIKE still catches substrings inside a
			// token (bug "1137" within "PR1137") that the tokenizer splits away.
			if ftsTerms := escapeFTS5Terms(f.Query); fb.hasFTS && ftsTerms != "" {
				clauses = append(clauses, `s.id IN (SELECT session_id FROM search_index WHERE content MATCH ?)`)
				fb.args = append(fb.args, ftsTerms)
			} else {
				// Without the index, assistant replies are only reachable by scan.
				clauses = append(clauses,
					`EXISTS (SELECT 1 FROM turns t4 WHERE t4.session_id = s.id AND t4.assistant_response LIKE ? ESCAPE '\')`)
				fb.args = append(fb.args, pattern)
			}
			fb.wheres = append(fb.wheres, "("+strings.Join(clauses, " OR ")+")")
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
	if f.HostType != "" {
		fb.wheres = append(fb.wheres, "s.host_type = ?")
		fb.args = append(fb.args, f.HostType)
	}
	if f.Since != nil {
		value := f.Since.UTC().Format(time.RFC3339)
		if fb.expandDatePredicates {
			fb.wheres = append(fb.wheres,
				`(COALESCE(s.updated_at, '') >= ? OR COALESCE(s.created_at, '') >= ?`+
					` OR EXISTS (SELECT 1 FROM turns t_since WHERE t_since.session_id = s.id AND COALESCE(t_since.timestamp, '') >= ?))`)
			fb.args = append(fb.args, value, value, value)
		} else {
			fb.wheres = append(fb.wheres, fb.lastActiveSQL()+" >= ?")
			fb.args = append(fb.args, value)
		}
	}
	if f.Until != nil {
		value := f.Until.UTC().Format(time.RFC3339)
		if fb.expandDatePredicates {
			fb.wheres = append(fb.wheres,
				`(COALESCE(s.updated_at, '') <= ? AND COALESCE(s.created_at, '') <= ?`+
					` AND NOT EXISTS (SELECT 1 FROM turns t_until WHERE t_until.session_id = s.id AND COALESCE(t_until.timestamp, '') > ?))`)
			fb.args = append(fb.args, value, value, value)
		} else {
			fb.wheres = append(fb.wheres, fb.lastActiveSQL()+" <= ?")
			fb.args = append(fb.args, value)
		}
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
	if len(f.ExcludedWords) > 0 {
		for _, word := range f.ExcludedWords {
			word = strings.TrimSpace(word)
			if word == "" {
				continue
			}
			pattern := "%" + escapeLIKE(strings.ToLower(word)) + "%"
			fb.wheres = append(fb.wheres,
				`(LOWER(COALESCE(s.summary,'')) NOT LIKE ? ESCAPE '\'`+
					` AND NOT EXISTS (SELECT 1 FROM turns t3 WHERE t3.session_id = s.id`+
					` AND (LOWER(COALESCE(t3.user_message,'')) LIKE ? ESCAPE '\'`+
					` OR LOWER(COALESCE(t3.assistant_response,'')) LIKE ? ESCAPE '\')))`)
			fb.args = append(fb.args, pattern, pattern, pattern)
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
		return lastActiveExpr
	case PivotByHost:
		return "COALESCE(s.host_type, '')"
	default: // PivotByFolder and any unknown value
		return coalesceCwd
	}
}

// sessionColumnsBase is the shared SELECT list used by session queries,
// excluding the host_type column which may not exist in older schemas.
var sessionColumnsBase = `s.id, ` + coalesceCwd + `, COALESCE(s.repository,''), COALESCE(s.branch,''),
	COALESCE(s.summary,''), COALESCE(s.created_at,''), COALESCE(s.updated_at,'')`

// sessionColumnsSuffix is the computed columns appended after host_type.
// Indexed correlated aggregates avoid materializing complete turns and files
// summaries before filtering and limiting the session rows.
var sessionColumnsSuffix = lastActiveExpr + ` AS last_active_at,
	COALESCE((SELECT COUNT(*) FROM turns t_count WHERE t_count.session_id = s.id), 0) AS turn_count,
	COALESCE((SELECT COUNT(DISTINCT sf_count.file_path)
		FROM session_files sf_count WHERE sf_count.session_id = s.id), 0) AS file_count`

// sessionColumns returns the full SELECT column list, including host_type
// when the schema supports it.
func (s *Store) sessionColumns() string {
	if s.hasHostType {
		return sessionColumnsBase + `,
	COALESCE(s.host_type,''),
	` + sessionColumnsSuffix
	}
	return sessionColumnsBase + `,
	'' AS host_type,
	` + sessionColumnsSuffix
}

var sessionColumnsJoinedSuffix = lastActiveJoinedExpr + ` AS last_active_at,
	COALESCE(tc.turn_count, 0) AS turn_count,
	COALESCE((SELECT COUNT(DISTINCT sf_count.file_path)
		FROM session_files sf_count WHERE sf_count.session_id = s.id), 0) AS file_count`

func (s *Store) sessionColumnsJoined() string {
	if s.hasHostType {
		return sessionColumnsBase + `,
	COALESCE(s.host_type,''),
	` + sessionColumnsJoinedSuffix
	}
	return sessionColumnsBase + `,
	'' AS host_type,
	` + sessionColumnsJoinedSuffix
}

const turnStatsJoin = ` LEFT JOIN (
	SELECT session_id, COUNT(*) AS turn_count, MAX(timestamp) AS last_turn_at
	FROM turns GROUP BY session_id
) tc ON tc.session_id = s.id`

// scanner is the subset of *sql.Row and *sql.Rows used to read columns.
type scanner interface{ Scan(dest ...any) error }

func scanSession(sc scanner) (Session, error) {
	var sess Session
	err := sc.Scan(
		&sess.ID, &sess.Cwd, &sess.Repository, &sess.Branch,
		&sess.Summary, &sess.CreatedAt, &sess.UpdatedAt,
		&sess.HostType,
		&sess.LastActiveAt, &sess.TurnCount, &sess.FileCount,
	)
	return sess, err
}

// scanGroupedSession scans a row that has a leading pivot label followed
// by the standard session columns. This keeps GroupSessions in sync with
// scanSession so new columns only need adding in one place.
func scanGroupedSession(sc scanner, label *string) (Session, error) {
	var sess Session
	err := sc.Scan(
		label,
		&sess.ID, &sess.Cwd, &sess.Repository, &sess.Branch,
		&sess.Summary, &sess.CreatedAt, &sess.UpdatedAt,
		&sess.HostType,
		&sess.LastActiveAt, &sess.TurnCount, &sess.FileCount,
	)
	return sess, err
}

// ---------------------------------------------------------------------------
// Public query methods
// ---------------------------------------------------------------------------

// withAutoExclusions returns a copy of the filter with automatic exclusions
// applied: OS temp directory and hidden dotfolders under the user's home
// directory (e.g. ~/.devx/, ~/.config/). These are tool infrastructure
// directories, not user project workspaces.
func (s *Store) withAutoExclusions(f FilterOptions) FilterOptions {
	// Copy the slice to avoid mutating the caller's.
	dirs := append([]string(nil), f.ExcludedDirs...)
	if s.tempDir != "" {
		dirs = append(dirs, s.tempDir)
	}
	if s.homeDir != "" {
		// Exclude hidden dotfolders directly under home.
		// filepath.Join would strip the trailing ".", so we build the prefix manually.
		dirs = append(dirs, s.homeDir+string(filepath.Separator)+".")
	}
	f.ExcludedDirs = dirs
	return f
}

// ListSessions returns sessions matching the filter, ordered and limited as
// specified. TurnCount and FileCount are computed via subqueries.
func (s *Store) ListSessions(ctx context.Context, filter FilterOptions, sort SortOptions, limit int) ([]Session, error) {
	var fb filterBuilder
	fb.hasFTS = s.hasFTS5
	useTurnStatsJoin := sort.Field != SortByCreated &&
		sort.Field != SortByTurns &&
		sort.Field != SortByName &&
		sort.Field != SortByFolder
	fb.expandDatePredicates = !useTurnStatsJoin
	if useTurnStatsJoin {
		fb.lastActiveExpr = lastActiveJoinedExpr
	}
	fb.apply(s.withAutoExclusions(filter))

	columns := s.sessionColumns()
	joins := ""
	sortExpr := sortColumn(sort.Field)
	if useTurnStatsJoin {
		columns = s.sessionColumnsJoined()
		joins = turnStatsJoin
		if sortExpr == lastActiveExpr {
			sortExpr = lastActiveJoinedExpr
		}
	}
	q := "SELECT " + columns + " FROM sessions s" + joins + fb.joinSQL() + fb.whereSQL()
	q += fmt.Sprintf(" ORDER BY %s %s", sortExpr, sortDir(sort.Order))

	if limit <= 0 {
		limit = defaultQueryLimit
	}
	q += limitClause
	fb.args = append(fb.args, limit)

	rows, err := s.db.QueryContext(ctx, q, fb.args...)
	if err != nil {
		return nil, fmt.Errorf("querying sessions: %w", err)
	}
	defer closeRows(rows)

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
//
// Callers may pass more IDs than SQLite's bound-variable limit allows in one
// statement, so the lookup is issued in batches of maxIDsPerQuery and the
// results merged. Truncating instead would silently drop the trailing IDs,
// which callers cannot distinguish from "those sessions no longer exist" —
// a large launch set would then report most of its members as missing.
func (s *Store) ListSessionsByIDs(ctx context.Context, ids []string) ([]Session, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	// Index results by ID for order-preserving assembly.
	byID := make(map[string]Session, len(ids))
	for start := 0; start < len(ids); start += maxIDsPerQuery {
		end := min(start+maxIDsPerQuery, len(ids))
		if err := s.appendSessionsBatch(ctx, ids[start:end], byID); err != nil {
			return nil, err
		}
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

// appendSessionsBatch runs one batched session lookup and merges the rows into
// byID. The batch must be no larger than maxIDsPerQuery.
func (s *Store) appendSessionsBatch(ctx context.Context, ids []string, byID map[string]Session) error {
	// Build "WHERE s.id IN (?,?,...)" with one placeholder per ID.
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	q := "SELECT " + s.sessionColumns() + " FROM sessions s WHERE s.id IN (" +
		strings.Join(placeholders, ",") + ")"

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("querying sessions by IDs: %w", err)
	}
	defer closeRows(rows)

	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return fmt.Errorf("scanning session row: %w", err)
		}
		byID[sess.ID] = sess
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating session rows: %w", err)
	}
	return nil
}

// SessionRefsBySessionIDs returns refs for the requested sessions, grouped by
// session ID. IDs not found in session_refs are omitted.
//
// Callers may pass more IDs than SQLite's bound-variable limit allows in one
// statement, so the lookup is issued in batches of maxIDsPerQuery and the
// results merged. Truncating instead would silently return no refs for the
// dropped sessions, which callers cannot distinguish from "this session has
// no refs" — related-session ranking would then quietly mis-rank every
// session past the cap.
func (s *Store) SessionRefsBySessionIDs(ctx context.Context, ids []string) (map[string][]SessionRef, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	refs := make(map[string][]SessionRef)
	for start := 0; start < len(ids); start += maxIDsPerQuery {
		end := min(start+maxIDsPerQuery, len(ids))
		if err := s.appendSessionRefsBatch(ctx, ids[start:end], refs); err != nil {
			return nil, err
		}
	}
	return refs, nil
}

// appendSessionRefsBatch runs one batched refs query and merges the rows into
// refs. The batch must be no larger than maxIDsPerQuery.
func (s *Store) appendSessionRefsBatch(ctx context.Context, ids []string, refs map[string][]SessionRef) error {
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	q := `SELECT session_id, COALESCE(ref_type,''), COALESCE(ref_value,''), turn_index, COALESCE(created_at,'')
		FROM session_refs WHERE session_id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY session_id, turn_index`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("querying session refs: %w", err)
	}
	defer closeRows(rows)

	for rows.Next() {
		var r SessionRef
		if err := rows.Scan(&r.SessionID, &r.RefType, &r.RefValue, &r.TurnIndex, &r.CreatedAt); err != nil {
			return fmt.Errorf("scanning session ref row: %w", err)
		}
		refs[r.SessionID] = append(refs[r.SessionID], r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating session refs: %w", err)
	}
	return nil
}

// AllSessionIDs returns the IDs of every session in the store, including
// "empty" sessions (no turns/files/checkpoints/refs) that ListSessions filters
// out. Callers that need to know whether a session still exists on disk — such
// as `dispatch prune`, which deletes config metadata for vanished sessions —
// must use this rather than ListSessions, otherwise metadata for real-but-empty
// sessions would be wrongly treated as stale.
func (s *Store) AllSessionIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM sessions")
	if err != nil {
		return nil, fmt.Errorf("querying session IDs: %w", err)
	}
	defer closeRows(rows)

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning session ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating session IDs: %w", err)
	}
	return ids, nil
}

// GetSession loads a single session and all of its related turns,
// checkpoints, files, and refs.
func (s *Store) GetSession(ctx context.Context, id string) (*SessionDetail, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+s.sessionColumns()+" FROM sessions s WHERE s.id = ?", id)
	sess, err := scanSession(row)
	if err != nil {
		return nil, fmt.Errorf("loading session %s: %w", id, err)
	}

	detail := &SessionDetail{Session: sess}

	// The four child queries are independent of each other; run them in
	// parallel to reduce wall-clock latency.
	var (
		turns       []Turn
		checkpoints []Checkpoint
		files       []SessionFile
		refs        []SessionRef
	)

	var eg errgroup.Group

	// Turns
	eg.Go(func() error {
		tRows, err := s.db.QueryContext(
			ctx,
			`SELECT session_id, turn_index, COALESCE(user_message,''), COALESCE(assistant_response,''), COALESCE(timestamp,'')
			 FROM turns WHERE session_id = ? ORDER BY turn_index`, id,
		)
		if err != nil {
			return fmt.Errorf("loading turns for session %s: %w", id, err)
		}
		defer closeRows(tRows)
		for tRows.Next() {
			var t Turn
			if err := tRows.Scan(&t.SessionID, &t.TurnIndex, &t.UserMessage, &t.AssistantResponse, &t.Timestamp); err != nil {
				return fmt.Errorf("scanning turn row: %w", err)
			}
			turns = append(turns, t)
		}
		return tRows.Err()
	})

	// Checkpoints
	eg.Go(func() error {
		cRows, err := s.db.QueryContext(
			ctx,
			`SELECT session_id, checkpoint_number, COALESCE(title,''), COALESCE(overview,''),
			        COALESCE(history,''), COALESCE(work_done,''), COALESCE(technical_details,''),
			        COALESCE(important_files,''), COALESCE(next_steps,'')
			 FROM checkpoints WHERE session_id = ? ORDER BY checkpoint_number`, id,
		)
		if err != nil {
			return fmt.Errorf("loading checkpoints for session %s: %w", id, err)
		}
		defer closeRows(cRows)
		for cRows.Next() {
			var c Checkpoint
			if err := cRows.Scan(&c.SessionID, &c.CheckpointNumber, &c.Title, &c.Overview,
				&c.History, &c.WorkDone, &c.TechnicalDetails, &c.ImportantFiles, &c.NextSteps); err != nil {
				return fmt.Errorf("scanning checkpoint row: %w", err)
			}
			checkpoints = append(checkpoints, c)
		}
		return cRows.Err()
	})

	// Files
	eg.Go(func() error {
		fRows, err := s.db.QueryContext(
			ctx,
			`SELECT session_id, COALESCE(file_path,''), COALESCE(tool_name,''), turn_index, COALESCE(first_seen_at,'')
			 FROM session_files WHERE session_id = ? ORDER BY turn_index, file_path`, id,
		)
		if err != nil {
			return fmt.Errorf("loading files for session %s: %w", id, err)
		}
		defer closeRows(fRows)
		for fRows.Next() {
			var f SessionFile
			if err := fRows.Scan(&f.SessionID, &f.FilePath, &f.ToolName, &f.TurnIndex, &f.FirstSeenAt); err != nil {
				return fmt.Errorf("scanning file row: %w", err)
			}
			files = append(files, f)
		}
		return fRows.Err()
	})

	// Refs
	eg.Go(func() error {
		rRows, err := s.db.QueryContext(
			ctx,
			`SELECT session_id, COALESCE(ref_type,''), COALESCE(ref_value,''), turn_index, COALESCE(created_at,'')
			 FROM session_refs WHERE session_id = ? ORDER BY turn_index`, id,
		)
		if err != nil {
			return fmt.Errorf("loading refs for session %s: %w", id, err)
		}
		defer closeRows(rRows)
		for rRows.Next() {
			var r SessionRef
			if err := rRows.Scan(&r.SessionID, &r.RefType, &r.RefValue, &r.TurnIndex, &r.CreatedAt); err != nil {
				return fmt.Errorf("scanning ref row: %w", err)
			}
			refs = append(refs, r)
		}
		return rRows.Err()
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	detail.Turns = turns
	detail.Checkpoints = checkpoints
	detail.Files = files
	detail.Refs = refs

	return detail, nil
}

// SearchSessions performs a fuzzy substring search across session metadata
// and turn content, returning matches ranked by source type. Sessions with
// zero turns are excluded. When FTS5 is available it is tried first for
// faster BM25-ranked results; on failure it falls back to LIKE.
func (s *Store) SearchSessions(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	// Try FTS5 path first (faster, BM25-ranked).
	if s.hasFTS5 {
		results, err := s.SearchSessionsFTS(ctx, query, limit)
		if err == nil && results != nil {
			// Merge in session_refs matches.
			refResults := s.searchRefs(ctx, query, limit)
			return mergeSearchResults(results, refResults, limit), nil
		}
		// FTS5 failed (syntax error, etc.) — fall through to LIKE.
	}

	results, err := s.searchSessionsLIKE(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	refResults := s.searchRefs(ctx, query, limit)
	return mergeSearchResults(results, refResults, limit), nil
}

// searchSessionsLIKE is the LIKE-based fallback for SearchSessions.
func (s *Store) searchSessionsLIKE(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	pattern := "%" + escapeLIKE(query) + "%"

	// Build automatic exclusion clauses (temp dir + hidden dotfolders).
	var autoClause string
	var extraArgs []any
	if s.tempDir != "" {
		autoClause += ` AND s2.cwd NOT LIKE ? ESCAPE '\'`
		extraArgs = append(extraArgs, escapeLIKE(s.tempDir)+"%")
	}
	if s.homeDir != "" {
		autoClause += ` AND s2.cwd NOT LIKE ? ESCAPE '\'`
		extraArgs = append(extraArgs, escapeLIKE(s.homeDir+string(filepath.Separator)+".")+"%")
	}

	q := `SELECT content, session_id, source_type, source_id, 0 AS rank FROM (
		SELECT COALESCE(s2.summary,'') AS content, s2.id AS session_id,
			'session' AS source_type, '' AS source_id
		FROM sessions s2
		WHERE (s2.summary LIKE ? ESCAPE '\' OR s2.branch LIKE ? ESCAPE '\' OR s2.repository LIKE ? ESCAPE '\')
			AND (EXISTS (SELECT 1 FROM turns t WHERE t.session_id = s2.id)
				OR EXISTS (SELECT 1 FROM session_files sf WHERE sf.session_id = s2.id)
				OR EXISTS (SELECT 1 FROM checkpoints cp WHERE cp.session_id = s2.id)
				OR EXISTS (SELECT 1 FROM session_refs sr WHERE sr.session_id = s2.id))` + autoClause + `
		UNION ALL
		SELECT COALESCE(t.user_message,'') AS content, t.session_id,
			'turn' AS source_type, CAST(t.turn_index AS TEXT) AS source_id
		FROM turns t
		JOIN sessions s2 ON s2.id = t.session_id
		WHERE t.user_message LIKE ? ESCAPE '\'` + autoClause + `
	) sub`
	args := []any{pattern, pattern, pattern} //nolint:prealloc // literal init is clearer than make+append
	args = append(args, extraArgs...)
	args = append(args, pattern)
	args = append(args, extraArgs...)

	if limit <= 0 {
		limit = defaultQueryLimit
	}
	q += limitClause
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("searching sessions: %w", err)
	}
	defer closeRows(rows)

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

// searchRefs returns SearchResult entries for sessions whose session_refs
// match the query. This catches PR numbers, issue numbers, and commit refs
// typed as bare numbers (e.g. "42" matches PR #42).
func (s *Store) searchRefs(ctx context.Context, query string, limit int) []SearchResult {
	// Extract numeric portion if present (strip leading # or "PR" prefix).
	refQuery := strings.TrimPrefix(strings.TrimPrefix(query, "#"), "PR")
	refQuery = strings.TrimPrefix(refQuery, "pr")
	refQuery = strings.TrimSpace(refQuery)
	if refQuery == "" {
		return nil
	}

	pattern := "%" + escapeLIKE(refQuery) + "%"
	q := `SELECT sr.ref_value, sr.session_id, sr.ref_type, CAST(sr.turn_index AS TEXT)
		  FROM session_refs sr
		  JOIN sessions s ON s.id = sr.session_id
		  WHERE sr.ref_value LIKE ? ESCAPE '\'`

	// Apply auto-exclusions.
	var args []any
	args = append(args, pattern)
	if s.tempDir != "" {
		q += ` AND s.cwd NOT LIKE ? ESCAPE '\'`
		args = append(args, escapeLIKE(s.tempDir)+"%")
	}
	if s.homeDir != "" {
		q += ` AND s.cwd NOT LIKE ? ESCAPE '\'`
		args = append(args, escapeLIKE(s.homeDir+string(filepath.Separator)+".")+"%")
	}
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	q += " LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		slog.Debug("searchRefs query failed", "error", err)
		return nil
	}
	defer closeRows(rows)

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.Content, &r.SessionID, &r.SourceType, &r.SourceID); err != nil {
			slog.Debug("searchRefs scan failed", "error", err)
			continue
		}
		r.Rank = -100 // boost ref matches to top
		results = append(results, r)
	}
	return results
}

// mergeSearchResults combines FTS5 results with ref results, deduplicating
// by session ID and respecting the limit.
func mergeSearchResults(primary, secondary []SearchResult, limit int) []SearchResult {
	if len(secondary) == 0 {
		return primary
	}
	seen := make(map[string]struct{}, len(primary))
	for _, r := range primary {
		seen[r.SessionID] = struct{}{}
	}
	// Prepend ref matches (they're boosted).
	merged := make([]SearchResult, 0, len(primary)+len(secondary))
	for _, r := range secondary {
		if _, ok := seen[r.SessionID]; !ok {
			merged = append(merged, r)
			seen[r.SessionID] = struct{}{}
		}
	}
	merged = append(merged, primary...)
	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}

// escapeFTS5 wraps each whitespace-separated term in double quotes so that
// FTS5 special characters (-, *, OR, AND, NOT, NEAR, etc.) are treated as
// literals. Returns the escaped query suitable for MATCH.
func escapeFTS5(query string) string {
	if terms := escapeFTS5Terms(query); terms != "" {
		return terms
	}
	return `""`
}

// escapeFTS5Terms is escapeFTS5 without the empty-query placeholder: it
// returns "" when the query has no terms, letting callers skip the MATCH
// clause entirely rather than issuing an empty phrase query.
func escapeFTS5Terms(query string) string {
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return ""
	}
	quoted := make([]string, len(terms))
	for i, t := range terms {
		// Escape any embedded double quotes by doubling them.
		quoted[i] = `"` + strings.ReplaceAll(t, `"`, `""`) + `"`
	}
	return strings.Join(quoted, " ")
}

// SearchSessionsFTS performs a full-text search using the FTS5 search_index
// virtual table maintained by the Copilot CLI. Results are ranked by BM25
// relevance. Returns nil, nil if the FTS5 table is not available.
func (s *Store) SearchSessionsFTS(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if !s.hasFTS5 {
		return nil, nil
	}

	escaped := escapeFTS5(query)

	// Build automatic exclusion clauses (temp dir + hidden dotfolders).
	var autoClause string
	var extraArgs []any
	if s.tempDir != "" {
		autoClause += ` AND ` + coalesceCwd + ` NOT LIKE ? ESCAPE '\'`
		extraArgs = append(extraArgs, escapeLIKE(s.tempDir)+"%")
	}
	if s.homeDir != "" {
		autoClause += ` AND ` + coalesceCwd + ` NOT LIKE ? ESCAPE '\'`
		extraArgs = append(extraArgs, escapeLIKE(s.homeDir+string(filepath.Separator)+".")+"%")
	}

	q := `SELECT si.content, si.session_id, si.source_type, si.source_id, si.rank
		FROM search_index si
		JOIN sessions s ON s.id = si.session_id
		WHERE si.content MATCH ?` + autoClause + `
		ORDER BY si.rank` + limitClause

	if limit <= 0 {
		limit = defaultQueryLimit
	}

	args := make([]any, 0, 1+len(extraArgs)+1)
	args = append(args, escaped)
	args = append(args, extraArgs...)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		// FTS5 MATCH can fail on malformed queries — return empty, not error.
		if isFTS5QueryError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("FTS5 search: %w", err)
	}
	defer closeRows(rows)

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.Content, &r.SessionID, &r.SourceType, &r.SourceID, &r.Rank); err != nil {
			return nil, fmt.Errorf("scanning FTS5 result: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating FTS5 results: %w", err)
	}
	return results, nil
}

// ListFolders returns the distinct cwd values across all sessions, sorted
// alphabetically.
func (s *Store) ListFolders(ctx context.Context) ([]string, error) {
	return s.distinctStrings(ctx, "SELECT DISTINCT COALESCE(cwd,'') FROM sessions WHERE cwd IS NOT NULL AND cwd != '' ORDER BY cwd")
}

// ListRepositories returns the distinct non-empty repository values across
// all sessions, sorted alphabetically.
func (s *Store) ListRepositories(ctx context.Context) ([]string, error) {
	return s.distinctStrings(ctx, "SELECT DISTINCT repository FROM sessions WHERE repository IS NOT NULL AND repository != '' ORDER BY repository")
}

// ListBranches returns distinct branch values. If repository is non-empty,
// results are filtered to that repository.
func (s *Store) ListBranches(ctx context.Context, repository string) ([]string, error) {
	if repository != "" {
		return s.distinctStrings(
			ctx,
			"SELECT DISTINCT branch FROM sessions WHERE branch IS NOT NULL AND branch != '' AND repository = ? ORDER BY branch",
			repository,
		)
	}
	return s.distinctStrings(ctx, "SELECT DISTINCT branch FROM sessions WHERE branch IS NOT NULL AND branch != '' ORDER BY branch")
}

// CountSessions returns the total number of sessions stored.
func (s *Store) CountSessions(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions").Scan(&n); err != nil {
		return 0, fmt.Errorf("counting sessions: %w", err)
	}
	return n, nil
}

// resolveIDPrefixLimit caps how many candidate IDs an ambiguous prefix lists
// so an error message stays readable when a short prefix matches many sessions.
const resolveIDPrefixLimit = 10

// AmbiguousIDPrefixError is returned by ResolveIDPrefix when a session ID
// prefix matches more than one stored session. Candidates lists the matching
// full IDs (capped) so the caller can tell the user how to disambiguate.
type AmbiguousIDPrefixError struct {
	Prefix     string
	Candidates []string
}

func (e *AmbiguousIDPrefixError) Error() string {
	return fmt.Sprintf("session ID prefix %q is ambiguous; add more characters to match one of: %s",
		e.Prefix, strings.Join(e.Candidates, ", "))
}

// ResolveIDPrefix expands a session ID or a unique ID prefix to a full session
// ID, mirroring git's short-SHA behavior. An exact ID match always wins, even
// when that value is also a prefix of longer IDs. With no exact match the value
// is treated as a prefix: a single match returns that ID, multiple matches
// return an *AmbiguousIDPrefixError listing the candidates, and no match
// returns sql.ErrNoRows. An empty prefix is rejected.
func (s *Store) ResolveIDPrefix(ctx context.Context, prefix string) (string, error) {
	if prefix == "" {
		return "", errors.New("session ID prefix is empty")
	}

	// An exact match short-circuits: a full ID beats any prefix collision.
	var exact string
	err := s.db.QueryRowContext(ctx, "SELECT id FROM sessions WHERE id = ?", prefix).Scan(&exact)
	switch {
	case err == nil:
		return exact, nil
	case !errors.Is(err, sql.ErrNoRows):
		return "", fmt.Errorf("looking up session %q: %w", prefix, err)
	}

	// No exact match: resolve as a prefix. Fetch one more than the display cap
	// so the candidate list stays bounded even for very short prefixes.
	pattern := escapeLIKE(prefix) + "%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM sessions WHERE id LIKE ? ESCAPE '\' ORDER BY id LIMIT ?`,
		pattern, resolveIDPrefixLimit+1)
	if err != nil {
		return "", fmt.Errorf("resolving session ID prefix %q: %w", prefix, err)
	}
	defer closeRows(rows)

	var candidates []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("scanning session ID: %w", err)
		}
		candidates = append(candidates, id)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("resolving session ID prefix %q: %w", prefix, err)
	}

	switch len(candidates) {
	case 0:
		return "", sql.ErrNoRows
	case 1:
		return candidates[0], nil
	default:
		return "", &AmbiguousIDPrefixError{Prefix: prefix, Candidates: candidates}
	}
}

// GroupSessions groups sessions by the specified pivot field, applying the
// given filter and sort order within each group.
func (s *Store) GroupSessions(ctx context.Context, pivot PivotField, filter FilterOptions, sort SortOptions, limit int) ([]SessionGroup, error) {
	var fb filterBuilder
	fb.hasFTS = s.hasFTS5
	fb.apply(s.withAutoExclusions(filter))

	expr := pivotExpr(pivot)
	// Older schemas (pre-v3) have no host_type column. Fall back to a single
	// empty-label group rather than issuing a query that would error.
	if pivot == PivotByHost && !s.hasHostType {
		expr = "''"
	}
	q := fmt.Sprintf("SELECT %s AS pivot_label, %s FROM sessions s%s%s ORDER BY pivot_label, %s %s",
		expr, s.sessionColumns(), fb.joinSQL(), fb.whereSQL(), sortColumn(sort.Field), sortDir(sort.Order))

	if limit <= 0 {
		limit = defaultGroupLimit
	}
	q += limitClause
	fb.args = append(fb.args, limit)

	rows, err := s.db.QueryContext(ctx, q, fb.args...)
	if err != nil {
		return nil, fmt.Errorf("querying grouped sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	groupMap := make(map[string]*SessionGroup)
	var order []string

	for rows.Next() {
		var label string
		sess, err := scanGroupedSession(rows, &label)
		if err != nil {
			return nil, fmt.Errorf("scanning grouped session row: %w", err)
		}
		// For date pivots the SQL returns a full RFC 3339 timestamp
		// (UTC). Convert to the local timezone before extracting the
		// calendar date so groups reflect the user's wall-clock day.
		if pivot == PivotByDate {
			if t, err := time.Parse(time.RFC3339, label); err == nil {
				label = t.Local().Format("2006-01-02")
			} else {
				// Fallback: if the value is already YYYY-MM-DD (10 chars) keep it.
				if len(label) > 10 {
					label = label[:10]
				}
			}
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

func (s *Store) distinctStrings(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
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
