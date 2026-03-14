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

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

const (
	// copilotSearchTimeout limits a single Copilot AI search request.
	// Must be generous: Init ~1s + Search ~10s + retries (3 × [0.5s + 1s + 10s]) ≈ 45s.
	copilotSearchTimeout = 45 * time.Second

	// statusReindexCancelled is the status message shown when the user
	// cancels an in-flight reindex operation.
	statusReindexCancelled = "Reindex cancelled"
)

// timeRanges defines the time-range filter options shown in the header.
var timeRanges = []struct{ key, label string }{
	{"1", "1h"}, {"2", "1d"}, {"3", "7d"}, {"4", "all"},
}

// ---------------------------------------------------------------------------
// Application states
// ---------------------------------------------------------------------------

type appState int

const (
	stateLoading     appState = iota
	stateSessionList          // main view
	stateFilterPanel          // filter overlay open
	stateHelpOverlay          // help modal open
	stateShellPicker          // shell selection overlay
	stateConfigPanel          // settings overlay
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
// Layout — computed once per resize, consumed by all rendering functions.
// ---------------------------------------------------------------------------

type layout struct {
	totalWidth    int
	totalHeight   int
	headerHeight  int
	footerHeight  int
	contentHeight int
	listWidth     int
	previewWidth  int
}

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

	// Data layer.
	store *data.Store
	cfg   *config.Config

	// Query parameters.
	filter    data.FilterOptions
	sort      data.SortOptions
	timeRange string // "1h", "1d", "7d", "all"
	pivot     string // "none", "folder", "repo", "branch", "date"

	// Loaded data.
	sessions []data.Session
	groups   []data.SessionGroup
	detail   *data.SessionDetail

	// Detected shells and terminals for launch flow.
	shells    []platform.ShellInfo
	terminals []platform.TerminalInfo

	// Sub-components.
	sessionList components.SessionList
	searchBar   components.SearchBar
	filterPanel components.FilterPanel
	preview     components.PreviewPanel
	help        components.HelpOverlay
	shellPicker components.ShellPicker
	configPanel components.ConfigPanel
	spinner     spinner.Model

	// UI toggles.
	showPreview   bool
	showHidden    bool
	hiddenSet     map[string]struct{} // session ID → struct{} for fast hidden-session lookup
	reindexing    bool
	reindexLog    []string                  // log lines streamed from chronicle reindex
	reindexVP     viewport.Model            // scrollable viewport for reindex overlay
	reindexCancel *components.ReindexHandle // cancel handle for running reindex

	// Click debounce: delays single-click action so double-click can be
	// detected without the first click firing prematurely.
	pendingClickVersion int
	pendingClickY       int
	pendingClickItemIdx int
	pendingClickCtrl    bool
	pendingClickShift   bool

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
	attentionFilter bool // when true, only show sessions with AttentionWaiting
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
		YoloMode:      cfg.YoloMode,
		Agent:         cfg.Agent,
		Model:         cfg.Model,
		LaunchMode:    cfg.EffectiveLaunchMode(),
		Terminal:      cfg.DefaultTerminal,
		Shell:         cfg.DefaultShell,
		CustomCommand: cfg.CustomCommand,
		Theme:         cfg.Theme,
	})

	// Build the list of available theme names for the config panel.
	themeNames := make([]string, 0, 1+len(styles.BuiltinSchemeNames())+len(cfg.Schemes))
	themeNames = append(themeNames, "auto")
	themeNames = append(themeNames, styles.BuiltinSchemeNames()...)
	for _, cs := range cfg.Schemes {
		themeNames = append(themeNames, cs.Name)
	}
	cp.SetThemeOptions(themeNames)

	hiddenSet := make(map[string]struct{}, len(cfg.HiddenSessions))
	for _, id := range cfg.HiddenSessions {
		hiddenSet[id] = struct{}{}
	}

	m := Model{
		state: stateLoading,
		cfg:   cfg,

		sort: data.SortOptions{
			Field: sortFieldFromConfig(cfg.DefaultSort),
			Order: data.Descending,
		},
		timeRange:   cfg.DefaultTimeRange,
		pivot:       cfg.DefaultPivot,
		showPreview: cfg.ShowPreview,
		hiddenSet:   hiddenSet,

		sessionList: components.NewSessionList(),
		searchBar:   components.NewSearchBar(),
		filterPanel: components.NewFilterPanel(),
		preview:     components.NewPreviewPanel(),
		help:        components.NewHelpOverlay(),
		shellPicker: components.NewShellPicker(),
		configPanel: cp,
		spinner:     s,
	}

	m.filter.Since = timeRangeToSince(m.timeRange)
	m.filter.ExcludedDirs = cfg.ExcludedDirs
	return m
}

// resolveTheme applies a user-chosen color scheme.
//
// When the config field is empty or "auto" we keep the legacy
// adaptive-color defaults set by styles.init().  Those use
// lipgloss.AdaptiveColor which adapts to the terminal's own
// light/dark mode, so the UI looks correct on every terminal
// without any detection logic.
//
// Only when the user explicitly names a scheme (built-in or
// user-defined) do we derive a fixed-hex theme and apply it.
func resolveTheme(cfg *config.Config) {
	themeName := cfg.Theme
	if themeName == "" || themeName == "auto" {
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
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

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
		return m, m.loadSessionsCmd()

	case storeErrorMsg:
		m.statusErr = "Store: " + msg.err.Error()
		m.state = stateSessionList
		return m, nil

	// ----- Reindex ---------------------------------------------------------
	case components.ReindexLogPump:
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
			m.statusInfo = "Reindexed ✓"
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
		// Execute deferred single-click action.
		m.sessionList.MoveTo(m.pendingClickItemIdx)
		if m.sessionList.IsFolderSelected() {
			m.sessionList.ToggleFolder()
			return m, nil
		}
		m.detailVersion++
		return m, m.loadSelectedDetailCmd()

	// ----- Data loading ----------------------------------------------------
	case sessionsLoadedMsg:
		m.sessions = m.filterHiddenSessions(msg.sessions)
		m.sessions = m.filterAttentionSessions(m.sessions)
		m.groups = nil
		m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
		m.sessionList.SetAttentionStatuses(m.attentionMap)
		m.sessionList.SetSessions(m.sessions)
		m.state = stateSessionList
		m.searchBar.SetResultCount(m.sessionList.SessionCount())
		m.detailVersion++
		return m, tea.Batch(m.loadSelectedDetailCmd(), m.scanAttentionCmd())

	case groupsLoadedMsg:
		m.groups = m.filterHiddenGroups(msg.groups)
		m.groups = m.filterAttentionGroups(m.groups)
		m.sessions = nil
		m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
		m.sessionList.SetAttentionStatuses(m.attentionMap)
		m.sessionList.SetGroups(m.groups)
		m.state = stateSessionList
		m.searchBar.SetResultCount(m.sessionList.SessionCount())
		m.detailVersion++
		return m, tea.Batch(m.loadSelectedDetailCmd(), m.scanAttentionCmd())

	case sessionDetailMsg:
		if msg.version != m.detailVersion {
			return m, nil // stale result — selection changed since request
		}
		m.detail = msg.detail
		m.preview.SetDetail(m.detail)
		return m, nil

	case dataErrorMsg:
		m.statusErr = "Data: " + msg.err.Error()
		m.state = stateSessionList
		return m, nil

	// ----- Attention scanning ---------------------------------------------
	case attentionScannedMsg:
		m.attentionMap = msg.statuses
		m.sessionList.SetAttentionStatuses(m.attentionMap)
		// If attention filter is active, reload to apply filtering.
		if m.attentionFilter {
			return m, m.loadSessionsCmd()
		}
		return m, m.scheduleAttentionTick()

	case attentionTickMsg:
		return m, m.scanAttentionCmd()

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
			m.groups = nil
			m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
			m.sessionList.SetSessions(m.sessions)
		} else if msg.groups != nil {
			m.groups = m.filterHiddenGroups(msg.groups)
			m.sessions = nil
			m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
			m.sessionList.SetGroups(m.groups)
		}
		m.state = stateSessionList
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
		return m, m.copilotSearchCmd(msg.version)

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
		// Filter out hidden/excluded sessions before merging.
		incoming := m.filterHiddenSessions(msg.sessions)
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
			m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
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

	// ----- Font check / install -------------------------------------------
	case fontCheckMsg:
		styles.SetNerdFontEnabled(msg.installed)
		if msg.installed {
			return m, nil
		}
		// Not installed — attempt background installation.
		m.statusInfo = "Installing Nerd Font…"
		return m, installNerdFontCmd()

	case fontInstalledMsg:
		if msg.err != nil {
			// Installation failed — keep fallback icons, clear status.
			m.statusInfo = ""
			return m, nil
		}
		styles.SetNerdFontEnabled(true)
		m.statusInfo = "Nerd Font installed " + styles.IconCheck()
		return m, clearStatusAfter(3 * time.Second)

	// ----- Session exit (in-place resume finished) -------------------------
	case sessionExitMsg:
		m.closeStore()
		return m, tea.Quit

	// ----- Keyboard --------------------------------------------------------
	case tea.KeyMsg:
		return m.handleKey(msg)

	// ----- Mouse -----------------------------------------------------------
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	switch m.state {
	case stateLoading:
		return m.renderLoadingView()

	case stateHelpOverlay:
		return m.help.View()

	case stateShellPicker:
		return m.shellPicker.View()

	case stateFilterPanel:
		return m.filterPanel.View()

	case stateConfigPanel:
		return m.configPanel.View()

	default: // stateSessionList
		if m.reindexing && len(m.reindexLog) > 0 {
			return m.renderReindexOverlay()
		}
		return m.renderMainView()
	}
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
				return m, m.launchExternal(sh, m.selectedSessionID(), m.selectedSessionCwd(), launchStyle)
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
		case msg.Type == tea.KeyUp:
			m.searchBar.Blur()
			m.sessionList.MoveUp()
			m.detailVersion++
			return m, m.loadSelectedDetailCmd()
		case msg.Type == tea.KeyDown:
			m.searchBar.Blur()
			m.sessionList.MoveDown()
			m.detailVersion++
			return m, m.loadSelectedDetailCmd()
		default:
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
			YoloMode:      m.cfg.YoloMode,
			Agent:         m.cfg.Agent,
			Model:         m.cfg.Model,
			LaunchMode:    m.cfg.EffectiveLaunchMode(),
			PaneDirection: m.cfg.EffectivePaneDirection(),
			Terminal:      m.cfg.DefaultTerminal,
			Shell:         m.cfg.DefaultShell,
			CustomCommand: m.cfg.CustomCommand,
			Theme:         m.cfg.Theme,
		})
		m.state = stateConfigPanel
		return m, nil

	case key.Matches(msg, keys.Search):
		cmd := m.searchBar.Focus()
		return m, cmd

	case key.Matches(msg, keys.Filter):
		m.state = stateFilterPanel
		return m, loadFilterDataCmd(m.store)

	case key.Matches(msg, keys.Up):
		m.sessionList.MoveUp()
		m.detailVersion++
		return m, m.loadSelectedDetailCmd()

	case key.Matches(msg, keys.Down):
		m.sessionList.MoveDown()
		m.detailVersion++
		return m, m.loadSelectedDetailCmd()

	case key.Matches(msg, keys.Enter):
		if m.sessionList.IsFolderSelected() {
			m.sessionList.ToggleFolder()
			return m, nil
		}
		return m, m.launchSelected()

	case key.Matches(msg, keys.LaunchWindow):
		if !m.sessionList.IsFolderSelected() {
			if m.sessionList.SelectionCount() > 0 {
				return m, m.launchMultipleWithMode(config.LaunchModeWindow)
			}
			return m, m.launchWithMode(config.LaunchModeWindow)
		}
		return m, nil

	case key.Matches(msg, keys.LaunchTab):
		if !m.sessionList.IsFolderSelected() {
			if m.sessionList.SelectionCount() > 0 {
				return m, m.launchMultipleWithMode(config.LaunchModeTab)
			}
			return m, m.launchWithMode(config.LaunchModeTab)
		}
		return m, nil

	case key.Matches(msg, keys.LaunchPane):
		if !m.sessionList.IsFolderSelected() {
			if m.sessionList.SelectionCount() > 0 {
				return m, m.launchMultipleWithMode(config.LaunchModePane)
			}
			return m, m.launchWithMode(config.LaunchModePane)
		}
		return m, nil

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

	case key.Matches(msg, keys.Sort):
		m.cycleSort()
		return m, m.loadSessionsCmd()

	case key.Matches(msg, keys.SortOrder):
		m.toggleSortOrder()
		return m, m.loadSessionsCmd()

	case key.Matches(msg, keys.Pivot):
		m.cyclePivot()
		return m, m.loadSessionsCmd()

	case key.Matches(msg, keys.Preview):
		m.showPreview = !m.showPreview
		m.recalcLayout()
		if m.showPreview {
			m.detailVersion++
			return m, m.loadSelectedDetailCmd()
		}
		return m, nil

	case key.Matches(msg, keys.PreviewScrollUp):
		if m.showPreview {
			m.preview.PageUp()
		}
		return m, nil

	case key.Matches(msg, keys.PreviewScrollDown):
		if m.showPreview {
			m.preview.PageDown()
		}
		return m, nil

	case key.Matches(msg, keys.Reindex):
		if !m.reindexing {
			m.reindexing = true
			m.reindexLog = []string{"Starting reindex…"}
			m.reindexVP = viewport.New(0, reindexOverlayHeight)
			m.reindexVP.MouseWheelEnabled = true
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
		return m, m.loadSessionsCmd()

	case key.Matches(msg, keys.JumpNextAttention):
		return m.handleJumpNextAttention()

	case key.Matches(msg, keys.FilterAttention):
		m.attentionFilter = !m.attentionFilter
		return m, m.loadSessionsCmd()

	case key.Matches(msg, keys.Space):
		m.sessionList.ToggleSelected()
		m.updateSelectionStatus()
		return m, nil

	case key.Matches(msg, keys.LaunchAll):
		return m, m.launchMultiple()

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

	// Sync config and persist.
	m.cfg.HiddenSessions = m.hiddenSetToSlice()
	if err := config.Save(m.cfg); err != nil {
		m.statusErr = "config save: " + err.Error()
	}

	m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
	return m, m.loadSessionsCmd()
}

// hiddenSetToSlice converts the hiddenSet map back to a sorted slice for
// deterministic config serialisation.
func (m *Model) hiddenSetToSlice() []string {
	if len(m.hiddenSet) == 0 {
		return nil
	}
	ids := slices.Sorted(maps.Keys(m.hiddenSet))
	return ids
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
func (m Model) handleConfigKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.configPanel.IsEditing() {
		switch {
		case key.Matches(msg, keys.Escape):
			m.configPanel.CancelEdit()
			return m, nil
		case key.Matches(msg, keys.Enter):
			m.configPanel.ConfirmEdit()
			return m, nil
		default:
			var cmd tea.Cmd
			m.configPanel, cmd = m.configPanel.Update(msg)
			return m, cmd
		}
	}

	switch {
	case key.Matches(msg, keys.Escape):
		// Save config on close.
		m.saveConfigFromPanel()
		m.state = stateSessionList
		return m, nil
	case key.Matches(msg, keys.Up):
		m.configPanel.MoveUp()
	case key.Matches(msg, keys.Down):
		m.configPanel.MoveDown()
	case key.Matches(msg, keys.Enter):
		cmd := m.configPanel.HandleEnter()
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
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.reindexVP.ScrollUp(3)
		case tea.MouseButtonWheelDown:
			m.reindexVP.ScrollDown(3)
		case tea.MouseButtonLeft:
			if msg.Action == tea.MouseActionRelease {
				// Check if click is on the cancel button area.
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
	overPreview := m.layout.previewWidth > 0 && msg.X >= m.layout.listWidth

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if overPreview {
			m.preview.ScrollUp(3)
		} else {
			m.sessionList.ScrollBy(-3)
			m.detailVersion++
			return m, m.loadSelectedDetailCmd()
		}
		return m, nil

	case tea.MouseButtonWheelDown:
		if overPreview {
			m.preview.ScrollDown(3)
		} else {
			m.sessionList.ScrollBy(3)
			m.detailVersion++
			return m, m.loadSelectedDetailCmd()
		}
		return m, nil

	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionRelease {
			return m, nil
		}

		// --- Clickable header area (Y < HeaderLines) ---
		if msg.Y < styles.HeaderLines {
			return m.handleHeaderClick(msg.X, msg.Y)
		}

		// Map Y coordinate to a list item.
		listRow := msg.Y - styles.HeaderLines
		if listRow >= m.layout.contentHeight {
			return m, nil
		}
		// Only clicks within the list width (not the preview pane).
		if m.layout.previewWidth > 0 && msg.X >= m.layout.listWidth {
			return m, nil
		}
		itemIdx := m.sessionList.ScrollOffset() + listRow

		// Detect double-click: second click on the same row while a
		// pending single-click timer is still running.
		isDoubleClick := m.pendingClickVersion > 0 &&
			msg.Y == m.pendingClickY

		if isDoubleClick {
			// Invalidate the pending single-click so it won't fire.
			m.pendingClickVersion = 0

			m.sessionList.MoveTo(itemIdx)
			if m.sessionList.IsFolderSelected() {
				cwd := m.sessionList.SelectedFolderPath()
				mode := m.cfg.EffectiveLaunchMode()
				if msg.Ctrl {
					mode = config.LaunchModeWindow
				} else if msg.Shift {
					mode = config.LaunchModeTab
				}
				return m, m.launchNewSession(cwd, mode)
			}
			// Double-click session: if it's part of a multi-selection,
			// launch all selected. Otherwise launch just this session.
			if m.sessionList.SelectionCount() > 0 && m.sessionList.IsSelected(m.selectedSessionID()) {
				return m, m.launchMultiple()
			}
			if msg.Ctrl {
				return m, m.launchWithMode(config.LaunchModeWindow)
			}
			if msg.Shift {
				return m, m.launchWithMode(config.LaunchModeTab)
			}
			return m, m.launchSelected()
		}

		// Ctrl+click: toggle multi-select immediately (no deferred action).
		if msg.Ctrl {
			m.sessionList.MoveTo(itemIdx)
			m.sessionList.ToggleSelected()
			m.updateSelectionStatus()
			m.detailVersion++
			return m, m.loadSelectedDetailCmd()
		}

		// First click — defer the single-click action behind a timer so
		// a potential second click (double-click) can cancel it.
		m.pendingClickVersion++
		m.pendingClickY = msg.Y
		m.pendingClickItemIdx = itemIdx
		m.pendingClickCtrl = msg.Ctrl
		m.pendingClickShift = msg.Shift

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

	// Sort indicator.
	arrow := styles.IconSortDown()
	if m.sort.Order == data.Ascending {
		arrow = styles.IconSortUp()
	}
	sortLabel := arrow + " " + sortDisplayLabel(m.sort.Field)
	sortRendered := styles.KeyStyle.Render("s") + styles.DimmedStyle.Render(":Sort: "+sortLabel)
	w := lipgloss.Width(sortRendered)
	if x >= cursor && x < cursor+w {
		return "sort"
	}
	cursor += w + 2

	// Pivot indicator (always present).
	pivotLabel := m.pivot
	if pivotLabel == pivotNone {
		pivotLabel = "list"
	}
	pivotRendered := styles.KeyStyle.Render("tab") + styles.DimmedStyle.Render(":Pivot: "+pivotLabel)
	pw := lipgloss.Width(pivotRendered)
	if x >= cursor && x < cursor+pw {
		return "pivot"
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

	// Recompute content height based on actual rendered heights.
	headerH := lipgloss.Height(header) + lipgloss.Height(badges) + lipgloss.Height(sep)
	footerH := lipgloss.Height(footer)
	contentH := m.height - headerH - footerH
	if contentH < 1 {
		contentH = 1
	}

	// Compute preview width.
	previewW := 0
	if m.showPreview && m.width >= styles.PreviewMinWidth {
		previewW = int(float64(m.width) * styles.PreviewWidthRatio)
	}
	const gapWidth = 2
	listW := m.width - previewW
	if previewW > 0 {
		listW -= gapWidth
	}

	m.sessionList.SetSize(listW, contentH)
	m.preview.SetSize(previewW, contentH)

	var content string
	if previewW > 0 {
		gap := strings.Repeat(" ", gapWidth)
		content = lipgloss.JoinHorizontal(lipgloss.Top,
			m.sessionList.View(),
			gap,
			m.preview.View(),
		)
	} else {
		content = m.sessionList.View()
	}

	return strings.Join([]string{header, badges, sep, content, footer}, "\n")
}

func (m Model) renderHeader() string {
	title := styles.TitleStyle.Render(styles.IconTitle() + " Copilot Dispatch")

	// Search bar (always visible).
	// Reserve space for the right side (reindex spinner) only when active.
	rightReserve := 4
	if m.reindexing {
		rightReserve = 16 // " ⣾ Reindexing…" ≈ 15 chars
	}
	searchW := m.width - lipgloss.Width(title) - rightReserve
	if searchW < 15 {
		searchW = 15
	}
	m.searchBar.SetWidth(searchW)
	search := m.searchBar.View()

	// Clamp rendered search bar to its allocated width so the header
	// never exceeds the terminal width (which would wrap and hide the
	// badges row underneath).
	if lipgloss.Width(search) > searchW {
		search = lipgloss.NewStyle().MaxWidth(searchW).Render(search)
	}

	// Right side: reindex status.
	var right string
	if m.reindexing {
		right = m.spinner.View() + " Reindexing…"
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
	parts = append(parts, styles.KeyStyle.Render("s")+styles.DimmedStyle.Render(":Sort: "+sortLabel))

	// Pivot indicator with shortcut (always shown).
	pivotLabel := m.pivot
	if pivotLabel == pivotNone {
		pivotLabel = "list"
	}
	parts = append(parts, styles.KeyStyle.Render("tab")+styles.DimmedStyle.Render(":Pivot: "+pivotLabel))

	// Attention filter indicator.
	if m.attentionFilter {
		parts = append(parts, styles.ActiveBadgeStyle.Render("! waiting only"))
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

	// Left: session count + hidden count + active filter summary.
	left := fmt.Sprintf(" %d sessions", count)
	if hc := m.hiddenCount(); hc > 0 {
		badge := fmt.Sprintf("%d hidden", hc)
		if m.showHidden {
			badge += " " + styles.IconHidden()
		}
		left += "  " + styles.BadgeStyle.Render(badge)
	}
	if wc := m.waitingCount(); wc > 0 {
		left += "  " + styles.AttentionWaitingStyle.Render(fmt.Sprintf("● %d waiting", wc))
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
	usedWidth := lipgloss.Width(left) + lipgloss.Width(version) + 4
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
func (m *Model) handleReindexClick(msg tea.MouseMsg) {
	innerW := m.reindexInnerWidth()
	// OverlayStyle: RoundedBorder (1 each side) + Padding(1,2) (2 each side) = 6 horizontal, 4 vertical
	overlayW := innerW + 6
	// title (with bottom padding 1) + viewport + cancel row + padding top/bottom from border
	overlayH := 1 + 1 + reindexOverlayHeight + 1 + 4

	startX := (m.width - overlayW) / 2
	startY := (m.height - overlayH) / 2

	// The cancel button is on the last content row before bottom border/padding.
	btnY := startY + overlayH - 3 // 1 border + 1 padding from bottom
	btnLabel := "[ Cancel (esc) ]"
	btnW := lipgloss.Width(btnLabel)
	btnX := startX + (overlayW-btnW)/2

	if msg.Y == btnY && msg.X >= btnX && msg.X < btnX+btnW {
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

	var sb strings.Builder
	for i, l := range m.reindexLog {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(styles.DimStyle.Render(l))
	}

	m.reindexVP.Width = innerW
	m.reindexVP.Height = reindexOverlayHeight

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
		Render("[ Cancel (esc) ]")

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

// filterAttentionSessions removes sessions that don't match the attention
// filter. When attentionFilter is false, all sessions pass through.
func (m *Model) filterAttentionSessions(sessions []data.Session) []data.Session {
	if !m.attentionFilter || len(m.attentionMap) == 0 {
		return sessions
	}
	filtered := make([]data.Session, 0, len(sessions))
	for _, s := range sessions {
		if status, ok := m.attentionMap[s.ID]; ok && status == data.AttentionWaiting {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// filterAttentionGroups removes sessions that don't match the attention
// filter from grouped results. Empty groups are dropped.
func (m *Model) filterAttentionGroups(groups []data.SessionGroup) []data.SessionGroup {
	if !m.attentionFilter || len(m.attentionMap) == 0 {
		return groups
	}
	filtered := make([]data.SessionGroup, 0, len(groups))
	for _, g := range groups {
		var sessions []data.Session
		for _, s := range g.Sessions {
			if status, ok := m.attentionMap[s.ID]; ok && status == data.AttentionWaiting {
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
			return
		}
	}
	m.pivot = pivotNone
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

// batchLaunchSessions builds launch commands for each session, clears the
// selection state, and returns a tea.Batch of all commands.
func (m *Model) batchLaunchSessions(sessions []data.Session, mode string) tea.Cmd {
	var cmds []tea.Cmd
	for _, sess := range sessions {
		cmd := m.resolveShellAndLaunchDirect(sess.ID, sess.Cwd, mode)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	m.sessionList.DeselectAll()
	m.statusInfo = ""

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// updateSelectionStatus sets the status bar to reflect the current selection count.
func (m *Model) updateSelectionStatus() {
	if n := m.sessionList.SelectionCount(); n > 0 {
		m.statusInfo = fmt.Sprintf("%d selected — ⇧⏎ to open", n)
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

// resolveShellAndLaunch picks the right shell (configured, single, or picker)
// and launches the session externally. Shared by launchWithMode and
// launchNewSession to avoid duplicating shell-resolution logic.
func (m *Model) resolveShellAndLaunch(sessionID, cwd, mode string) tea.Cmd {
	launchStyle := launchStyleForMode(mode)

	if m.cfg.DefaultShell != "" {
		sh := m.findShellByName(m.cfg.DefaultShell)
		return m.launchExternal(sh, sessionID, cwd, launchStyle)
	}
	if len(m.shells) <= 1 {
		sh := platform.DefaultShell()
		if len(m.shells) == 1 {
			sh = m.shells[0]
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
			return dataErrorMsg{err: fmt.Errorf("launching session: %w", err)}
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
// Layout
// ---------------------------------------------------------------------------

func (m *Model) recalcLayout() {
	contentH := m.height - styles.HeaderLines - styles.FooterLines
	if contentH < 1 {
		contentH = 1
	}
	previewW := 0
	if m.showPreview && m.width >= styles.PreviewMinWidth {
		previewW = int(float64(m.width) * styles.PreviewWidthRatio)
	}
	const gapWidth = 2
	listW := m.width - previewW
	if previewW > 0 {
		listW -= gapWidth
	}

	m.layout = layout{
		totalWidth:    m.width,
		totalHeight:   m.height,
		headerHeight:  styles.HeaderLines,
		footerHeight:  styles.FooterLines,
		contentHeight: contentH,
		listWidth:     listW,
		previewWidth:  previewW,
	}

	m.sessionList.SetSize(listW, contentH)
	m.preview.SetSize(previewW, contentH)
	m.help.SetSize(m.width, m.height)
	m.shellPicker.SetSize(m.width, m.height)
	m.filterPanel.SetSize(m.width, m.height)
	m.configPanel.SetSize(m.width, m.height)
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

	return func() tea.Msg {
		if store == nil {
			return dataErrorMsg{err: errors.New("store not available")}
		}
		if pivot != pivotNone {
			pf := pivotFieldFromString(pivot)
			groups, err := store.GroupSessions(pf, filter, sortOpts, 0)
			if err != nil {
				return dataErrorMsg{err: err}
			}
			// When sorting by updated time, reorder groups so the most
			// recently active folder appears first.
			if sortOpts.Field == data.SortByUpdated {
				sortGroupsByLatest(groups, sortOpts.Order)
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

// scanAttentionCmd runs the session-state directory scanner in the background
// and returns an attentionScannedMsg with the results.
func (m Model) scanAttentionCmd() tea.Cmd {
	threshold := m.cfg.EffectiveAttentionThreshold()
	return func() tea.Msg {
		statuses := data.ScanAttention(threshold)
		return attentionScannedMsg{statuses: statuses}
	}
}

// scheduleAttentionTick schedules the next periodic attention scan.
func (m Model) scheduleAttentionTick() tea.Cmd {
	return tea.Tick(attentionRefreshInterval, func(time.Time) tea.Msg {
		return attentionTickMsg{}
	})
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

	return func() tea.Msg {
		if store == nil {
			return nil
		}
		if pivot != pivotNone {
			pf := pivotFieldFromString(pivot)
			groups, err := store.GroupSessions(pf, filter, sortOpts, 0)
			if err != nil {
				return dataErrorMsg{err: err}
			}
			if sortOpts.Field == data.SortByUpdated {
				sortGroupsByLatest(groups, sortOpts.Order)
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

func installNerdFontCmd() tea.Cmd {
	return func() tea.Msg {
		err := platform.InstallNerdFont()
		return fontInstalledMsg{err: err}
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
	case "folder", "cwd":
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
