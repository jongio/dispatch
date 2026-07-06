package tui

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/jongio/dispatch/internal/copilot"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/components"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// ---------------------------------------------------------------------------
// Handler methods — extracted from the Update switch for readability.
// Each returns (Model, tea.Cmd) matching the Bubble Tea update pattern.
//
// TODO(#113): Consider handler registry pattern to reduce switch complexity.
// See https://github.com/jongio/dispatch/issues/113 for full analysis.
// ---------------------------------------------------------------------------

// ----- Background color detection ------------------------------------------

func (m Model) handleBackgroundColor(msg tea.BackgroundColorMsg) (Model, tea.Cmd) { //nolint:unparam
	m.hasDarkBackground = msg.IsDark()
	// Re-apply the auto theme with the correct light/dark variant.
	themeName := m.cfg.Theme
	if themeName == "" || themeName == themeAuto {
		styles.ApplyAutoTheme(msg.IsDark())
	}
	return m, nil
}

// ----- Window resize -------------------------------------------------------

func (m Model) handleResize(msg tea.WindowSizeMsg) (Model, tea.Cmd) { //nolint:unparam
	m.width = msg.Width
	m.height = msg.Height
	m.recalcLayout()
	if m.state == stateCompareView {
		m.compareView.SetSize(m.width, m.height)
	}
	return m, nil
}

// ----- Spinner tick --------------------------------------------------------

func (m Model) handleSpinnerTick(msg spinner.TickMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// ----- Store lifecycle -----------------------------------------------------

func (m Model) handleStoreOpened(msg storeOpenedMsg) (Model, tea.Cmd) {
	m.store = msg.store
	m.state = stateSessionList
	// Apply a command-line search query before building the load command so
	// the first load is already filtered.
	var extra []tea.Cmd
	if m.initialQuery != "" {
		extra = m.applyInitialQuery(m.initialQuery)
		m.initialQuery = ""
	}
	// Quick scan first (lock files only), then full scan follows.
	cmds := append([]tea.Cmd{m.loadSessionsCmd(), m.scanAttentionQuickCmd()}, extra...)
	return m, tea.Batch(cmds...)
}

func (m Model) handleStoreError(msg storeErrorMsg) (Model, tea.Cmd) { //nolint:unparam
	m.statusErr = "Store: " + msg.err.Error()
	m.state = stateSessionList
	return m, nil
}

// ----- Reindex -------------------------------------------------------------

func (m Model) handleReindexLogPump(msg components.ReindexLogPump) (Model, tea.Cmd) {
	if !m.reindexing {
		return m, nil // Discard stale log pump after cancel.
	}
	m.reindexLog = append(m.reindexLog, msg.Lines...)
	// Cap log to prevent unbounded growth.
	if len(m.reindexLog) > maxReindexLogLines {
		m.reindexLog = m.reindexLog[len(m.reindexLog)-maxReindexLogLines:]
	}
	m.updateReindexViewport()
	return m, msg.NextLogCmd()
}

func (m Model) handleReindexFinished(msg components.ReindexFinishedMsg) (Model, tea.Cmd) {
	m.reindexing = false
	m.reindexCancel = nil
	if msg.Err != nil {
		if errors.Is(msg.Err, data.ErrIndexBusy) {
			m.statusErr = "Index busy — Copilot is rebuilding, try again shortly"
		} else if errors.Is(msg.Err, data.ErrReindexCancelled) {
			m.statusInfo = statusReindexCancelled
		} else {
			m.statusErr = "Rebuild index: " + msg.Err.Error()
		}
	} else {
		m.statusInfo = statusReindexDone
	}
	m.reindexLog = nil
	// Reload sessions to pick up changes from chronicle reindex,
	// and reset the DBWatcher baseline so it doesn't immediately
	// trigger a duplicate refresh.
	cmds := []tea.Cmd{clearStatusAfter(2 * time.Second)}
	if m.store != nil {
		cmds = append(cmds, m.loadSessionsCmd())
	}
	if m.dbWatcher != nil {
		m.dbWatcher.ResetBaseline()
	}
	return m, tea.Batch(cmds...)
}

// ----- DB watcher (external session store changes) -------------------------

func (m Model) handleSessionsChanged() (Model, tea.Cmd) {
	var cmds []tea.Cmd
	cmds = append(cmds, m.waitForDBChangeCmd()) // re-arm the listener
	if m.store != nil {
		cmds = append(cmds, m.loadSessionsCmd())
	}
	return m, tea.Batch(cmds...)
}

// ----- Transient status clear ----------------------------------------------

func (m Model) handleClearStatus() (Model, tea.Cmd) { //nolint:unparam
	m.statusInfo = ""
	m.statusErr = ""
	return m, nil
}

// ----- File opened result --------------------------------------------------

func (m Model) handleFileOpened(msg fileOpenedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.filePicker.SetWarning(msg.err.Error())
		return m, nil
	}
	m.filePicker.ClearWarning()
	m.statusInfo = "Opened " + msg.path
	return m, clearStatusAfter(2 * time.Second)
}

// ----- Directory opened result ---------------------------------------------

func (m Model) handleDirOpened(msg dirOpenedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.statusErr = msg.err.Error()
		return m, clearStatusAfter(2 * time.Second)
	}
	m.statusInfo = "Opened " + msg.path
	return m, clearStatusAfter(2 * time.Second)
}

// ----- Pending click fire (single-click debounce) --------------------------

func (m Model) handlePendingClickFire(msg pendingClickFireMsg) (Model, tea.Cmd) {
	if msg.version != m.click.pendingClickVersion {
		return m, nil // stale — a double-click already consumed this
	}
	// Timer fired — no second click arrived, so this is a single click.
	// Reset pending state so the next click isn't mistaken for a double.
	m.click.pendingClickVersion = 0
	// Normal click clears multi-selection (Windows Explorer behavior).
	if m.sessionList.SelectionCount() > 0 {
		m.sessionList.DeselectAll()
		m.statusInfo = ""
	}
	// Execute deferred single-click action.
	m.sessionList.MoveTo(m.click.pendingClickItemIdx)
	m.sessionList.SetAnchor()
	if m.sessionList.IsFolderSelected() {
		m.sessionList.ToggleFolder()
		return m, nil
	}
	m.detailVersion++
	return m, m.loadSelectedDetailCmd()
}

// ----- Session data loading ------------------------------------------------

func (m Model) handleSessionsLoaded(msg sessionsLoadedMsg) (Model, tea.Cmd) {
	prevID := m.selectedSessionID()
	m.sessions = m.applySessionFilters(msg.sessions)
	m.sortByAttention(m.sessions)
	m.groups = nil
	m.syncSessionListStatuses()
	m.sessionList.SetSessions(m.sessions)
	// Restore cursor to the previously selected session if possible.
	if prevID != "" {
		m.sessionList.SelectByID(prevID)
	}
	// Only transition from loading to session-list; never clobber an
	// active modal/overlay state with an async data load.
	if m.state == stateLoading {
		if m.cfg.DefaultCollapsed {
			m.sessionList.CollapseAll()
		}
		m.state = stateSessionList
	}
	m.searchBar.SetResultCount(m.sessionList.SessionCount())
	m.detailVersion++
	return m, tea.Batch(m.loadSelectedDetailCmd(), m.scanPlansCmd(), m.scanGitStatesCmd())
}

func (m Model) handleGroupsLoaded(msg groupsLoadedMsg) (Model, tea.Cmd) {
	prevID := m.selectedSessionID()
	m.groups = m.applyGroupFilters(msg.groups)
	for i := range m.groups {
		m.sortByAttention(m.groups[i].Sessions)
	}
	m.sessions = nil
	m.syncSessionListStatuses()
	m.sessionList.SetPivotField(m.pivot)
	m.sessionList.SetGroups(m.groups)
	if prevID != "" {
		m.sessionList.SelectByID(prevID)
	}
	if m.state == stateLoading {
		if m.cfg.DefaultCollapsed {
			m.sessionList.CollapseAll()
		}
		m.state = stateSessionList
	}
	m.searchBar.SetResultCount(m.sessionList.SessionCount())
	m.detailVersion++
	return m, tea.Batch(m.loadSelectedDetailCmd(), m.scanPlansCmd(), m.scanGitStatesCmd())
}

func (m Model) handleSessionDetail(msg sessionDetailMsg) (Model, tea.Cmd) {
	if msg.version != m.detailVersion {
		return m, nil // stale result — selection changed since request
	}
	// Detect whether this is a reload of the same session or a new session.
	previousID := ""
	if m.detail != nil {
		previousID = m.detail.Session.ID
	}
	m.detail = msg.detail
	m.preview.SetDetail(m.detail)
	// Set the user note for this session (if any).
	if m.cfg.SessionNotes != nil {
		m.preview.SetNote(m.cfg.SessionNotes[m.detail.Session.ID])
	} else {
		m.preview.SetNote("")
	}
	m.preview.SetAttentionStatus(m.attentionStatusForSession(m.detail.Session.ID))
	m.preview.SetHasPlan(m.planMap[m.detail.Session.ID])
	if result, ok := m.workStatus.workStatusMap[m.detail.Session.ID]; ok {
		m.preview.SetWorkStatus(result)
	} else {
		m.preview.SetWorkStatus(data.WorkStatusResult{})
	}
	// Only exit plan view when switching to a different session.
	// If the user pressed 'v' to view the plan, preserve that state
	// across detail reloads for the same session.
	if m.detail.Session.ID != previousID {
		m.preview.ExitPlanView()
	}
	if m.planMap[m.detail.Session.ID] {
		return m, m.loadPlanContentCmd(m.detail.Session.ID)
	}
	m.preview.SetPlanContent("")
	return m, nil
}

func (m Model) handleDataError(msg dataErrorMsg) (Model, tea.Cmd) { //nolint:unparam
	m.statusErr = "Data: " + msg.err.Error()
	if m.state == stateLoading {
		m.state = stateSessionList
	}
	return m, nil
}

// bellFn writes the terminal bell (BEL) character. It is a package variable so
// tests can swap it out to observe that the bell fired without touching stdout.
var bellFn = func() {
	fmt.Fprint(os.Stdout, "\a")
}

// bellCmd returns a command that rings the terminal bell.
func bellCmd() tea.Cmd {
	return func() tea.Msg {
		bellFn()
		return nil
	}
}

// ----- Attention scanning --------------------------------------------------

func (m Model) handleAttentionQuickScanned(msg attentionQuickScannedMsg) (Model, tea.Cmd) {
	m.attentionMap = msg.statuses
	m.sessionList.SetAttentionStatuses(m.attentionMap)
	// Quick scan done — immediately fire full (deep) scan.
	return m, m.scanAttentionCmd()
}

func (m Model) handleAttentionScanned(msg attentionScannedMsg) (Model, tea.Cmd) {
	m.attentionMap = msg.statuses
	m.sessionList.SetAttentionStatuses(m.attentionMap)
	// Update preview panel status if a session is selected.
	if m.detail != nil {
		m.preview.SetAttentionStatus(m.attentionStatusForSession(m.detail.Session.ID))
	}
	// Always schedule the next periodic scan. When the attention filter
	// is active, also reload sessions so the list reflects updated
	// statuses. The reload no longer fires another scan (that was an
	// infinite loop), so the tick is the sole driver of periodic scans.
	cmds := []tea.Cmd{m.scheduleAttentionTick(), m.scanPlansCmd(), m.scanGitStatesCmd()}
	if len(m.attentionFilter) > 0 {
		cmds = append(cmds, m.loadSessionsCmd())
	}
	// Ring the bell when a session newly enters the waiting state.
	if bell := m.notifyWaiting(msg.statuses); bell != nil {
		cmds = append(cmds, bell)
	}
	return m, tea.Batch(cmds...)
}

// notifyWaiting detects sessions that transitioned into the waiting state
// since the previous scan and, when the notify_on_waiting setting is enabled,
// rings the terminal bell once and sets a short footer message. The first scan
// after startup only records a baseline so sessions already waiting when
// dispatch launches do not trigger the bell. It returns a tea.Cmd that rings
// the bell (and clears the footer), or nil when nothing should be signalled.
func (m *Model) notifyWaiting(statuses map[string]data.AttentionStatus) tea.Cmd {
	newly := m.recordWaitingTransitions(statuses)

	// The first scan just establishes the baseline; never notify on it.
	if !m.attentionScanned {
		m.attentionScanned = true
		return nil
	}
	if newly == 0 || !m.cfg.NotifyOnWaiting {
		return nil
	}

	waiting := len(m.waitingNotified)
	if waiting == 1 {
		m.statusInfo = "1 session is waiting"
	} else {
		m.statusInfo = fmt.Sprintf("%d sessions are waiting", waiting)
	}
	return tea.Batch(bellCmd(), clearStatusAfter(4*time.Second))
}

// recordWaitingTransitions updates the set of sessions that have already
// triggered a waiting notification and returns how many sessions newly entered
// the waiting state. Sessions that leave the waiting state (or disappear) are
// dropped so a later re-entry notifies again.
func (m *Model) recordWaitingTransitions(statuses map[string]data.AttentionStatus) int {
	if m.waitingNotified == nil {
		m.waitingNotified = make(map[string]struct{})
	}
	newly := 0
	for id, st := range statuses {
		if st == data.AttentionWaiting {
			if _, seen := m.waitingNotified[id]; !seen {
				m.waitingNotified[id] = struct{}{}
				newly++
			}
		} else {
			delete(m.waitingNotified, id)
		}
	}
	// Forget sessions that are no longer reported at all.
	for id := range m.waitingNotified {
		if _, ok := statuses[id]; !ok {
			delete(m.waitingNotified, id)
		}
	}
	return newly
}

func (m Model) handleAttentionTick() (Model, tea.Cmd) {
	return m, m.scanAttentionCmd()
}

// ----- Plan scanning -------------------------------------------------------

func (m Model) handlePlansScanned(msg plansScannedMsg) (Model, tea.Cmd) {
	m.planMap = msg.plans
	m.sessionList.SetPlanStatuses(m.planMap)
	// When the plan filter is active, reload sessions so the list
	// reflects any newly discovered (or removed) plan.md files.
	var cmds []tea.Cmd
	if m.filterPlans {
		cmds = append(cmds, m.loadSessionsCmd())
	}
	// Update preview plan indicator and content if a session is selected.
	if m.detail != nil {
		m.preview.SetHasPlan(m.planMap[m.detail.Session.ID])
		if m.planMap[m.detail.Session.ID] {
			cmds = append(cmds, m.loadPlanContentCmd(m.detail.Session.ID))
		}
	}
	// Chain: after plans are known, do a quick work status classification
	// — but only when a work-status scan has been explicitly requested
	// (W key or reindex completion).
	if m.workStatus.workStatusScanning {
		cmds = append(cmds, m.scanWorkStatusQuickCmd())
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handlePlanContent(msg planContentMsg) (Model, tea.Cmd) { //nolint:unparam
	if msg.err != nil || msg.content == "" {
		m.preview.SetPlanContent("")
		m.workStatus.autoShowPlan = false
		return m, nil
	}
	// Only apply if the content matches the currently selected session.
	if m.detail != nil && m.detail.Session.ID == msg.sessionID {
		m.preview.SetPlanContent(msg.content)
		// After a work status scan with continuation plans, auto-switch
		// to plan view so the user sees the freshly written plan.
		if m.workStatus.autoShowPlan {
			m.workStatus.autoShowPlan = false
			m.preview.ShowPlanView()
		}
	}
	return m, nil
}

// ----- Work status scanning ------------------------------------------------

func (m Model) handleWorkStatusQuickScanned(msg workStatusQuickScannedMsg) (Model, tea.Cmd) {
	m.workStatus.workStatusMap = msg.statuses
	m.syncSessionListWorkStatuses()
	if sel, ok := m.sessionList.Selected(); ok {
		if result, exists := m.workStatus.workStatusMap[sel.ID]; exists {
			m.preview.SetWorkStatus(result)
		}
	}
	// Chain the full work status scan to parse plan.md content.
	return m, m.scanWorkStatusCmd()
}

func (m Model) handleWorkStatusScanned(msg workStatusScannedMsg) (Model, tea.Cmd) {
	// Merge full-scan results into the existing map so that NoPlan
	// entries from the quick scan are preserved (the full scan only
	// covers sessions with plans).
	if m.workStatus.workStatusMap == nil {
		m.workStatus.workStatusMap = msg.statuses
	} else {
		maps.Copy(m.workStatus.workStatusMap, msg.statuses)
	}
	m.syncSessionListWorkStatuses()
	if sel, ok := m.sessionList.Selected(); ok {
		if result, exists := m.workStatus.workStatusMap[sel.ID]; exists {
			m.preview.SetWorkStatus(result)
		}
	}
	// Chain to optional AI-enhanced analysis for sessions with
	// incomplete work. If the Copilot SDK is unavailable the command
	// returns nil and the chain stops here.
	aiCmd := m.scanWorkStatusAICmd()
	if aiCmd != nil {
		return m, aiCmd
	}
	// AI unavailable — write continuation plans from non-AI remaining
	// items (parsed from unchecked plan.md checkboxes).
	var sessionsWithRemaining []string
	for id, result := range m.workStatus.workStatusMap {
		if len(result.RemainingItems) > 0 {
			sessionsWithRemaining = append(sessionsWithRemaining, id)
		}
	}
	if len(sessionsWithRemaining) > 0 {
		if contCmd := m.writeContinuationPlansCmd(sessionsWithRemaining); contCmd != nil {
			return m, contCmd
		}
	}
	return m, m.completeWorkStatusScan()
}

func (m Model) handleWorkStatusAIScanned(msg workStatusAIScannedMsg) (Model, tea.Cmd) {
	// Merge AI analysis results into the work status map. The AI may
	// provide richer detail, updated task counts, and remaining items.
	var sessionsWithRemaining []string
	for id, analysis := range msg.analyses {
		if analysis == nil {
			continue
		}
		existing, ok := m.workStatus.workStatusMap[id]
		if !ok {
			continue
		}
		// Update detail with AI summary if provided.
		if analysis.Summary != "" {
			existing.Detail = analysis.Summary
		}
		// Trust AI task counts when they differ from local parse.
		if analysis.TotalTasks > 0 {
			existing.TotalTasks = analysis.TotalTasks
			existing.DoneTasks = analysis.CompletedTasks
		}
		// Overwrite status if the AI determined completion.
		if analysis.Complete && existing.Status == data.WorkStatusIncomplete {
			existing.Status = data.WorkStatusComplete
		}
		existing.RemainingItems = analysis.RemainingItems
		m.workStatus.workStatusMap[id] = existing

		// Only queue continuation plan writes for sessions that remain
		// incomplete after AI analysis — don't write "remaining work"
		// into plans the AI classified as complete.
		if existing.Status == data.WorkStatusIncomplete && len(analysis.RemainingItems) > 0 {
			sessionsWithRemaining = append(sessionsWithRemaining, id)
		}
	}
	// Also include sessions that were incomplete with local remaining
	// items but weren't in the AI results (AI failure/timeout/skipped).
	// This ensures continuation plans are written even when AI is partial.
	for id, result := range m.workStatus.workStatusMap {
		if _, hadAI := msg.analyses[id]; hadAI {
			continue // already handled above
		}
		if result.Status == data.WorkStatusIncomplete && len(result.RemainingItems) > 0 {
			sessionsWithRemaining = append(sessionsWithRemaining, id)
		}
	}
	m.syncSessionListWorkStatuses()
	if sel, ok := m.sessionList.Selected(); ok {
		if result, exists := m.workStatus.workStatusMap[sel.ID]; exists {
			m.preview.SetWorkStatus(result)
		}
	}
	// Chain: write continuation plans for sessions with remaining items.
	if len(sessionsWithRemaining) > 0 {
		contCmd := m.writeContinuationPlansCmd(sessionsWithRemaining)
		if contCmd != nil {
			return m, contCmd
		}
	}
	// Chain ends here — no continuation plans to write.
	return m, m.completeWorkStatusScan()
}

func (m Model) handleContinuationPlanCreated(msg continuationPlanCreatedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		slog.Debug("continuation plan write error", "error", msg.err)
	} else if msg.updated > 0 {
		// Check whether the currently selected session has remaining work
		// and reload its plan content so the preview shows the fresh plan.
		if m.detail != nil {
			if result, ok := m.workStatus.workStatusMap[m.detail.Session.ID]; ok && len(result.RemainingItems) > 0 {
				m.workStatus.autoShowPlan = true
				m.statusInfo = fmt.Sprintf("Updated %d plan(s) with remaining work — showing plan for current session", msg.updated)
				scanCmd := m.completeWorkStatusScan()
				reloadCmd := m.loadPlanContentCmd(m.detail.Session.ID)
				return m, tea.Batch(scanCmd, reloadCmd)
			}
		}
		m.statusInfo = fmt.Sprintf("Updated %d plan(s) with remaining work — press v on a session to view", msg.updated)
	}
	return m, m.completeWorkStatusScan()
}

// ----- Git workspace state scanning ----------------------------------------

func (m Model) handleGitStateScanned(msg gitStateScannedMsg) (Model, tea.Cmd) {
	m.gitStateMap = msg.states
	m.sessionList.SetGitStates(m.gitStateMap)
	// When the git-dirty filter is active, reload sessions so the list
	// reflects the detected states.
	if m.filterGitDirty {
		return m, m.loadSessionsCmd()
	}
	return m, nil
}

// ----- Deep search ---------------------------------------------------------

func (m Model) handleDeepSearchTick(msg deepSearchTickMsg) (Model, tea.Cmd) {
	if msg.version != m.search.deepSearchVersion || m.filter.Query == "" {
		return m, nil // stale tick — query changed since scheduling
	}
	return m, m.deepSearchCmd(msg.version)
}

func (m Model) handleDeepSearchResult(msg deepSearchResultMsg) (Model, tea.Cmd) {
	if msg.version != m.search.deepSearchVersion {
		return m, nil // stale result — query changed since search started
	}
	m.search.deepSearchPending = false
	m.filter.DeepSearch = true // keep deep mode for subsequent reloads (time range, sort, etc.)
	m.searchBar.SetSearching(false)
	if msg.sessions != nil {
		m.sessions = m.applySessionFilters(msg.sessions)
		m.groups = nil
		m.syncSessionListStatuses()
		m.sessionList.SetSessions(m.sessions)
	} else if msg.groups != nil {
		m.groups = m.applyGroupFilters(msg.groups)
		m.sessions = nil
		m.syncSessionListStatuses()
		m.sessionList.SetPivotField(m.pivot)
		m.sessionList.SetGroups(m.groups)
	}
	if m.state == stateLoading {
		m.state = stateSessionList
	}
	m.searchBar.SetResultCount(m.sessionList.SessionCount())
	m.detailVersion++
	return m, m.loadSelectedDetailCmd()
}

// ----- Copilot SDK search --------------------------------------------------

func (m Model) handleCopilotReady() (Model, tea.Cmd) {
	// Client is ready — no UI action needed; search will use it.
	return m, nil
}

func (m Model) handleCopilotError() (Model, tea.Cmd) {
	// SDK init/search error — silently ignore, search degrades gracefully.
	return m, nil
}

func (m Model) handleCopilotSearchTick(msg copilotSearchTickMsg) (Model, tea.Cmd) {
	if msg.version != m.search.copilotSearchVersion || m.filter.Query == "" {
		return m, nil // stale tick — query changed since scheduling
	}
	// If copilot client needs lazy init, do it here on the main goroutine
	// then kick off search.
	if m.copilotClient == nil && m.store != nil {
		m.copilotClient = copilot.New(m.store)
	}
	// Cancel any in-flight search so it releases the searchMu quickly
	// and doesn't block this new search for up to 45 seconds.
	if m.search.copilotSearchCancel != nil {
		m.search.copilotSearchCancel()
		m.search.copilotSearchCancel = nil
	}
	m.search.copilotSearching = true
	m.searchBar.SetAISearching(true)
	// Show "connecting" if SDK hasn't been initialised yet.
	if m.copilotClient != nil && !m.copilotClient.Available() {
		m.searchBar.SetAIStatus("connecting")
	}
	cmd := m.copilotSearchCmd(msg.version)
	return m, cmd
}

func (m Model) handleCopilotSearchResult(msg copilotSearchResultMsg) (Model, tea.Cmd) {
	if msg.version != m.search.copilotSearchVersion {
		// Stale result — a newer search is in flight. Don't update
		// status here; the newer search will set it when it completes.
		return m, nil
	}
	m.search.copilotSearching = false
	m.searchBar.SetAISearching(false)
	if msg.err != nil {
		// AI search is best-effort — show a brief "(✦ unavailable)"
		// indicator but never block or alarm the user.
		m.searchBar.SetAIStatus("unavailable")
		m.searchBar.SetAIResults(0)
		m.searchBar.SetAIError("")
		slog.Debug("copilot search failed", "error", msg.err)
		return m, nil
	}
	m.searchBar.SetAIStatus("ready")
	m.searchBar.SetAIResults(len(msg.sessionIDs))
	if len(msg.sessionIDs) == 0 {
		return m, nil
	}
	// Store AI session IDs.
	m.search.aiSessionIDs = make(map[string]struct{}, len(msg.sessionIDs))
	for _, id := range msg.sessionIDs {
		m.search.aiSessionIDs[id] = struct{}{}
	}
	m.sessionList.SetAISessions(m.search.aiSessionIDs)
	// Find IDs not already in the current session list and fetch them.
	missingIDs := m.findMissingAISessionIDs(msg.sessionIDs)
	if len(missingIDs) > 0 {
		return m, m.fetchAISessionsCmd(missingIDs, msg.version)
	}
	return m, nil
}

func (m Model) handleAISessionsLoaded(msg aiSessionsLoadedMsg) (Model, tea.Cmd) { //nolint:unparam
	if msg.version != m.search.copilotSearchVersion {
		return m, nil // stale
	}
	if len(msg.sessions) == 0 {
		return m, nil
	}
	// Apply full filter chain before merging (matches sessionsLoadedMsg).
	incoming := m.applySessionFilters(msg.sessions)
	if len(incoming) == 0 {
		return m, nil
	}
	// Only merge into flat session mode — skip if in groups/pivot mode.
	if m.sessions != nil {
		existing := make(map[string]struct{}, len(m.sessions))
		for _, s := range m.sessions {
			existing[s.ID] = struct{}{}
		}
		for _, s := range incoming {
			if _, ok := existing[s.ID]; !ok {
				m.sessions = append(m.sessions, s)
			}
		}
		m.sortByAttention(m.sessions)
		m.syncSessionListStatuses()
		m.sessionList.SetSessions(m.sessions)
		m.searchBar.SetResultCount(m.sessionList.SessionCount())
	}
	return m, nil
}

// ----- Filter picker data --------------------------------------------------

func (m Model) handleFilterData(msg filterDataMsg) (Model, tea.Cmd) { //nolint:unparam
	m.filterPanel.SetFolders(msg.folders, m.cfg.ExcludedDirs)
	return m, nil
}

// ----- Shell detection -----------------------------------------------------

func (m Model) handleShellsDetected(msg shellsDetectedMsg) (Model, tea.Cmd) { //nolint:unparam
	m.shells = msg.shells
	m.configPanel.SetShellOptions(m.shells)
	return m, nil
}

// ----- Terminal detection --------------------------------------------------

func (m Model) handleTerminalsDetected(msg terminalsDetectedMsg) (Model, tea.Cmd) { //nolint:unparam
	m.terminals = msg.terminals
	names := make([]string, 0, len(m.terminals))
	for _, t := range m.terminals {
		names = append(names, t.Name)
	}
	m.configPanel.SetTerminals(names)
	return m, nil
}

// ----- Font check ----------------------------------------------------------

func (m Model) handleFontCheck(msg fontCheckMsg) (Model, tea.Cmd) { //nolint:unparam
	styles.SetNerdFontEnabled(msg.installed)
	return m, nil
}

// ----- Session exit (in-place resume finished) -----------------------------

func (m Model) handleSessionExit(msg sessionExitMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.statusErr = fmt.Sprintf("Session failed: %v", msg.err)
		return m, nil
	}
	m.closeStore()
	return m, tea.Quit
}
