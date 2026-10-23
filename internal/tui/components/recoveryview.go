package components

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// RecoveryView renders evidence and actions for interrupted or stalled
// sessions independently of Chronicle-backed rows.
type RecoveryView struct {
	candidates []data.RecoveryCandidate
	cursor     int
	width      int
	height     int
	confirming bool
	busy       bool
	status     string
}

func NewRecoveryView() RecoveryView { return RecoveryView{} }

func (v *RecoveryView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

func (v *RecoveryView) SetCandidates(candidates map[string]data.RecoveryCandidate) {
	selectedID := ""
	if selected, ok := v.Selected(); ok {
		selectedID = selected.SessionID
	}
	v.candidates = v.candidates[:0]
	for _, candidate := range candidates {
		v.candidates = append(v.candidates, candidate)
	}
	slices.SortFunc(v.candidates, func(a, b data.RecoveryCandidate) int {
		if a.Status != b.Status {
			return int(b.Status - a.Status)
		}
		return strings.Compare(a.SessionID, b.SessionID)
	})
	v.cursor = min(v.cursor, max(0, len(v.candidates)-1))
	for i, candidate := range v.candidates {
		if candidate.SessionID == selectedID {
			v.cursor = i
			break
		}
	}
	if len(v.candidates) == 0 {
		v.confirming = false
		v.busy = false
	}
}

func (v *RecoveryView) MoveUp() {
	if !v.busy && !v.confirming && v.cursor > 0 {
		v.cursor--
	}
}

func (v *RecoveryView) MoveDown() {
	if !v.busy && !v.confirming && v.cursor < len(v.candidates)-1 {
		v.cursor++
	}
}

func (v RecoveryView) Selected() (data.RecoveryCandidate, bool) {
	if v.cursor < 0 || v.cursor >= len(v.candidates) {
		return data.RecoveryCandidate{}, false
	}
	return v.candidates[v.cursor], true
}

func (v *RecoveryView) BeginConfirmation() bool {
	candidate, ok := v.Selected()
	if !ok || (!candidate.CanResume && !candidate.CanRestart) || candidate.BlockedReason != "" {
		return false
	}
	v.confirming = true
	return true
}

func (v *RecoveryView) CancelConfirmation() { v.confirming = false }
func (v RecoveryView) Confirming() bool     { return v.confirming }
func (v RecoveryView) Busy() bool           { return v.busy }

func (v *RecoveryView) SetBusy(status string) {
	v.confirming = false
	v.busy = true
	v.status = status
}

func (v *RecoveryView) ClearBusy() {
	v.busy = false
	v.status = ""
}

func (v RecoveryView) View() string {
	title := styles.OverlayTitleStyle.Render("Session Recovery")
	var body strings.Builder
	body.WriteString(title)
	body.WriteString("\n")

	if len(v.candidates) == 0 {
		body.WriteString("\n")
		body.WriteString(styles.DimmedStyle.Render("No interrupted or stalled sessions found."))
		body.WriteString("\n\n")
		body.WriteString(styles.DimmedStyle.Render("Esc to close"))
		return v.place(body.String())
	}

	if v.busy {
		body.WriteString("\n\n")
		body.WriteString(v.status)
		body.WriteString("\n\n")
		body.WriteString(styles.DimmedStyle.Render("Dispatch will not resume until process exit is confirmed."))
		return v.place(body.String())
	}

	candidate, _ := v.Selected()
	if v.confirming {
		action := "resume"
		if candidate.CanRestart {
			action = "stop the verified process tree and resume"
		}
		body.WriteString("\n\n")
		body.WriteString(styles.ErrorStyle.Render("Confirm recovery"))
		body.WriteString("\n\n")
		body.WriteString(fmt.Sprintf("Session: %s\nWorkspace: %s\nPID: %d\nLast event: %s\nInactive: %s\n\n",
			candidate.SessionID, displayRecoveryValue(candidate.Workspace), candidate.PID,
			displayRecoveryValue(candidate.LastEvent.Type), formatRecoveryDuration(candidate.InactiveFor)))
		body.WriteString("This may repeat external side effects that completed without an event.\n")
		body.WriteString("A safe graceful signal may be unavailable; forceful stop is used only after the grace window.\n")
		body.WriteString(fmt.Sprintf("Press y to %s, or n/Esc to cancel.", action))
		return v.place(body.String())
	}

	body.WriteString("\n")
	for i, item := range v.candidates {
		indicator := "  "
		if i == v.cursor {
			indicator = "> "
		}
		line := fmt.Sprintf("%s%-18s %s", indicator, item.Status.String(), item.SessionID)
		if i == v.cursor {
			line = styles.SelectedStyle.Render(line)
		}
		body.WriteString(line)
		body.WriteString("\n")
	}

	body.WriteString("\n")
	body.WriteString(fmt.Sprintf("Workspace: %s\nPID: %d\nLast event: %s\nInactive: %s\nReason: %s\n",
		displayRecoveryValue(candidate.Workspace), candidate.PID, displayRecoveryValue(candidate.LastEvent.Type),
		formatRecoveryDuration(candidate.InactiveFor), candidate.Reason))
	if candidate.BlockedReason != "" {
		body.WriteString(styles.ErrorStyle.Render("Restart unavailable: " + candidate.BlockedReason))
		body.WriteString("\n")
	}
	body.WriteString("\n")
	body.WriteString(styles.DimmedStyle.Render("Up/Down select  Enter/r recover  c copy command  w wait  Esc close"))
	return v.place(body.String())
}

func (v RecoveryView) place(content string) string {
	width := min(82, max(42, v.width-4))
	overlay := styles.OverlayStyle.Width(width).Render(content)
	return lipgloss.Place(v.width, v.height, lipgloss.Center, lipgloss.Center, overlay)
}

func displayRecoveryValue(value string) string {
	if value == "" {
		return "(unknown)"
	}
	return value
}

func formatRecoveryDuration(duration time.Duration) string {
	if duration < time.Minute {
		return "less than a minute"
	}
	return duration.Round(time.Minute).String()
}
