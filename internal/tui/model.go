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
	"os"
	"path/filepath"
	"slices"
	"strconv"
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
	"github.com/jongio/dispatch/internal/version"
)

// ---------------------------------------------------------------------------
// Package-level configuration
// ---------------------------------------------------------------------------

// doubleClickTimeout is the maximum interval between two mouse clicks at the
// same Y position for them to be treated as a double-click.
const doubleClickTimeout = 300 * time.Millisecond

// themeAuto is the reserved theme name for terminal-adaptive colours.
const themeAuto = "auto"

const (
	// copilotSearchTimeout limits a single Copilot AI search request.
	// Must be generous: Init ~1s + Search ~10s + retries (3 × [0.5s + 1s + 10s]) ≈ 45s.
	copilotSearchTimeout = 45 * time.Second

	// statusReindexDone is the status message shown when a repair reindex
	// completes successfully.
	statusReindexDone = "Rebuild complete ✓"

	// statusCopiedID is the status message shown when a session ID is
	// copied to the clipboard.
	statusCopiedID = "Copied session ID ✓"

	// statusCopiedPath is the status message shown when a session or folder
	// working directory path is copied to the clipboard.
	statusCopiedPath = "Copied path ✓"

	// statusCopiedResumeCommand is the status message shown when a session's
	// resume command is copied to the clipboard.
	statusCopiedResumeCommand = "Copied resume command ✓"

	// statusCopiedPreview is the status message shown when preview content
	// is copied to the clipboard via the y key.
	statusCopiedPreview = "Copied to clipboard ✓"

	// statusCopiedSelection is the status message shown when a mouse
	// text selection from the preview pane is copied to the clipboard.
	statusCopiedSelection = "Copied selection ✓"

	// statusReindexCancelled is the status message shown when the user
	// cancels an in-flight rebuild operation.
	statusReindexCancelled = "Rebuild cancelled"

	// cancelBtnLabel is the button label shown in overlays that can be
	// dismissed with Escape (reindex, etc.).
	cancelBtnLabel = "[ Cancel (esc) ]"

	// headerRightReserve is the default column reserve on the right side
	// of the header row (accounts for a potential trailing space or cursor).
	headerRightReserve = 4

	// headerReindexReserve is the wider column reserve when the rebuild
	// spinner is active (" ⣾ Rebuilding…" ≈ 15 chars + padding).
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
	stateViewPicker               // named view selection overlay
	stateFilePicker               // file picker overlay
	stateCompareView              // compare two sessions overlay
	stateCmdPalette               // command palette overlay
)

// Pivot mode constants used by Model.pivot to control session grouping.
const (
	pivotNone   = "none"
	pivotFolder = "folder"
	pivotRepo   = "repo"
	pivotBranch = "branch"
	pivotDate   = "date"
	pivotHost   = "host"
)

// ---------------------------------------------------------------------------
// Focused sub-models
// ---------------------------------------------------------------------------

// searchState groups fields related to search tracking.
type searchState struct {
	deepSearchVersion    int
	deepSearchPending    bool
	copilotSearchVersion int                 // version counter for SDK search debounce
	copilotSearching     bool                // true when SDK search is in progress
	copilotSearchCancel  context.CancelFunc  // cancels the in-flight SDK search
	aiSessionIDs         map[string]struct{} // session IDs found by SDK search
	lastRawInput         string              // last raw search bar text (for change detection)
	history              []string            // committed search queries, oldest last (for up/down recall)
	historyIdx           int                 // recall cursor into history; == len(history) means not navigating
}

// maxSearchHistory caps how many recent queries are retained for up/down recall.
const maxSearchHistory = 20

// pushHistory records a committed query for later recall. Blank queries are
// ignored. Duplicates are collapsed so the same query never appears twice, and
// the most recent entry is kept at the end. The recall cursor is reset to the
// end (not navigating) after every push.
func (s *searchState) pushHistory(q string) {
	q = strings.TrimSpace(q)
	if q == "" {
		s.historyIdx = len(s.history)
		return
	}
	// Drop any earlier identical entry so recall order stays useful.
	for i, h := range s.history {
		if h == q {
			s.history = append(s.history[:i], s.history[i+1:]...)
			break
		}
	}
	s.history = append(s.history, q)
	if len(s.history) > maxSearchHistory {
		s.history = s.history[len(s.history)-maxSearchHistory:]
	}
	s.historyIdx = len(s.history)
}

// recallPrev steps to the previous (older) history entry and returns it. The
// bool is false when there is no history to recall.
func (s *searchState) recallPrev() (string, bool) {
	if len(s.history) == 0 {
		return "", false
	}
	if s.historyIdx > 0 {
		s.historyIdx--
	}
	return s.history[s.historyIdx], true
}

// recallNext steps to the next (newer) history entry and returns it. When the
// cursor moves past the newest entry it returns an empty string so the caller
// can clear the input. The bool is false when there is no history to recall.
func (s *searchState) recallNext() (string, bool) {
	if len(s.history) == 0 {
		return "", false
	}
	if s.historyIdx < len(s.history) {
		s.historyIdx++
	}
	if s.historyIdx >= len(s.history) {
		return "", true
	}
	return s.history[s.historyIdx], true
}

// triggerSearch reacts to a new search bar value: it records the raw input,
// parses structured tokens, kicks off the quick reload, and schedules the
// debounced deep and Copilot SDK searches. inputCmd, when non-nil, is the
// command returned by the text input update and is batched in first.
func (m *Model) triggerSearch(inputCmd tea.Cmd) tea.Cmd {
	newQuery := m.searchBar.Value()
	m.search.lastRawInput = newQuery
	// Parse structured tokens from the input.
	m.searchFilter = ParseSearchTokens(newQuery)
	m.applySearchTokens()
	m.filter.DeepSearch = false
	// Quick search fires immediately; schedule deep search.
	m.search.deepSearchVersion++
	m.search.deepSearchPending = true
	m.searchBar.SetSearching(true)

	cmds := make([]tea.Cmd, 0, 4)
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}
	cmds = append(cmds, m.loadSessionsCmd(), m.scheduleDeepSearch(m.search.deepSearchVersion))

	// Copilot SDK search is gated by config flag.
	if m.cfg.AISearch {
		m.search.copilotSearchVersion++
		m.searchBar.SetAISearching(false) // reset until tick fires
		m.searchBar.SetAIResults(0)       // clear stale count
		m.search.aiSessionIDs = nil
		m.sessionList.SetAISessions(nil)
		cmds = append(cmds, m.scheduleCopilotSearch(m.search.copilotSearchVersion))
	}

	return tea.Batch(cmds...)
}

// workStatusState groups fields related to work status scanning.
type workStatusState struct {
	workStatusMap      map[string]data.WorkStatusResult
	filterWorkStatus   map[data.WorkStatus]struct{} // when non-empty, only show sessions with matching work status
	workStatusScanned  bool                         // true after first successful work status scan
	workStatusScanning bool                         // true while work status scan chain is in progress
	workStatusAICancel context.CancelFunc           // cancels in-flight AI work status analysis
	autoShowPlan       bool                         // when true, auto-switch to plan view on next planContentMsg
}

// clickState groups fields related to click debounce.
type clickState struct {
	pendingClickVersion int
	pendingClickY       int
	pendingClickItemIdx int
}

// ---------------------------------------------------------------------------
// Root model
// ---------------------------------------------------------------------------

// Model is the top-level Bubble Tea model for the Session Browser TUI.
//
// TODO(#113): Model is a God Object (60+ fields). Further extraction into
// feature-specific sub-structs (FilterState, DataState) would improve
// maintainability. See https://github.com/jongio/dispatch/issues/113.
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
	pivot      string         // "none", "folder", "repo", "branch", "date", "host"
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
	noteInput       components.NoteInput
	tagInput        components.TagInput
	aliasInput      components.AliasInput
	filterPanel     components.FilterPanel
	preview         components.PreviewPanel
	help            components.HelpOverlay
	shellPicker     components.ShellPicker
	configPanel     components.ConfigPanel
	attentionPicker components.AttentionPicker
	viewPicker      components.ViewPicker
	filePicker      components.FilePicker
	compareView     components.CompareView
	cmdPalette      components.CmdPalette
	spinner         spinner.Model

	// UI toggles.
	showPreview       bool
	previewFullscreen bool
	previewPosition   string // "right", "bottom", "left", "top"
	showHidden        bool
	hiddenSet         map[string]struct{} // session ID → struct{} for fast hidden-session lookup
	favoritedSet      map[string]struct{} // session ID → struct{} for fast favorited-session lookup
	notesSet          map[string]struct{} // session ID → struct{} for fast note-existence lookup
	tagsSet           map[string]struct{}
	showFavorited     bool
	activeView        string // name of the active named view (empty means Default)
	reindexing        bool
	reindexLog        []string                  // log lines streamed from chronicle reindex
	reindexVP         viewport.Model            // scrollable viewport for reindex overlay
	reindexCancel     *components.ReindexHandle // cancel handle for running reindex

	// Focused sub-models.
	click        clickState
	search       searchState
	workStatus   workStatusState
	searchFilter SearchFilter // structured tokens parsed from search bar input
	tagFilter    string       // active tag: token; when set only show sessions carrying the tag

	// initialQuery is a search string passed on the command line. It is
	// applied once, when the session store first opens, then cleared.
	initialQuery string

	// Launch mode requested when showing the shell picker.
	pendingLaunchMode string

	// Detail loading version — incremented on each loadSelectedDetailCmd
	// call to discard stale results from slower prior queries.
	detailVersion int

	// Transient status bar messages.
	statusErr  string
	statusInfo string

	// Copilot SDK search.
	copilotClient *copilot.Client

	// Attention status tracking — scanned from session-state directories.
	attentionMap    map[string]data.AttentionStatus
	attentionFilter map[data.AttentionStatus]struct{} // when non-empty, only show sessions with matching status

	// Waiting notification tracking. waitingNotified holds the IDs of
	// sessions currently in the waiting state that have already triggered a
	// bell, so re-entering waiting notifies again but a steady waiting state
	// does not. attentionScanned becomes true after the first scan so the
	// initial population never rings the bell.
	waitingNotified  map[string]struct{}
	attentionScanned bool

	// Plan status tracking — scanned from session-state directories.
	planMap     map[string]bool
	filterPlans bool // when true, only show sessions with a plan.md file

	// Git workspace state tracking — scanned from session directories.
	gitStateMap    map[string]platform.GitState
	filterGitDirty bool // when true, only show sessions with local git changes

	// DB watcher — monitors session-store.db for external modifications.
	dbWatcher *data.DBWatcher
	dbWatchCh chan struct{} // receives pings from the watcher callback
}

// NewModel creates the root Model with default configuration.
func NewModel() Model {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = config.Default()
	}

	// ── Theme resolution ────────────────────────────────────────────
	resolveTheme(cfg)

	// ── Keybinding overrides ────────────────────────────────────────
	// Apply user remaps to the global key map before the UI reads it.
	if len(cfg.Keybindings) > 0 {
		remapped, warnings := applyKeybindingOverrides(defaultKeyMap(), cfg.Keybindings)
		keys = remapped
		for _, w := range warnings {
			slog.Warn(w)
		}
	}

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
		RedactSecrets:     cfg.RedactPreviewSecrets,
		ExcludedWords:     strings.Join(cfg.ExcludedWords, ", "),
		AutoRefresh:       autoRefreshFieldValue(cfg.AutoRefreshSeconds),
		NotifyOnWaiting:   cfg.NotifyOnWaiting,
		ShowRepoColumn:    cfg.ColumnVisible(config.ColumnRepo),
		ShowFolderColumn:  cfg.ColumnVisible(config.ColumnFolder),
		ShowTurnsColumn:   cfg.ColumnVisible(config.ColumnTurns),
		ShowHostColumn:    cfg.ColumnVisible(config.ColumnHost),
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

	notesSet := make(map[string]struct{}, len(cfg.SessionNotes))
	for id := range cfg.SessionNotes {
		notesSet[id] = struct{}{}
	}

	tagsSet := make(map[string]struct{}, len(cfg.SessionTags))
	for id := range cfg.SessionTags {
		tagsSet[id] = struct{}{}
	}

	m := Model{
		state: stateLoading,
		cfg:   cfg,

		sort: data.SortOptions{
			Field: sortFieldFromConfig(cfg.DefaultSort),
			Order: sortOrderFromConfig(cfg.EffectiveSortOrder()),
		},
		timeRange:       cfg.DefaultTimeRange,
		pivot:           cfg.DefaultPivot,
		showPreview:     cfg.ShowPreview,
		previewPosition: cfg.EffectivePreviewPosition(),
		hiddenSet:       hiddenSet,
		favoritedSet:    favoritedSet,
		notesSet:        notesSet,
		tagsSet:         tagsSet,

		sessionList:     components.NewSessionList(),
		searchBar:       components.NewSearchBar(),
		noteInput:       components.NewNoteInput(),
		tagInput:        components.NewTagInput(),
		aliasInput:      components.NewAliasInput(),
		filterPanel:     components.NewFilterPanel(),
		preview:         components.NewPreviewPanel(),
		help:            components.NewHelpOverlay(),
		shellPicker:     components.NewShellPicker(),
		configPanel:     cp,
		spinner:         s,
		attentionPicker: components.NewAttentionPicker(),
		viewPicker:      components.NewViewPicker(),
		filePicker:      components.NewFilePicker(),
		compareView:     components.NewCompareView(),
		cmdPalette:      components.NewCmdPalette(),
		attentionFilter: make(map[data.AttentionStatus]struct{}),
		waitingNotified: make(map[string]struct{}),
		dbWatchCh:       make(chan struct{}, 1),
	}

	m.dbWatcher = data.NewDBWatcher(func() {
		// Non-blocking send so the watcher goroutine never stalls.
		select {
		case m.dbWatchCh <- struct{}{}:
		default:
		}
	})
	// Honour the configured auto-refresh interval. When disabled (0), the
	// watcher is never activated so no polling happens; the list still
	// refreshes on explicit reload and after reindex.
	if interval, enabled := cfg.EffectiveAutoRefreshInterval(); enabled {
		m.dbWatcher.SetInterval(interval)
		m.dbWatcher.SetActive(true)
	}

	m.filter.Since = timeRangeToSince(m.timeRange)
	m.filter.ExcludedDirs = cfg.ExcludedDirs
	m.filter.ExcludedWords = cfg.ExcludedWords
	m.preview.SetConversationSort(cfg.ConversationNewestFirst)
	m.sessionList.SetHiddenColumns(cfg.HiddenColumns)
	m.preview.SetRedactSecrets(cfg.RedactPreviewSecrets)

	// Named views: populate picker and apply the persisted active view.
	m.viewPicker.SetViews(cfg.ValidViews())
	if cfg.ActiveView != "" && cfg.ActiveView != "Default" {
		if v := cfg.FindView(cfg.ActiveView); v != nil {
			m.activeView = cfg.ActiveView
			m.applyNamedView(v)
		}
	}

	return m
}

// NewModelWithQuery creates the root Model and seeds an initial search query
// that is applied once the session store opens. An empty query behaves
// exactly like NewModel.
func NewModelWithQuery(query string) Model {
	m := NewModel()
	m.initialQuery = query
	return m
}

// applyInitialQuery seeds the search bar with a command-line query and puts
// the model into the same search state as if the user had typed it: the
// search bar is focused and populated, structured tokens are parsed, and a
// quick search runs immediately with a deep search scheduled to follow. It
// returns the commands the caller should run (focus blink and the deep-search
// timer).
func (m *Model) applyInitialQuery(query string) []tea.Cmd {
	focusCmd := m.searchBar.Focus()
	m.searchBar.SetValue(query)
	m.search.lastRawInput = query
	m.searchFilter = ParseSearchTokens(query)
	m.applySearchTokens()
	m.filter.DeepSearch = false
	m.search.deepSearchVersion++
	m.search.deepSearchPending = true
	m.searchBar.SetSearching(true)
	return []tea.Cmd{focusCmd, m.scheduleDeepSearch(m.search.deepSearchVersion)}
}

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
		m.waitForDBChangeCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ----- Background color detection --------------------------------------
	case tea.BackgroundColorMsg:
		return m.handleBackgroundColor(msg)

	// ----- Window resize ---------------------------------------------------
	case tea.WindowSizeMsg:
		return m.handleResize(msg)

	// ----- Spinner tick ----------------------------------------------------
	case spinner.TickMsg:
		return m.handleSpinnerTick(msg)

	// ----- Store lifecycle -------------------------------------------------
	case storeOpenedMsg:
		return m.handleStoreOpened(msg)

	case storeErrorMsg:
		return m.handleStoreError(msg)

	// ----- Reindex ---------------------------------------------------------
	case components.ReindexLogPump:
		return m.handleReindexLogPump(msg)

	case components.ReindexFinishedMsg:
		return m.handleReindexFinished(msg)

	// ----- DB watcher (external session store changes) --------------------
	case sessionsChangedMsg:
		return m.handleSessionsChanged()

	// ----- Export done -----------------------------------------------------
	case exportDoneMsg:
		return m.handleExportDone(msg)

	case fileOpenedMsg:
		return m.handleFileOpened(msg)

	case dirOpenedMsg:
		return m.handleDirOpened(msg)

	case compareDetailMsg:
		return m.handleCompareDetail(msg)

	// ----- Transient status clear -----------------------------------------
	case clearStatusMsg:
		return m.handleClearStatus()

	// ----- Pending click fire (single-click debounce) ---------------------
	case pendingClickFireMsg:
		return m.handlePendingClickFire(msg)

	// ----- Data loading ----------------------------------------------------
	case sessionsLoadedMsg:
		return m.handleSessionsLoaded(msg)

	case groupsLoadedMsg:
		return m.handleGroupsLoaded(msg)

	case sessionDetailMsg:
		return m.handleSessionDetail(msg)

	case dataErrorMsg:
		return m.handleDataError(msg)

	// ----- Attention scanning ---------------------------------------------
	case attentionQuickScannedMsg:
		return m.handleAttentionQuickScanned(msg)

	case attentionScannedMsg:
		return m.handleAttentionScanned(msg)

	case attentionTickMsg:
		return m.handleAttentionTick()

	// ----- Plan scanning --------------------------------------------------
	case plansScannedMsg:
		return m.handlePlansScanned(msg)

	case planContentMsg:
		return m.handlePlanContent(msg)

	// ----- Work status scanning -------------------------------------------
	case workStatusQuickScannedMsg:
		return m.handleWorkStatusQuickScanned(msg)

	case workStatusScannedMsg:
		return m.handleWorkStatusScanned(msg)

	case workStatusAIScannedMsg:
		return m.handleWorkStatusAIScanned(msg)

	case continuationPlanCreatedMsg:
		return m.handleContinuationPlanCreated(msg)

	// ----- Git workspace state scanning ------------------------------------
	case gitStateScannedMsg:
		return m.handleGitStateScanned(msg)

	// ----- Deep search debounce -------------------------------------------
	case deepSearchTickMsg:
		return m.handleDeepSearchTick(msg)

	case deepSearchResultMsg:
		return m.handleDeepSearchResult(msg)

	// ----- Copilot SDK search ------------------------------------------------
	case copilotReadyMsg:
		return m.handleCopilotReady()

	case copilotErrorMsg:
		return m.handleCopilotError()

	case copilotSearchTickMsg:
		return m.handleCopilotSearchTick(msg)

	case copilotSearchResultMsg:
		return m.handleCopilotSearchResult(msg)

	case aiSessionsLoadedMsg:
		return m.handleAISessionsLoaded(msg)

	// ----- Filter picker data ---------------------------------------------
	case filterDataMsg:
		return m.handleFilterData(msg)

	// ----- Shell detection -------------------------------------------------
	case shellsDetectedMsg:
		return m.handleShellsDetected(msg)

	// ----- Terminal detection ----------------------------------------------
	case terminalsDetectedMsg:
		return m.handleTerminalsDetected(msg)

	// ----- Font check -------------------------------------------------------
	case fontCheckMsg:
		return m.handleFontCheck(msg)

	// ----- Session exit (in-place resume finished) -------------------------
	case sessionExitMsg:
		return m.handleSessionExit(msg)

	// ----- Command palette action ------------------------------------------
	case cmdPaletteActionMsg:
		return m.handleCmdPaletteAction(msg)

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

		case stateViewPicker:
			content = m.viewPicker.View()

		case stateFilePicker:
			content = m.filePicker.View()

		case stateCompareView:
			content = m.compareView.View()

		case stateCmdPalette:
			content = m.cmdPalette.View()

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
			m.workStatus.filterWorkStatus = m.attentionPicker.WorkStatusFilter()
			m.filterGitDirty = m.attentionPicker.FilterGitDirty()
			m.state = stateSessionList
			return m, m.loadSessionsCmd()
		}
		return m, nil

	case stateViewPicker:
		switch {
		case key.Matches(msg, keys.Escape):
			m.state = stateSessionList
		case key.Matches(msg, keys.Up):
			m.viewPicker.MoveUp()
		case key.Matches(msg, keys.Down):
			m.viewPicker.MoveDown()
		case key.Matches(msg, keys.Enter):
			selected := m.viewPicker.Selected()
			if selected == "Default" {
				m.activeView = ""
			} else {
				m.activeView = selected
			}
			// Persist active view.
			m.cfg.ActiveView = m.activeView
			if err := config.Save(m.cfg); err != nil {
				m.statusErr = "config save: " + err.Error()
			}
			// Apply the view settings.
			if m.activeView != "" {
				if v := m.cfg.FindView(m.activeView); v != nil {
					m.applyNamedView(v)
				}
			} else {
				// Reset to config defaults when switching back to Default.
				m.timeRange = m.cfg.DefaultTimeRange
				m.filter.Since = timeRangeToSince(m.timeRange)
				m.sort.Field = sortFieldFromConfig(m.cfg.DefaultSort)
				m.sort.Order = sortOrderFromConfig(m.cfg.EffectiveSortOrder())
				m.pivot = m.cfg.DefaultPivot
				m.showFavorited = false
				m.showHidden = false
				m.filter.ExcludedDirs = m.cfg.ExcludedDirs
				m.searchBar.SetValue("")
			}
			m.state = stateSessionList
			return m, m.loadSessionsCmd()
		}
		return m, nil

	case stateFilePicker:
		switch {
		case key.Matches(msg, keys.Escape):
			m.state = stateSessionList
		case key.Matches(msg, keys.Up):
			m.filePicker.MoveUp()
		case key.Matches(msg, keys.Down):
			m.filePicker.MoveDown()
		case key.Matches(msg, keys.Enter):
			if sf, ok := m.filePicker.Selected(); ok {
				return m, m.openFileCmd(sf.FilePath)
			}
		}
		return m, nil

	case stateCompareView:
		switch {
		case key.Matches(msg, keys.Escape):
			m.state = stateSessionList
		case key.Matches(msg, keys.Up), key.Matches(msg, keys.PreviewScrollUp):
			m.compareView.ScrollUp()
		case key.Matches(msg, keys.Down), key.Matches(msg, keys.PreviewScrollDown):
			m.compareView.ScrollDown()
		case key.Matches(msg, keys.CopyID):
			// 'c' copies compare summary to clipboard.
			txt := m.compareView.PlainText()
			if err := clipboardWrite(txt); err != nil {
				m.statusErr = "clipboard: " + err.Error()
			} else {
				m.statusInfo = "Copied compare summary \u2713"
			}
			return m, clearStatusAfter(2 * time.Second)
		}
		return m, nil

	case stateCmdPalette:
		switch {
		case key.Matches(msg, keys.Escape):
			m.state = stateSessionList
		case key.Matches(msg, keys.Up):
			m.cmdPalette.MoveUp()
		case key.Matches(msg, keys.Down):
			m.cmdPalette.MoveDown()
		case key.Matches(msg, keys.Enter):
			if action, ok := m.cmdPalette.Selected(); ok {
				m.state = stateSessionList
				return m, func() tea.Msg { return cmdPaletteActionMsg{action: action} }
			}
		case msg.Code == tea.KeyBackspace:
			m.cmdPalette.Backspace()
		default:
			// Any printable rune is typed into the filter.
			if msg.Text != "" && msg.Mod == 0 {
				for _, r := range msg.Text {
					m.cmdPalette.TypeRune(r)
				}
			}
		}
		return m, nil

	default:
		// stateLoading and stateSessionList fall through to the
		// main key handler below.
	}

	// ---------- Note input focused ------------------------------------------
	if m.noteInput.Focused() {
		switch {
		case key.Matches(msg, keys.Escape):
			m.noteInput.Blur()
			return m, nil
		case key.Matches(msg, keys.Enter):
			sessionID := m.noteInput.SessionID()
			noteText := m.noteInput.Value()
			m.noteInput.Blur()

			// Update or remove the note in config.
			if noteText == "" {
				delete(m.cfg.SessionNotes, sessionID)
				delete(m.notesSet, sessionID)
			} else {
				if m.cfg.SessionNotes == nil {
					m.cfg.SessionNotes = make(map[string]string)
				}
				m.cfg.SessionNotes[sessionID] = noteText
				m.notesSet[sessionID] = struct{}{}
			}

			if err := config.Save(m.cfg); err != nil {
				m.statusErr = "config save: " + err.Error()
			}

			// Update UI state.
			m.sessionList.SetNoteSessions(m.notesSet)
			m.preview.SetNote(noteText)
			m.statusInfo = "Note saved"
			return m, clearStatusAfter(2 * time.Second)
		default:
			var cmd tea.Cmd
			ni := m.noteInput
			ni, cmd = ni.Update(msg)
			m.noteInput = ni
			return m, cmd
		}
	}

	// ---------- Tag input focused -------------------------------------------
	if m.tagInput.Focused() {
		switch {
		case key.Matches(msg, keys.Escape):
			m.tagInput.Blur()
			return m, nil
		case key.Matches(msg, keys.Enter):
			sessionID := m.tagInput.SessionID()
			tags := config.ParseTags(m.tagInput.Value())
			m.tagInput.Blur()

			m.cfg.SetTags(sessionID, tags)
			if len(tags) == 0 {
				delete(m.tagsSet, sessionID)
			} else {
				m.tagsSet[sessionID] = struct{}{}
			}

			if err := config.Save(m.cfg); err != nil {
				m.statusErr = "config save: " + err.Error()
			}

			m.sessionList.SetTagSessions(m.tagsSet)
			m.statusInfo = "Tags saved"
			return m, clearStatusAfter(2 * time.Second)
		default:
			var cmd tea.Cmd
			ti := m.tagInput
			ti, cmd = ti.Update(msg)
			m.tagInput = ti
			return m, cmd
		}
	}

	// ---------- Alias input focused -----------------------------------------
	if m.aliasInput.Focused() {
		switch {
		case key.Matches(msg, keys.Escape):
			m.aliasInput.Blur()
			return m, nil
		case key.Matches(msg, keys.Enter):
			sessionID := m.aliasInput.SessionID()
			aliasText := m.aliasInput.Value()
			m.aliasInput.Blur()

			if err := m.cfg.SetAlias(sessionID, aliasText); err != nil {
				m.statusErr = err.Error()
				return m, clearStatusAfter(3 * time.Second)
			}

			if err := config.Save(m.cfg); err != nil {
				m.statusErr = "config save: " + err.Error()
				return m, clearStatusAfter(3 * time.Second)
			}

			m.preview.SetAlias(m.cfg.AliasFor(sessionID))
			if m.cfg.AliasFor(sessionID) == "" {
				m.statusInfo = "Alias cleared"
			} else {
				m.statusInfo = "Alias saved"
			}
			return m, clearStatusAfter(2 * time.Second)
		default:
			var cmd tea.Cmd
			ai := m.aliasInput
			ai, cmd = ai.Update(msg)
			m.aliasInput = ai
			return m, cmd
		}
	}

	// ---------- Search bar focused ----------------------------------------
	if m.searchBar.Focused() {
		switch {
		case key.Matches(msg, keys.Escape):
			m.searchBar.Blur()
			m.search.deepSearchPending = false
			m.searchBar.SetSearching(false)
			m.searchBar.SetAISearching(false)
			m.search.copilotSearching = false
			// Cancel any in-flight SDK search.
			if m.search.copilotSearchCancel != nil {
				m.search.copilotSearchCancel()
				m.search.copilotSearchCancel = nil
			}
			// Keep the query active — Escape only dismisses the input focus.
			// The filter stays applied so subsequent operations (time range,
			// sort, pivot) continue to honour the search term. To clear the
			// search, press Escape again from the session list.
			if m.filter.Query != "" || m.searchFilter.HasTokens() {
				m.search.pushHistory(m.searchBar.Value())
				m.filter.DeepSearch = true
				return m, m.loadSessionsCmd()
			}
			return m, nil
		case key.Matches(msg, keys.Enter):
			m.searchBar.Blur()
			m.search.pushHistory(m.searchBar.Value())
			// If deep search hasn't run yet, trigger it now.
			if m.search.deepSearchPending && (m.filter.Query != "" || m.searchFilter.HasTokens()) {
				m.search.deepSearchPending = false
				m.filter.DeepSearch = true
				return m, m.loadSessionsCmd()
			}
			// Ensure deep mode is active for any confirmed query so that
			// subsequent reloads (time range, sort, pivot) also search deeply.
			if m.filter.Query != "" || m.searchFilter.HasTokens() {
				m.filter.DeepSearch = true
			}
			return m, nil
		case msg.String() == "up":
			// Recall an older query. Only the literal arrow key triggers
			// history; the k alias for Up is left to be typed normally.
			if q, ok := m.search.recallPrev(); ok {
				m.searchBar.SetValue(q)
				m.searchBar.CursorEnd()
				return m, m.triggerSearch(nil)
			}
			return m, nil
		case msg.String() == "down":
			// Recall a newer query, clearing the input when moving past the
			// most recent entry. The j alias for Down is left to typing.
			if q, ok := m.search.recallNext(); ok {
				m.searchBar.SetValue(q)
				m.searchBar.CursorEnd()
				return m, m.triggerSearch(nil)
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
			if newQuery != m.search.lastRawInput {
				return m, m.triggerSearch(cmd)
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
		// Exit fullscreen preview back to the split layout.
		if m.previewFullscreen {
			m.previewFullscreen = false
			m.recalcLayout()
			return m, nil
		}
		// Clear active search query when Escape is pressed in the session list.
		if m.filter.Query != "" || m.searchFilter.HasTokens() {
			m.filter.Query = ""
			m.filter.DeepSearch = false
			m.searchFilter = SearchFilter{}
			m.search.lastRawInput = ""
			m.search.historyIdx = len(m.search.history)
			m.clearSearchTokenFilters()
			m.searchBar.SetValue("")
			m.searchBar.SetSearching(false)
			m.searchBar.SetAISearching(false)
			m.searchBar.SetAIResults(0)
			m.search.copilotSearching = false
			m.search.aiSessionIDs = nil
			m.sessionList.SetAISessions(nil)
			return m, m.loadSessionsCmd()
		}
		return m, nil

	case key.Matches(msg, keys.Help):
		m.state = stateHelpOverlay
		return m, nil

	case key.Matches(msg, keys.CmdPalette):
		m.openCmdPalette()
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
			PreviewPosition:   m.cfg.EffectivePreviewPosition(),
			RedactSecrets:     m.cfg.RedactPreviewSecrets,
			ExcludedWords:     strings.Join(m.cfg.ExcludedWords, ", "),
			AutoRefresh:       autoRefreshFieldValue(m.cfg.AutoRefreshSeconds),
			NotifyOnWaiting:   m.cfg.NotifyOnWaiting,
			ShowRepoColumn:    m.cfg.ColumnVisible(config.ColumnRepo),
			ShowFolderColumn:  m.cfg.ColumnVisible(config.ColumnFolder),
			ShowTurnsColumn:   m.cfg.ColumnVisible(config.ColumnTurns),
			ShowHostColumn:    m.cfg.ColumnVisible(config.ColumnHost),
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
		m.cfg.ShowPreview = m.showPreview
		m.saveConfig()
		m.recalcLayout()
		if m.showPreview {
			m.detailVersion++
			return m, m.loadSelectedDetailCmd()
		}
		return m, nil

	case key.Matches(msg, keys.PreviewFullscreen):
		m.previewFullscreen = !m.previewFullscreen
		m.recalcLayout()
		if m.previewFullscreen {
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
		if m.showPreview || m.previewFullscreen {
			before := m.preview.ScrollOffset()
			m.preview.PageUp()
			slog.Debug("preview scroll up", "before", before, "after", m.preview.ScrollOffset())
		}
		return m, nil

	case key.Matches(msg, keys.PreviewScrollDown):
		if m.showPreview || m.previewFullscreen {
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

	case key.Matches(msg, keys.Timeline):
		if m.showPreview && m.detail != nil {
			m.preview.ToggleTimeline()
		}
		return m, nil

	case key.Matches(msg, keys.Reindex):
		if !m.reindexing {
			m.reindexing = true
			m.reindexLog = []string{"Starting rebuild index…"}
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

	case key.Matches(msg, keys.Note):
		return m.handleEditNote()

	case key.Matches(msg, keys.Tags):
		return m.handleEditTags()
	case key.Matches(msg, keys.Alias):
		return m.handleEditAlias()

	case key.Matches(msg, keys.CopyID):
		return m.handleCopyID()

	case key.Matches(msg, keys.CopyPath):
		return m.handleCopyPath()

	case key.Matches(msg, keys.CopyPreview):
		return m.handleCopyPreview()

	case key.Matches(msg, keys.Export):
		return m.handleExport()

	case key.Matches(msg, keys.CopyResumeCommand):
		return m.handleCopyResumeCommand()

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
		m.attentionPicker.SetWorkStatusFilter(m.workStatus.filterWorkStatus)
		m.attentionPicker.SetWorkStatusCounts(m.workStatusCounts())
		m.attentionPicker.SetWorkStatusScanned(m.workStatus.workStatusScanned)
		m.attentionPicker.SetFilterGitDirty(m.filterGitDirty)
		m.attentionPicker.SetGitDirtyCount(m.gitDirtySessionCount())
		m.attentionPicker.SetSize(m.width, m.height)
		m.state = stateAttentionPicker
		return m, nil

	case key.Matches(msg, keys.ViewSwitch):
		m.viewPicker.SetViews(m.cfg.ValidViews())
		m.viewPicker.SetActiveView(m.activeView)
		m.viewPicker.SetSize(m.width, m.height)
		m.state = stateViewPicker
		return m, nil

	case key.Matches(msg, keys.OpenFile):
		if m.detail != nil && len(m.detail.Files) > 0 {
			m.filePicker.SetFiles(m.detail.Files)
			m.filePicker.SetSize(m.width, m.height)
			m.state = stateFilePicker
		}
		return m, nil

	case key.Matches(msg, keys.OpenDir):
		cwd := m.selectedSessionCwd()
		if cwd == "" {
			m.statusErr = "No working directory for this session"
			return m, clearStatusAfter(2 * time.Second)
		}
		return m, m.openDirCmd(cwd)

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
		if !m.workStatus.workStatusScanning {
			m.workStatus.workStatusScanning = true
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

	case key.Matches(msg, keys.Compare):
		return m.handleCompare()
	}

	return m, nil
}

// handleHideSession toggles the hidden state of the currently selected session.
func (m Model) handleHideSession() (tea.Model, tea.Cmd) {
	// When one or more sessions are marked with Space, act on the whole set.
	if m.sessionList.SelectionCount() > 0 {
		return m.handleBulkHide(m.sessionList.SelectedSessions())
	}

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

// handleBulkHide hides or unhides every session in sessions with a single
// deterministic target state: if any session is currently visible, the whole
// set is hidden; otherwise the whole set is unhidden. Config is written once.
func (m Model) handleBulkHide(sessions []data.Session) (tea.Model, tea.Cmd) {
	if len(sessions) == 0 {
		return m, nil
	}

	// If any selected session is visible (not hidden), hide the set.
	hide := false
	for _, sess := range sessions {
		if _, hidden := m.hiddenSet[sess.ID]; !hidden {
			hide = true
			break
		}
	}

	for _, sess := range sessions {
		if hide {
			m.hiddenSet[sess.ID] = struct{}{}
			// Hiding a favorited session also removes it from favorites.
			delete(m.favoritedSet, sess.ID)
		} else {
			delete(m.hiddenSet, sess.ID)
		}
	}

	m.cfg.HiddenSessions = sortedKeys(m.hiddenSet)
	m.cfg.FavoriteSessions = sortedKeys(m.favoritedSet)
	if err := config.Save(m.cfg); err != nil {
		m.statusErr = "config save: " + err.Error()
	}

	m.sessionList.DeselectAll()
	m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
	m.sessionList.SetFavoritedSessions(m.favoritedSet)
	cmd := m.loadSessionsCmd()
	return m, cmd
}

// handleToggleFavorite toggles the favorited state of the currently selected session.
func (m Model) handleToggleFavorite() (tea.Model, tea.Cmd) {
	// When one or more sessions are marked with Space, act on the whole set.
	if m.sessionList.SelectionCount() > 0 {
		return m.handleBulkFavorite(m.sessionList.SelectedSessions())
	}

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

// handleBulkFavorite favorites or unfavorites every eligible session in
// sessions with a single deterministic target state: if any non-hidden
// session is not yet a favorite, the whole eligible set is favorited;
// otherwise it is unfavorited. Hidden sessions are skipped (they cannot be
// favorited). Config is written once.
func (m Model) handleBulkFavorite(sessions []data.Session) (tea.Model, tea.Cmd) {
	// Only non-hidden sessions are eligible to be favorited.
	eligible := make([]data.Session, 0, len(sessions))
	for _, sess := range sessions {
		if _, hidden := m.hiddenSet[sess.ID]; !hidden {
			eligible = append(eligible, sess)
		}
	}
	if len(eligible) == 0 {
		return m, nil
	}

	// If any eligible session is not a favorite, favorite the whole set.
	favorite := false
	for _, sess := range eligible {
		if _, fav := m.favoritedSet[sess.ID]; !fav {
			favorite = true
			break
		}
	}

	for _, sess := range eligible {
		if favorite {
			m.favoritedSet[sess.ID] = struct{}{}
		} else {
			delete(m.favoritedSet, sess.ID)
		}
	}

	m.cfg.FavoriteSessions = sortedKeys(m.favoritedSet)
	if err := config.Save(m.cfg); err != nil {
		m.statusErr = "config save: " + err.Error()
	}

	m.sessionList.SetFavoritedSessions(m.favoritedSet)
	cmd := m.loadSessionsCmd()
	return m, cmd
}

// handleEditNote opens the inline note input for the currently selected session.
func (m Model) handleEditNote() (tea.Model, tea.Cmd) {
	sess, ok := m.sessionList.Selected()
	if !ok {
		return m, nil
	}
	currentNote := ""
	if m.cfg.SessionNotes != nil {
		currentNote = m.cfg.SessionNotes[sess.ID]
	}
	cmd := m.noteInput.Focus(sess.ID, currentNote)
	return m, cmd
}

// handleEditTags opens the inline tag input for the currently selected session.
func (m Model) handleEditTags() (tea.Model, tea.Cmd) {
	sess, ok := m.sessionList.Selected()
	if !ok {
		return m, nil
	}
	current := strings.Join(m.cfg.TagsFor(sess.ID), ", ")
	cmd := m.tagInput.Focus(sess.ID, current)
	return m, cmd
}

// handleEditAlias opens the inline alias input for the currently selected session.
func (m Model) handleEditAlias() (tea.Model, tea.Cmd) {
	sess, ok := m.sessionList.Selected()
	if !ok {
		return m, nil
	}
	current := m.cfg.AliasFor(sess.ID)
	cmd := m.aliasInput.Focus(sess.ID, current)
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

// handleCopyPath copies the selected session's working directory to the
// system clipboard. When a folder row is selected under a folder or repo
// pivot, the folder's path is copied instead.
func (m Model) handleCopyPath() (tea.Model, tea.Cmd) {
	var path string
	if sess, ok := m.sessionList.Selected(); ok {
		path = sess.Cwd
	} else {
		path = m.sessionList.SelectedFolderCwd()
	}
	if path == "" {
		m.statusInfo = "No path to copy"
		return m, clearStatusAfter(2 * time.Second)
	}
	if err := clipboardWrite(path); err != nil {
		m.statusErr = "clipboard: " + err.Error()
		return m, clearStatusAfter(2 * time.Second)
	}
	m.statusInfo = statusCopiedPath
	return m, clearStatusAfter(2 * time.Second)
}

// handleCopyResumeCommand copies the resume command(s) to the system clipboard.
// When multi-select is active, one resume command per selected session is
// copied, joined by newlines and ordered to match the list. Otherwise only the
// current session's command is copied. Copied commands mirror the same launch
// options Dispatch uses when opening sessions so they match the configured
// workflow.
func (m Model) handleCopyResumeCommand() (tea.Model, tea.Cmd) {
	var sessions []data.Session
	if sel := m.sessionList.SelectedSessions(); len(sel) > 0 {
		sessions = sel
	} else if sess, ok := m.sessionList.Selected(); ok {
		sessions = []data.Session{sess}
	}
	if len(sessions) == 0 {
		return m, nil
	}

	cmds := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		cmd, err := platform.BuildResumeCommandString(sess.ID, m.resumeConfigForSession(sess.Cwd))
		if err != nil {
			m.statusErr = "resume command: " + err.Error()
			return m, clearStatusAfter(2 * time.Second)
		}
		cmds = append(cmds, cmd)
	}

	if err := clipboardWrite(strings.Join(cmds, "\n")); err != nil {
		m.statusErr = "clipboard: " + err.Error()
		return m, clearStatusAfter(2 * time.Second)
	}

	if len(cmds) == 1 {
		m.statusInfo = statusCopiedResumeCommand
	} else {
		m.statusInfo = fmt.Sprintf("Copied %d resume commands ✓", len(cmds))
	}
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

// handleExport exports the selected session(s) to Markdown files in the
// config directory's exports folder. When multi-select is active, all
// selected sessions are exported; otherwise only the current session.
func (m Model) handleExport() (tea.Model, tea.Cmd) {
	if m.store == nil {
		return m, nil
	}

	store := m.store
	var ids []string

	if sel := m.sessionList.SelectedSessions(); len(sel) > 0 {
		for _, s := range sel {
			ids = append(ids, s.ID)
		}
	} else if sess, ok := m.sessionList.Selected(); ok {
		ids = append(ids, sess.ID)
	}

	if len(ids) == 0 {
		return m, nil
	}

	return m, func() tea.Msg {
		exportDir, err := data.ExportDir()
		if err != nil {
			return exportDoneMsg{err: err}
		}

		var paths []string
		for _, id := range ids {
			detail, err := store.GetSession(context.Background(), id)
			if err != nil {
				slog.Warn("export: failed to load session", "id", id, "err", err)
				continue
			}
			path, err := data.ExportSession(detail, exportDir)
			if err != nil {
				return exportDoneMsg{err: err}
			}
			paths = append(paths, path)
		}
		return exportDoneMsg{paths: paths}
	}
}

// handleExportDone processes the result of an async export operation.
func (m Model) handleExportDone(msg exportDoneMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.statusErr = "export: " + msg.err.Error()
		return m, clearStatusAfter(3 * time.Second)
	}
	if len(msg.paths) == 0 {
		m.statusErr = "export: no sessions exported"
		return m, clearStatusAfter(2 * time.Second)
	}
	if len(msg.paths) == 1 {
		m.statusInfo = "Exported: " + filepath.Base(msg.paths[0])
	} else {
		m.statusInfo = fmt.Sprintf("Exported %d sessions", len(msg.paths))
	}
	return m, clearStatusAfter(3 * time.Second)
}

// handleCompare opens the compare view when exactly two sessions are selected.
// Otherwise, it shows a status hint.
func (m Model) handleCompare() (tea.Model, tea.Cmd) {
	sel := m.sessionList.SelectedSessions()
	if len(sel) != 2 {
		m.statusInfo = "Select exactly 2 sessions to compare (space to toggle)"
		return m, clearStatusAfter(3 * time.Second)
	}

	store := m.store
	if store == nil {
		return m, nil
	}

	idA, idB := sel[0].ID, sel[1].ID
	return m, func() tea.Msg {
		left, err := store.GetSession(context.Background(), idA)
		if err != nil {
			return compareDetailMsg{err: err}
		}
		right, err := store.GetSession(context.Background(), idB)
		if err != nil {
			return compareDetailMsg{err: err}
		}
		return compareDetailMsg{left: left, right: right}
	}
}

// handleCompareDetail processes the async result of loading two sessions for comparison.
func (m Model) handleCompareDetail(msg compareDetailMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.statusErr = "compare: " + msg.err.Error()
		return m, clearStatusAfter(3 * time.Second)
	}
	m.compareView.SetSessions(msg.left, msg.right)
	m.compareView.SetSize(m.width, m.height)
	m.state = stateCompareView
	return m, nil
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
			return m, m.loadSessionsCmd()
		default:
			var cmd tea.Cmd
			m.configPanel, cmd = m.configPanel.Update(msg)
			return m, cmd
		}
	}

	switch {
	case key.Matches(msg, keys.Escape):
		m.state = stateSessionList
		return m, m.loadSessionsCmd()
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
	m.cfg.RedactPreviewSecrets = v.RedactSecrets
	m.preview.SetRedactSecrets(v.RedactSecrets)
	m.cfg.ExcludedWords = parseExcludedWords(v.ExcludedWords)
	m.filter.ExcludedWords = m.cfg.ExcludedWords
	m.cfg.AutoRefreshSeconds = parseAutoRefresh(v.AutoRefresh)
	m.cfg.NotifyOnWaiting = v.NotifyOnWaiting
	m.cfg.HiddenColumns = hiddenColumnsFromPanel(v)
	m.sessionList.SetHiddenColumns(m.cfg.HiddenColumns)
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

// hiddenColumnsFromPanel builds the config's HiddenColumns list from the
// settings panel's per-column visibility flags. Only disabled columns are
// recorded, so the default (all columns shown) is an empty list.
func hiddenColumnsFromPanel(v components.ConfigValues) []string {
	var hidden []string
	if !v.ShowRepoColumn {
		hidden = append(hidden, config.ColumnRepo)
	}
	if !v.ShowFolderColumn {
		hidden = append(hidden, config.ColumnFolder)
	}
	if !v.ShowTurnsColumn {
		hidden = append(hidden, config.ColumnTurns)
	}
	if !v.ShowHostColumn {
		hidden = append(hidden, config.ColumnHost)
	}
	return hidden
}

// parseExcludedWords splits a comma-separated string into trimmed, non-empty words.
func parseExcludedWords(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	words := make([]string, 0, len(parts))
	for _, p := range parts {
		w := strings.TrimSpace(p)
		if w != "" {
			words = append(words, w)
		}
	}
	if len(words) == 0 {
		return nil
	}
	return words
}

// autoRefreshFieldValue renders the config's AutoRefreshSeconds as the string
// the settings panel edits: empty for unset (default), or the integer seconds.
func autoRefreshFieldValue(secs *int) string {
	if secs == nil {
		return ""
	}
	return strconv.Itoa(*secs)
}

// parseAutoRefresh converts the settings-panel auto-refresh string back into a
// config value. An empty or unparseable string means unset (nil), so the
// default interval is used; otherwise the parsed integer is stored (including
// 0 to disable polling). Range handling is applied by EffectiveAutoRefreshInterval.
func parseAutoRefresh(s string) *int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
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
		isDoubleClick := m.click.pendingClickVersion > 0 &&
			mouse.Y == m.click.pendingClickY

		if isDoubleClick {
			// Invalidate the pending single-click so it won't fire.
			m.click.pendingClickVersion = 0

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
		m.click.pendingClickVersion++
		m.click.pendingClickY = mouse.Y
		m.click.pendingClickItemIdx = itemIdx

		// Immediately move selection so the highlight follows the cursor,
		// but do NOT toggle folders or load details yet.
		m.sessionList.MoveTo(itemIdx)

		ver := m.click.pendingClickVersion
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
		m.attentionPicker.SetWorkStatusFilter(m.workStatus.filterWorkStatus)
		m.attentionPicker.SetWorkStatusCounts(m.workStatusCounts())
		m.attentionPicker.SetWorkStatusScanned(m.workStatus.workStatusScanned)
		m.attentionPicker.SetFilterGitDirty(m.filterGitDirty)
		m.attentionPicker.SetGitDirtyCount(m.gitDirtySessionCount())
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
	sortArrowRendered := styles.DimmedStyle.Render(arrow + " ")
	sortArrowW := lipgloss.Width(sortArrowRendered)
	sortFullRendered := sortKeyRendered + styles.DimmedStyle.Render(": "+sortLabel)
	w := lipgloss.Width(sortFullRendered)
	if x >= cursor && x < cursor+w {
		// Click on the arrow portion (including trailing space) toggles order;
		// elsewhere cycles sort field.
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
		pivotArrowRendered := styles.DimmedStyle.Render(styles.IconSortDown() + " ")
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
		expandIcon := styles.IconExpandAll()
		if m.sessionList.AllExpanded() {
			expandIcon = styles.IconCollapseAll()
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

	// Fullscreen preview fills the entire content area; the list is hidden.
	if m.previewFullscreen {
		return strings.Join([]string{header, badges, sep, m.preview.View(), footer}, "\n")
	}

	var content string
	hasPreview := l.previewWidth > 0 && l.previewHeight > 0

	if hasPreview {
		gap := strings.Repeat(" ", gapWidth)
		switch l.previewPosition {
		case config.PreviewPositionLeft:
			content = lipgloss.JoinHorizontal(
				lipgloss.Top,
				m.preview.View(),
				gap,
				m.sessionList.View(),
			)
		case config.PreviewPositionTop:
			content = lipgloss.JoinVertical(
				lipgloss.Left,
				m.preview.View(),
				"",
				m.sessionList.View(),
			)
		case config.PreviewPositionBottom:
			content = lipgloss.JoinVertical(
				lipgloss.Left,
				m.sessionList.View(),
				"",
				m.preview.View(),
			)
		default: // right
			content = lipgloss.JoinHorizontal(
				lipgloss.Top,
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
	} else if m.workStatus.workStatusScanning {
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
		right = m.spinner.View() + " Rebuilding index…"
	} else if m.workStatus.workStatusScanning {
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
	parts := make([]string, 0, len(badges)+4)
	for _, b := range badges {
		parts = append(parts, styles.BadgeStyle.Render(b))
	}

	// Search token badge — shows active structured tokens.
	if summary := m.searchFilter.TokenSummary(); summary != "" {
		parts = append(parts, styles.ActiveBadgeStyle.Render(summary))
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
		expandIcon := styles.IconExpandAll()
		if m.sessionList.AllExpanded() {
			expandIcon = styles.IconCollapseAll()
		}
		parts = append(parts, styles.KeyStyle.Render("x")+styles.DimmedStyle.Render(": "+expandIcon))
	}

	// Favorites filter indicator.
	if m.showFavorited {
		parts = append(parts, styles.ActiveBadgeStyle.Render("★ Favorites"))
	}

	// Active named view indicator.
	if m.activeView != "" {
		parts = append(parts, styles.ActiveBadgeStyle.Render("⊞ "+m.activeView))
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
	// When note input is active, show it instead of the normal footer.
	if m.noteInput.Focused() {
		m.noteInput.SetWidth(m.width)
		return m.noteInput.View()
	}

	// When tag input is active, show it instead of the normal footer.
	if m.tagInput.Focused() {
		m.tagInput.SetWidth(m.width)
		return m.tagInput.View()
	}

	// When alias input is active, show it instead of the normal footer.
	if m.aliasInput.Focused() {
		m.aliasInput.SetWidth(m.width)
		return m.aliasInput.View()
	}

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
	if m.filterGitDirty {
		left += "  " + styles.ActiveBadgeStyle.Render("! git changes")
	}
	if len(m.workStatus.filterWorkStatus) > 0 {
		var wsNames []string
		if _, ok := m.workStatus.filterWorkStatus[data.WorkStatusIncomplete]; ok {
			wsNames = append(wsNames, "incomplete")
		}
		if _, ok := m.workStatus.filterWorkStatus[data.WorkStatusComplete]; ok {
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
			left += "  " + styles.DimStyle.Render("indexed "+components.RelativeTime(t.UTC().Format(time.RFC3339))+" · r rebuild index")
		} else {
			left += "  " + styles.DimStyle.Render("r rebuild index")
		}
	}

	ver := styles.DimStyle.Render(version.Version)

	// Right: context-sensitive keybinding hints from help.Model.
	right := m.help.ShortView()

	// If hints + left + version exceed width, drop the hints entirely
	// to avoid wrapping. Byte-level truncation corrupts ANSI codes.
	usedWidth := lipgloss.Width(left) + lipgloss.Width(ver) + footerGapReserve
	if usedWidth+lipgloss.Width(right) > m.width {
		right = ""
	}

	// Use m.width-2 so the footer totals m.width-1 characters, avoiding
	// exact-terminal-width lines that could autowrap.
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - lipgloss.Width(ver) - 2
	if gap < 1 {
		gap = 1
	}

	line := left + strings.Repeat(" ", gap) + right + " " + ver
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

	title := styles.OverlayTitleStyle.Render(m.spinner.View() + " Rebuild Index")

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
// Shared predicate-based filter helpers
// ---------------------------------------------------------------------------

// filterSessionsWhere returns only the sessions for which pred returns true.
func filterSessionsWhere(sessions []data.Session, pred func(data.Session) bool) []data.Session {
	filtered := make([]data.Session, 0, len(sessions))
	for _, s := range sessions {
		if pred(s) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// filterGroupsWhere applies pred to every session inside each group, keeping
// only sessions that match. Groups left empty after filtering are dropped and
// each retained group's Count is updated to reflect the new length.
func filterGroupsWhere(groups []data.SessionGroup, pred func(data.Session) bool) []data.SessionGroup {
	filtered := make([]data.SessionGroup, 0, len(groups))
	for _, g := range groups {
		var sessions []data.Session
		for _, s := range g.Sessions {
			if pred(s) {
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
// Session list synchronisation helpers
// ---------------------------------------------------------------------------

// syncSessionListStatuses pushes all current status maps (hidden, favorited,
// attention, plan, work status) to the sessionList component. Call this after
// loading or filtering sessions/groups to keep the list's decorations in sync.
func (m *Model) syncSessionListStatuses() {
	m.sessionList.SetHiddenSessions(m.visibleHiddenSet())
	m.sessionList.SetFavoritedSessions(m.favoritedSet)
	m.sessionList.SetNoteSessions(m.notesSet)
	m.sessionList.SetTagSessions(m.tagsSet)
	m.sessionList.SetAttentionStatuses(m.attentionMap)
	m.sessionList.SetPlanStatuses(m.planMap)
	m.sessionList.SetWorkStatuses(m.workStatus.workStatusMap)
	m.sessionList.SetGitStates(m.gitStateMap)
}

// syncSessionListWorkStatuses pushes only the work status map to the
// sessionList component. Use when only work statuses have changed.
func (m *Model) syncSessionListWorkStatuses() {
	m.sessionList.SetWorkStatuses(m.workStatus.workStatusMap)
}

// ---------------------------------------------------------------------------
// Composite filter helpers
// ---------------------------------------------------------------------------

// applySessionFilters runs the full session filter chain (hidden, favorited,
// attention, plan, work status) and returns the filtered result.
func (m *Model) applySessionFilters(sessions []data.Session) []data.Session {
	sessions = m.filterHiddenSessions(sessions)
	sessions = m.filterFavoritedSessions(sessions)
	sessions = m.filterAttentionSessions(sessions)
	sessions = m.filterPlanSessions(sessions)
	sessions = m.filterWorkStatusSessions(sessions)
	sessions = m.filterGitDirtySessions(sessions)
	sessions = m.filterTaggedSessions(sessions)
	return sessions
}

// applyGroupFilters runs the full group filter chain (hidden, favorited,
// attention, plan, work status, git state) and returns the filtered result.
func (m *Model) applyGroupFilters(groups []data.SessionGroup) []data.SessionGroup {
	groups = m.filterHiddenGroups(groups)
	groups = m.filterFavoritedGroups(groups)
	groups = m.filterAttentionGroups(groups)
	groups = m.filterPlanGroups(groups)
	groups = m.filterWorkStatusGroups(groups)
	groups = m.filterGitDirtyGroups(groups)
	groups = m.filterTaggedGroups(groups)
	return groups
}

// ---------------------------------------------------------------------------
// Tag filtering
// ---------------------------------------------------------------------------

// filterTaggedSessions keeps only sessions carrying the active tag filter.
// When no tag filter is set the input is returned unchanged.
func (m *Model) filterTaggedSessions(sessions []data.Session) []data.Session {
	if m.tagFilter == "" {
		return sessions
	}
	return filterSessionsWhere(sessions, func(s data.Session) bool {
		return m.cfg.HasTag(s.ID, m.tagFilter)
	})
}

// filterTaggedGroups keeps only sessions carrying the active tag filter
// within each group. When no tag filter is set the input is returned unchanged.
func (m *Model) filterTaggedGroups(groups []data.SessionGroup) []data.SessionGroup {
	if m.tagFilter == "" {
		return groups
	}
	return filterGroupsWhere(groups, func(s data.Session) bool {
		return m.cfg.HasTag(s.ID, m.tagFilter)
	})
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
	return filterSessionsWhere(sessions, func(s data.Session) bool {
		_, ok := m.hiddenSet[s.ID]
		return !ok
	})
}

// filterHiddenGroups removes hidden sessions from grouped results unless
// showHidden mode is active. Empty groups are dropped.
func (m *Model) filterHiddenGroups(groups []data.SessionGroup) []data.SessionGroup {
	if m.showHidden || len(m.hiddenSet) == 0 {
		return groups
	}
	return filterGroupsWhere(groups, func(s data.Session) bool {
		_, ok := m.hiddenSet[s.ID]
		return !ok
	})
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
	return filterSessionsWhere(sessions, func(s data.Session) bool {
		_, ok := m.favoritedSet[s.ID]
		return ok
	})
}

// filterFavoritedGroups returns only favorited sessions within each group
// when showFavorited mode is active. Empty groups are dropped.
func (m *Model) filterFavoritedGroups(groups []data.SessionGroup) []data.SessionGroup {
	if !m.showFavorited {
		return groups
	}
	return filterGroupsWhere(groups, func(s data.Session) bool {
		_, ok := m.favoritedSet[s.ID]
		return ok
	})
}

// ---------------------------------------------------------------------------
// Attention session filtering
// ---------------------------------------------------------------------------

// filterAttentionSessions removes sessions that don't match the attention
// filter. When attentionFilter is empty, all sessions pass through.
func (m *Model) filterAttentionSessions(sessions []data.Session) []data.Session {
	if len(m.attentionFilter) == 0 || len(m.attentionMap) == 0 {
		return sessions
	}
	return filterSessionsWhere(sessions, func(s data.Session) bool {
		status := m.attentionMap[s.ID]
		_, ok := m.attentionFilter[status]
		return ok
	})
}

// filterAttentionGroups removes sessions that don't match the attention
// filter from grouped results. Empty groups are dropped.
func (m *Model) filterAttentionGroups(groups []data.SessionGroup) []data.SessionGroup {
	if len(m.attentionFilter) == 0 || len(m.attentionMap) == 0 {
		return groups
	}
	return filterGroupsWhere(groups, func(s data.Session) bool {
		status := m.attentionMap[s.ID]
		_, ok := m.attentionFilter[status]
		return ok
	})
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
	return filterSessionsWhere(sessions, func(s data.Session) bool {
		return m.planMap[s.ID]
	})
}

// filterPlanGroups removes sessions without a plan.md file from grouped
// results when filterPlans is active. Empty groups are dropped.
func (m *Model) filterPlanGroups(groups []data.SessionGroup) []data.SessionGroup {
	if !m.filterPlans || len(m.planMap) == 0 {
		return groups
	}
	return filterGroupsWhere(groups, func(s data.Session) bool {
		return m.planMap[s.ID]
	})
}

// ---------------------------------------------------------------------------
// Work status session filtering
// ---------------------------------------------------------------------------

// filterWorkStatusSessions removes sessions that don't match the active
// work status filter set. When filterWorkStatus is empty, all sessions
// pass through.
func (m *Model) filterWorkStatusSessions(sessions []data.Session) []data.Session {
	if len(m.workStatus.filterWorkStatus) == 0 || len(m.workStatus.workStatusMap) == 0 {
		return sessions
	}
	return filterSessionsWhere(sessions, func(s data.Session) bool {
		result, ok := m.workStatus.workStatusMap[s.ID]
		if !ok {
			return false
		}
		_, match := m.workStatus.filterWorkStatus[result.Status]
		return match
	})
}

// filterWorkStatusGroups removes sessions that don't match the active
// work status filter set from grouped results. Empty groups are dropped.
func (m *Model) filterWorkStatusGroups(groups []data.SessionGroup) []data.SessionGroup {
	if len(m.workStatus.filterWorkStatus) == 0 || len(m.workStatus.workStatusMap) == 0 {
		return groups
	}
	return filterGroupsWhere(groups, func(s data.Session) bool {
		result, ok := m.workStatus.workStatusMap[s.ID]
		if !ok {
			return false
		}
		_, match := m.workStatus.filterWorkStatus[result.Status]
		return match
	})
}

// ---------------------------------------------------------------------------
// Git state session filtering
// ---------------------------------------------------------------------------

// filterGitDirtySessions removes sessions that don't have local git changes
// (dirty, untracked, ahead, or behind) when filterGitDirty is active.
func (m *Model) filterGitDirtySessions(sessions []data.Session) []data.Session {
	if !m.filterGitDirty || len(m.gitStateMap) == 0 {
		return sessions
	}
	return filterSessionsWhere(sessions, func(s data.Session) bool {
		state := m.gitStateMap[s.ID]
		return state == platform.GitStateDirty ||
			state == platform.GitStateUntracked ||
			state == platform.GitStateAhead ||
			state == platform.GitStateBehind
	})
}

// filterGitDirtyGroups removes sessions without local git changes from grouped
// results when filterGitDirty is active. Empty groups are dropped.
func (m *Model) filterGitDirtyGroups(groups []data.SessionGroup) []data.SessionGroup {
	if !m.filterGitDirty || len(m.gitStateMap) == 0 {
		return groups
	}
	return filterGroupsWhere(groups, func(s data.Session) bool {
		state := m.gitStateMap[s.ID]
		return state == platform.GitStateDirty ||
			state == platform.GitStateUntracked ||
			state == platform.GitStateAhead ||
			state == platform.GitStateBehind
	})
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

// sortByFrecency re-sorts the session slice by frecency when the current
// sort field is SortByFrecency. Sessions the user launches often and
// recently sort first; sessions with no launch history keep their incoming
// (updated-time) order via a stable sort.
func (m *Model) sortByFrecency(sessions []data.Session) {
	if m.sort.Field != data.SortByFrecency {
		return
	}
	now := time.Now()
	slices.SortStableFunc(sessions, func(a, b data.Session) int {
		sa := config.FrecencyScore(m.cfg.SessionLaunches[a.ID], now)
		sb := config.FrecencyScore(m.cfg.SessionLaunches[b.ID], now)
		if m.sort.Order == data.Ascending {
			return cmp.Compare(sa, sb)
		}
		return cmp.Compare(sb, sa)
	})
}

// ---------------------------------------------------------------------------
// Sort / pivot cycling
// ---------------------------------------------------------------------------

var sortFields = []data.SortField{
	data.SortByUpdated,
	data.SortByFolder,
	data.SortByName,
	data.SortByAttention,
	data.SortByFrecency,
}

func (m *Model) cycleSort() {
	for i, f := range sortFields {
		if f == m.sort.Field {
			m.sort.Field = sortFields[(i+1)%len(sortFields)]
			m.cfg.DefaultSort = sortFieldToConfig(m.sort.Field)
			m.saveConfig()
			return
		}
	}
	m.sort.Field = data.SortByUpdated
	m.cfg.DefaultSort = sortFieldToConfig(m.sort.Field)
	m.saveConfig()
}

func (m *Model) toggleSortOrder() {
	if m.sort.Order == data.Descending {
		m.sort.Order = data.Ascending
	} else {
		m.sort.Order = data.Descending
	}
	m.cfg.DefaultSortOrder = sortOrderToConfig(m.sort.Order)
	m.saveConfig()
}

var pivotModes = []string{pivotNone, pivotFolder, pivotRepo, pivotBranch, pivotDate, pivotHost}

func (m *Model) cyclePivot() {
	for i, p := range pivotModes {
		if p == m.pivot {
			m.pivot = pivotModes[(i+1)%len(pivotModes)]
			m.pivotOrder = defaultPivotOrder(m.pivot)
			m.cfg.DefaultPivot = m.pivot
			m.saveConfig()
			return
		}
	}
	m.pivot = pivotNone
	m.pivotOrder = data.Ascending
	m.cfg.DefaultPivot = m.pivot
	m.saveConfig()
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
// Config persistence
// ---------------------------------------------------------------------------

// saveConfig writes the current config to disk. On failure it sets statusErr
// so the user sees a transient notification.
func (m *Model) saveConfig() {
	if err := config.Save(m.cfg); err != nil {
		m.statusErr = "config save: " + err.Error()
	}
}

// ---------------------------------------------------------------------------
// Time range
// ---------------------------------------------------------------------------

func (m *Model) setTimeRange(tr string) tea.Cmd {
	m.timeRange = tr
	m.cfg.DefaultTimeRange = tr
	m.saveConfig()
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
	case data.SortByFrecency:
		return "frecency"
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
// launchNewInFolder to avoid duplicating shell-resolution logic.
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
	m.recordLaunch(sessionID)
	cfg := m.resumeConfigForSession(cwd)
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
	m.recordLaunch(sessionID)
	cfg := m.resumeConfigForSession(cwd)
	cfg.LaunchStyle = launchStyle
	return func() tea.Msg {
		if err := platform.LaunchSession(shell, sessionID, cfg); err != nil {
			detail := fmt.Sprintf("launch failed: %v (shell=%s, terminal=%s)",
				err, shell.Name, cfg.Terminal)
			return dataErrorMsg{err: errors.New(detail)}
		}
		return nil
	}
}

func (m Model) resumeConfigForSession(cwd string) platform.ResumeConfig {
	return platform.ResumeConfig{
		YoloMode:      m.cfg.YoloMode,
		Agent:         m.cfg.Agent,
		Model:         m.cfg.Model,
		Terminal:      m.cfg.DefaultTerminal,
		CustomCommand: m.cfg.CustomCommand,
		Cwd:           cwd,
		PaneDirection: m.cfg.EffectivePaneDirection(),
	}
}

// recordLaunch stamps a session launch in the config so the frecency sort
// can rank frequently and recently launched sessions first. Empty IDs (new
// sessions started from a folder) are ignored.
func (m *Model) recordLaunch(sessionID string) {
	if sessionID == "" {
		return
	}
	m.cfg.RecordLaunch(sessionID, time.Now())
	m.saveConfig()
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
	// Cancel any in-flight copilot search context so the goroutine
	// holding searchMu can unblock and exit.
	if m.search.copilotSearchCancel != nil {
		m.search.copilotSearchCancel()
		m.search.copilotSearchCancel = nil
	}
	// Cancel any in-flight AI work-status analysis.
	if m.workStatus.workStatusAICancel != nil {
		m.workStatus.workStatusAICancel()
		m.workStatus.workStatusAICancel = nil
	}
	if m.dbWatcher != nil {
		m.dbWatcher.Stop()
	}
	if m.dbWatchCh != nil {
		close(m.dbWatchCh)
		m.dbWatchCh = nil
	}
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

// waitForDBChangeCmd blocks on the dbWatchCh channel until the DB watcher
// fires, then returns a sessionsChangedMsg to trigger a session list reload.
func (m Model) waitForDBChangeCmd() tea.Cmd {
	ch := m.dbWatchCh
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		_, ok := <-ch
		if !ok {
			return nil // channel closed — watcher stopped
		}
		return sessionsChangedMsg{}
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
			groups, err := store.GroupSessions(context.Background(), pf, filter, sortOpts, limit)
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
		sessions, err := store.ListSessions(context.Background(), filter, sortOpts, limit)
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

// scanGitStatesCmd runs git workspace state detection for all visible sessions.
// It collects session IDs and their working directories, then runs bounded
// git commands in the background.
func (m Model) scanGitStatesCmd() tea.Cmd {
	// Build a map of session ID to directory for all loaded sessions.
	sessionDirs := make(map[string]string)
	if m.sessions != nil {
		for _, s := range m.sessions {
			if s.Cwd != "" {
				sessionDirs[s.ID] = s.Cwd
			}
		}
	}
	for _, g := range m.groups {
		for _, s := range g.Sessions {
			if s.Cwd != "" {
				sessionDirs[s.ID] = s.Cwd
			}
		}
	}
	if len(sessionDirs) == 0 {
		return nil
	}

	// In demo mode, return synthetic git states so all badge types are visible
	// without requiring real git repos on disk.
	if os.Getenv("DISPATCH_DEMO_GIT_STATES") != "" {
		return func() tea.Msg {
			states := demoGitStates(sessionDirs)
			return gitStateScannedMsg{states: states}
		}
	}

	return func() tea.Msg {
		states := platform.ScanGitStates(sessionDirs)
		return gitStateScannedMsg{states: states}
	}
}

// demoGitStates assigns a rotating set of git states to sessions so all
// badge types are visible in demo mode. The order cycles through dirty,
// ahead, untracked, behind, clean, and missing.
func demoGitStates(sessionDirs map[string]string) map[string]platform.GitState {
	cycle := []platform.GitState{
		platform.GitStateDirty,
		platform.GitStateAhead,
		platform.GitStateUntracked,
		platform.GitStateBehind,
		platform.GitStateClean,
		platform.GitStateMissing,
	}
	states := make(map[string]platform.GitState, len(sessionDirs))
	i := 0
	for id := range sessionDirs {
		states[id] = cycle[i%len(cycle)]
		i++
	}
	return states
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
	for id, result := range m.workStatus.workStatusMap {
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
	if m.workStatus.workStatusAICancel != nil {
		m.workStatus.workStatusAICancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	m.workStatus.workStatusAICancel = cancel

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
		result, ok := m.workStatus.workStatusMap[id]
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
	m.workStatus.workStatusScanned = true
	wasScanning := m.workStatus.workStatusScanning
	m.workStatus.workStatusScanning = false

	if wasScanning {
		// Count incomplete/complete from the work status map.
		var incomplete, complete int
		for _, r := range m.workStatus.workStatusMap {
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
	if wasScanning && len(m.workStatus.filterWorkStatus) > 0 {
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
	for _, r := range m.workStatus.workStatusMap {
		counts[r.Status]++
	}
	return counts
}

// gitDirtySessionCount returns the number of sessions with local git changes
// (dirty, untracked, ahead, or behind).
func (m Model) gitDirtySessionCount() int {
	n := 0
	for _, state := range m.gitStateMap {
		switch state {
		case platform.GitStateDirty, platform.GitStateUntracked, platform.GitStateAhead, platform.GitStateBehind:
			n++
		case platform.GitStateUnknown, platform.GitStateClean, platform.GitStateMissing:
			// Not counted as "dirty".
		}
	}
	return n
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
			groups, err := store.GroupSessions(context.Background(), pf, filter, sortOpts, limit)
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
		sessions, err := store.ListSessions(context.Background(), filter, sortOpts, limit)
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
		detail, err := store.GetSession(context.Background(), id)
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
		folders, err := store.ListFolders(context.Background())
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
// The search context is stored in m.search.copilotSearchCancel so that a newer
// search (or Escape) can cancel this one, unblocking the searchMu.
func (m *Model) copilotSearchCmd(version int) tea.Cmd {
	client := m.copilotClient
	query := m.filter.Query

	ctx, cancel := context.WithTimeout(context.Background(), copilotSearchTimeout)
	m.search.copilotSearchCancel = cancel

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
		sessions, err := store.ListSessionsByIDs(context.Background(), ids)
		if err != nil {
			// Silently degrade — don't break the UI for fetch errors.
			return aiSessionsLoadedMsg{version: version}
		}
		return aiSessionsLoadedMsg{version: version, sessions: sessions}
	}
}

// openFileCmd opens a file using the platform default application. It checks
// that the file exists before attempting to open it.
func (m Model) openFileCmd(path string) tea.Cmd {
	return func() tea.Msg {
		if _, err := os.Stat(path); err != nil {
			return fileOpenedMsg{path: path, err: fmt.Errorf("file not found: %s", path)}
		}
		err := platform.OpenFile(path)
		return fileOpenedMsg{path: path, err: err}
	}
}

// openDirCmd opens a directory in the platform file manager. Validation of the
// path lives in platform.OpenDir so the failure message is consistent.
func (m Model) openDirCmd(path string) tea.Cmd {
	return func() tea.Msg {
		err := platform.OpenDir(path)
		return dirOpenedMsg{path: path, err: err}
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
	case "frecency":
		return data.SortByFrecency
	default:
		return data.SortByUpdated
	}
}

func sortFieldToConfig(f data.SortField) string {
	switch f {
	case data.SortByCreated:
		return "created"
	case data.SortByTurns:
		return "turns"
	case data.SortByName:
		return "name"
	case data.SortByFolder:
		return "folder"
	case data.SortByFrecency:
		return "frecency"
	default:
		return "updated"
	}
}

func sortOrderFromConfig(s string) data.SortOrder {
	if s == config.SortOrderAsc {
		return data.Ascending
	}
	return data.Descending
}

func sortOrderToConfig(o data.SortOrder) string {
	if o == data.Ascending {
		return config.SortOrderAsc
	}
	return config.SortOrderDesc
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
	case pivotHost:
		return data.PivotByHost
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

// applyNamedView applies the settings from a named view to the model state.
// It does not trigger a session reload; the caller must do so.
func (m *Model) applyNamedView(v *config.NamedView) {
	if v.TimeRange != "" {
		m.timeRange = v.TimeRange
		m.filter.Since = timeRangeToSince(m.timeRange)
	}
	if v.Sort != "" {
		m.sort.Field = sortFieldFromConfig(v.Sort)
	}
	if v.SortOrder != "" {
		m.sort.Order = sortOrderFromConfig(v.SortOrder)
	}
	if v.Pivot != "" {
		m.pivot = v.Pivot
	}
	if v.Search != "" {
		m.searchBar.SetValue(v.Search)
	}
	m.showFavorited = v.FavoritesOnly
	m.showHidden = v.ShowHidden
	if len(v.ExcludedDirs) > 0 {
		m.filter.ExcludedDirs = v.ExcludedDirs
	}
}
