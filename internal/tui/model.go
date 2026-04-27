// Package tui implements the Bubble Tea terminal user interface for
// browsing and launching Copilot CLI sessions.
package tui

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/atotto/clipboard"
	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/copilot"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/tui/components"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// ---------------------------------------------------------------------------
// Package-level configuration
// ---------------------------------------------------------------------------

// Version is set at build time via ldflags. Defaults to "dev" for
// development builds.
var Version = "dev"

// doubleClickTimeout is the maximum interval between two mouse clicks at the
// same Y position for them to be treated as a double-click.
const doubleClickTimeout = 300 * time.Millisecond

// themeAuto is the reserved theme name for terminal-adaptive colours.
const themeAuto = "auto"

const (
	// copilotSearchTimeout limits a single Copilot AI search request.
	// Must be generous: Init ~1s + Search ~10s + retries (3 × [0.5s + 1s + 10s]) ≈ 45s.
	copilotSearchTimeout = 45 * time.Second

	// statusReindexDone is the status message shown when a reindex
	// completes successfully.
	statusReindexDone = "Reindexed ✓"

	// statusCopiedID is the status message shown when a session ID is
	// copied to the clipboard.
	statusCopiedID = "Copied session ID ✓"

	// statusCopiedPreview is the status message shown when preview content
	// is copied to the clipboard via the y key.
	statusCopiedPreview = "Copied to clipboard ✓"

	// statusCopiedSelection is the status message shown when a mouse
	// text selection from the preview pane is copied to the clipboard.
	statusCopiedSelection = "Copied selection ✓"

	// statusReindexCancelled is the status message shown when the user
	// cancels an in-flight reindex operation.
	statusReindexCancelled = "Reindex cancelled"

	// cancelBtnLabel is the button label shown in overlays that can be
	// dismissed with Escape (reindex, etc.).
	cancelBtnLabel = "[ Cancel (esc) ]"

	// headerRightReserve is the default column reserve on the right side
	// of the header row (accounts for a potential trailing space or cursor).
	headerRightReserve = 4

	// headerReindexReserve is the wider column reserve when the reindex
	// spinner is active (" ⣾ Reindexing…" ≈ 15 chars + padding).
	headerReindexReserve = 16

	// headerScanReserve is the column reserve when the work-status
	// scan spinner is active ("⣾ Scanning work status…" ≈ 25 chars).
	headerScanReserve = 28

	// minSearchBarWidth is the minimum width for the search bar to remain
	// usable when the header is cramped.
	minSearchBarWidth = 15

	// footerGapReserve is the minimum column count reserved for spacing
	// between the footer's left content, right hints, and version label.
	footerGapReserve = 4

	// gapWidth is the number of columns between the session list and the
	// preview panel.
	gapWidth = 2

	// overlayBorderPadding is the total horizontal overhead added by the
	// OverlayStyle (border 1 + padding 2 on each side = 6).
	overlayBorderPadding = 6
)

// timeRanges defines the time-range filter options shown in the header.
var timeRanges = []struct{ key, label string }{
	{"1", "1h"}, {"2", "1d"}, {"3", "7d"}, {"4", "all"},
}

// clipboardWrite is the function used to write text to the system clipboard.
// It is a package-level variable so tests can substitute a fake.
var clipboardWrite = clipboard.WriteAll

// ---------------------------------------------------------------------------
// Application states
// ---------------------------------------------------------------------------

type appState int

const (
	stateLoading         appState = iota
	stateSessionList              // main view
	stateFilterPanel              // filter overlay open
	stateHelpOverlay              // help modal open
	stateShellPicker              // shell selection overlay
	stateConfigPanel              // settings overlay
	stateAttentionPicker          // attention status filter overlay
)

// Pivot mode constants used by Model.pivot to control session grouping.
const (
	pivotNone   = "none"
	pivotFolder = "folder"
	pivotRepo   = "repo"
	pivotBranch = "branch"
	pivotDate   = "date"
)

// ---------------------------------------------------------------------------
// Root model
// ---------------------------------------------------------------------------

// Model is the top-level Bubble Tea model for the Session Browser TUI.
type Model struct {
	// Current UI state.
	state  appState
	width  int
	height int
	layout layout

	// hasDarkBackground is set by tea.BackgroundColorMsg on startup.
	// Used to select the correct light/dark color variant for the "auto" theme.
	hasDarkBackground bool

	// Data layer.
	store *data.Store
	cfg   *config.Config

	// Query parameters.
	filter     data.FilterOptions
	sort       data.SortOptions
	timeRange  string         // "1h", "1d", "7d", "all"
	pivot      string         // "none", "folder", "repo", "branch", "date"
	pivotOrder data.SortOrder // group header sort direction

	// Loaded data.
	sessions []data.Session
	groups   []data.SessionGroup
	detail   *data.SessionDetail

	// Detected shells and terminals for launch flow.
	shells    []platform.ShellInfo
	terminals []platform.TerminalInfo

	// Sub-components.
	sessionList     components.SessionList
	searchBar       components.SearchBar
	filterPanel     components.FilterPanel
	preview         components.PreviewPanel
	help            components.HelpOverlay
	shellPicker     components.ShellPicker
	configPanel     components.ConfigPanel
	attentionPicker components.AttentionPicker
	spinner         spinner.Model

	// UI toggles.
	showPreview     bool
	previewPosition string // "right", "bottom", "left", "top"
	showHidden      bool
	hiddenSet       map[string]struct{} // session ID → struct{} for fast hidden-session lookup
	favoritedSet    map[string]struct{} // session ID → struct{} for fast favorited-session lookup
	showFavorited   bool
	reindexing      bool
	reindexLog      []string                  // log lines streamed from chronicle reindex
	reindexVP       viewport.Model            // scrollable viewport for reindex overlay
	reindexCancel   *components.ReindexHandle // cancel handle for running reindex

	// Click debounce: delays single-click action so double-click can be
	// detected without the first click firing prematurely.
	pendingClickVersion int
	pendingClickY       int
	pendingClickItemIdx int

	// Launch mode requested when showing the shell picker.
	pendingLaunchMode string

	// Deep search debounce tracking.
	deepSearchVersion int
	deepSearchPending bool

	// Detail loading version — incremented on each loadSelectedDetailCmd
	// call to discard stale results from slower prior queries.
	detailVersion int

	// Transient status bar messages.
	statusErr  string
	statusInfo string

	// Copilot SDK search.
	copilotClient        *copilot.Client
	copilotSearchVersion int                 // version counter for SDK search debounce
	copilotSearching     bool                // true when SDK search is in progress
	copilotSearchCancel  context.CancelFunc  // cancels the in-flight SDK search
	aiSessionIDs         map[string]struct{} // session IDs found by SDK search

	// Attention status tracking — scanned from session-state directories.
	attentionMap    map[string]data.AttentionStatus
	attentionFilter map[data.AttentionStatus]struct{} // when non-empty, only show sessions with matching status

	// Plan status tracking — scanned from session-state directories.
	planMap     map[string]bool
	filterPlans bool // when true, only show sessions with a plan.md file

	// Work status tracking — computed from plan.md analysis.
	workStatusMap      map[string]data.WorkStatusResult
	filterWorkStatus   map[data.WorkStatus]struct{} // when non-empty, only show sessions with matching work status
	workStatusScanned  bool                         // true after first successful work status scan
	workStatusScanning bool                         // true while work status scan chain is in progress
	workStatusAICancel context.CancelFunc           // cancels in-flight AI work status analysis
	autoShowPlan       bool                         // when true, auto-switch to plan view on next planContentMsg
}

// NewModel creates the root Model with default configuration.
func NewModel() Model {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = config.Default()
	}

	// ── Theme resolution ────────────────────────────────────────────
	resolveTheme(cfg)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.SpinnerStyle

	cp := components.NewConfigPanel()
	cp.SetValues(components.ConfigValues{
		YoloMode:          cfg.YoloMode,
		Agent:             cfg.Agent,
		Model:             cfg.Model,
		LaunchMode:        cfg.EffectiveLaunchMode(),
		Terminal:          cfg.DefaultTerminal,
		Shell:             cfg.DefaultShell,
		CustomCommand:     cfg.CustomCommand,
		Theme:             cfg.Theme,
		WorkspaceRecovery: cfg.WorkspaceRecovery,
		PreviewPosition:   cfg.EffectivePreviewPosition(),
	})

	// Build the list of available theme names for the config panel.
	themeNames := make([]string, 0, 1+len(styles.BuiltinSchemeNames())+len(cfg.Schemes))
	themeNames = append(themeNames, themeAuto)
	themeNames = append(themeNames, styles.BuiltinSchemeNames()...)
	for _, cs := range cfg.Schemes {
		themeNames = append(themeNames, cs.Name)
	}
	cp.SetThemeOptions(themeNames)

	hiddenSet := make(map[string]struct{}, len(cfg.HiddenSessions))
	for _, id := range cfg.HiddenSessions {
		hiddenSet[id] = struct{}{}
	}

	favoritedSet := make(map[string]struct{}, len(cfg.FavoriteSessions))
	for _, id := range cfg.FavoriteSessions {
		favoritedSet[id] = struct{}{}
	}

	m := Model{
		state: stateLoading,
		cfg:   cfg,

		sort: data.SortOptions{
			Field: sortFieldFromConfig(cfg.DefaultSort),
			Order: data.Descending,
		},
		timeRange:       cfg.DefaultTimeRange,
		pivot:           cfg.DefaultPivot,
		showPreview:     cfg.ShowPreview,
		previewPosition: cfg.EffectivePreviewPosition(),
		hiddenSet:       hiddenSet,
		favoritedSet:    favoritedSet,

		sessionList:     components.NewSessionList(),
		searchBar:       components.NewSearchBar(),
		filterPanel:     components.NewFilterPanel(),
		preview:         components.NewPreviewPanel(),
		help:            components.NewHelpOverlay(),
		shellPicker:     components.NewShellPicker(),
		configPanel:     cp,
		spinner:         s,
		attentionPicker: components.NewAttentionPicker(),
		attentionFilter: make(map[data.AttentionStatus]struct{}),
	}

	m.filter.Since = timeRangeToSince(m.timeRange)
	m.filter.ExcludedDirs = cfg.ExcludedDirs
	m.preview.SetConversationSort(cfg.ConversationNewestFirst)
	return m
}

// resolveTheme applies a user-chosen color scheme.
//
// When the config field is empty or "auto" we keep the legacy
// defaults from styles.init().  The correct light/dark variant
// is applied later when tea.BackgroundColorMsg is received
// (see the Update handler).
//
// Only when the user explicitly names a scheme (built-in or
// user-defined) do we derive a fixed-hex theme and apply it.
func resolveTheme(cfg *config.Config) {
	themeName := cfg.Theme
	if themeName == "" || themeName == themeAuto {
		// Keep the legacy adaptive-color defaults from
		// styles.init() — no override needed.
		return
	}

	// ── Explicit scheme name ────────────────────────────────────────
	// Check user-defined schemes first.
	for _, cs := range cfg.Schemes {
		if cs.Name == themeName {
			if cs.Validate() == nil {
				styles.SetTheme(styles.DeriveTheme(cs))
				return
			}
		}
	}
	// Check built-in schemes.
	if cs, ok := styles.BuiltinSchemes[themeName]; ok {
		styles.SetTheme(styles.DeriveTheme(cs))
		return
	}
	// Unknown name — keep legacy defaults.
}

// ---------------------------------------------------------------------------
// tea.Model interface
// ---------------------------------------------------------------------------

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		openStoreCmd(),
		detectShellsCmd(),
		detectTerminalsCmd(),
		checkNerdFontCmd(),
		m.spinner.Tick,
		tea.RequestBackgroundColor,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ----- Background color detection --------------------------------------
	case tea.BackgroundColorMsg:
		m.hasDarkBackground = msg.IsDark()
		// Re-apply the auto theme with the correct light/dark variant.
		themeName := m.cfg.Theme
		if themeName == "" || themeName == themeAuto {
			styles.ApplyAutoTheme(msg.IsDark())
		}
		return m, nil

	// ----- Window resize ---------------------------------------------------
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		return m, nil

	// ----- Spinner tick ----------------------------------------------------
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	// ----- Store lifecycle -------------------------------------------------
	case storeOpenedMsg:
		m.store = msg.store
		m.state = stateSessionList
		// Quick scan first (lock files only), then full scan follows.
		return m, tea.Batch(m.loadSessionsCmd(), m.scanAttentionQuickCmd())

	case storeErrorMsg:
		m.statusErr = "Store: " + msg.err.Error()
		m.state = stateSessionList
		return m, nil

	// ----- Reindex ---------------------------------------------------------
	case components.ReindexLogPump:
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

	case components.ReindexFinishedMsg:
		m.reindexing = false
		m.reindexCancel = nil
		if msg.Err != nil {
			if errors.Is(msg.Err, data.ErrIndexBusy) {
				m.statusErr = "Index busy — Copilot is reindexing, try again shortly"
			} else if errors.Is(msg.Err, data.ErrReindexCancelled) {
				m.statusInfo = statusReindexCancelled
			} else {
				m.statusErr = "Reindex: " + msg.Err.Error()
			}
		} else {
			m.statusInfo = statusReindexDone
		}
		m.reindexLog = nil
		// Reload sessions to pick up changes from chronicle reindex.
		cmds := []tea.Cmd{clearStatusAfter(2 * time.Second)}
		if m.store != nil {
			cmds = append(cmds, m.loadSessionsCmd())
		}
		return m, tea.Batch(cmds...)

	// ----- Transient status clear -----------------------------------------
	case clearStatusMsg:
		m.statusInfo = ""
		m.statusErr = ""
		return m, nil

	// ----- Pending click fire (single-click debounce) ---------------------
	case pendingClickFireMsg:
		if msg.version != m.pendingClickVersion {
			return m, nil // stale — a double-click already consumed this
		}
		// Timer fired — no second click arrived, so this is a single click.
		// Reset pending state so the next click isn't mistaken for a double.
		m.pendingClickVersion = 0
		// Normal click clears multi-selection (Windows Explorer behavior).
		if m.sessionList.SelectionCount() > 0 {
			m.sessionList.DeselectAll()
			m.statusInfo = ""
		}
		// Execute deferred single-click action.
		m.sessionList.MoveTo(m.pendingClickItemIdx)
		m.sessionList.SetAnchor()
		if m.sessionList.IsFolderSelected() {
			m.sessionList.ToggleFolder()
			return m, nil
		}
		m.detailVersion++
		return m, m.loadSelectedDetailCmd()

	// ----- Data loading ----------------------------------------------------
	case sessionsLoadedMsg:
		m.sessions = m.filterHiddenSessions(msg.sessions)
		m.sessions = m.filterFavoritedSessions(m.sessions)
		m.sessions = m.filterAttentionSessions(m.sessions)
		m.sessions = m.filterPlanSessions(m.sessions)
		m.sessions = m.filterWorkStatusSessions(m.sessions)
		m.sortByAttention(m.sessions)
		m.groups = nil
		m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
		m.sessionList.SetFavoritedSessions(m.favoritedSet)
		m.sessionList.SetAttentionStatuses(m.attentionMap)
		m.sessionList.SetPlanStatuses(m.planMap)
		m.sessionList.SetWorkStatuses(m.workStatusMap)
		m.sessionList.SetSessions(m.sessions)
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
		return m, tea.Batch(m.loadSelectedDetailCmd(), m.scanPlansCmd())

	case groupsLoadedMsg:
		m.groups = m.filterHiddenGroups(msg.groups)
		m.groups = m.filterFavoritedGroups(m.groups)
		m.groups = m.filterAttentionGroups(m.groups)
		m.groups = m.filterPlanGroups(m.groups)
		m.groups = m.filterWorkStatusGroups(m.groups)
		for i := range m.groups {
			m.sortByAttention(m.groups[i].Sessions)
		}
		m.sessions = nil
		m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
		m.sessionList.SetFavoritedSessions(m.favoritedSet)
		m.sessionList.SetAttentionStatuses(m.attentionMap)
		m.sessionList.SetPlanStatuses(m.planMap)
		m.sessionList.SetWorkStatuses(m.workStatusMap)
		m.sessionList.SetPivotField(m.pivot)
		m.sessionList.SetGroups(m.groups)
		if m.state == stateLoading {
			if m.cfg.DefaultCollapsed {
				m.sessionList.CollapseAll()
			}
			m.state = stateSessionList
		}
		m.searchBar.SetResultCount(m.sessionList.SessionCount())
		m.detailVersion++
		return m, tea.Batch(m.loadSelectedDetailCmd(), m.scanPlansCmd())

	case sessionDetailMsg:
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
		m.preview.SetAttentionStatus(m.attentionStatusForSession(m.detail.Session.ID))
		m.preview.SetHasPlan(m.planMap[m.detail.Session.ID])
		if result, ok := m.workStatusMap[m.detail.Session.ID]; ok {
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

	case dataErrorMsg:
		m.statusErr = "Data: " + msg.err.Error()
		if m.state == stateLoading {
			m.state = stateSessionList
		}
		return m, nil

	// ----- Attention scanning ---------------------------------------------
	case attentionQuickScannedMsg:
		m.attentionMap = msg.statuses
		m.sessionList.SetAttentionStatuses(m.attentionMap)
		// Quick scan done — immediately fire full (deep) scan.
		return m, m.scanAttentionCmd()

	case attentionScannedMsg:
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
		cmds := []tea.Cmd{m.scheduleAttentionTick(), m.scanPlansCmd()}
		if len(m.attentionFilter) > 0 {
			cmds = append(cmds, m.loadSessionsCmd())
		}
		return m, tea.Batch(cmds...)

	case attentionTickMsg:
		return m, m.scanAttentionCmd()

	// ----- Plan scanning --------------------------------------------------
	case plansScannedMsg:
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
		if m.workStatusScanning {
			cmds = append(cmds, m.scanWorkStatusQuickCmd())
		}
		return m, tea.Batch(cmds...)

	case planContentMsg:
		if msg.err != nil || msg.content == "" {
			m.preview.SetPlanContent("")
			m.autoShowPlan = false
			return m, nil
		}
		// Only apply if the content matches the currently selected session.
		if m.detail != nil && m.detail.Session.ID == msg.sessionID {
			m.preview.SetPlanContent(msg.content)
			// After a work status scan with continuation plans, auto-switch
			// to plan view so the user sees the freshly written plan.
			if m.autoShowPlan {
				m.autoShowPlan = false
				m.preview.ShowPlanView()
			}
		}
		return m, nil

	// ----- Work status scanning -------------------------------------------
	case workStatusQuickScannedMsg:
		m.workStatusMap = msg.statuses
		m.sessionList.SetWorkStatuses(m.workStatusMap)
		if sel, ok := m.sessionList.Selected(); ok {
			if result, exists := m.workStatusMap[sel.ID]; exists {
				m.preview.SetWorkStatus(result)
			}
		}
		// Chain the full work status scan to parse plan.md content.
		return m, m.scanWorkStatusCmd()

	case workStatusScannedMsg:
		// Merge full-scan results into the existing map so that NoPlan
		// entries from the quick scan are preserved (the full scan only
		// covers sessions with plans).
		if m.workStatusMap == nil {
			m.workStatusMap = msg.statuses
		} else {
			maps.Copy(m.workStatusMap, msg.statuses)
		}
		m.sessionList.SetWorkStatuses(m.workStatusMap)
		if sel, ok := m.sessionList.Selected(); ok {
			if result, exists := m.workStatusMap[sel.ID]; exists {
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
		for id, result := range m.workStatusMap {
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

	case workStatusAIScannedMsg:
		// Merge AI analysis results into the work status map. The AI may
		// provide richer detail, updated task counts, and remaining items.
		var sessionsWithRemaining []string
		for id, analysis := range msg.analyses {
			if analysis == nil {
				continue
			}
			existing, ok := m.workStatusMap[id]
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
			m.workStatusMap[id] = existing

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
		for id, result := range m.workStatusMap {
			if _, hadAI := msg.analyses[id]; hadAI {
				continue // already handled above
			}
			if result.Status == data.WorkStatusIncomplete && len(result.RemainingItems) > 0 {
				sessionsWithRemaining = append(sessionsWithRemaining, id)
			}
		}
		m.sessionList.SetWorkStatuses(m.workStatusMap)
		if sel, ok := m.sessionList.Selected(); ok {
			if result, exists := m.workStatusMap[sel.ID]; exists {
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

	case continuationPlanCreatedMsg:
		if msg.err != nil {
			slog.Debug("continuation plan write error", "error", msg.err)
		} else if msg.updated > 0 {
			// Check whether the currently selected session has remaining work
			// and reload its plan content so the preview shows the fresh plan.
			if m.detail != nil {
				if result, ok := m.workStatusMap[m.detail.Session.ID]; ok && len(result.RemainingItems) > 0 {
					m.autoShowPlan = true
					m.statusInfo = fmt.Sprintf("Updated %d plan(s) with remaining work — showing plan for current session", msg.updated)
					scanCmd := m.completeWorkStatusScan()
					reloadCmd := m.loadPlanContentCmd(m.detail.Session.ID)
					return m, tea.Batch(scanCmd, reloadCmd)
				}
			}
			m.statusInfo = fmt.Sprintf("Updated %d plan(s) with remaining work — press v on a session to view", msg.updated)
		}
		return m, m.completeWorkStatusScan()

	// ----- Deep search debounce -------------------------------------------
	case deepSearchTickMsg:
		if msg.version != m.deepSearchVersion || m.filter.Query == "" {
			return m, nil // stale tick — query changed since scheduling
		}
		return m, m.deepSearchCmd(msg.version)

	case deepSearchResultMsg:
		if msg.version != m.deepSearchVersion {
			return m, nil // stale result — query changed since search started
		}
		m.deepSearchPending = false
		m.filter.DeepSearch = true // keep deep mode for subsequent reloads (time range, sort, etc.)
		m.searchBar.SetSearching(false)
		if msg.sessions != nil {
			m.sessions = m.filterHiddenSessions(msg.sessions)
			m.sessions = m.filterFavoritedSessions(m.sessions)
			m.sessions = m.filterAttentionSessions(m.sessions)
			m.sessions = m.filterPlanSessions(m.sessions)
			m.sessions = m.filterWorkStatusSessions(m.sessions)
			m.groups = nil
			m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
			m.sessionList.SetFavoritedSessions(m.favoritedSet)
			m.sessionList.SetSessions(m.sessions)
		} else if msg.groups != nil {
			m.groups = m.filterHiddenGroups(msg.groups)
			m.groups = m.filterFavoritedGroups(m.groups)
			m.sessions = nil
			m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
			m.sessionList.SetFavoritedSessions(m.favoritedSet)
			m.sessionList.SetPivotField(m.pivot)
			m.sessionList.SetGroups(m.groups)
		}
		if m.state == stateLoading {
			m.state = stateSessionList
		}
		m.searchBar.SetResultCount(m.sessionList.SessionCount())
		m.detailVersion++
		return m, m.loadSelectedDetailCmd()

	// ----- Copilot SDK search ------------------------------------------------
	case copilotReadyMsg:
		// Client is ready — no UI action needed; search will use it.
		return m, nil

	case copilotErrorMsg:
		// SDK init/search error — silently ignore, search degrades gracefully.
		return m, nil

	case copilotSearchTickMsg:
		if msg.version != m.copilotSearchVersion || m.filter.Query == "" {
			return m, nil // stale tick — query changed since scheduling
		}
		// If copilot client needs lazy init, do it here on the main goroutine
		// then kick off search.
		if m.copilotClient == nil && m.store != nil {
			m.copilotClient = copilot.New(m.store)
		}
		// Cancel any in-flight search so it releases the searchMu quickly
		// and doesn't block this new search for up to 45 seconds.
		if m.copilotSearchCancel != nil {
			m.copilotSearchCancel()
			m.copilotSearchCancel = nil
		}
		m.copilotSearching = true
		m.searchBar.SetAISearching(true)
		// Show "connecting" if SDK hasn't been initialised yet.
		if m.copilotClient != nil && !m.copilotClient.Available() {
			m.searchBar.SetAIStatus("connecting")
		}
		cmd := m.copilotSearchCmd(msg.version)
		return m, cmd

	case copilotSearchResultMsg:
		if msg.version != m.copilotSearchVersion {
			// Stale result — a newer search is in flight. Don't update
			// status here; the newer search will set it when it completes.
			return m, nil
		}
		m.copilotSearching = false
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
		m.aiSessionIDs = make(map[string]struct{}, len(msg.sessionIDs))
		for _, id := range msg.sessionIDs {
			m.aiSessionIDs[id] = struct{}{}
		}
		m.sessionList.SetAISessions(m.aiSessionIDs)
		// Find IDs not already in the current session list and fetch them.
		missingIDs := m.findMissingAISessionIDs(msg.sessionIDs)
		if len(missingIDs) > 0 {
			return m, m.fetchAISessionsCmd(missingIDs, msg.version)
		}
		return m, nil

	case aiSessionsLoadedMsg:
		if msg.version != m.copilotSearchVersion {
			return m, nil // stale
		}
		if len(msg.sessions) == 0 {
			return m, nil
		}
		// Apply full filter chain before merging (matches sessionsLoadedMsg).
		incoming := m.filterHiddenSessions(msg.sessions)
		incoming = m.filterFavoritedSessions(incoming)
		incoming = m.filterAttentionSessions(incoming)
		incoming = m.filterPlanSessions(incoming)
		incoming = m.filterWorkStatusSessions(incoming)
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
			m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
			m.sessionList.SetFavoritedSessions(m.favoritedSet)
			m.sessionList.SetSessions(m.sessions)
			m.searchBar.SetResultCount(m.sessionList.SessionCount())
		}
		return m, nil

	// ----- Filter picker data ---------------------------------------------
	case filterDataMsg:
		m.filterPanel.SetFolders(msg.folders, m.cfg.ExcludedDirs)
		return m, nil

	// ----- Shell detection -------------------------------------------------
	case shellsDetectedMsg:
		m.shells = msg.shells
		m.configPanel.SetShellOptions(m.shells)
		return m, nil

	// ----- Terminal detection ----------------------------------------------
	case terminalsDetectedMsg:
		m.terminals = msg.terminals
		var names []string
		for _, t := range m.terminals {
			names = append(names, t.Name)
		}
		m.configPanel.SetTerminals(names)
		return m, nil

	// ----- Font check -------------------------------------------------------
	case fontCheckMsg:
		styles.SetNerdFontEnabled(msg.installed)
		return m, nil

	// ----- Session exit (in-place resume finished) -------------------------
	case sessionExitMsg:
		if msg.err != nil {
			m.statusErr = fmt.Sprintf("Session failed: %v", msg.err)
			return m, nil
		}
		m.closeStore()
		return m, tea.Quit

	// ----- Keyboard --------------------------------------------------------
	case tea.KeyPressMsg:
		return m.handleKey(msg)

	// ----- Mouse -----------------------------------------------------------
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}

	return m, nil
}

func (m Model) View() tea.View {
	var content string
	if m.width == 0 || m.height == 0 {
		content = ""
	} else {
		switch m.state {
		case stateLoading:
			content = m.renderLoadingView()

		case stateHelpOverlay:
			content = m.help.View()

		case stateShellPicker:
			content = m.shellPicker.View()

		case stateFilterPanel:
			content = m.filterPanel.View()

		case stateConfigPanel:
			content = m.configPanel.View()

		case stateAttentionPicker:
			content = m.attentionPicker.View()

		default: // stateSessionList
			if m.reindexing && len(m.reindexLog) > 0 {
				content = m.renderReindexOverlay()
			} else {
				content = m.renderMainView()
			}
		}
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	slog.Debug("handleKey", "key", msg.String(), "state", m.state, "showPreview", m.showPreview, "searchFocused", m.searchBar.Focused())

	// Force-quit always works.
	if key.Matches(msg, keys.ForceQuit) {
		m.closeStore()
		return m, tea.Quit
	}

	// ---------- Reindex overlay captures all keys -------------------------
	if m.reindexing && len(m.reindexLog) > 0 {
		if key.Matches(msg, keys.Escape) {
			return m.cancelReindex()
		}
		// Swallow all other keys while reindex overlay is showing.
		return m, nil
	}

	// ---------- Overlay / modal states ------------------------------------
	switch m.state {
	case stateHelpOverlay:
		if key.Matches(msg, keys.Help) || key.Matches(msg, keys.Escape) {
			m.state = stateSessionList
		}
		return m, nil

	case stateShellPicker:
		switch {
		case key.Matches(msg, keys.Escape):
			m.state = stateSessionList
		case key.Matches(msg, keys.Up):
			m.shellPicker.MoveUp()
		case key.Matches(msg, keys.Down):
			m.shellPicker.MoveDown()
		case key.Matches(msg, keys.Enter):
			if sh, ok := m.shellPicker.Selected(); ok {
				m.state = stateSessionList
				launchStyle := launchStyleForMode(m.pendingLaunchMode)
				cmd := m.launchExternal(sh, m.selectedSessionID(), m.selectedSessionCwd(), launchStyle)
				return m, cmd
			}
		}
		return m, nil

	case stateFilterPanel:
		switch {
		case key.Matches(msg, keys.Escape):
			m.filterPanel.Cancel()
			m.state = stateSessionList
		case key.Matches(msg, keys.Up):
			m.filterPanel.MoveUp()
		case key.Matches(msg, keys.Down):
			m.filterPanel.MoveDown()
		case key.Matches(msg, keys.Left):
			m.filterPanel.CollapseGroup()
		case key.Matches(msg, keys.Right):
			m.filterPanel.ExpandGroup()
		case key.Matches(msg, keys.Space):
			m.filterPanel.ToggleExclusion()
		case key.Matches(msg, keys.Enter):
			excluded := m.filterPanel.Apply()
			m.cfg.ExcludedDirs = excluded
			if err := config.Save(m.cfg); err != nil {
				m.statusErr = "config save: " + err.Error()
			}
			m.filter.ExcludedDirs = excluded
			m.state = stateSessionList
			return m, m.loadSessionsCmd()
		}
		return m, nil

	case stateConfigPanel:
		return m.handleConfigKey(msg)

	case stateAttentionPicker:
		switch {
		case key.Matches(msg, keys.Escape):
			m.state = stateSessionList
		case key.Matches(msg, keys.Up):
			m.attentionPicker.MoveUp()
		case key.Matches(msg, keys.Down):
			m.attentionPicker.MoveDown()
		case key.Matches(msg, keys.Space):
			m.attentionPicker.Toggle()
		case key.Matches(msg, keys.Enter):
			m.attentionFilter = m.attentionPicker.Selected()
			m.filterPlans = m.attentionPicker.FilterPlans()
			m.showFavorited = m.attentionPicker.FilterFavorites()
			m.sessionList.SetFavoritedSessions(m.favoritedSet)
			m.filterWorkStatus = m.attentionPicker.WorkStatusFilter()
			m.state = stateSessionList
			return m, m.loadSessionsCmd()
		}
		return m, nil

	default:
		// stateLoading and stateSessionList fall through to the
		// main key handler below.
	}

	// ---------- Search bar focused ----------------------------------------
	if m.searchBar.Focused() {
		switch {
		case key.Matches(msg, keys.Escape):
			m.searchBar.Blur()
			m.deepSearchPending = false
			m.searchBar.SetSearching(false)
			m.searchBar.SetAISearching(false)
			m.copilotSearching = false
			// Cancel any in-flight SDK search.
			if m.copilotSearchCancel != nil {
				m.copilotSearchCancel()
				m.copilotSearchCancel = nil
			}
			// Keep the query active — Escape only dismisses the input focus.
			// The filter stays applied so subsequent operations (time range,
			// sort, pivot) continue to honour the search term. To clear the
			// search, press Escape again from the session list.
			if m.filter.Query != "" {
				m.filter.DeepSearch = true
				return m, m.loadSessionsCmd()
			}
			return m, nil
		case key.Matches(msg, keys.Enter):
			m.searchBar.Blur()
			// If deep search hasn't run yet, trigger it now.
			if m.deepSearchPending && m.filter.Query != "" {
				m.deepSearchPending = false
				m.filter.DeepSearch = true
				return m, m.loadSessionsCmd()
			}
			// Ensure deep mode is active for any confirmed query so that
			// subsequent reloads (time range, sort, pivot) also search deeply.
			if m.filter.Query != "" {
				m.filter.DeepSearch = true
			}
			return m, nil
		default:
			// All other keys (including j/k which alias Up/Down) are
			// forwarded to the search text input so they appear as typed
			// characters instead of triggering shortcuts.
			var cmd tea.Cmd
			sb := m.searchBar
			sb, cmd = sb.Update(msg)
			m.searchBar = sb
			newQuery := m.searchBar.Value()
			if newQuery != m.filter.Query {
				m.filter.Query = newQuery
				m.filter.DeepSearch = false
				// Quick search fires immediately; schedule deep search.
				m.deepSearchVersion++
				m.deepSearchPending = true
				m.searchBar.SetSearching(true)

				cmds := []tea.Cmd{cmd, m.loadSessionsCmd(), m.scheduleDeepSearch(m.deepSearchVersion)}

				// Copilot SDK search is gated by config flag.
				if m.cfg.AISearch {
					m.copilotSearchVersion++
					m.searchBar.SetAISearching(false) // reset until tick fires
					m.searchBar.SetAIResults(0)       // clear stale count
					m.aiSessionIDs = nil
					m.sessionList.SetAISessions(nil)
					cmds = append(cmds, m.scheduleCopilotSearch(m.copilotSearchVersion))
				}

				return m, tea.Batch(cmds...)
			}
			return m, cmd
		}
	}

	// ---------- Session-list global keys ----------------------------------
	switch {
	case key.Matches(msg, keys.Quit):
		m.closeStore()
		return m, tea.Quit

	case key.Matches(msg, keys.Escape):
		// Clear text selection first, if active.
		if m.preview.HasSelection() {
			m.preview.ClearSelection()
			return m, nil
		}
		// Exit plan view mode first, if active.
		if m.preview.PlanViewMode() {
			m.preview.ExitPlanView()
			return m, nil
		}
		// Clear active search query when Escape is pressed in the session list.
		if m.filter.Query != "" {
			m.filter.Query = ""
			m.filter.DeepSearch = false
			m.searchBar.SetValue("")
			m.searchBar.SetSearching(false)
			m.searchBar.SetAISearching(false)
			m.searchBar.SetAIResults(0)
			m.copilotSearching = false
			m.aiSessionIDs = nil
			m.sessionList.SetAISessions(nil)
			return m, m.loadSessionsCmd()
		}
		return m, nil

	case key.Matches(msg, keys.Help):
		m.state = stateHelpOverlay
		return m, nil

	case key.Matches(msg, keys.Config):
		m.configPanel.SetValues(components.ConfigValues{
			YoloMode:          m.cfg.YoloMode,
			Agent:             m.cfg.Agent,
			Model:             m.cfg.Model,
			LaunchMode:        m.cfg.EffectiveLaunchMode(),
			PaneDirection:     m.cfg.EffectivePaneDirection(),
			Terminal:          m.cfg.DefaultTerminal,
			Shell:             m.cfg.DefaultShell,
			CustomCommand:     m.cfg.CustomCommand,
			Theme:             m.cfg.Theme,
			WorkspaceRecovery: m.cfg.WorkspaceRecovery,
		})
		m.state = stateConfigPanel
		return m, nil

	case key.Matches(msg, keys.Search):
		cmd := m.searchBar.Focus()
		return m, cmd

	case key.Matches(msg, keys.Filter):
		m.state = stateFilterPanel
		return m, loadFilterDataCmd(m.store)

	case key.Matches(msg, keys.ShiftUp):
		m.sessionList.MoveUpShift()
		m.updateSelectionStatus()
		m.detailVersion++
		return m, m.loadSelectedDetailCmd()

	case key.Matches(msg, keys.ShiftDown):
		m.sessionList.MoveDownShift()
		m.updateSelectionStatus()
		m.detailVersion++
		return m, m.loadSelectedDetailCmd()

	case key.Matches(msg, keys.Up):
		m.sessionList.ResetShift()
		m.sessionList.MoveUp()
		m.detailVersion++
		return m, m.loadSelectedDetailCmd()

	case key.Matches(msg, keys.Down):
		m.sessionList.ResetShift()
		m.sessionList.MoveDown()
		m.detailVersion++
		return m, m.loadSelectedDetailCmd()

	case key.Matches(msg, keys.Enter):
		if m.sessionList.IsFolderSelected() {
			cmd := m.launchNewInFolder(m.cfg.EffectiveLaunchMode())
			return m, cmd
		}
		cmd := m.launchSelected()
		return m, cmd

	case key.Matches(msg, keys.LaunchWindow):
		if m.sessionList.IsFolderSelected() {
			cmd := m.launchNewInFolder(config.LaunchModeWindow)
			return m, cmd
		}
		if m.sessionList.SelectionCount() > 0 {
			cmd := m.launchMultipleWithMode(config.LaunchModeWindow)
			return m, cmd
		}
		cmd := m.launchWithMode(config.LaunchModeWindow)
		return m, cmd

	case key.Matches(msg, keys.LaunchTab):
		if m.sessionList.IsFolderSelected() {
			cmd := m.launchNewInFolder(config.LaunchModeTab)
			return m, cmd
		}
		if m.sessionList.SelectionCount() > 0 {
			cmd := m.launchMultipleWithMode(config.LaunchModeTab)
			return m, cmd
		}
		cmd := m.launchWithMode(config.LaunchModeTab)
		return m, cmd

	case key.Matches(msg, keys.LaunchPane):
		if m.sessionList.IsFolderSelected() {
			cmd := m.launchNewInFolder(config.LaunchModePane)
			return m, cmd
		}
		if m.sessionList.SelectionCount() > 0 {
			cmd := m.launchMultipleWithMode(config.LaunchModePane)
			return m, cmd
		}
		cmd := m.launchWithMode(config.LaunchModePane)
		return m, cmd

	case key.Matches(msg, keys.Left):
		if m.sessionList.IsFolderSelected() {
			m.sessionList.CollapseFolder()
		}
		return m, nil

	case key.Matches(msg, keys.Right):
		if m.sessionList.IsFolderSelected() {
			m.sessionList.ExpandFolder()
		}
		return m, nil

	case key.Matches(msg, keys.ExpandCollapseAll):
		if m.sessionList.AllExpanded() {
			m.sessionList.CollapseAll()
		} else {
			m.sessionList.ExpandAll()
		}
		return m, nil

	case key.Matches(msg, keys.Sort):
		m.cycleSort()
		return m, m.loadSessionsCmd()

	case key.Matches(msg, keys.SortOrder):
		m.toggleSortOrder()
		return m, m.loadSessionsCmd()

	case key.Matches(msg, keys.Pivot):
		m.cyclePivot()
		return m, m.loadSessionsCmd()

	case key.Matches(msg, keys.PivotOrder):
		m.togglePivotOrder()
		return m, m.loadSessionsCmd()

	case key.Matches(msg, keys.Preview):
		m.showPreview = !m.showPreview
		m.recalcLayout()
		if m.showPreview {
			m.detailVersion++
			return m, m.loadSelectedDetailCmd()
		}
		return m, nil

	case key.Matches(msg, keys.PreviewPosition):
		m.cyclePreviewPosition()
		m.recalcLayout()
		m.cfg.PreviewPosition = m.previewPosition
		if err := config.Save(m.cfg); err != nil {
			m.statusErr = "config save: " + err.Error()
		}
		if m.showPreview {
			m.detailVersion++
			return m, m.loadSelectedDetailCmd()
		}
		return m, nil

	case key.Matches(msg, keys.PreviewScrollUp):
		if m.showPreview {
			before := m.preview.ScrollOffset()
			m.preview.PageUp()
			slog.Debug("preview scroll up", "before", before, "after", m.preview.ScrollOffset())
		}
		return m, nil

	case key.Matches(msg, keys.PreviewScrollDown):
		if m.showPreview {
			before := m.preview.ScrollOffset()
			m.preview.PageDown()
			slog.Debug("preview scroll down", "before", before, "after", m.preview.ScrollOffset())
		}
		return m, nil

	case key.Matches(msg, keys.ConversationSort):
		if m.showPreview && m.detail != nil {
			newVal := m.preview.ToggleConversationSort()
			slog.Debug("conversation sort toggled", "newestFirst", newVal)
			m.cfg.ConversationNewestFirst = newVal
			if err := config.Save(m.cfg); err != nil {
				m.statusErr = "config save: " + err.Error()
			}
		}
		return m, nil

	case key.Matches(msg, keys.ViewPlan):
		if m.showPreview && m.detail != nil {
			if m.preview.HasPlanContent() {
				m.preview.TogglePlanView()
			} else {
				m.statusInfo = "No plan for this session"
			}
		}
		return m, nil

	case key.Matches(msg, keys.Reindex):
		if !m.reindexing {
			m.reindexing = true
			m.reindexLog = []string{"Starting reindex…"}
			m.reindexVP = viewport.New(viewport.WithHeight(reindexOverlayHeight))
			// MouseWheelEnabled is default in v2
			m.updateReindexViewport()
			handle, cmds := components.StartChronicleReindex()
			m.reindexCancel = &handle
			return m, tea.Batch(cmds...)
		}
		return m, nil

	case key.Matches(msg, keys.TimeRange1):
		cmd := m.setTimeRange("1h")
		return m, cmd
	case key.Matches(msg, keys.TimeRange2):
		cmd := m.setTimeRange("1d")
		return m, cmd
	case key.Matches(msg, keys.TimeRange3):
		cmd := m.setTimeRange("7d")
		return m, cmd
	case key.Matches(msg, keys.TimeRange4):
		cmd := m.setTimeRange("all")
		return m, cmd

	case key.Matches(msg, keys.Hide):
		return m.handleHideSession()

	case key.Matches(msg, keys.ToggleHidden):
		m.showHidden = !m.showHidden
		m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
		cmd := m.loadSessionsCmd()
		return m, cmd

	case key.Matches(msg, keys.Star):
		return m.handleToggleFavorite()

	case key.Matches(msg, keys.CopyID):
		return m.handleCopyID()

	case key.Matches(msg, keys.CopyPreview):
		return m.handleCopyPreview()

	case key.Matches(msg, keys.JumpNextAttention):
		return m.handleJumpNextAttention()

	case key.Matches(msg, keys.FilterAttention):
		counts := m.attentionStatusCounts()
		m.attentionPicker.SetCounts(counts)
		m.attentionPicker.SetSelected(m.attentionFilter)
		m.attentionPicker.SetFilterPlans(m.filterPlans)
		m.attentionPicker.SetPlanCount(m.planSessionCount())
		m.attentionPicker.SetFilterFavorites(m.showFavorited)
		m.attentionPicker.SetFavoriteCount(len(m.favoritedSet))
		m.attentionPicker.SetWorkStatusFilter(m.filterWorkStatus)
		m.attentionPicker.SetWorkStatusCounts(m.workStatusCounts())
		m.attentionPicker.SetWorkStatusScanned(m.workStatusScanned)
		m.attentionPicker.SetSize(m.width, m.height)
		m.state = stateAttentionPicker
		return m, nil

	case key.Matches(msg, keys.Space):
		m.sessionList.ToggleSelected()
		m.updateSelectionStatus()
		return m, nil

	case key.Matches(msg, keys.LaunchAll):
		cmd := m.launchMultiple()
		return m, cmd

	case key.Matches(msg, keys.ResumeInterrupted):
		return m.handleResumeInterrupted()

	case key.Matches(msg, keys.ScanWorkStatus):
		if !m.workStatusScanning {
			m.workStatusScanning = true
			m.statusInfo = "Scanning work status..."
			// Trigger a fresh plan scan first — the plansScannedMsg
			// handler chains into scanWorkStatusQuickCmd when
			// workStatusScanning is set.  This ensures newly created
			// or deleted plan.md files are picked up.
			return m, m.scanPlansCmd()
		}
		return m, nil

	case key.Matches(msg, keys.SelectAll):
		m.sessionList.SelectAll()
		m.updateSelectionStatus()
		return m, nil

	case key.Matches(msg, keys.DeselectAll):
		m.sessionList.DeselectAll()
		m.statusInfo = ""
		return m, nil
	}

	return m, nil
}

// handleHideSession toggles the hidden state of the currently selected session.
func (m Model) handleHideSession() (tea.Model, tea.Cmd) {
	sess, ok := m.sessionList.Selected()
	if !ok {
		return m, nil
	}

	if _, ok := m.hiddenSet[sess.ID]; ok {
		// Unhide: remove from set and config.
		delete(m.hiddenSet, sess.ID)
	} else {
		// Hide: add to set and config.
		m.hiddenSet[sess.ID] = struct{}{}
	}

	// If hiding a favorited session, also remove it from favorites.
	if _, fav := m.favoritedSet[sess.ID]; fav {
		delete(m.favoritedSet, sess.ID)
		m.cfg.FavoriteSessions = sortedKeys(m.favoritedSet)
	}

	// Sync config and persist.
	m.cfg.HiddenSessions = sortedKeys(m.hiddenSet)
	if err := config.Save(m.cfg); err != nil {
		m.statusErr = "config save: " + err.Error()
	}

	m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
	m.sessionList.SetFavoritedSessions(m.favoritedSet)
	cmd := m.loadSessionsCmd()
	return m, cmd
}

// handleToggleFavorite toggles the favorited state of the currently selected session.
func (m Model) handleToggleFavorite() (tea.Model, tea.Cmd) {
	sess, ok := m.sessionList.Selected()
	if !ok {
		return m, nil
	}

	if _, hidden := m.hiddenSet[sess.ID]; hidden {
		return m, nil // don't favorite hidden sessions
	}

	if _, ok := m.favoritedSet[sess.ID]; ok {
		delete(m.favoritedSet, sess.ID)
	} else {
		m.favoritedSet[sess.ID] = struct{}{}
	}

	m.cfg.FavoriteSessions = sortedKeys(m.favoritedSet)
	if err := config.Save(m.cfg); err != nil {
		m.statusErr = "config save: " + err.Error()
	}

	m.sessionList.SetFavoritedSessions(m.favoritedSet)
	cmd := m.loadSessionsCmd()
	return m, cmd
}

// handleCopyID copies the selected session's ID to the system clipboard.
func (m Model) handleCopyID() (tea.Model, tea.Cmd) {
	sess, ok := m.sessionList.Selected()
	if !ok {
		return m, nil
	}
	if err := clipboardWrite(sess.ID); err != nil {
		m.statusErr = "clipboard: " + err.Error()
		return m, clearStatusAfter(2 * time.Second)
	}
	m.statusInfo = statusCopiedID
	return m, clearStatusAfter(2 * time.Second)
}

// handleCopyPreview copies the preview pane content to the system clipboard.
// When there is an active mouse text selection, only the selected text is
// copied; otherwise the full preview/plan content is copied.
func (m Model) handleCopyPreview() (tea.Model, tea.Cmd) {
	if !m.showPreview {
		return m, nil
	}

	// Prefer active selection text.
	if m.preview.HasSelection() {
		text := m.preview.SelectedText()
		if text == "" {
			return m, nil
		}
		if err := clipboardWrite(text); err != nil {
			m.statusErr = "clipboard: " + err.Error()
			return m, clearStatusAfter(2 * time.Second)
		}
		m.statusInfo = statusCopiedSelection
		m.preview.ClearSelection()
		return m, clearStatusAfter(2 * time.Second)
	}

	text := m.preview.Content()
	if text == "" {
		return m, nil
	}
	if err := clipboardWrite(text); err != nil {
		m.statusErr = "clipboard: " + err.Error()
		return m, clearStatusAfter(2 * time.Second)
	}
	m.statusInfo = statusCopiedPreview
	return m, clearStatusAfter(2 * time.Second)
}

// sortedKeys converts a string set to a sorted slice for deterministic
// config serialisation. Returns nil for empty sets.
func sortedKeys(m map[string]struct{}) []string {
	if len(m) == 0 {
		return nil
	}
	return slices.Sorted(maps.Keys(m))
}

// visibleHiddenSet returns the hiddenSet when showHidden is true (so the
// SessionList can render those rows dimmed), or an empty map otherwise.
func (m *Model) visibleHiddenSet() map[string]struct{} {
	if m.showHidden {
		return m.hiddenSet
	}
	return nil
}

// handleConfigKey processes keys while the config panel is open.
func (m Model) handleConfigKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.configPanel.IsEditing() {
		switch {
		case key.Matches(msg, keys.Escape):
			m.configPanel.CancelEdit()
			return m, nil
		case key.Matches(msg, keys.Enter):
			m.configPanel.ConfirmEdit()
			m.saveConfigFromPanel()
			return m, nil
		default:
			var cmd tea.Cmd
			m.configPanel, cmd = m.configPanel.Update(msg)
			return m, cmd
		}
	}

	switch {
	case key.Matches(msg, keys.Escape):
		// Cancel — close without saving (changes already persisted per-toggle).
		m.state = stateSessionList
		return m, nil
	case key.Matches(msg, keys.Up):
		m.configPanel.MoveUp()
	case key.Matches(msg, keys.Down):
		m.configPanel.MoveDown()
	case key.Matches(msg, keys.Enter):
		cmd := m.configPanel.HandleEnter()
		if cmd == nil {
			// Toggle/cycle completed — persist immediately.
			m.saveConfigFromPanel()
		}
		return m, cmd
	}
	return m, nil
}

// saveConfigFromPanel synchronises config panel values back to cfg and saves.
func (m *Model) saveConfigFromPanel() {
	v := m.configPanel.Values()
	m.cfg.YoloMode = v.YoloMode
	m.cfg.Agent = v.Agent
	m.cfg.Model = v.Model
	m.cfg.LaunchMode = v.LaunchMode
	m.cfg.LaunchInPlace = v.LaunchMode == config.LaunchModeInPlace // keep legacy field in sync
	m.cfg.PaneDirection = v.PaneDirection
	m.cfg.DefaultTerminal = v.Terminal
	m.cfg.DefaultShell = v.Shell
	m.cfg.CustomCommand = v.CustomCommand
	m.cfg.Theme = v.Theme
	m.cfg.WorkspaceRecovery = v.WorkspaceRecovery
	m.cfg.PreviewPosition = v.PreviewPosition
	m.previewPosition = v.PreviewPosition
	resolveTheme(m.cfg)
	// If the user switched back to "auto", re-apply with the detected
	// terminal brightness so colours adapt immediately.
	themeName := m.cfg.Theme
	if themeName == "" || themeName == themeAuto {
		styles.ApplyAutoTheme(m.hasDarkBackground)
	}
	m.recalcLayout()
	if err := config.Save(m.cfg); err != nil {
		m.statusErr = "config save: " + err.Error()
	}
}

// ---------------------------------------------------------------------------
// Mouse handling
// ---------------------------------------------------------------------------

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// When reindex overlay is active, handle scroll within overlay viewport
	// and block all other mouse events from reaching the background.
	if m.reindexing && len(m.reindexLog) > 0 {
		switch msg := msg.(type) {
		case tea.MouseWheelMsg:
			switch msg.Button {
			case tea.MouseWheelUp:
				m.reindexVP.ScrollUp(3)
			case tea.MouseWheelDown:
				m.reindexVP.ScrollDown(3)
			}
		case tea.MouseReleaseMsg:
			if msg.Button == tea.MouseLeft {
				m.handleReindexClick(msg)
			}
		}
		return m, nil
	}

	// Only handle mouse in the main session list view.
	if m.state != stateSessionList {
		return m, nil
	}

	// Determine which pane the mouse is over.
	mouse := msg.Mouse()
	overPreview := m.isOverPreview(mouse.X, mouse.Y)

	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			if overPreview {
				m.preview.ScrollUp(3)
			} else {
				m.sessionList.ScrollBy(-3)
				m.detailVersion++
				return m, m.loadSelectedDetailCmd()
			}
		case tea.MouseWheelDown:
			if overPreview {
				m.preview.ScrollDown(3)
			} else {
				m.sessionList.ScrollBy(3)
				m.detailVersion++
				return m, m.loadSelectedDetailCmd()
			}
		}
		return m, nil

	case tea.MouseClickMsg:
		// Left-click in preview content area starts text selection.
		if msg.Button == tea.MouseLeft && overPreview {
			contentLine, col := m.previewContentCoords(mouse.X, mouse.Y)
			if contentLine >= 0 {
				m.preview.StartSelection(contentLine, col)
			}
		}
		return m, nil

	case tea.MouseMotionMsg:
		// Left-button drag in preview extends text selection.
		if msg.Button == tea.MouseLeft && overPreview {
			contentLine, col := m.previewContentCoords(mouse.X, mouse.Y)
			if contentLine >= 0 {
				m.preview.UpdateSelection(contentLine, col)
			}
		}
		return m, nil

	case tea.MouseReleaseMsg:
		// Only handle left-button releases — right/middle clicks must not
		// trigger selection, double-click detection, or session launch.
		if msg.Button != tea.MouseLeft {
			return m, nil
		}

		// --- Clickable header area (Y < HeaderLines) ---
		if mouse.Y < styles.HeaderLines {
			return m.handleHeaderClick(mouse.X, mouse.Y)
		}

		// Map Y coordinate to a list item.
		listRow := mouse.Y - styles.HeaderLines
		if listRow >= m.layout.contentHeight {
			// Footer click — check for waiting badge.
			return m.handleFooterClick(mouse.X)
		}
		// Preview pane click — finalize selection or check interactive elements.
		if m.isOverPreview(mouse.X, mouse.Y) {
			// Finalize any active text selection and copy to clipboard.
			if m.preview.HasSelection() {
				text := m.preview.FinalizeSelection()
				if text != "" {
					if err := clipboardWrite(text); err != nil {
						m.statusErr = "clipboard: " + err.Error()
						return m, clearStatusAfter(2 * time.Second)
					}
					m.statusInfo = statusCopiedSelection
					return m, clearStatusAfter(2 * time.Second)
				}
			}

			var previewRow int
			switch m.layout.previewPosition {
			case config.PreviewPositionTop:
				previewRow = mouse.Y - styles.HeaderLines - 1 // -1 for top border
			case config.PreviewPositionBottom:
				previewRow = mouse.Y - styles.HeaderLines - m.layout.listHeight - 1 - 1 // gap + top border
			default:
				previewRow = mouse.Y - styles.HeaderLines - 1
			}
			contentRow := previewRow + m.preview.ScrollOffset()
			if m.preview.HitConversationSort(contentRow) {
				newVal := m.preview.ToggleConversationSort()
				m.cfg.ConversationNewestFirst = newVal
				if err := config.Save(m.cfg); err != nil {
					m.statusErr = "config save: " + err.Error()
				}
			}
			if m.preview.HitSessionID(contentRow) {
				if sid := m.preview.SessionID(); sid != "" {
					if err := clipboardWrite(sid); err != nil {
						m.statusErr = "clipboard: " + err.Error()
						return m, clearStatusAfter(2 * time.Second)
					}
					m.statusInfo = statusCopiedID
					return m, clearStatusAfter(2 * time.Second)
				}
			}
			return m, nil
		}
		itemIdx := m.sessionList.ScrollOffset() + listRow

		// Detect double-click: second click on the same row while a
		// pending single-click timer is still running.
		isDoubleClick := m.pendingClickVersion > 0 &&
			mouse.Y == m.pendingClickY

		if isDoubleClick {
			// Invalidate the pending single-click so it won't fire.
			m.pendingClickVersion = 0

			m.sessionList.MoveTo(itemIdx)

			// Guard: if the list is empty or the click resolved to a
			// non-existent row, bail out instead of launching.
			_, hasSession := m.sessionList.Selected()
			if !hasSession && !m.sessionList.IsFolderSelected() {
				return m, nil
			}

			if m.sessionList.IsFolderSelected() {
				mode := m.cfg.EffectiveLaunchMode()
				if msg.Mod.Contains(tea.ModCtrl) {
					mode = config.LaunchModeWindow
				} else if msg.Mod.Contains(tea.ModShift) {
					mode = config.LaunchModeTab
				}
				cmd := m.launchNewInFolder(mode)
				return m, cmd
			}
			// Double-click session: if it's part of a multi-selection,
			// launch all selected. Otherwise launch just this session.
			if m.sessionList.SelectionCount() > 0 && m.sessionList.IsSelected(m.selectedSessionID()) {
				cmd := m.launchMultiple()
				return m, cmd
			}
			if msg.Mod.Contains(tea.ModCtrl) {
				cmd := m.launchWithMode(config.LaunchModeWindow)
				return m, cmd
			}
			if msg.Mod.Contains(tea.ModShift) {
				cmd := m.launchWithMode(config.LaunchModeTab)
				return m, cmd
			}
			cmd := m.launchSelected()
			return m, cmd
		}

		// Ctrl+click: toggle multi-select immediately (no deferred action).
		// If nothing is selected yet, auto-select the previously focused
		// item first so it stays in the set (Windows Explorer behavior).
		if msg.Mod.Contains(tea.ModCtrl) {
			if m.sessionList.SelectionCount() == 0 {
				m.sessionList.ToggleSelected() // select current cursor item
			}
			m.sessionList.MoveTo(itemIdx)
			m.sessionList.ToggleSelected()
			m.sessionList.SetAnchor()
			m.updateSelectionStatus()
			m.detailVersion++
			return m, m.loadSelectedDetailCmd()
		}

		// Shift+click: range select from anchor to clicked item.
		if msg.Mod.Contains(tea.ModShift) {
			m.sessionList.MoveTo(itemIdx)
			m.sessionList.SelectRange(m.sessionList.Anchor(), itemIdx)
			m.updateSelectionStatus()
			m.detailVersion++
			return m, m.loadSelectedDetailCmd()
		}

		// First click — defer the single-click action behind a timer so
		// a potential second click (double-click) can cancel it.
		m.pendingClickVersion++
		m.pendingClickY = mouse.Y
		m.pendingClickItemIdx = itemIdx

		// Immediately move selection so the highlight follows the cursor,
		// but do NOT toggle folders or load details yet.
		m.sessionList.MoveTo(itemIdx)

		ver := m.pendingClickVersion
		return m, tea.Tick(doubleClickTimeout, func(time.Time) tea.Msg {
			return pendingClickFireMsg{version: ver}
		})
	}

	return m, nil
}

// handleFooterClick dispatches left-clicks on the footer status bar.
// Currently supports clicking the "● N waiting" badge to toggle the
// attention filter (same as pressing !).
func (m Model) handleFooterClick(x int) (tea.Model, tea.Cmd) {
	wc := m.waitingCount()
	if wc == 0 {
		return m, nil
	}

	// Replay the footer layout to find the "● N waiting" badge X position.
	// This mirrors the left-side construction in renderFooter().
	left := fmt.Sprintf(" %d sessions", m.sessionList.SessionCount())
	badgeStart := lipgloss.Width(left) + 2 // +2 for the "  " separator before the badge
	badgeRendered := styles.AttentionWaitingStyle.Render(fmt.Sprintf("● %d waiting", wc))
	badgeEnd := badgeStart + lipgloss.Width(badgeRendered)

	if x >= badgeStart && x < badgeEnd {
		counts := m.attentionStatusCounts()
		m.attentionPicker.SetCounts(counts)
		m.attentionPicker.SetSelected(m.attentionFilter)
		m.attentionPicker.SetFilterPlans(m.filterPlans)
		m.attentionPicker.SetPlanCount(m.planSessionCount())
		m.attentionPicker.SetFilterFavorites(m.showFavorited)
		m.attentionPicker.SetFavoriteCount(len(m.favoritedSet))
		m.attentionPicker.SetWorkStatusFilter(m.filterWorkStatus)
		m.attentionPicker.SetWorkStatusCounts(m.workStatusCounts())
		m.attentionPicker.SetWorkStatusScanned(m.workStatusScanned)
		m.attentionPicker.SetSize(m.width, m.height)
		m.state = stateAttentionPicker
		return m, nil
	}
	return m, nil
}

// handleHeaderClick dispatches left-clicks that land in the header area
// (Y=0: title/search, Y=1: badges/time/sort/pivot, Y=2: separator).
func (m Model) handleHeaderClick(x, y int) (tea.Model, tea.Cmd) {
	switch y {
	case 0: // Header line — click on search bar area.
		titleW := lipgloss.Width(styles.TitleStyle.Render(styles.IconTitle()+" Copilot Dispatch")) + 1
		if x >= titleW {
			cmd := m.searchBar.Focus()
			return m, cmd
		}
	case 1: // Badge line — hit-test time range, sort, pivot.
		action := m.badgeClickAction(x)
		switch action {
		case "sort":
			m.cycleSort()
			return m, m.loadSessionsCmd()
		case "sortorder":
			m.toggleSortOrder()
			return m, m.loadSessionsCmd()
		case "pivot":
			m.cyclePivot()
			return m, m.loadSessionsCmd()
		case "pivotorder":
			m.togglePivotOrder()
			return m, m.loadSessionsCmd()
		case "expandall":
			if m.sessionList.AllExpanded() {
				m.sessionList.CollapseAll()
			} else {
				m.sessionList.ExpandAll()
			}
			return m, nil
		default:
			if rest, ok := strings.CutPrefix(action, "time:"); ok {
				cmd := m.setTimeRange(rest)
				return m, cmd
			}
		}
	}
	return m, nil
}

// badgeClickAction maps an X coordinate on the badge line to an action
// string ("time:1h", "sort", "pivot", etc.) by computing cumulative
// rendered widths of each element. Returns "" if no element was hit.
func (m Model) badgeClickAction(x int) string {
	cursor := 1 // leading space in renderBadges

	// Filter badges.
	badges := m.filterPanel.ActiveBadges()
	for _, b := range badges {
		w := lipgloss.Width(styles.BadgeStyle.Render(b))
		if x >= cursor && x < cursor+w {
			return "" // filter badges are display-only for now
		}
		cursor += w + 2 // "  " separator between parts
	}

	// Time range selector — individual items separated by " ".
	for i, tr := range timeRanges {
		var rendered string
		if tr.label == m.timeRange {
			rendered = styles.ActiveBadgeStyle.Render(tr.key + ":" + tr.label)
		} else {
			rendered = styles.KeyStyle.Render(tr.key) + styles.DimmedStyle.Render(":"+tr.label)
		}
		w := lipgloss.Width(rendered)
		if x >= cursor && x < cursor+w {
			return "time:" + tr.label
		}
		if i < len(timeRanges)-1 {
			cursor += w + 1 // single space within time group
		} else {
			cursor += w + 2 // double space to next group
		}
	}

	// Sort indicator — split into arrow (order toggle) and label (cycle field).
	arrow := styles.IconSortDown()
	if m.sort.Order == data.Ascending {
		arrow = styles.IconSortUp()
	}
	sortLabel := arrow + " " + sortDisplayLabel(m.sort.Field)
	sortKeyRendered := styles.KeyStyle.Render("s")
	sortKeyW := lipgloss.Width(sortKeyRendered)
	sortPrefix := styles.DimmedStyle.Render(": ")
	sortPrefixW := lipgloss.Width(sortPrefix)
	sortArrowRendered := styles.DimmedStyle.Render(arrow)
	sortArrowW := lipgloss.Width(sortArrowRendered)
	sortFullRendered := sortKeyRendered + styles.DimmedStyle.Render(": "+sortLabel)
	w := lipgloss.Width(sortFullRendered)
	if x >= cursor && x < cursor+w {
		// Click on the arrow portion toggles order; elsewhere cycles sort field.
		arrowStart := cursor + sortKeyW + sortPrefixW
		if x >= arrowStart && x < arrowStart+sortArrowW {
			return "sortorder"
		}
		return "sort"
	}
	cursor += w + 2

	// Pivot indicator — split into arrow (order toggle) and label (cycle mode).
	pivotLabel := m.pivot
	if pivotLabel == pivotNone {
		pivotLabel = "list"
	}
	pivotArrow := styles.IconSortDown()
	if m.pivotOrder == data.Ascending {
		pivotArrow = styles.IconSortUp()
	}
	pivotLabel = pivotArrow + " " + pivotLabel
	pivotKeyRendered := styles.KeyStyle.Render("tab")
	pivotKeyW := lipgloss.Width(pivotKeyRendered)
	pivotPrefix := styles.DimmedStyle.Render(": ")
	pivotPrefixW := lipgloss.Width(pivotPrefix)
	pivotRendered := pivotKeyRendered + styles.DimmedStyle.Render(": "+pivotLabel)
	pw := lipgloss.Width(pivotRendered)
	if x >= cursor && x < cursor+pw {
		pivotArrowRendered := styles.DimmedStyle.Render(styles.IconSortDown())
		pivotArrowW := lipgloss.Width(pivotArrowRendered)
		arrowStart := cursor + pivotKeyW + pivotPrefixW
		if x >= arrowStart && x < arrowStart+pivotArrowW {
			return "pivotorder"
		}
		return "pivot"
	}
	cursor += pw + 2

	// Expand/collapse all indicator — only in tree mode.
	if m.pivot != pivotNone {
		expandIcon := styles.IconSortUp() + styles.IconSortUp()
		if m.sessionList.AllExpanded() {
			expandIcon = styles.IconSortDown() + styles.IconSortDown()
		}
		expandRendered := styles.KeyStyle.Render("x") + styles.DimmedStyle.Render(": "+expandIcon)
		ew := lipgloss.Width(expandRendered)
		if x >= cursor && x < cursor+ew {
			return "expandall"
		}
	}

	return ""
}

// ---------------------------------------------------------------------------
// View rendering
// ---------------------------------------------------------------------------

func (m Model) renderLoadingView() string {
	header := m.renderHeader()
	badges := m.renderBadges()
	sep := m.renderSeparator()
	footer := m.renderFooter()

	headerH := lipgloss.Height(header) + lipgloss.Height(badges) + lipgloss.Height(sep)
	footerH := lipgloss.Height(footer)
	contentH := m.height - headerH - footerH
	if contentH < 1 {
		contentH = 1
	}

	loading := m.spinner.View() + " Loading sessions…"
	content := lipgloss.Place(m.width, contentH, lipgloss.Center, lipgloss.Center, loading)

	return strings.Join([]string{header, badges, sep, content, footer}, "\n")
}

func (m Model) renderMainView() string {
	header := m.renderHeader()
	badges := m.renderBadges()
	sep := m.renderSeparator()
	footer := m.renderFooter()

	// Use pre-computed layout dimensions from recalcLayout() so that
	// rendering and hit-testing always agree on panel positions/sizes.
	l := m.layout

	var content string
	hasPreview := l.previewWidth > 0 && l.previewHeight > 0

	if hasPreview {
		gap := strings.Repeat(" ", gapWidth)
		switch l.previewPosition {
		case config.PreviewPositionLeft:
			content = lipgloss.JoinHorizontal(lipgloss.Top,
				m.preview.View(),
				gap,
				m.sessionList.View(),
			)
		case config.PreviewPositionTop:
			content = lipgloss.JoinVertical(lipgloss.Left,
				m.preview.View(),
				"",
				m.sessionList.View(),
			)
		case config.PreviewPositionBottom:
			content = lipgloss.JoinVertical(lipgloss.Left,
				m.sessionList.View(),
				"",
				m.preview.View(),
			)
		default: // right
			content = lipgloss.JoinHorizontal(lipgloss.Top,
				m.sessionList.View(),
				gap,
				m.preview.View(),
			)
		}
	} else {
		content = m.sessionList.View()
	}

	return strings.Join([]string{header, badges, sep, content, footer}, "\n")
}

func (m Model) renderHeader() string {
	title := styles.TitleStyle.Render(styles.IconTitle() + " Copilot Dispatch")

	// Search bar (always visible).
	// Reserve space for the right side (reindex/scan spinner) only when active.
	rightReserve := headerRightReserve
	if m.reindexing {
		rightReserve = headerReindexReserve
	} else if m.workStatusScanning {
		rightReserve = headerScanReserve
	}
	searchW := m.width - lipgloss.Width(title) - rightReserve
	if searchW < minSearchBarWidth {
		searchW = minSearchBarWidth
	}
	m.searchBar.SetWidth(searchW)
	search := m.searchBar.View()

	// Clamp rendered search bar to its allocated width so the header
	// never exceeds the terminal width (which would wrap and hide the
	// badges row underneath).
	if lipgloss.Width(search) > searchW {
		search = lipgloss.NewStyle().MaxWidth(searchW).Render(search)
	}

	// Right side: reindex or work-status scan status.
	var right string
	if m.reindexing {
		right = m.spinner.View() + " Reindexing…"
	} else if m.workStatusScanning {
		right = m.spinner.View() + " Scanning work status…"
	}

	gap := m.width - lipgloss.Width(title) - lipgloss.Width(search) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}

	line := title + " " + search + strings.Repeat(" ", gap) + right

	// Ensure the header never exceeds terminal width, even when the
	// reindex spinner pushes total content beyond the available space.
	if m.width > 0 && lipgloss.Width(line) > m.width {
		line = lipgloss.NewStyle().MaxWidth(m.width).Render(line)
	}
	return styles.HeaderStyle.Render(line)
}

func (m Model) renderBadges() string {
	// Active filter badges.
	badges := m.filterPanel.ActiveBadges()
	parts := make([]string, 0, len(badges)+3)
	for _, b := range badges {
		parts = append(parts, styles.BadgeStyle.Render(b))
	}

	// Inline time range selector — show key shortcuts (1-4).
	var trParts []string
	for _, tr := range timeRanges {
		if tr.label == m.timeRange {
			trParts = append(trParts, styles.ActiveBadgeStyle.Render(tr.key+":"+tr.label))
		} else {
			trParts = append(trParts, styles.KeyStyle.Render(tr.key)+styles.DimmedStyle.Render(":"+tr.label))
		}
	}
	parts = append(parts, strings.Join(trParts, " "))

	// Sort indicator with shortcut.
	arrow := styles.IconSortDown()
	if m.sort.Order == data.Ascending {
		arrow = styles.IconSortUp()
	}
	sortLabel := arrow + " " + sortDisplayLabel(m.sort.Field)
	parts = append(parts, styles.KeyStyle.Render("s")+styles.DimmedStyle.Render(": "+sortLabel))

	// Pivot indicator with shortcut (always shown).
	pivotLabel := m.pivot
	if pivotLabel == pivotNone {
		pivotLabel = "list"
	}
	pivotArrow := styles.IconSortDown()
	if m.pivotOrder == data.Ascending {
		pivotArrow = styles.IconSortUp()
	}
	pivotLabel = pivotArrow + " " + pivotLabel
	parts = append(parts, styles.KeyStyle.Render("tab")+styles.DimmedStyle.Render(": "+pivotLabel))

	// Expand/collapse all indicator — only shown in tree mode.
	if m.pivot != pivotNone {
		expandIcon := styles.IconSortUp() + styles.IconSortUp()
		if m.sessionList.AllExpanded() {
			expandIcon = styles.IconSortDown() + styles.IconSortDown()
		}
		parts = append(parts, styles.KeyStyle.Render("x")+styles.DimmedStyle.Render(": "+expandIcon))
	}

	// Favorites filter indicator.
	if m.showFavorited {
		parts = append(parts, styles.ActiveBadgeStyle.Render("★ Favorites"))
	}

	if len(parts) == 0 {
		return ""
	}

	line := " " + strings.Join(parts, "  ")
	// Clamp to m.width-1 to prevent autowrap on exact-width terminals.
	return lipgloss.NewStyle().MaxWidth(max(0, m.width-1)).Render(line)
}

func (m Model) renderSeparator() string {
	// Use m.width-1 to avoid an exact-terminal-width line that could
	// trigger autowrap on some terminals, pushing the badges row off-screen.
	return styles.SeparatorStyle.Render(strings.Repeat("─", max(0, m.width-1)))
}

func (m Model) renderFooter() string {
	count := m.sessionList.SessionCount()

	// Left: session count + active filter summary.
	left := fmt.Sprintf(" %d sessions", count)
	if wc := m.waitingCount(); wc > 0 {
		left += "  " + styles.AttentionWaitingStyle.Render(fmt.Sprintf("● %d waiting", wc))
	}
	if ic := m.interruptedCount(); ic > 0 {
		left += "  " + styles.AttentionInterruptedStyle.Render(fmt.Sprintf("⚡ %d interrupted", ic))
	}
	if len(m.attentionFilter) > 0 {
		var names []string
		if _, ok := m.attentionFilter[data.AttentionWaiting]; ok {
			names = append(names, "waiting")
		}
		if _, ok := m.attentionFilter[data.AttentionActive]; ok {
			names = append(names, "active")
		}
		if _, ok := m.attentionFilter[data.AttentionStale]; ok {
			names = append(names, "stale")
		}
		if _, ok := m.attentionFilter[data.AttentionIdle]; ok {
			names = append(names, "idle")
		}
		if _, ok := m.attentionFilter[data.AttentionInterrupted]; ok {
			names = append(names, "interrupted")
		}
		left += "  " + styles.ActiveBadgeStyle.Render("! "+strings.Join(names, ", "))
	}
	if m.filterPlans {
		left += "  " + styles.ActiveBadgeStyle.Render("! plans")
	}
	if len(m.filterWorkStatus) > 0 {
		var wsNames []string
		if _, ok := m.filterWorkStatus[data.WorkStatusIncomplete]; ok {
			wsNames = append(wsNames, "incomplete")
		}
		if _, ok := m.filterWorkStatus[data.WorkStatusComplete]; ok {
			wsNames = append(wsNames, "complete")
		}
		left += "  " + styles.ActiveBadgeStyle.Render("! "+strings.Join(wsNames, ", "))
	}
	if m.statusErr != "" {
		left += "  " + styles.ErrorStyle.Render(m.statusErr)
	} else if m.statusInfo != "" {
		left += "  " + styles.SuccessStyle.Render(m.statusInfo)
	} else if !m.reindexing {
		// Show last reindex time as a subtle hint.
		if t := data.LastReindexTime(); !t.IsZero() {
			left += "  " + styles.DimStyle.Render("indexed "+components.RelativeTime(t.UTC().Format(time.RFC3339))+" · r reindex")
		} else {
			left += "  " + styles.DimStyle.Render("r reindex")
		}
	}

	version := styles.DimStyle.Render(Version)

	// Right: context-sensitive keybinding hints from help.Model.
	right := m.help.ShortView()

	// If hints + left + version exceed width, drop the hints entirely
	// to avoid wrapping. Byte-level truncation corrupts ANSI codes.
	usedWidth := lipgloss.Width(left) + lipgloss.Width(version) + footerGapReserve
	if usedWidth+lipgloss.Width(right) > m.width {
		right = ""
	}

	// Use m.width-2 so the footer totals m.width-1 characters, avoiding
	// exact-terminal-width lines that could autowrap.
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - lipgloss.Width(version) - 2
	if gap < 1 {
		gap = 1
	}

	line := left + strings.Repeat(" ", gap) + right + " " + version
	return styles.StatusBarStyle.Render(line)
}

// reindexOverlayHeight is the number of visible log lines in the
// reindex overlay viewport.
const reindexOverlayHeight = 14

// maxReindexLogLines caps the log buffer to prevent unbounded memory
// growth during verbose PTY output.
const maxReindexLogLines = 200

// cancelReindex aborts the running chronicle reindex.
func (m Model) cancelReindex() (tea.Model, tea.Cmd) {
	if m.reindexCancel != nil {
		m.reindexCancel.Cancel()
	}
	m.reindexing = false
	m.reindexCancel = nil
	m.reindexLog = nil
	m.statusInfo = statusReindexCancelled
	return m, clearStatusAfter(2 * time.Second)
}

// handleReindexClick checks whether a mouse click hit the cancel button
// in the reindex overlay and cancels if so.
func (m *Model) handleReindexClick(msg tea.MouseReleaseMsg) {
	innerW := m.reindexInnerWidth()
	overlayW := innerW + overlayBorderPadding
	// title (with bottom padding 1) + viewport + cancel row + padding top/bottom from border
	overlayH := 1 + 1 + reindexOverlayHeight + 1 + 4

	startX := (m.width - overlayW) / 2
	startY := (m.height - overlayH) / 2

	// The cancel button is on the last content row before bottom border/padding.
	btnY := startY + overlayH - 3 // 1 border + 1 padding from bottom
	btnLabel := cancelBtnLabel
	btnW := lipgloss.Width(btnLabel)
	btnX := startX + (overlayW-btnW)/2

	mouse := msg.Mouse()
	if mouse.Y == btnY && mouse.X >= btnX && mouse.X < btnX+btnW {
		if m.reindexCancel != nil {
			m.reindexCancel.Cancel()
		}
		m.reindexing = false
		m.reindexCancel = nil
		m.reindexLog = nil
		m.statusInfo = statusReindexCancelled
	}
}

// reindexInnerWidth returns the content width for the reindex overlay.
func (m Model) reindexInnerWidth() int {
	maxW := m.width * 3 / 4
	maxW = min(maxW, m.width-4)
	maxW = max(maxW, 44)
	// OverlayStyle has Padding(1,2) + RoundedBorder (1 char each side) = 6 total horizontal
	return maxW
}

// updateReindexViewport rebuilds the viewport content from reindexLog.
// Auto-scrolls to bottom only if the user hasn't scrolled up.
func (m *Model) updateReindexViewport() {
	innerW := m.reindexInnerWidth()

	// Derive a width-constrained style so each log line is
	// left-aligned and fills the viewport instead of floating.
	lineStyle := styles.DimStyle.Width(innerW)

	var sb strings.Builder
	for i, l := range m.reindexLog {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(lineStyle.Render(l))
	}

	m.reindexVP.SetWidth(innerW)
	m.reindexVP.SetHeight(reindexOverlayHeight)

	// Only auto-scroll if already at or past the bottom.
	wasAtBottom := m.reindexVP.AtBottom()
	m.reindexVP.SetContent(sb.String())
	if wasAtBottom {
		m.reindexVP.GotoBottom()
	}
}

// renderReindexOverlay draws a bordered overlay with a scrollable viewport
// of streaming log lines and a cancel button, centered on screen.
// Follows the same OverlayStyle + lipgloss.Place pattern as other dialogs.
func (m Model) renderReindexOverlay() string {
	maxW := m.width * 3 / 4
	maxW = min(maxW, m.width-4)
	maxW = max(maxW, 44)

	title := styles.OverlayTitleStyle.Render(m.spinner.View() + " Reindexing Sessions")

	cancelBtn := lipgloss.NewStyle().
		Foreground(styles.ColorPrimary).
		Render(cancelBtnLabel)

	body := title + m.reindexVP.View() + "\n" +
		lipgloss.PlaceHorizontal(maxW, lipgloss.Center, cancelBtn)

	overlay := styles.OverlayStyle.
		Width(maxW).
		Render(body)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
}

// ---------------------------------------------------------------------------
// Hidden session filtering
// ---------------------------------------------------------------------------

// filterHiddenSessions removes hidden sessions from a flat list unless
// showHidden mode is active.
func (m *Model) filterHiddenSessions(sessions []data.Session) []data.Session {
	if m.showHidden || len(m.hiddenSet) == 0 {
		return sessions
	}
	filtered := make([]data.Session, 0, len(sessions))
	for _, s := range sessions {
		if _, ok := m.hiddenSet[s.ID]; !ok {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// filterHiddenGroups removes hidden sessions from grouped results unless
// showHidden mode is active. Empty groups are dropped.
func (m *Model) filterHiddenGroups(groups []data.SessionGroup) []data.SessionGroup {
	if m.showHidden || len(m.hiddenSet) == 0 {
		return groups
	}
	filtered := make([]data.SessionGroup, 0, len(groups))
	for _, g := range groups {
		var sessions []data.Session
		for _, s := range g.Sessions {
			if _, ok := m.hiddenSet[s.ID]; !ok {
				sessions = append(sessions, s)
			}
		}
		if len(sessions) > 0 {
			g.Sessions = sessions
			g.Count = len(sessions)
			filtered = append(filtered, g)
		}
	}
	return filtered
}

// ---------------------------------------------------------------------------
// Favorited session filtering
// ---------------------------------------------------------------------------

// filterFavoritedSessions returns only favorited sessions when showFavorited
// mode is active. When the filter is off, all sessions pass through.
func (m *Model) filterFavoritedSessions(sessions []data.Session) []data.Session {
	if !m.showFavorited {
		return sessions
	}
	filtered := make([]data.Session, 0, len(sessions))
	for _, s := range sessions {
		if _, ok := m.favoritedSet[s.ID]; ok {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// filterFavoritedGroups returns only favorited sessions within each group
// when showFavorited mode is active. Empty groups are dropped.
func (m *Model) filterFavoritedGroups(groups []data.SessionGroup) []data.SessionGroup {
	if !m.showFavorited {
		return groups
	}
	filtered := make([]data.SessionGroup, 0, len(groups))
	for _, g := range groups {
		var sessions []data.Session
		for _, s := range g.Sessions {
			if _, ok := m.favoritedSet[s.ID]; ok {
				sessions = append(sessions, s)
			}
		}
		if len(sessions) > 0 {
			g.Sessions = sessions
			g.Count = len(sessions)
			filtered = append(filtered, g)
		}
	}
	return filtered
}

// filterAttentionSessions removes sessions that don't match the attention
// filter. When attentionFilter is empty, all sessions pass through.
func (m *Model) filterAttentionSessions(sessions []data.Session) []data.Session {
	if len(m.attentionFilter) == 0 || len(m.attentionMap) == 0 {
		return sessions
	}
	filtered := make([]data.Session, 0, len(sessions))
	for _, s := range sessions {
		status := m.attentionMap[s.ID]
		if _, ok := m.attentionFilter[status]; ok {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// filterAttentionGroups removes sessions that don't match the attention
// filter from grouped results. Empty groups are dropped.
func (m *Model) filterAttentionGroups(groups []data.SessionGroup) []data.SessionGroup {
	if len(m.attentionFilter) == 0 || len(m.attentionMap) == 0 {
		return groups
	}
	filtered := make([]data.SessionGroup, 0, len(groups))
	for _, g := range groups {
		var sessions []data.Session
		for _, s := range g.Sessions {
			status := m.attentionMap[s.ID]
			if _, ok := m.attentionFilter[status]; ok {
				sessions = append(sessions, s)
			}
		}
		if len(sessions) > 0 {
			g.Sessions = sessions
			g.Count = len(sessions)
			filtered = append(filtered, g)
		}
	}
	return filtered
}

// ---------------------------------------------------------------------------
// Plan session filtering
// ---------------------------------------------------------------------------

// filterPlanSessions removes sessions that don't have a plan.md file when
// filterPlans is active. When filterPlans is false, all sessions pass through.
func (m *Model) filterPlanSessions(sessions []data.Session) []data.Session {
	if !m.filterPlans || len(m.planMap) == 0 {
		return sessions
	}
	filtered := make([]data.Session, 0, len(sessions))
	for _, s := range sessions {
		if m.planMap[s.ID] {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// filterPlanGroups removes sessions without a plan.md file from grouped
// results when filterPlans is active. Empty groups are dropped.
func (m *Model) filterPlanGroups(groups []data.SessionGroup) []data.SessionGroup {
	if !m.filterPlans || len(m.planMap) == 0 {
		return groups
	}
	filtered := make([]data.SessionGroup, 0, len(groups))
	for _, g := range groups {
		var sessions []data.Session
		for _, s := range g.Sessions {
			if m.planMap[s.ID] {
				sessions = append(sessions, s)
			}
		}
		if len(sessions) > 0 {
			g.Sessions = sessions
			g.Count = len(sessions)
			filtered = append(filtered, g)
		}
	}
	return filtered
}

// ---------------------------------------------------------------------------
// Work status session filtering
// ---------------------------------------------------------------------------

// filterWorkStatusSessions removes sessions that don't match the active
// work status filter set. When filterWorkStatus is empty, all sessions
// pass through.
func (m *Model) filterWorkStatusSessions(sessions []data.Session) []data.Session {
	if len(m.filterWorkStatus) == 0 || len(m.workStatusMap) == 0 {
		return sessions
	}
	filtered := make([]data.Session, 0, len(sessions))
	for _, s := range sessions {
		if result, ok := m.workStatusMap[s.ID]; ok {
			if _, match := m.filterWorkStatus[result.Status]; match {
				filtered = append(filtered, s)
			}
		}
	}
	return filtered
}

// filterWorkStatusGroups removes sessions that don't match the active
// work status filter set from grouped results. Empty groups are dropped.
func (m *Model) filterWorkStatusGroups(groups []data.SessionGroup) []data.SessionGroup {
	if len(m.filterWorkStatus) == 0 || len(m.workStatusMap) == 0 {
		return groups
	}
	filtered := make([]data.SessionGroup, 0, len(groups))
	for _, g := range groups {
		var sessions []data.Session
		for _, s := range g.Sessions {
			if result, ok := m.workStatusMap[s.ID]; ok {
				if _, match := m.filterWorkStatus[result.Status]; match {
					sessions = append(sessions, s)
				}
			}
		}
		if len(sessions) > 0 {
			g.Sessions = sessions
			g.Count = len(sessions)
			filtered = append(filtered, g)
		}
	}
	return filtered
}

// sortByAttention re-sorts the session slice by attention status when the
// current sort field is SortByAttention. Attention priority is:
// Waiting (3) > Active (2) > Stale (1) > Idle (0). Sessions with higher
// attention priority sort first (descending by default).
func (m *Model) sortByAttention(sessions []data.Session) {
	if m.sort.Field != data.SortByAttention || len(m.attentionMap) == 0 {
		return
	}
	slices.SortStableFunc(sessions, func(a, b data.Session) int {
		pa := attentionPriority(m.attentionMap[a.ID])
		pb := attentionPriority(m.attentionMap[b.ID])
		if m.sort.Order == data.Ascending {
			return cmp.Compare(pa, pb)
		}
		return cmp.Compare(pb, pa) // descending: higher priority first
	})
}

// attentionPriority returns a numeric priority for sorting. Higher values
// represent statuses that need more urgent attention.
func attentionPriority(status data.AttentionStatus) int {
	switch status {
	case data.AttentionWaiting:
		return 3
	case data.AttentionInterrupted:
		return 2
	case data.AttentionActive:
		return 2
	case data.AttentionStale:
		return 1
	default: // AttentionIdle
		return 0
	}
}

// hiddenCount returns the total number of hidden sessions.
func (m *Model) hiddenCount() int {
	return len(m.hiddenSet)
}

// ---------------------------------------------------------------------------
// Sort / pivot cycling
// ---------------------------------------------------------------------------

var sortFields = []data.SortField{
	data.SortByUpdated,
	data.SortByFolder,
	data.SortByName,
	data.SortByAttention,
}

func (m *Model) cycleSort() {
	for i, f := range sortFields {
		if f == m.sort.Field {
			m.sort.Field = sortFields[(i+1)%len(sortFields)]
			return
		}
	}
	m.sort.Field = data.SortByUpdated
}

func (m *Model) toggleSortOrder() {
	if m.sort.Order == data.Descending {
		m.sort.Order = data.Ascending
	} else {
		m.sort.Order = data.Descending
	}
}

var pivotModes = []string{pivotNone, pivotFolder, pivotRepo, pivotBranch, pivotDate}

func (m *Model) cyclePivot() {
	for i, p := range pivotModes {
		if p == m.pivot {
			m.pivot = pivotModes[(i+1)%len(pivotModes)]
			m.pivotOrder = defaultPivotOrder(m.pivot)
			return
		}
	}
	m.pivot = pivotNone
	m.pivotOrder = data.Ascending
}

// defaultPivotOrder returns the natural default sort direction for a pivot.
// Date defaults to descending (newest first); others to ascending (A-Z).
func defaultPivotOrder(pivot string) data.SortOrder {
	if pivot == pivotDate {
		return data.Descending
	}
	return data.Ascending
}

func (m *Model) togglePivotOrder() {
	if m.pivotOrder == data.Descending {
		m.pivotOrder = data.Ascending
	} else {
		m.pivotOrder = data.Descending
	}
}

// ---------------------------------------------------------------------------
// Time range
// ---------------------------------------------------------------------------

func (m *Model) setTimeRange(tr string) tea.Cmd {
	m.timeRange = tr
	m.filter.Since = timeRangeToSince(tr)
	return m.loadSessionsCmd()
}

// ---------------------------------------------------------------------------
// Sort display
// ---------------------------------------------------------------------------

func sortDisplayLabel(f data.SortField) string {
	switch f {
	case data.SortByFolder:
		return pivotFolder
	case data.SortByName:
		return "name"
	case data.SortByAttention:
		return "attention"
	default:
		return "updated"
	}
}

// ---------------------------------------------------------------------------
// Launch flow
// ---------------------------------------------------------------------------

func (m *Model) launchSelected() tea.Cmd {
	return m.launchWithMode(m.cfg.EffectiveLaunchMode())
}

// launchMultiple opens multiple sessions at once. It resolves which sessions
// to open based on the current state:
//  1. If sessions are explicitly selected (checkmarked), open those.
//  2. Else if cursor is on a folder, open all sessions under that folder.
//  3. Else fall back to opening the single cursor session (same as Enter).
//
// In-place launch mode is not supported for multi-open; external mode is forced.
func (m *Model) launchMultiple() tea.Cmd {
	var sessions []data.Session

	if sel := m.sessionList.SelectedSessions(); len(sel) > 0 {
		sessions = sel
	} else if m.sessionList.IsFolderSelected() {
		sessions = m.sessionList.FolderSessions()
	} else {
		// No selections, not on folder — just launch cursor session.
		return m.launchSelected()
	}

	if len(sessions) == 0 {
		return nil
	}

	// Force external launch (never in-place for multi-open).
	mode := m.cfg.EffectiveLaunchMode()
	if mode == config.LaunchModeInPlace {
		mode = config.LaunchModeTab
	}

	return m.batchLaunchSessions(sessions, mode)
}

// launchMultipleWithMode opens all selected sessions with the given launch mode.
func (m *Model) launchMultipleWithMode(mode string) tea.Cmd {
	sessions := m.sessionList.SelectedSessions()
	if len(sessions) == 0 {
		return nil
	}
	return m.batchLaunchSessions(sessions, mode)
}

// maxBatchLaunch is the maximum number of sessions that can be launched
// simultaneously via multi-select. This prevents accidental resource
// exhaustion when a user selects hundreds of sessions.
const maxBatchLaunch = 50

// batchLaunchSessions builds launch commands for each session, clears the
// selection state, and returns a tea.Batch of all commands. At most
// maxBatchLaunch sessions are launched to prevent resource exhaustion.
func (m *Model) batchLaunchSessions(sessions []data.Session, mode string) tea.Cmd {
	if len(sessions) > maxBatchLaunch {
		sessions = sessions[:maxBatchLaunch]
		m.statusInfo = fmt.Sprintf("Launching first %d sessions (limit)", maxBatchLaunch)
	}
	var cmds []tea.Cmd
	for _, sess := range sessions {
		cmd := m.resolveShellAndLaunchDirect(sess.ID, sess.Cwd, mode)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Keep selections intact after launch — user can deselect with 'd'.
	m.statusInfo = ""

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// updateSelectionStatus sets the status bar to reflect the current selection count.
func (m *Model) updateSelectionStatus() {
	if n := m.sessionList.SelectionCount(); n > 0 {
		m.statusInfo = fmt.Sprintf("%d selected — O to open", n)
	} else {
		m.statusInfo = ""
	}
}

// launchNewSession opens a fresh copilot session (no --resume) in the given cwd.
func (m *Model) launchNewSession(cwd, mode string) tea.Cmd {
	if mode == config.LaunchModeInPlace {
		return m.launchInPlace("", cwd)
	}
	return m.resolveShellAndLaunch("", cwd, mode)
}

// launchWithMode opens the selected session using the specified launch mode.
func (m *Model) launchWithMode(mode string) tea.Cmd {
	sess, ok := m.sessionList.Selected()
	if !ok {
		return nil
	}
	if mode == config.LaunchModeInPlace {
		return m.launchInPlace(sess.ID, sess.Cwd)
	}
	return m.resolveShellAndLaunch(sess.ID, sess.Cwd, mode)
}

// launchNewInFolder starts a fresh Copilot session (no session ID) with the
// working directory set based on the selected folder's pivot type.
func (m *Model) launchNewInFolder(mode string) tea.Cmd {
	cwd := m.sessionList.SelectedFolderCwd()
	if cwd == "" {
		return nil
	}
	return m.resolveShellAndLaunch("", cwd, mode)
}

// resolveShellAndLaunch picks the right shell (configured, single, or picker)
// and launches the session externally. Shared by launchWithMode and
// launchNewSession to avoid duplicating shell-resolution logic.
func (m *Model) resolveShellAndLaunch(sessionID, cwd, mode string) tea.Cmd {
	launchStyle := launchStyleForMode(mode)

	if m.cfg.DefaultShell != "" {
		sh := m.findShellByName(m.cfg.DefaultShell)
		if sh.Path == "" {
			m.statusErr = fmt.Sprintf("Cannot launch: shell %q not found", m.cfg.DefaultShell)
			return nil
		}
		return m.launchExternal(sh, sessionID, cwd, launchStyle)
	}
	if len(m.shells) <= 1 {
		sh := platform.DefaultShell()
		if len(m.shells) == 1 {
			sh = m.shells[0]
		}
		if sh.Path == "" {
			m.statusErr = "Cannot launch: no shell detected on this system"
			return nil
		}
		return m.launchExternal(sh, sessionID, cwd, launchStyle)
	}
	m.pendingLaunchMode = mode
	m.shellPicker.SetShells(m.shells, m.cfg.DefaultShell)
	m.state = stateShellPicker
	return nil
}

// findShellByName returns the detected ShellInfo matching name, or the
// platform default if no match is found.
func (m *Model) findShellByName(name string) platform.ShellInfo {
	for _, sh := range m.shells {
		if sh.Name == name {
			return sh
		}
	}
	return platform.DefaultShell()
}

// resolveShellAndLaunchDirect launches a session without showing the shell
// picker overlay. It uses the configured default shell, or the first available
// shell, or the platform default. Used for multi-session batch launches where
// showing an interactive picker per-session is impractical.
func (m *Model) resolveShellAndLaunchDirect(sessionID, cwd, mode string) tea.Cmd {
	launchStyle := launchStyleForMode(mode)

	var sh platform.ShellInfo
	if m.cfg.DefaultShell != "" {
		sh = m.findShellByName(m.cfg.DefaultShell)
	} else if len(m.shells) > 0 {
		sh = m.shells[0]
	} else {
		sh = platform.DefaultShell()
	}

	if sh.Path == "" {
		m.statusErr = "Cannot launch: no shell detected on this system"
		return nil
	}

	return m.launchExternal(sh, sessionID, cwd, launchStyle)
}

// launchInPlace creates a tea.ExecProcess command that exits alt-screen,
// runs the Copilot CLI session resume in the current terminal, and quits
// the TUI when the session ends.
func (m *Model) launchInPlace(sessionID, cwd string) tea.Cmd {
	cfg := platform.ResumeConfig{
		YoloMode:      m.cfg.YoloMode,
		Agent:         m.cfg.Agent,
		Model:         m.cfg.Model,
		CustomCommand: m.cfg.CustomCommand,
		Cwd:           cwd,
	}
	cmd, err := platform.NewResumeCmd(sessionID, cfg)
	if err != nil {
		m.statusErr = err.Error()
		return nil
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return sessionExitMsg{err: err}
	})
}

// launchStyleForMode maps a config launch mode to a platform LaunchStyle constant.
func launchStyleForMode(mode string) string {
	switch mode {
	case config.LaunchModeWindow:
		return platform.LaunchStyleWindow
	case config.LaunchModePane:
		return platform.LaunchStylePane
	default:
		return platform.LaunchStyleTab
	}
}

// launchExternal opens the session in a new tab, window, or pane depending on launchStyle.
func (m *Model) launchExternal(shell platform.ShellInfo, sessionID, cwd, launchStyle string) tea.Cmd {
	cfg := platform.ResumeConfig{
		YoloMode:      m.cfg.YoloMode,
		Agent:         m.cfg.Agent,
		Model:         m.cfg.Model,
		Terminal:      m.cfg.DefaultTerminal,
		CustomCommand: m.cfg.CustomCommand,
		Cwd:           cwd,
		LaunchStyle:   launchStyle,
		PaneDirection: m.cfg.EffectivePaneDirection(),
	}
	return func() tea.Msg {
		if err := platform.LaunchSession(shell, sessionID, cfg); err != nil {
			detail := fmt.Sprintf("launch failed: %v (shell=%s, terminal=%s)",
				err, shell.Name, cfg.Terminal)
			return dataErrorMsg{err: errors.New(detail)}
		}
		return nil
	}
}

func (m Model) selectedSessionID() string {
	if sess, ok := m.sessionList.Selected(); ok {
		return sess.ID
	}
	return ""
}

func (m Model) selectedSessionCwd() string {
	if sess, ok := m.sessionList.Selected(); ok {
		return sess.Cwd
	}
	return ""
}

// ---------------------------------------------------------------------------
// Cleanup
// ---------------------------------------------------------------------------

func (m *Model) closeStore() {
	if m.copilotClient != nil {
		m.copilotClient.Close()
		m.copilotClient = nil
	}
	if m.store != nil {
		_ = m.store.Close()
		m.store = nil
	}
}

// ---------------------------------------------------------------------------
// Async commands (tea.Cmd factories)
// ---------------------------------------------------------------------------

func openStoreCmd() tea.Cmd {
	return func() tea.Msg {
		store, err := data.Open()
		if err != nil {
			return storeErrorMsg{err: err}
		}
		return storeOpenedMsg{store: store}
	}
}

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

func detectShellsCmd() tea.Cmd {
	return func() tea.Msg {
		return shellsDetectedMsg{shells: platform.DetectShells()}
	}
}

func detectTerminalsCmd() tea.Cmd {
	return func() tea.Msg {
		return terminalsDetectedMsg{terminals: platform.DetectTerminals()}
	}
}

func (m Model) loadSessionsCmd() tea.Cmd {
	store := m.store
	filter := m.filter
	sortOpts := m.sort
	limit := m.cfg.MaxSessions
	pivot := m.pivot
	pivotOrd := m.pivotOrder

	return func() tea.Msg {
		if store == nil {
			return dataErrorMsg{err: errors.New("store not available")}
		}
		if pivot != pivotNone {
			pf := pivotFieldFromString(pivot)
			groups, err := store.GroupSessions(pf, filter, sortOpts, limit)
			if err != nil {
				return dataErrorMsg{err: err}
			}
			// When sorting by updated time, reorder groups so the most
			// recently active group appears first; otherwise sort groups
			// alphabetically by their pivot label.
			if sortOpts.Field == data.SortByUpdated {
				sortGroupsByLatest(groups, sortOpts.Order)
			} else {
				sortGroupsByLabel(groups, pivotOrd)
			}
			return groupsLoadedMsg{groups: groups}
		}
		sessions, err := store.ListSessions(filter, sortOpts, limit)
		if err != nil {
			return dataErrorMsg{err: err}
		}
		return sessionsLoadedMsg{sessions: sessions}
	}
}

// attentionRefreshInterval controls how often the attention scanner polls
// the session-state directories.
const attentionRefreshInterval = 30 * time.Second

// scanAttentionQuickCmd runs a fast first-pass scan that only checks lock
// files for live sessions. Dead sessions get AttentionIdle without reading
// events.jsonl. This gets dots visible immediately on startup.
func (m Model) scanAttentionQuickCmd() tea.Cmd {
	threshold := m.cfg.EffectiveAttentionThreshold()
	wr := m.cfg.WorkspaceRecovery
	return func() tea.Msg {
		statuses := data.ScanAttentionQuick(threshold, wr)
		return attentionQuickScannedMsg{statuses: statuses}
	}
}

// scanAttentionCmd runs the full session-state directory scanner in the
// background and returns an attentionScannedMsg with the results.
func (m Model) scanAttentionCmd() tea.Cmd {
	threshold := m.cfg.EffectiveAttentionThreshold()
	wr := m.cfg.WorkspaceRecovery
	return func() tea.Msg {
		statuses := data.ScanAttention(threshold, wr)
		return attentionScannedMsg{statuses: statuses}
	}
}

// attentionStatusForSession returns the attention status for a given session
// ID, defaulting to AttentionIdle if not in the map.
func (m Model) attentionStatusForSession(sessionID string) data.AttentionStatus {
	if m.attentionMap == nil {
		return data.AttentionIdle
	}
	if status, ok := m.attentionMap[sessionID]; ok {
		return status
	}
	return data.AttentionIdle
}

// scheduleAttentionTick schedules the next periodic attention scan.
func (m Model) scheduleAttentionTick() tea.Cmd {
	return tea.Tick(attentionRefreshInterval, func(time.Time) tea.Msg {
		return attentionTickMsg{}
	})
}

// scanPlansCmd checks for plan.md files across all session-state directories.
func (m Model) scanPlansCmd() tea.Cmd {
	return func() tea.Msg {
		plans := data.ScanAllPlans()
		return plansScannedMsg{plans: plans}
	}
}

// loadPlanContentCmd reads the plan.md content for a specific session.
func (m Model) loadPlanContentCmd(sessionID string) tea.Cmd {
	return func() tea.Msg {
		content, err := data.ReadPlanContent(sessionID)
		return planContentMsg{sessionID: sessionID, content: content, err: err}
	}
}

// scanWorkStatusQuickCmd runs a quick work status classification using the
// plan map (NoPlan vs Unknown) without reading plan.md content.
func (m *Model) scanWorkStatusQuickCmd() tea.Cmd {
	planMap := m.planMap
	visibleIDs := m.sessionList.VisibleSessionIDs()
	return func() tea.Msg {
		// Build a filtered planMap with only visible sessions.
		filtered := make(map[string]bool, len(visibleIDs))
		for _, id := range visibleIDs {
			if hasPlan, ok := planMap[id]; ok {
				filtered[id] = hasPlan
			}
		}
		statuses := data.ScanWorkStatusQuick(filtered)
		return workStatusQuickScannedMsg{statuses: statuses}
	}
}

// scanWorkStatusCmd runs the full work status scan for sessions that have
// plan.md files, parsing task completion state from the plan content.
func (m *Model) scanWorkStatusCmd() tea.Cmd {
	visibleSet := make(map[string]struct{})
	for _, id := range m.sessionList.VisibleSessionIDs() {
		visibleSet[id] = struct{}{}
	}
	planIDs := make([]string, 0, len(m.planMap))
	for id, hasPlan := range m.planMap {
		if hasPlan {
			if _, visible := visibleSet[id]; visible {
				planIDs = append(planIDs, id)
			}
		}
	}
	return func() tea.Msg {
		statuses := data.ScanWorkStatus(planIDs, nil)
		return workStatusScannedMsg{statuses: statuses}
	}
}

// maxAIAnalysisBatch limits the number of sessions analysed per AI pass
// to keep costs and latency reasonable.
const maxAIAnalysisBatch = 5

// scanWorkStatusAICmd runs AI-enhanced completion analysis for sessions
// whose local scan found incomplete work. It calls
// copilotClient.AnalyzeCompletion for each (up to maxAIAnalysisBatch)
// and returns a workStatusAIScannedMsg with the results.
//
// Returns nil if the Copilot SDK is unavailable or no sessions qualify.
func (m *Model) scanWorkStatusAICmd() tea.Cmd {
	// Lazy-init the copilot client if needed.
	if m.copilotClient == nil && m.store != nil {
		m.copilotClient = copilot.New(m.store)
	}
	client := m.copilotClient
	if client == nil {
		return nil
	}

	// Build visible set for filtering.
	visibleSet := make(map[string]struct{})
	for _, id := range m.sessionList.VisibleSessionIDs() {
		visibleSet[id] = struct{}{}
	}

	// Collect visible sessions with incomplete work that have plan content.
	// Only capture session IDs here — file I/O (ReadPlanContent) is
	// deferred to the background closure to avoid blocking the UI thread.
	var candidateIDs []string
	for id, result := range m.workStatusMap {
		if result.Status != data.WorkStatusIncomplete {
			continue
		}
		// Only scan visible sessions.
		if _, visible := visibleSet[id]; !visible {
			continue
		}
		candidateIDs = append(candidateIDs, id)
		if len(candidateIDs) >= maxAIAnalysisBatch {
			break
		}
	}
	if len(candidateIDs) == 0 {
		return nil
	}

	// Cancel any previous AI analysis and create a fresh cancellable context.
	if m.workStatusAICancel != nil {
		m.workStatusAICancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	m.workStatusAICancel = cancel

	return func() tea.Msg {
		results := make(map[string]*copilot.CompletionAnalysis, len(candidateIDs))
		for _, id := range candidateIDs {
			// Check for cancellation between iterations to allow the user
			// to interrupt long-running AI analysis batches.
			if ctx.Err() != nil {
				slog.Debug("AI work status analysis cancelled", "completed", len(results))
				break
			}
			content, err := data.ReadPlanContent(id)
			if err != nil || content == "" {
				continue
			}
			// Use the outer ctx for cancellation awareness, but pass
			// context.Background() to AnalyzeCompletion to prevent
			// JSON-RPC pipe corruption (same pattern as copilotSearchCmd).
			analysis, err := client.AnalyzeCompletion(context.Background(), id, content)
			if err != nil {
				slog.Debug("AI completion analysis failed",
					"session", id, "error", err)
				continue
			}
			if analysis != nil {
				results[id] = analysis
			}
		}
		return workStatusAIScannedMsg{analyses: results}
	}
}

// writeContinuationPlansCmd writes or updates the "Remaining Work" section
// in plan.md for the given sessions using their RemainingItems from the
// work status map.
func (m *Model) writeContinuationPlansCmd(sessionIDs []string) tea.Cmd {
	// Snapshot the data we need — the closure must not read from m.
	type planData struct {
		id        string
		remaining []string
		summary   string
	}
	var plans []planData
	for _, id := range sessionIDs {
		result, ok := m.workStatusMap[id]
		if !ok || len(result.RemainingItems) == 0 {
			continue
		}
		plans = append(plans, planData{
			id:        id,
			remaining: result.RemainingItems,
			summary:   result.Detail,
		})
	}
	if len(plans) == 0 {
		return nil
	}

	return func() tea.Msg {
		var updated int
		for _, p := range plans {
			if err := data.WriteContinuationPlan(p.id, p.remaining, p.summary); err != nil {
				slog.Debug("failed to write continuation plan",
					"session", p.id, "error", err)
				continue
			}
			updated++
		}
		return continuationPlanCreatedMsg{updated: updated}
	}
}

// completeWorkStatusScan marks the work-status scan chain as finished,
// shows a summary in the status bar, and reloads sessions when a
// work-status filter is active.
func (m *Model) completeWorkStatusScan() tea.Cmd {
	m.workStatusScanned = true
	wasScanning := m.workStatusScanning
	m.workStatusScanning = false

	if wasScanning {
		// Count incomplete/complete from the work status map.
		var incomplete, complete int
		for _, r := range m.workStatusMap {
			switch r.Status {
			case data.WorkStatusIncomplete:
				incomplete++
			case data.WorkStatusComplete:
				complete++
			case data.WorkStatusUnknown, data.WorkStatusNoPlan, data.WorkStatusAnalyzing, data.WorkStatusError:
				// Not counted in summary.
			}
		}
		m.statusInfo = fmt.Sprintf("Work scan complete (%d incomplete, %d complete)", incomplete, complete)
	}

	var cmds []tea.Cmd
	if wasScanning {
		cmds = append(cmds, clearStatusAfter(3*time.Second))
	}
	// When a work-status filter is active, reload sessions so the list
	// reflects the new scan results.
	if wasScanning && len(m.filterWorkStatus) > 0 {
		cmds = append(cmds, m.loadSessionsCmd())
	}
	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}

// handleJumpNextAttention moves the cursor to the next session with
// AttentionWaiting status, wrapping around to the beginning if needed.
func (m Model) handleJumpNextAttention() (tea.Model, tea.Cmd) {
	if len(m.attentionMap) == 0 {
		return m, nil
	}
	idx := m.sessionList.FindNextWaiting(m.attentionMap)
	if idx < 0 {
		return m, nil
	}
	m.sessionList.SetCursor(idx)
	m.detailVersion++
	return m, m.loadSelectedDetailCmd()
}

// waitingCount returns the number of sessions with AttentionWaiting status.
func (m Model) waitingCount() int {
	count := 0
	for _, status := range m.attentionMap {
		if status == data.AttentionWaiting {
			count++
		}
	}
	return count
}

// interruptedCount returns the number of sessions with AttentionInterrupted status.
func (m Model) interruptedCount() int {
	count := 0
	for _, status := range m.attentionMap {
		if status == data.AttentionInterrupted {
			count++
		}
	}
	return count
}

// handleResumeInterrupted collects all interrupted sessions and batch-launches them.
func (m Model) handleResumeInterrupted() (tea.Model, tea.Cmd) {
	if len(m.attentionMap) == 0 {
		return m, nil
	}

	// Collect interrupted session IDs.
	var ids []string
	for id, status := range m.attentionMap {
		if status == data.AttentionInterrupted {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		m.statusInfo = "No interrupted sessions"
		return m, nil
	}

	// Match to loaded sessions (need Cwd for launch).
	var sessions []data.Session
	for _, sess := range m.sessions {
		for _, id := range ids {
			if sess.ID == id {
				sessions = append(sessions, sess)
				break
			}
		}
	}
	if len(sessions) == 0 {
		m.statusInfo = "No interrupted sessions in current view"
		return m, nil
	}

	// Force external mode (never in-place for batch).
	mode := m.cfg.EffectiveLaunchMode()
	if mode == config.LaunchModeInPlace {
		mode = config.LaunchModeTab
	}

	m.statusInfo = fmt.Sprintf("Resuming %d interrupted sessions", len(sessions))
	cmd := m.batchLaunchSessions(sessions, mode)
	return m, cmd
}

// attentionStatusCounts returns the number of sessions per attention status.
func (m Model) attentionStatusCounts() map[data.AttentionStatus]int {
	counts := make(map[data.AttentionStatus]int)
	for _, status := range m.attentionMap {
		counts[status]++
	}
	return counts
}

// planSessionCount returns the number of sessions that have a plan doc.
func (m Model) planSessionCount() int {
	n := 0
	for _, has := range m.planMap {
		if has {
			n++
		}
	}
	return n
}

// workStatusCounts returns the number of sessions in each work status category.
func (m Model) workStatusCounts() map[data.WorkStatus]int {
	counts := make(map[data.WorkStatus]int)
	for _, r := range m.workStatusMap {
		counts[r.Status]++
	}
	return counts
}

const deepSearchDelay = 300 * time.Millisecond

// scheduleDeepSearch returns a tea.Cmd that fires a deepSearchTickMsg after
// a short delay. The version counter lets the handler ignore stale ticks.
func (m Model) scheduleDeepSearch(version int) tea.Cmd {
	return tea.Tick(deepSearchDelay, func(time.Time) tea.Msg {
		return deepSearchTickMsg{version: version}
	})
}

// deepSearchCmd fires a deep (all-metadata) search and returns the results
// as a deepSearchResultMsg with the associated version counter.
func (m Model) deepSearchCmd(version int) tea.Cmd {
	store := m.store
	filter := m.filter
	filter.DeepSearch = true
	sortOpts := m.sort
	limit := m.cfg.MaxSessions
	pivot := m.pivot
	pivotOrd := m.pivotOrder

	return func() tea.Msg {
		if store == nil {
			return nil
		}
		if pivot != pivotNone {
			pf := pivotFieldFromString(pivot)
			groups, err := store.GroupSessions(pf, filter, sortOpts, limit)
			if err != nil {
				return dataErrorMsg{err: err}
			}
			if sortOpts.Field == data.SortByUpdated {
				sortGroupsByLatest(groups, sortOpts.Order)
			} else {
				sortGroupsByLabel(groups, pivotOrd)
			}
			return deepSearchResultMsg{version: version, groups: groups}
		}
		sessions, err := store.ListSessions(filter, sortOpts, limit)
		if err != nil {
			return dataErrorMsg{err: err}
		}
		return deepSearchResultMsg{version: version, sessions: sessions}
	}
}

func (m Model) loadSelectedDetailCmd() tea.Cmd {
	if !m.showPreview {
		return nil
	}
	sess, ok := m.sessionList.Selected()
	if !ok {
		return nil
	}
	store := m.store
	id := sess.ID
	version := m.detailVersion
	return func() tea.Msg {
		if store == nil {
			return nil
		}
		detail, err := store.GetSession(id)
		if err != nil {
			return dataErrorMsg{err: err}
		}
		return sessionDetailMsg{detail: detail, version: version}
	}
}

func loadFilterDataCmd(store *data.Store) tea.Cmd {
	return func() tea.Msg {
		if store == nil {
			return filterDataMsg{}
		}
		folders, err := store.ListFolders()
		if err != nil {
			return dataErrorMsg{err: fmt.Errorf("listing folders: %w", err)}
		}
		return filterDataMsg{folders: folders}
	}
}

func checkNerdFontCmd() tea.Cmd {
	return func() tea.Msg {
		return fontCheckMsg{installed: platform.IsNerdFontInstalled()}
	}
}

// ---------------------------------------------------------------------------
// Copilot SDK search helpers
// ---------------------------------------------------------------------------

// copilotSearchDelay is the debounce delay before starting a Copilot SDK
// search. Longer than deep search since SDK search is more expensive.
const copilotSearchDelay = 500 * time.Millisecond

// scheduleCopilotSearch returns a tea.Cmd that fires a copilotSearchTickMsg
// after a debounce delay. The version counter lets the handler ignore stale ticks.
func (m Model) scheduleCopilotSearch(version int) tea.Cmd {
	return tea.Tick(copilotSearchDelay, func(time.Time) tea.Msg {
		return copilotSearchTickMsg{version: version}
	})
}

// copilotSearchCmd runs the Copilot SDK search in a background goroutine.
// Search() handles lazy initialisation and retries internally.
// The search context is stored in m.copilotSearchCancel so that a newer
// search (or Escape) can cancel this one, unblocking the searchMu.
func (m *Model) copilotSearchCmd(version int) tea.Cmd {
	client := m.copilotClient
	query := m.filter.Query

	ctx, cancel := context.WithTimeout(context.Background(), copilotSearchTimeout)
	m.copilotSearchCancel = cancel

	return func() tea.Msg {
		defer cancel()
		if client == nil || query == "" {
			return copilotSearchResultMsg{version: version}
		}

		ids, err := client.Search(ctx, query)
		return copilotSearchResultMsg{version: version, sessionIDs: ids, err: err}
	}
}

// findMissingAISessionIDs returns session IDs from the AI results that
// are not already present in the current session list.
func (m *Model) findMissingAISessionIDs(aiIDs []string) []string {
	existing := make(map[string]struct{}, len(m.sessions))
	for _, s := range m.sessions {
		existing[s.ID] = struct{}{}
	}
	var missing []string
	for _, id := range aiIDs {
		if _, ok := existing[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing
}

// fetchAISessionsCmd fetches sessions by ID from the store for AI-found
// results not already in the current list.
func (m Model) fetchAISessionsCmd(ids []string, version int) tea.Cmd {
	store := m.store
	return func() tea.Msg {
		if store == nil || len(ids) == 0 {
			return aiSessionsLoadedMsg{version: version}
		}
		sessions, err := store.ListSessionsByIDs(ids)
		if err != nil {
			// Silently degrade — don't break the UI for fetch errors.
			return aiSessionsLoadedMsg{version: version}
		}
		return aiSessionsLoadedMsg{version: version, sessions: sessions}
	}
}

// ---------------------------------------------------------------------------
// Group sorting helpers
// ---------------------------------------------------------------------------

// sortGroupsByLatest reorders groups so that the group containing the most
// recently updated session appears first (or last, for ascending).
func sortGroupsByLatest(groups []data.SessionGroup, order data.SortOrder) {
	slices.SortFunc(groups, func(a, b data.SessionGroup) int {
		c := cmp.Compare(latestUpdate(a.Sessions), latestUpdate(b.Sessions))
		if order == data.Descending {
			return -c
		}
		return c
	})
}

func latestUpdate(sessions []data.Session) string {
	latest := ""
	for _, s := range sessions {
		if s.LastActiveAt > latest {
			latest = s.LastActiveAt
		}
	}
	return latest
}

// sortGroupsByLabel sorts groups alphabetically by their label.
// Descending reverses the order (e.g. newest date first).
func sortGroupsByLabel(groups []data.SessionGroup, order data.SortOrder) {
	slices.SortFunc(groups, func(a, b data.SessionGroup) int {
		c := cmp.Compare(a.Label, b.Label)
		if order == data.Descending {
			return -c
		}
		return c
	})
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func sortFieldFromConfig(s string) data.SortField {
	switch s {
	case "created":
		return data.SortByCreated
	case "turns":
		return data.SortByTurns
	case "name", "summary":
		return data.SortByName
	case pivotFolder, "cwd":
		return data.SortByFolder
	default:
		return data.SortByUpdated
	}
}

func pivotFieldFromString(s string) data.PivotField {
	switch s {
	case pivotFolder:
		return data.PivotByFolder
	case pivotRepo:
		return data.PivotByRepo
	case pivotBranch:
		return data.PivotByBranch
	case pivotDate:
		return data.PivotByDate
	default:
		return data.PivotByFolder
	}
}

func timeRangeToSince(r string) *time.Time {
	now := time.Now()
	switch r {
	case "1h":
		t := now.Add(-time.Hour)
		return &t
	case "1d":
		// Use start-of-yesterday so everything displaying "1d ago" (24-48h) is included.
		y := now.AddDate(0, 0, -1)
		t := time.Date(y.Year(), y.Month(), y.Day(), 0, 0, 0, 0, y.Location())
		return &t
	case "7d":
		w := now.AddDate(0, 0, -7)
		t := time.Date(w.Year(), w.Month(), w.Day(), 0, 0, 0, 0, w.Location())
		return &t
	default: // "all"
		return nil
	}
}
