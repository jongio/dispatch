package components

import (
	"strings"
	"testing"
	"time"

	"github.com/jongio/dispatch/internal/data"
)

func TestRecoveryView_RendersEvidenceAndConfirmation(t *testing.T) {
	view := NewRecoveryView()
	view.SetSize(100, 40)
	view.SetCandidates(map[string]data.RecoveryCandidate{
		"session-1": {
			SessionID:   "session-1",
			Workspace:   "C:\\code\\teko",
			PID:         1234,
			LastEvent:   data.SessionEvent{Type: "tool.execution_start"},
			InactiveFor: 31 * time.Minute,
			Status:      data.RecoveryLikelyStalled,
			Reason:      "No observed progress",
			CanRestart:  true,
		},
	})

	rendered := view.View()
	for _, want := range []string{"likely stalled", "session-1", "C:\\code\\teko", "tool.execution_start"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("view missing %q", want)
		}
	}
	if !view.BeginConfirmation() || !view.Confirming() {
		t.Fatal("restartable candidate should enter confirmation")
	}
	if rendered = view.View(); !strings.Contains(rendered, "repeat external side effects") {
		t.Fatal("confirmation must warn about duplicate side effects")
	}
}

func TestRecoveryView_BlockedCandidateCannotConfirm(t *testing.T) {
	view := NewRecoveryView()
	view.SetCandidates(map[string]data.RecoveryCandidate{
		"session-1": {
			SessionID:     "session-1",
			Status:        data.RecoverySuspectedStalled,
			BlockedReason: "identity ambiguous",
		},
	})
	if view.BeginConfirmation() {
		t.Fatal("blocked candidate must not enter confirmation")
	}
}

func TestRecoveryView_PreservesSelectionAcrossUpdates(t *testing.T) {
	view := NewRecoveryView()
	view.SetCandidates(map[string]data.RecoveryCandidate{
		"a": {SessionID: "a", Status: data.RecoveryInterrupted},
		"b": {SessionID: "b", Status: data.RecoveryInterrupted},
	})
	view.MoveDown()
	selected, _ := view.Selected()
	view.SetCandidates(map[string]data.RecoveryCandidate{
		"b": {SessionID: "b", Status: data.RecoveryInterrupted},
		"c": {SessionID: "c", Status: data.RecoveryInterrupted},
	})
	after, _ := view.Selected()
	if after.SessionID != selected.SessionID {
		t.Fatalf("selection changed from %q to %q", selected.SessionID, after.SessionID)
	}
}
