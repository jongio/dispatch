package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestWordWrap_LongTokenOverflow demonstrates the overflow: a spaceless token
// longer than the width must not produce a line wider than the width.
func TestWordWrap_LongTokenOverflow(t *testing.T) {
	long := `C:\Users\jong\.copilot\skills\devx-pr-create\references\pr-drafts.md`
	out := wordWrap(long, 30)
	for _, line := range strings.Split(out, "\n") {
		if w := len([]rune(line)); w > 30 {
			t.Fatalf("wrapped line width %d exceeds 30: %q", w, line)
		}
	}
}

// TestRenderChatBubble_NoOverflowOnLongPath verifies a bubble never renders a
// visual line wider than the content width when the message has a long path.
func TestRenderChatBubble_NoOverflowOnLongPath(t *testing.T) {
	msg := "Related files:\n\n" +
		`C:\Users\jong\.copilot\skills\devx-pr-create\references\pr-drafts.md`
	contentWidth := 40
	out := RenderChatBubble(msg, "You", contentWidth, true)
	for _, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > contentWidth {
			t.Fatalf("bubble line visual width %d exceeds contentWidth %d: %q", w, contentWidth, ansi.Strip(line))
		}
	}
}
