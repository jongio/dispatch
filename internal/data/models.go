// Package data provides Go data models that map to the Copilot CLI
// session store SQLite database, along with types for filtering,
// sorting, and pivoting session queries.
package data

import "time"

// ---------------------------------------------------------------------------
// Core database models
// ---------------------------------------------------------------------------

// Session maps to the sessions table and carries computed aggregate counts
// populated at query time.
type Session struct {
	ID         string `json:"id"`
	Cwd        string `json:"cwd"`
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	Summary    string `json:"summary"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`

	// LastActiveAt is computed at query time as the MAX of the latest
	// turn timestamp, updated_at, and created_at — whichever is most
	// recent.  Indexed turns may lag (stale reindex) while updated_at
	// may be noisy, so taking the maximum gives the best estimate.
	LastActiveAt string `json:"last_active_at"`

	// Computed fields – populated by JOIN aggregates, not stored in the table.
	TurnCount int `json:"turn_count"`
	FileCount int `json:"file_count"`
}

// Turn maps to the turns table and represents a single conversational
// exchange within a session.
type Turn struct {
	SessionID         string `json:"session_id"`
	TurnIndex         int    `json:"turn_index"`
	UserMessage       string `json:"user_message"`
	AssistantResponse string `json:"assistant_response"`
	Timestamp         string `json:"timestamp"`
}

// Checkpoint maps to the checkpoints table and captures a point-in-time
// snapshot of session progress.
type Checkpoint struct {
	SessionID        string `json:"session_id"`
	CheckpointNumber int    `json:"checkpoint_number"`
	Title            string `json:"title"`
	Overview         string `json:"overview"`
	History          string `json:"history"`
	WorkDone         string `json:"work_done"`
	TechnicalDetails string `json:"technical_details"`
	ImportantFiles   string `json:"important_files"`
	NextSteps        string `json:"next_steps"`
}

// SessionFile maps to the session_files table and records a file touched
// during a session.
type SessionFile struct {
	SessionID   string `json:"session_id"`
	FilePath    string `json:"file_path"`
	ToolName    string `json:"tool_name"`
	TurnIndex   int    `json:"turn_index"`
	FirstSeenAt string `json:"first_seen_at"`
}

// SessionRef maps to the session_refs table and links a session to an
// external reference such as a commit, pull request, or issue.
type SessionRef struct {
	SessionID string `json:"session_id"`
	RefType   string `json:"ref_type"` // "commit", "pr", or "issue"
	RefValue  string `json:"ref_value"`
	TurnIndex int    `json:"turn_index"`
	CreatedAt string `json:"created_at"`
}

// SearchResult holds a single row returned from the FTS5 search_index
// virtual table, including the relevance rank.
type SearchResult struct {
	Content    string  `json:"content"`
	SessionID  string  `json:"session_id"`
	SourceType string  `json:"source_type"`
	SourceID   string  `json:"source_id"`
	Rank       float64 `json:"rank"`
}

// ---------------------------------------------------------------------------
// Attention status — computed from session-state directory scanning
// ---------------------------------------------------------------------------

// AttentionStatus indicates whether a session needs the user's attention.
// It is determined by scanning the Copilot CLI session-state directory
// for lock files and events.jsonl, not from the session-store.db.
type AttentionStatus int

const (
	// AttentionIdle means the session is not running (no lock file or PID dead).
	AttentionIdle AttentionStatus = iota
	// AttentionStale means the session is running but has no recent activity.
	AttentionStale
	// AttentionActive means the AI is currently working on a response.
	AttentionActive
	// AttentionWaiting means the AI has responded and is waiting for user input.
	AttentionWaiting
	// AttentionInterrupted means the session was interrupted by crash or reboot
	// (stale lock file with dead PID, last event not a turn-end).
	AttentionInterrupted
)

// String returns a human-readable label for the attention status.
func (a AttentionStatus) String() string {
	switch a {
	case AttentionWaiting:
		return "waiting"
	case AttentionActive:
		return "active"
	case AttentionStale:
		return "stale"
	case AttentionInterrupted:
		return "interrupted"
	default:
		return "idle"
	}
}

// ---------------------------------------------------------------------------
// Work status — computed by analyzing plan.md against code state
// ---------------------------------------------------------------------------

// WorkStatus indicates whether a session's planned work has been completed.
// It is determined by parsing the plan.md file and optionally cross-referencing
// against the session's git branch state.
type WorkStatus int

const (
	// WorkStatusUnknown means the session has not been analyzed yet.
	WorkStatusUnknown WorkStatus = iota
	// WorkStatusComplete means all planned work appears to be done.
	WorkStatusComplete
	// WorkStatusIncomplete means there are outstanding tasks remaining.
	WorkStatusIncomplete
	// WorkStatusNoPlan means the session has no plan.md file.
	WorkStatusNoPlan
	// WorkStatusAnalyzing means analysis is currently in progress.
	WorkStatusAnalyzing
	// WorkStatusError means the analysis failed.
	WorkStatusError
)

// String returns a human-readable label for the work status.
func (w WorkStatus) String() string {
	switch w {
	case WorkStatusComplete:
		return "complete"
	case WorkStatusIncomplete:
		return "incomplete"
	case WorkStatusNoPlan:
		return "no plan"
	case WorkStatusAnalyzing:
		return "analyzing"
	case WorkStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// WorkStatusResult holds the computed work status for a session along with
// detail about the tasks found in the plan.
type WorkStatusResult struct {
	Status         WorkStatus
	TotalTasks     int      // total tasks found in plan
	DoneTasks      int      // tasks marked as complete
	Detail         string   // human-readable summary (e.g., "3/7 tasks complete")
	RemainingItems []string // outstanding items from plan parsing or AI analysis (may be nil)
	Error          error    // non-nil when Status == WorkStatusError
}

// ---------------------------------------------------------------------------
// Filter, sort, and pivot types
// ---------------------------------------------------------------------------

// FilterOptions describes the criteria used to narrow session queries.
type FilterOptions struct {
	Query        string     `json:"query,omitempty"`
	Folder       string     `json:"folder,omitempty"`
	Repository   string     `json:"repository,omitempty"`
	Branch       string     `json:"branch,omitempty"`
	Since        *time.Time `json:"since,omitempty"`
	Until        *time.Time `json:"until,omitempty"`
	HasRefs      bool       `json:"has_refs,omitempty"`
	ExcludedDirs []string   `json:"excluded_dirs,omitempty"`

	// DeepSearch controls the breadth of the text search. When false
	// (default / quick mode), only session-level fields are searched
	// (summary, branch, repository, cwd). When true (deep mode), related
	// tables are also searched: turns.user_message, checkpoints.title,
	// checkpoints.overview, session_files.file_path, session_refs.ref_value.
	DeepSearch bool `json:"deep_search,omitempty"`
}

// SortField identifies which column to sort sessions by.
type SortField string

const (
	// SortByUpdated sorts sessions by last update time.
	SortByUpdated SortField = "updated_at"
	// SortByCreated sorts sessions by creation time.
	SortByCreated SortField = "created_at"
	// SortByTurns sorts sessions by conversation turn count.
	SortByTurns SortField = "turn_count"
	// SortByName sorts sessions alphabetically by summary.
	SortByName SortField = "summary"
	// SortByFolder sorts sessions alphabetically by working directory.
	SortByFolder SortField = "cwd"
	// SortByAttention sorts sessions by attention status priority
	// (Waiting > Active > Stale > Idle). This is applied post-load
	// in the TUI layer since attention status is computed at runtime.
	SortByAttention SortField = "attention"
)

// SortOrder indicates ascending or descending sort direction.
type SortOrder string

const (
	// Ascending sorts results from lowest to highest value.
	Ascending SortOrder = "ASC"
	// Descending sorts results from highest to lowest value.
	Descending SortOrder = "DESC"
)

// SortOptions combines a field and direction for ordering query results.
type SortOptions struct {
	Field SortField `json:"field"`
	Order SortOrder `json:"order"`
}

// PivotField identifies the dimension used to group sessions.
type PivotField string

const (
	// PivotByFolder groups sessions by their working directory.
	PivotByFolder PivotField = "cwd"
	// PivotByRepo groups sessions by repository name.
	PivotByRepo PivotField = "repository"
	// PivotByBranch groups sessions by git branch name.
	PivotByBranch PivotField = "branch"
	// PivotByDate groups sessions by date (YYYY-MM-DD).
	PivotByDate PivotField = "date"
)

// SessionGroup holds a set of sessions that share a common pivot label.
type SessionGroup struct {
	Label    string    `json:"label"`
	Sessions []Session `json:"sessions"`
	Count    int       `json:"count"`
}

// SessionDetail is an aggregated view that combines a session with all of
// its related turns, checkpoints, files, and external references.
type SessionDetail struct {
	Session     Session       `json:"session"`
	Turns       []Turn        `json:"turns"`
	Checkpoints []Checkpoint  `json:"checkpoints"`
	Files       []SessionFile `json:"files"`
	Refs        []SessionRef  `json:"refs"`
}
