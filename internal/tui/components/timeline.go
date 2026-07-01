package components

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// TimelineEntry represents a single timestamped event in the activity timeline.
type TimelineEntry struct {
	Time        time.Time
	Icon        string
	Description string
	Category    string // "turn", "checkpoint", "file", "ref"
}

// BuildTimeline merges turns, checkpoints, files, and refs from a session
// detail into a chronologically sorted list of timeline entries.
func BuildTimeline(detail *data.SessionDetail) []TimelineEntry {
	if detail == nil {
		return nil
	}

	var entries []TimelineEntry

	// Turns
	for _, t := range detail.Turns {
		ts := parseTimelineTimestamp(t.Timestamp)
		if ts.IsZero() {
			continue
		}
		msg := truncateMessage(t.UserMessage, 60)
		if msg == "" {
			msg = fmt.Sprintf("Turn %d", t.TurnIndex)
		}
		entries = append(entries, TimelineEntry{
			Time:        ts,
			Icon:        "💬",
			Description: msg,
			Category:    "turn",
		})
	}

	// Checkpoints
	for _, cp := range detail.Checkpoints {
		// Checkpoints don't have their own timestamp in the model, so we
		// approximate using the turn they are closest to. If no turns exist,
		// fall back to session created_at.
		ts := approximateCheckpointTime(cp, detail)
		title := cp.Title
		if title == "" {
			title = fmt.Sprintf("Checkpoint %d", cp.CheckpointNumber)
		}
		entries = append(entries, TimelineEntry{
			Time:        ts,
			Icon:        "📍",
			Description: title,
			Category:    "checkpoint",
		})
	}

	// Files
	for _, f := range detail.Files {
		ts := parseTimelineTimestamp(f.FirstSeenAt)
		if ts.IsZero() {
			continue
		}
		action := f.ToolName
		if action == "" {
			action = "touched"
		}
		entries = append(entries, TimelineEntry{
			Time:        ts,
			Icon:        "📄",
			Description: fmt.Sprintf("%s %s", action, abbreviateFilePath(f.FilePath)),
			Category:    "file",
		})
	}

	// Refs
	for _, r := range detail.Refs {
		ts := parseTimelineTimestamp(r.CreatedAt)
		if ts.IsZero() {
			continue
		}
		entries = append(entries, TimelineEntry{
			Time:        ts,
			Icon:        refIcon(r.RefType),
			Description: fmt.Sprintf("%s: %s", r.RefType, r.RefValue),
			Category:    "ref",
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Time.Before(entries[j].Time)
	})

	return entries
}

// RenderTimeline produces a styled string rendering of timeline entries
// suitable for display in the preview panel.
func RenderTimeline(entries []TimelineEntry, contentW int) string {
	if len(entries) == 0 {
		return lipgloss.Place(
			contentW, 3,
			lipgloss.Center, lipgloss.Center,
			styles.DimmedStyle.Render("No timeline data"),
		)
	}

	var b strings.Builder
	b.WriteString(styles.PreviewTitleStyle.Render("Activity Timeline") + "\n\n")

	for i, e := range entries {
		ts := e.Time.Local().Format("Jan 2 15:04")
		timeStr := styles.DimmedStyle.Render(ts)
		line := fmt.Sprintf("%s  %s %s", timeStr, e.Icon, e.Description)
		b.WriteString(line)
		if i < len(entries)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseTimelineTimestamp parses a timestamp string into a time.Time.
// Returns the zero time on failure.
func parseTimelineTimestamp(timestamp string) time.Time {
	if timestamp == "" {
		return time.Time{}
	}
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

// approximateCheckpointTime estimates a checkpoint's timestamp by looking at
// nearby turns. Uses the turn at checkpoint_number index if available,
// otherwise falls back to session created_at.
func approximateCheckpointTime(cp data.Checkpoint, detail *data.SessionDetail) time.Time {
	// Try to find a turn at or near the checkpoint number.
	if cp.CheckpointNumber > 0 && cp.CheckpointNumber <= len(detail.Turns) {
		ts := parseTimelineTimestamp(detail.Turns[cp.CheckpointNumber-1].Timestamp)
		if !ts.IsZero() {
			return ts
		}
	}
	// Fall back to session created_at.
	ts := parseTimelineTimestamp(detail.Session.CreatedAt)
	if !ts.IsZero() {
		return ts
	}
	return time.Time{}
}

// truncateMessage returns the first line of a message, truncated to maxLen.
func truncateMessage(msg string, maxLen int) string {
	if msg == "" {
		return ""
	}
	// Take first line only.
	if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
		msg = msg[:idx]
	}
	msg = strings.TrimSpace(msg)
	if len(msg) > maxLen {
		return msg[:maxLen-1] + "…"
	}
	return msg
}

// abbreviateFilePath shortens a file path to its last two components.
func abbreviateFilePath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 2 {
		return path
	}
	return ".../" + strings.Join(parts[len(parts)-2:], "/")
}

// refIcon returns an appropriate icon for a ref type.
func refIcon(refType string) string {
	switch refType {
	case "commit":
		return "🔨"
	case "pr":
		return "🔀"
	case "issue":
		return "📋"
	default:
		return "🔗"
	}
}
