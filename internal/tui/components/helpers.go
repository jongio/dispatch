// Package components provides the sub-models (session list, search bar,
// filter panel, preview, help overlay, shell picker, reindex) that compose
// the Copilot CLI Session Browser TUI.
package components

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// emptyPlaceholder is the en-dash displayed when a value is missing or empty.
const emptyPlaceholder = "–"

// ---------------------------------------------------------------------------
// Text helpers shared by multiple components.
// ---------------------------------------------------------------------------

// Truncate returns s trimmed to at most width runes, appending "…" when
// truncation occurs.
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

// PadRight returns s padded with spaces on the right to exactly width runes.
// If s is longer than width it is truncated.
func PadRight(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) > width {
		return Truncate(s, width)
	}
	return s + strings.Repeat(" ", width-len(runes))
}

// PadLeft returns s padded with spaces on the left to exactly width runes.
func PadLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(runes)) + s
}

// CleanSummary strips chat-style prefixes (e.g. "[user]: ", "[assistant]: ")
// from a session summary, collapses whitespace to a single line, and returns
// a display-friendly string. This prevents raw conversation fragments stored
// by the Copilot CLI from appearing as "chat messages" in the session list.
func CleanSummary(s string) string {
	// Collapse to single line first.
	s = strings.Join(strings.Fields(s), " ")

	// Strip leading chat prefix: "[user]: ...", "[assistant]: ..."
	for _, prefix := range []string{"[user]: ", "[assistant]: ", "[user]:", "[assistant]:"} {
		if strings.HasPrefix(strings.ToLower(s), prefix) {
			s = strings.TrimSpace(s[len(prefix):])
			break
		}
	}

	// Remove any remaining "[assistant]: ..." tail after the user message.
	for _, marker := range []string{" [assistant]: ", " [assistant]:"} {
		if idx := strings.Index(strings.ToLower(s), marker); idx > 0 {
			s = strings.TrimSpace(s[:idx])
		}
	}

	if s == "" {
		return "(untitled)"
	}
	return s
}

// AbbrevPath returns the last two path components, prefixed with "…" and the
// OS path separator if the path is deeper.
func AbbrevPath(path string) string {
	if path == "" {
		return emptyPlaceholder
	}
	clean := filepath.FromSlash(path)
	parts := strings.Split(clean, string(filepath.Separator))
	// Remove empty trailing element (e.g. from trailing slash).
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) <= 2 {
		return clean
	}
	return "…" + string(filepath.Separator) + strings.Join(parts[len(parts)-2:], string(filepath.Separator))
}

// AbbrevHome returns the path with the user's home directory replaced by "~".
// Path separators are normalised to the OS-native format for display.
func AbbrevHome(path string) string {
	if path == "" {
		return emptyPlaceholder
	}
	clean := filepath.FromSlash(path)
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		if strings.HasPrefix(strings.ToLower(clean), strings.ToLower(home)) {
			return "~" + clean[len(home):]
		}
	}
	return clean
}

// SplitDirLeaf splits a path into its parent directory and leaf (last
// component). Both are normalised to OS-native separators.
func SplitDirLeaf(path string) (parent, leaf string) {
	clean := filepath.FromSlash(path)
	clean = strings.TrimRight(clean, string(filepath.Separator))
	idx := strings.LastIndex(clean, string(filepath.Separator))
	if idx < 0 {
		return "", clean
	}
	return clean[:idx], clean[idx+1:]
}

// RelativeTime parses a timestamp string and returns a human-friendly
// relative time such as "2h ago" or "3d ago".
func RelativeTime(timestamp string) string {
	if timestamp == "" {
		return emptyPlaceholder
	}
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05", timestamp)
		if err != nil {
			t, err = time.Parse("2006-01-02 15:04:05", timestamp)
			if err != nil {
				return emptyPlaceholder
			}
		}
	}

	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return formatDuration(d.Minutes(), "m") + " ago"
	case d < 24*time.Hour:
		return formatDuration(d.Hours(), "h") + " ago"
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return FormatInt(days) + "d ago"
	}
}

func formatDuration(value float64, unit string) string {
	v := int(value)
	if v <= 0 {
		v = 1
	}
	return FormatInt(v) + unit
}

// FormatInt formats an integer as a decimal string.
// It delegates to strconv.Itoa which handles all edge cases including
// math.MinInt (where negation overflows in two's complement).
func FormatInt(v int) string {
	return strconv.Itoa(v)
}
