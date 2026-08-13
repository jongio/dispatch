package tui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
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
	if m.state == stateGitStatusView {
		m.gitStatusView.SetSize(m.width, m.height)
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
		cmds = append(cmds, m.loadSessionsCmdWithIndicator(false))
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

// ----- Reference opened result ---------------------------------------------

// handleOpenRef opens the current session's most relevant linked reference (a
// pull request, then an issue, then a commit) in the browser.
func (m Model) handleOpenRef() (Model, tea.Cmd) {
	if m.detail == nil {
		m.statusErr = "No session selected"
		return m, clearStatusAfter(2 * time.Second)
	}
	ref, ok := data.BestRef(m.detail.Refs)
	if !ok {
		m.statusErr = "No linked PR, issue, or commit for this session"
		return m, clearStatusAfter(2 * time.Second)
	}
	repo := m.detail.Session.Repository
	if repo == "" {
		m.statusErr = "No repository recorded for this session"
		return m, clearStatusAfter(2 * time.Second)
	}
	url, ok := data.RefURL(repo, ref.RefType, ref.RefValue)
	if !ok {
		m.statusErr = "Cannot build a URL for " + ref.RefType + " " + ref.RefValue
		return m, clearStatusAfter(2 * time.Second)
	}
	label := ref.RefType + " " + ref.RefValue
	return m, m.openRefCmd(url, label)
}

func (m Model) handleRefOpened(msg refOpenedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.statusErr = msg.err.Error()
		return m, clearStatusAfter(2 * time.Second)
	}
	m.statusInfo = "Opened " + msg.label
	return m, clearStatusAfter(2 * time.Second)
}

// ----- Launch sets ---------------------------------------------------------

func (m *Model) openLaunchSetPicker() {
	m.launchSetPicker.SetLaunchSets(m.cfg.ValidLaunchSets(), m.launchSetExistingIDs())
	m.launchSetPicker.SetSize(m.width, m.height)
	m.launchSetPicker.CancelMode()
	m.state = stateLaunchSetPicker
}

func (m *Model) beginSaveLaunchSet() tea.Cmd {
	if len(m.selectedLaunchSetSessionIDs()) == 0 {
		m.statusErr = "Select sessions before saving a launch set"
		return clearStatusAfter(2 * time.Second)
	}
	m.launchSetPicker.SetLaunchSets(m.cfg.ValidLaunchSets(), m.launchSetExistingIDs())
	m.launchSetPicker.SetSize(m.width, m.height)
	m.state = stateLaunchSetPicker
	return m.launchSetPicker.BeginSave(fmt.Sprintf("Launch set %d", len(m.cfg.LaunchSets)+1))
}

func (m *Model) saveLaunchSet(name string) tea.Cmd {
	name = strings.TrimSpace(name)
	if name == "" {
		m.statusErr = "Launch set name is required"
		return clearStatusAfter(2 * time.Second)
	}
	if m.cfg.FindLaunchSet(name) != nil {
		m.statusErr = "Launch set already exists: " + name
		return clearStatusAfter(2 * time.Second)
	}
	ids := m.selectedLaunchSetSessionIDs()
	if len(ids) == 0 {
		m.statusErr = "Select sessions before saving a launch set"
		return clearStatusAfter(2 * time.Second)
	}
	m.cfg.LaunchSets = append(m.cfg.LaunchSets, config.LaunchSet{Name: name, SessionIDs: ids})
	m.saveConfig()
	m.launchSetPicker.CancelMode()
	m.openLaunchSetPicker()
	m.statusInfo = "Saved launch set " + name
	return clearStatusAfter(2 * time.Second)
}

func (m *Model) renameLaunchSet(name string) tea.Cmd {
	name = strings.TrimSpace(name)
	if name == "" {
		m.statusErr = "Launch set name is required"
		return clearStatusAfter(2 * time.Second)
	}
	set, ok := m.launchSetPicker.Selected()
	if !ok {
		return nil
	}
	if name != set.Name && m.cfg.FindLaunchSet(name) != nil {
		m.statusErr = "Launch set already exists: " + name
		return clearStatusAfter(2 * time.Second)
	}
	if existing := m.cfg.FindLaunchSet(set.Name); existing != nil {
		existing.Name = name
		m.saveConfig()
	}
	m.launchSetPicker.CancelMode()
	m.openLaunchSetPicker()
	m.statusInfo = "Renamed launch set " + name
	return clearStatusAfter(2 * time.Second)
}

func (m *Model) deleteLaunchSet() tea.Cmd {
	set, ok := m.launchSetPicker.Selected()
	if !ok {
		return nil
	}
	for i := range m.cfg.LaunchSets {
		if m.cfg.LaunchSets[i].Name == set.Name {
			m.cfg.LaunchSets = append(m.cfg.LaunchSets[:i], m.cfg.LaunchSets[i+1:]...)
			break
		}
	}
	m.saveConfig()
	m.launchSetPicker.CancelMode()
	m.openLaunchSetPicker()
	m.statusInfo = "Deleted launch set " + set.Name
	return clearStatusAfter(2 * time.Second)
}

func (m *Model) launchSelectedLaunchSet() tea.Cmd {
	set, ok := m.launchSetPicker.Selected()
	if !ok {
		return nil
	}
	sessions, missing := m.sessionsForLaunchSet(set)
	if len(sessions) == 0 {
		m.statusErr = "No saved sessions found for " + set.Name
		return clearStatusAfter(2 * time.Second)
	}
	mode := m.cfg.EffectiveLaunchMode()
	if mode == config.LaunchModeInPlace {
		mode = config.LaunchModeTab
	}
	m.state = stateSessionList
	cmd := m.batchLaunchSessions(sessions, mode)
	if missing > 0 {
		m.statusErr = fmt.Sprintf("Skipped %d missing saved session(s)", missing)
	}
	return cmd
}

func (m *Model) selectedLaunchSetSessionIDs() []string {
	selected := m.sessionList.SelectedSessions()
	ids := make([]string, 0, len(selected))
	for _, sess := range selected {
		ids = append(ids, sess.ID)
	}
	return ids
}

func (m *Model) launchSetExistingIDs() map[string]struct{} {
	ids := map[string]struct{}{}
	if m.store != nil {
		all, err := m.store.AllSessionIDs(context.Background())
		if err == nil {
			for _, id := range all {
				ids[id] = struct{}{}
			}
			return ids
		}
		m.statusErr = "launch sets: " + err.Error()
	}
	for _, sess := range m.sessions {
		ids[sess.ID] = struct{}{}
	}
	return ids
}

func (m *Model) sessionsForLaunchSet(set config.LaunchSet) ([]data.Session, int) {
	var sessions []data.Session
	if m.store != nil {
		loaded, err := m.store.ListSessionsByIDs(context.Background(), set.SessionIDs)
		if err == nil {
			sessions = loaded
		} else {
			m.statusErr = "launch sets: " + err.Error()
		}
	}
	if sessions == nil {
		byID := make(map[string]data.Session, len(m.sessions))
		for _, sess := range m.sessions {
			byID[sess.ID] = sess
		}
		for _, id := range set.SessionIDs {
			if sess, ok := byID[id]; ok {
				sessions = append(sessions, sess)
			}
		}
	}
	found := make(map[string]struct{}, len(sessions))
	for _, sess := range sessions {
		found[sess.ID] = struct{}{}
	}
	missing := 0
	for _, id := range set.SessionIDs {
		if _, ok := found[id]; !ok {
			missing++
		}
	}
	return sessions, missing
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
	if msg.version != m.sessionLoadVersion {
		return m, nil
	}
	m.finishSessionLoad()
	previousSessions := m.loadedSessions()
	prevID := m.selectedSessionID()
	m.sessions = m.applySessionFilters(msg.sessions)
	m.sortByAttention(m.sessions)
	m.groups = nil
	m.syncSessionListStatuses()
	m.refreshProjectQuickStarts()
	m.sessionList.SetSessionsWithQuickStarts(m.sessions, m.quickStarts)
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
	return m, tea.Batch(
		m.refreshSelectedDetailCmd(previousSessions),
		m.loadProjectReposCmd(),
	)
}

func (m Model) handleGroupsLoaded(msg groupsLoadedMsg) (Model, tea.Cmd) {
	if msg.version != m.sessionLoadVersion {
		return m, nil
	}
	m.finishSessionLoad()
	previousSessions := m.loadedSessions()
	prevID := m.selectedSessionID()
	m.groups = m.applyGroupFilters(msg.groups)
	for i := range m.groups {
		m.sortByAttention(m.groups[i].Sessions)
	}
	m.sessions = nil
	m.syncSessionListStatuses()
	m.sessionList.SetPivotField(m.pivot)
	m.refreshProjectQuickStarts()
	m.sessionList.SetGroupsWithQuickStarts(m.groups, m.quickStarts)
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
	return m, tea.Batch(
		m.refreshSelectedDetailCmd(previousSessions),
		m.loadProjectReposCmd(),
	)
}

func (m Model) handleProjectQuickStarts(msg projectQuickStartsMsg) (Model, tea.Cmd) {
	m.projectReposLoading = false
	if msg.err != nil {
		m.statusErr = "project scan: " + msg.err.Error()
		return m, clearStatusAfter(3 * time.Second)
	}
	m.projectRepos = msg.repos
	m.projectReposLoaded = true
	m.refreshProjectQuickStarts()
	if m.groups != nil {
		m.sessionList.SetPivotField(m.pivot)
		m.sessionList.SetGroupsWithQuickStarts(m.groups, m.quickStarts)
	} else {
		m.sessionList.SetSessionsWithQuickStarts(m.sessions, m.quickStarts)
	}
	m.searchBar.SetResultCount(m.sessionList.SessionCount())
	return m, nil
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
	m.preview.SetRelatedSessions(msg.related)
	// Set the user note for this session (if any).
	if m.cfg.SessionNotes != nil {
		m.preview.SetNote(m.cfg.SessionNotes[m.detail.Session.ID])
	} else {
		m.preview.SetNote("")
	}
	m.preview.SetAlias(m.cfg.AliasFor(m.detail.Session.ID))
	m.preview.SetAttentionStatus(m.attentionStatusForSession(m.detail.Session.ID))
	m.preview.SetLastEvent(data.LastSessionEvent(m.detail.Session.ID))
	m.syncPreviewWorkspaceMissing()
	m.syncPreviewGitStatus()
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

func (m Model) handleSessionLoadError(msg sessionLoadErrorMsg) (Model, tea.Cmd) { //nolint:unparam
	if msg.version != m.sessionLoadVersion {
		return m, nil
	}
	m.finishSessionLoad()
	if !errors.Is(msg.err, context.Canceled) {
		m.statusErr = "Data: " + msg.err.Error()
	}
	if m.state == stateLoading {
		m.state = stateSessionList
	}
	return m, nil
}

func (m *Model) finishSessionLoad() {
	m.sessionsLoading = false
	if m.sessionLoadCancel != nil {
		m.sessionLoadCancel()
		m.sessionLoadCancel = nil
	}
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
		if m.detail.Session.ID == msg.previewID {
			m.preview.SetLastEvent(msg.previewLastEvt)
		}
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
	// with the R key.
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
	// Write continuation plans from remaining items parsed from plan.md.
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
	m.gitStatusMap = msg.statuses
	// Derive the collapsed badge enum from the detailed statuses so existing
	// badge rendering and git-dirty / missing-workspace filters keep working.
	m.gitStateMap = make(map[string]platform.GitState, len(msg.statuses))
	for id, st := range msg.statuses {
		m.gitStateMap[id] = st.State()
	}
	m.sessionList.SetGitStates(m.gitStateMap)
	m.sessionList.SetGitStatuses(m.gitStatusMap)
	m.syncPreviewWorkspaceMissing()
	m.syncPreviewGitStatus()
	// When a git-state filter is active, reload sessions so the list
	// reflects the detected states.
	if m.filterGitDirty || m.filterMissingWorkspace {
		cmd := m.loadSessionsCmd()
		return m, cmd
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
		m.sessionList.SetSessionsWithQuickStarts(m.sessions, m.quickStarts)
	} else if msg.groups != nil {
		m.groups = m.applyGroupFilters(msg.groups)
		m.sessions = nil
		m.syncSessionListStatuses()
		m.sessionList.SetPivotField(m.pivot)
		m.sessionList.SetGroupsWithQuickStarts(m.groups, m.quickStarts)
	}
	if m.state == stateLoading {
		m.state = stateSessionList
	}
	m.searchBar.SetResultCount(m.sessionList.SessionCount())
	m.detailVersion++
	return m, m.loadSelectedDetailCmd()
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
