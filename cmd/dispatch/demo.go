package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

// demoAttention describes a fake attention state for a demo session.
type demoAttention struct {
	sessionID   string
	eventType   string // last event type (determines waiting/active/stale)
	stale       bool   // if true, event timestamp is old (→ stale status)
	idle        bool   // if true, no lock file created (→ idle status)
	interrupted bool   // if true, lock file uses a dead PID (→ interrupted status)
}

// demoAttentionSessions defines which sessions get attention status
// circles in demo mode. Sessions not listed here appear with no dot.
var demoAttentionSessions = []demoAttention{
	// Waiting (purple) — assistant finished, waiting for user input.
	{sessionID: "fa800b7b-3a24-4e3b-9f2d-a414198b27ab", eventType: "assistant.turn_end"},
	{sessionID: "ses-004", eventType: "assistant.turn_end"},

	// Active (green) — AI is currently working.
	{sessionID: "ses-026", eventType: "tool.execution"},
	{sessionID: "ses-002", eventType: "tool.execution"},

	// Stale (yellow) — running but no recent activity.
	{sessionID: "ses-003", eventType: "tool.execution", stale: true},

	// Interrupted (orange ⚡) — dead PID lock file, recent tool.execution event.
	{sessionID: "ses-005", eventType: "tool.execution", interrupted: true},
	{sessionID: "ses-007", eventType: "tool.execution", interrupted: true},

	// Idle (gray) — session not running (no lock file).
	{sessionID: "ses-006", idle: true},
	{sessionID: "ses-027", idle: true},
}

// setupDemo prepares a fresh demo environment with timestamps relative
// to now and fake session-state directories for attention status circles.
// It returns a cleanup function that removes all temporary artifacts.
func setupDemo() (cleanup func(), err error) {
	srcDB := findDemoDB()
	if srcDB == "" {
		return nil, fmt.Errorf("demo db not found; set DISPATCH_DB or run from the repo root")
	}

	tmpDir, err := os.MkdirTemp("", "dispatch-demo-*")
	if err != nil {
		return nil, fmt.Errorf("demo temp dir: %w", err)
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(tmpDir)
		}
	}()

	// Copy the demo DB so we can modify timestamps without touching
	// the checked-in file.
	tmpDB := filepath.Join(tmpDir, "demo.db")
	if err := copyDemoDB(srcDB, tmpDB); err != nil {
		return nil, err
	}

	// Shift all timestamps so the newest session is recent.
	if err := shiftDemoTimestamps(tmpDB); err != nil {
		return nil, err
	}

	// Create fake session-state directories for attention circles.
	stateDir := filepath.Join(tmpDir, "session-state")
	if err := createDemoSessionState(stateDir); err != nil {
		return nil, err
	}

	// Write plan.md files for a few sessions so the plan indicator dot
	// is visible in demo mode and screenshots.
	if err := createDemoPlanFiles(stateDir); err != nil {
		return nil, err
	}

	_ = os.Setenv("DISPATCH_DB", tmpDB)
	_ = os.Setenv("DISPATCH_SESSION_STATE", stateDir)

	// Force workspace recovery on so interrupted session dots are visible
	// regardless of the user's config setting.
	_ = os.Setenv("DISPATCH_WORKSPACE_RECOVERY", "1")

	ok = true
	return func() { os.RemoveAll(tmpDir) }, nil
}

// copyDemoDB copies src to dst using a simple read/write.
func copyDemoDB(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("demo db open: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("demo db create: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("demo db copy: %w", err)
	}
	return out.Close()
}

// shiftDemoTimestamps adjusts every timestamp in the demo database so
// the most recent session updated_at is ~5 minutes ago.
func shiftDemoTimestamps(dbPath string) error {
	const targetAge = 5 * time.Minute

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("demo db shift open: %w", err)
	}
	defer db.Close()

	// Find the newest updated_at.
	var maxTS string
	ctx := context.Background()
	err = db.QueryRowContext(ctx, `SELECT MAX(updated_at) FROM sessions`).Scan(&maxTS)
	if err != nil {
		return fmt.Errorf("demo db max ts: %w", err)
	}

	newest, err := time.Parse(time.RFC3339, maxTS)
	if err != nil {
		return fmt.Errorf("demo db parse ts %q: %w", maxTS, err)
	}

	// delta shifts all timestamps forward so newest becomes now-targetAge.
	delta := time.Since(newest) - targetAge
	deltaSec := int(delta.Seconds())
	if deltaSec <= 0 {
		return nil // already fresh enough
	}

	// Use strftime to output RFC3339 format so string comparisons with
	// time.RFC3339-formatted filter values work correctly. SQLite's
	// datetime() outputs "YYYY-MM-DD HH:MM:SS" (space separator) which
	// breaks same-day comparisons against "YYYY-MM-DDTHH:MM:SSZ" because
	// ' ' < 'T' in ASCII.
	shift := strconv.Itoa(deltaSec)
	shiftExpr := func(col string) string {
		return "strftime('%Y-%m-%dT%H:%M:%SZ', " + col + ", '+" + shift + " seconds')"
	}

	// Update all timestamp columns in one transaction.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	updates := []string{
		`UPDATE sessions SET created_at = ` + shiftExpr("created_at") + `, updated_at = ` + shiftExpr("updated_at"),
		`UPDATE turns SET timestamp = ` + shiftExpr("timestamp"),
		`UPDATE checkpoints SET created_at = ` + shiftExpr("created_at"),
		`UPDATE session_files SET first_seen_at = ` + shiftExpr("first_seen_at"),
		`UPDATE session_refs SET created_at = ` + shiftExpr("created_at"),
	}

	for _, q := range updates {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("demo db shift: %w", err)
		}
	}

	return tx.Commit()
}

// createDemoSessionState builds a temporary session-state directory tree
// that the attention scanner will read. For each configured session it
// creates the expected lock file and events.jsonl.
func createDemoSessionState(stateDir string) error {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}

	pid := os.Getpid()
	now := time.Now().UTC()
	staleTime := now.Add(-10 * time.Minute) // old enough to exceed threshold

	// deadPID is a PID near the maxPID ceiling (4194304) that is almost
	// certainly not running. Using 99999 collided with live Windows
	// processes (e.g. OpenConsole), so we match the test convention.
	const deadPID = 4194000

	for _, s := range demoAttentionSessions {
		sessDir := filepath.Join(stateDir, s.sessionID)
		if err := os.MkdirAll(sessDir, 0o700); err != nil {
			return err
		}

		if s.idle {
			// Idle sessions have no lock file — the scanner will
			// classify them as idle automatically.
			continue
		}

		lockPID := pid
		if s.interrupted {
			lockPID = deadPID
		}

		// Create lock file. Live PIDs → active/stale; dead PIDs → interrupted.
		lockPath := filepath.Join(sessDir, "inuse."+strconv.Itoa(lockPID)+".lock")
		if err := os.WriteFile(lockPath, []byte(strconv.Itoa(lockPID)), 0o600); err != nil {
			return err
		}

		// Create events.jsonl with a single event line.
		evtTime := now
		if s.stale {
			evtTime = staleTime
		}
		eventLine := fmt.Sprintf(
			`{"type":"%s","timestamp":"%s"}`,
			s.eventType,
			evtTime.Format(time.RFC3339),
		)
		eventsPath := filepath.Join(sessDir, "events.jsonl")
		if err := os.WriteFile(eventsPath, []byte(eventLine+"\n"), 0o600); err != nil {
			return err
		}
	}

	return nil
}

// demoPlanSessions lists session IDs that get a plan.md file in demo mode.
// Chosen to overlap with interesting attention states (one waiting, one active,
// one stale, one interrupted) so the dot indicator is clearly visible.
var demoPlanSessions = []string{
	"fa800b7b-3a24-4e3b-9f2d-a414198b27ab", // Waiting (purple)
	"ses-026",                              // Active (green)
	"ses-003",                              // Stale (yellow)
	"ses-005",                              // Interrupted (orange ⚡)
}

// createDemoPlanFiles writes minimal plan.md files into session-state
// directories so the plan indicator dot appears in demo mode.
func createDemoPlanFiles(stateDir string) error {
	planContent := `# Implementation Plan

## Tasks
- [ ] Design API endpoints
- [ ] Implement database schema
- [x] Set up project structure
`

	for _, id := range demoPlanSessions {
		sessDir := filepath.Join(stateDir, id)
		if err := os.MkdirAll(sessDir, 0o700); err != nil {
			return err
		}
		planPath := filepath.Join(sessDir, "plan.md")
		if err := os.WriteFile(planPath, []byte(planContent), 0o600); err != nil {
			return err
		}
	}
	return nil
}
