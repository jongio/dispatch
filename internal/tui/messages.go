package tui

import (
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
)

// ---------------------------------------------------------------------------
// Async message types used by tea.Cmd functions in the root model.
// ---------------------------------------------------------------------------

// Store lifecycle.
type (
	storeOpenedMsg struct{ store *data.Store }
	storeErrorMsg  struct{ err error }
)

// Session data loading.
type (
	sessionsLoadedMsg struct{ sessions []data.Session }
	groupsLoadedMsg   struct{ groups []data.SessionGroup }
	sessionDetailMsg  struct {
		detail  *data.SessionDetail
		version int // matches Model.detailVersion to discard stale results
	}
)
type dataErrorMsg struct{ err error }

// Filter picker data.
type filterDataMsg struct {
	folders []string
}

// Shell detection.
type shellsDetectedMsg struct{ shells []platform.ShellInfo }

// Terminal detection.
type terminalsDetectedMsg struct{ terminals []platform.TerminalInfo }

// Session resume lifecycle — sent after tea.ExecProcess completes.
type sessionExitMsg struct{ err error }

// Font detection lifecycle.
type fontCheckMsg struct{ installed bool }

// Transient status clear.
type clearStatusMsg struct{}

// Pending click debounce — fires after doubleClickTimeout to execute a
// single-click action when no second click arrived.
type pendingClickFireMsg struct {
	version int
}

// Deep search debounce — carries the version counter so stale ticks are ignored.
type deepSearchTickMsg struct {
	version int
}

// Deep search results — carries version for staleness check.
type deepSearchResultMsg struct {
	version  int
	sessions []data.Session
	groups   []data.SessionGroup
}

// ---------------------------------------------------------------------------
// Copilot SDK search messages
// ---------------------------------------------------------------------------

// copilotReadyMsg is sent when the Copilot SDK client initialises successfully.
type copilotReadyMsg struct{}

// copilotErrorMsg is sent when the Copilot SDK fails to initialise or encounters an error.
type copilotErrorMsg struct{ err error }

// copilotSearchTickMsg fires after the debounce delay to start an SDK search.
type copilotSearchTickMsg struct {
	version int
}

// copilotSearchResultMsg delivers session IDs found by Copilot SDK search.
type copilotSearchResultMsg struct {
	version    int
	sessionIDs []string
	err        error
}

// aiSessionsLoadedMsg delivers sessions fetched by ID for AI-found results
// not already in the current list.
type aiSessionsLoadedMsg struct {
	version  int
	sessions []data.Session
}

// ---------------------------------------------------------------------------
// Attention scan messages
// ---------------------------------------------------------------------------

// attentionQuickScannedMsg delivers session attention statuses from the
// fast first-pass scanner (lock files only, no events.jsonl for dead sessions).
type attentionQuickScannedMsg struct {
	statuses map[string]data.AttentionStatus
}

// attentionScannedMsg delivers session attention statuses from the
// full session-state directory scanner (includes events.jsonl analysis).
type attentionScannedMsg struct {
	statuses map[string]data.AttentionStatus
}

// attentionTickMsg fires periodically to trigger the next attention scan.
type attentionTickMsg struct{}

// ---------------------------------------------------------------------------
// Plan scan messages
// ---------------------------------------------------------------------------

// plansScannedMsg delivers plan.md existence data from the data layer.
type plansScannedMsg struct {
	plans map[string]bool
}

// planContentMsg delivers the contents of a plan.md file for preview.
type planContentMsg struct {
	sessionID string
	content   string
	err       error
}
