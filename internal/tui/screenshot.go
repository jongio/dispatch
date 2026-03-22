//go:build screenshots

// screenshot.go provides programmatic screenshot capture for the web site.
//
// It drives the TUI model through each visual state and captures the
// rendered ANSI output without requiring the Bubble Tea event loop.
package tui

import (

	"charm.land/lipgloss/v2"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/tui/components"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// Highlight defines a rectangular region to visually emphasise in a
// screenshot. Coordinates are in terminal cells (0-indexed col, row).
type Highlight struct {
	Row, Col   int // top-left corner (0-indexed)
	Rows, Cols int // size in terminal cells
}

// Screenshot holds a captured ANSI render of the TUI in a named state.
type Screenshot struct {
	Name       string      // file name without extension (e.g. "hero-main")
	SubDir     string      // optional subdirectory (e.g. "dispatch-dark")
	ANSI       string      // rendered output with ANSI escape codes
	FG         string      // theme foreground hex (e.g. "#E4E4E7")
	BG         string      // theme background hex (e.g. "#111111")
	Palette    [16]string  // ANSI palette: [0]=Black … [15]=BrightWhite
	Highlights []Highlight // optional regions to emphasise
}

// captureCtx holds shared data used across all screenshot captures.
type captureCtx struct {
	width, height int
	store         *data.Store
	sessions      []data.Session
	folders       []string
	folderGroups  []data.SessionGroup
	repoGroups    []data.SessionGroup
	branchGroups  []data.SessionGroup
	dateGroups    []data.SessionGroup
	detail        *data.SessionDetail
	flatFilter    data.FilterOptions
	flatSort      data.SortOptions
	configVals    components.ConfigValues
	themeNames    []string
	attentionMap  map[string]data.AttentionStatus
	planMap       map[string]bool
}

// captureFeatures captures the full set of feature screenshots using the
// currently active theme. SubDir is prepended to each screenshot's path.
func (c *captureCtx) captureFeatures(subDir string) []Screenshot {
	var shots []Screenshot

	add := func(name string, m *Model, highlights ...Highlight) {
		t := styles.CurrentTheme()
		shots = append(shots, Screenshot{
			Name: name, SubDir: subDir,
			ANSI: m.View(), FG: t.Text, BG: t.Background,
			Palette:    t.ANSIPalette,
			Highlights: highlights,
		})
	}
	addOverlay := func(name string, m *Model) {
		t := styles.CurrentTheme()
		shots = append(shots, Screenshot{
			Name: name, SubDir: subDir,
			ANSI: m.View(), FG: t.Text, BG: t.Background,
			Palette: t.ANSIPalette,
		})
	}

	newBase := func() *Model {
		m := newScreenshotModel(c.width, c.height)
		m.store = c.store
		m.pivot = pivotFolder
		m.groups = c.folderGroups
		m.planMap = c.planMap
		m.sessionList.SetPivotField(m.pivot)
		m.sessionList.SetGroups(c.folderGroups)
		m.sessionList.SetAttentionStatuses(c.attentionMap)
		m.sessionList.SetPlanStatuses(c.planMap)
		m.timeRange = "7d"
		m.recalcLayout()
		return m
	}

	// ── Hero / default view ───────────────────────────────────────────
	{
		m := newBase()
		m.showPreview = true
		m.detail = c.detail
		m.preview.SetDetail(c.detail)
		m.recalcLayout()
		add("hero-main", m)
	}

	// ── Search ────────────────────────────────────────────────────────
	{
		m := newBase()
		m.searchBar.SetValue("auth")
		m.searchBar.Focus()
		m.searchBar.SetResultCount(len(c.sessions))
		add("search-active", m, Highlight{Row: 0, Col: 0, Rows: 1, Cols: 120})
	}
	{
		m := newBase()
		m.searchBar.SetValue("middleware")
		m.searchBar.Focus()
		m.searchBar.SetSearching(true)
		m.searchBar.SetResultCount(5)
		add("search-deep", m, Highlight{Row: 0, Col: 0, Rows: 1, Cols: 120})
	}

	// ── Filter panel ──────────────────────────────────────────────────
	{
		m := newBase()
		m.state = stateFilterPanel
		m.filterPanel.SetFolders(c.folders, nil)
		m.recalcLayout()
		addOverlay("filter-panel", m)
	}

	// ── Sort badges ───────────────────────────────────────────────────
	{
		m := newBase()
		m.sort.Field = data.SortByUpdated
		add("sort-updated", m)
	}
	{
		m := newBase()
		m.sort.Field = data.SortByFolder
		m.sort.Order = data.Ascending
		if sorted, err := c.store.GroupSessions(data.PivotByFolder, c.flatFilter, m.sort, 0); err == nil {
			sortGroupsByLatest(sorted, data.Ascending)
			m.groups = sorted
			m.sessionList.SetPivotField(m.pivot)
			m.sessionList.SetGroups(sorted)
		}
		add("sort-folder", m)
	}

	// ── Pivot / grouping modes ────────────────────────────────────────
	{
		m := newBase()
		m.pivot = pivotNone
		m.groups = nil
		m.sessions = c.sessions
		m.sessionList.SetSessions(c.sessions)
		m.recalcLayout()
		add("pivot-flat", m)
	}
	for _, tc := range []struct {
		name   string
		pivot  string
		groups []data.SessionGroup
	}{
		{"pivot-folder", pivotFolder, c.folderGroups},
		{"pivot-repo", pivotRepo, c.repoGroups},
		{"pivot-branch", pivotBranch, c.branchGroups},
		{"pivot-date", pivotDate, c.dateGroups},
	} {
		m := newBase()
		m.pivot = tc.pivot
		m.sessions = nil
		m.groups = tc.groups
		m.sessionList.SetPivotField(m.pivot)
		m.sessionList.SetGroups(tc.groups)
		m.recalcLayout()
		add(tc.name, m)
	}

	// ── Time range badges ─────────────────────────────────────────────
	for _, tc := range []struct {
		name string
		tr   string
	}{
		{"time-range-1h", "1h"},
		{"time-range-7d", "7d"},
		{"time-range-all", "all"},
	} {
		m := newBase()
		m.timeRange = tc.tr
		trFilter := data.FilterOptions{Since: timeRangeToSince(tc.tr)}
		if trGroups, err := c.store.GroupSessions(data.PivotByFolder, trFilter, c.flatSort, 0); err == nil {
			sortGroupsByLatest(trGroups, data.Descending)
			m.groups = trGroups
			m.sessionList.SetPivotField(m.pivot)
			m.sessionList.SetGroups(trGroups)
		}
		add(tc.name, m)
	}

	// ── Preview panel ─────────────────────────────────────────────────
	{
		m := newBase()
		m.showPreview = true
		m.detail = c.detail
		m.preview.SetDetail(c.detail)
		m.recalcLayout()
		add("preview-panel", m)
	}
	{
		m := newBase()
		m.showPreview = true
		m.detail = c.detail
		m.preview.SetDetail(c.detail)
		m.recalcLayout()
		m.preview.PageDown()
		m.preview.PageDown()
		add("preview-scroll", m)
	}

	// ── Preview positions ─────────────────────────────────────────────
	for _, tc := range []struct {
		name string
		pos  string
	}{
		{"preview-right", config.PreviewPositionRight},
		{"preview-bottom", config.PreviewPositionBottom},
		{"preview-left", config.PreviewPositionLeft},
		{"preview-top", config.PreviewPositionTop},
	} {
		m := newBase()
		m.showPreview = true
		m.detail = c.detail
		m.preview.SetDetail(c.detail)
		m.previewPosition = tc.pos
		m.recalcLayout()
		add(tc.name, m)
	}

	// ── Hidden sessions ───────────────────────────────────────────────
	{
		m := newBase()
		m.showHidden = true
		var hiddenIDs []string
		for _, g := range c.folderGroups {
			for _, s := range g.Sessions {
				hiddenIDs = append(hiddenIDs, s.ID)
				if len(hiddenIDs) >= 2 {
					break
				}
			}
			if len(hiddenIDs) >= 2 {
				break
			}
		}
		m.hiddenSet = make(map[string]struct{})
		for _, id := range hiddenIDs {
			m.hiddenSet[id] = struct{}{}
		}
		m.sessionList.SetHiddenSessions(m.hiddenSet)
		m.sessionList.SetPivotField(m.pivot)
		m.sessionList.SetGroups(c.folderGroups)
		m.recalcLayout()
		add("hidden-sessions", m)
	}

	// ── Favorites ────────────────────────────────────────────────────
	{
		m := newBase()
		var favIDs []string
		for _, g := range c.folderGroups {
			for _, s := range g.Sessions {
				favIDs = append(favIDs, s.ID)
				if len(favIDs) >= 3 {
					break
				}
			}
			if len(favIDs) >= 3 {
				break
			}
		}
		m.favoritedSet = make(map[string]struct{})
		for _, id := range favIDs {
			m.favoritedSet[id] = struct{}{}
		}
		m.sessionList.SetFavoritedSessions(m.favoritedSet)
		m.sessionList.SetPivotField(m.pivot)
		m.sessionList.SetGroups(c.folderGroups)
		m.recalcLayout()
		add("favorites", m)
	}

	// ── Multi-select ─────────────────────────────────────────────────
	{
		m := newBase()
		// Select a few sessions to show ✓ indicators by toggling at
		// different cursor positions.
		m.sessionList.MoveDown() // skip folder header
		m.sessionList.ToggleSelected()
		m.sessionList.MoveDown()
		m.sessionList.ToggleSelected()
		m.sessionList.MoveDown()
		m.sessionList.MoveDown() // skip one to show partial selection
		m.sessionList.ToggleSelected()
		m.recalcLayout()
		add("multi-select", m)
	}

	// ── Attention picker overlay ─────────────────────────────────────
	{
		m := newBase()
		m.state = stateAttentionPicker
		m.attentionPicker.SetCounts(map[data.AttentionStatus]int{
			data.AttentionWaiting:     2,
			data.AttentionActive:      2,
			data.AttentionStale:       1,
			data.AttentionInterrupted: 2,
			data.AttentionIdle:        2,
		})
		m.recalcLayout()
		addOverlay("attention-picker", m)
	}

	// ── Attention filtered to interrupted only ──────────────────────
	{
		m := newBase()
		m.attentionFilter = map[data.AttentionStatus]struct{}{
			data.AttentionInterrupted: {},
		}
		m.showPreview = false
		// Rebuild the session list with the filter applied
		var filtered []data.SessionGroup
		for _, g := range c.folderGroups {
			var sessions []data.Session
			for _, s := range g.Sessions {
				if status, ok := c.attentionMap[s.ID]; ok && status == data.AttentionInterrupted {
					sessions = append(sessions, s)
				}
			}
			if len(sessions) > 0 {
				g.Sessions = sessions
				g.Count = len(sessions)
				filtered = append(filtered, g)
			}
		}
		m.groups = filtered
		m.sessionList.SetGroups(filtered)
		m.recalcLayout()
		add("attention-filtered-interrupted", m)
	}

	// ── Plan indicator ───────────────────────────────────────────────
	{
		m := newBase()
		m.showPreview = false
		m.recalcLayout()
		add("plan-indicator", m)
	}

	// ── Plan preview ─────────────────────────────────────────────────
	{
		m := newBase()
		m.showPreview = true
		m.detail = c.detail
		m.preview.SetDetail(c.detail)
		m.preview.SetPlanContent("# Implementation Plan\n\n## Tasks\n- [ ] Design API endpoints\n- [ ] Implement database schema\n- [x] Set up project structure\n")
		m.preview.TogglePlanView()
		m.recalcLayout()
		add("plan-preview", m)
	}

	// ── Plan filter ──────────────────────────────────────────────────
	{
		m := newBase()
		m.filterPlans = true
		m.showPreview = false
		// Rebuild the session list with the plan filter applied.
		var filtered []data.SessionGroup
		for _, g := range c.folderGroups {
			var sessions []data.Session
			for _, s := range g.Sessions {
				if c.planMap[s.ID] {
					sessions = append(sessions, s)
				}
			}
			if len(sessions) > 0 {
				g.Sessions = sessions
				g.Count = len(sessions)
				filtered = append(filtered, g)
			}
		}
		m.groups = filtered
		m.sessionList.SetGroups(filtered)
		m.recalcLayout()
		add("plan-filter", m)
	}

	// ── Tree collapsed / expanded ─────────────────────────────────────
	{
		m := newBase()
		m.pivot = pivotFolder
		m.sessions = nil
		m.groups = c.folderGroups
		m.sessionList.SetPivotField(m.pivot)
		m.sessionList.SetGroups(c.folderGroups)
		m.sessionList.CollapseAll()
		m.recalcLayout()
		add("tree-collapsed", m)
	}
	{
		m := newBase()
		m.pivot = pivotFolder
		m.sessions = nil
		m.groups = c.folderGroups
		m.sessionList.SetPivotField(m.pivot)
		m.sessionList.SetGroups(c.folderGroups)
		m.recalcLayout()
		add("tree-expanded", m)
	}

	// ── Config panel ──────────────────────────────────────────────────
	{
		m := newBase()
		m.state = stateConfigPanel
		m.configPanel.SetValues(c.configVals)
		m.configPanel.SetThemeOptions(c.themeNames)
		m.recalcLayout()
		addOverlay("config-panel", m)
	}
	{
		m := newBase()
		m.state = stateConfigPanel
		m.configPanel.SetValues(c.configVals)
		m.configPanel.SetThemeOptions(c.themeNames)
		m.configPanel.MoveDown()
		m.configPanel.HandleEnter()
		m.recalcLayout()
		addOverlay("config-editing", m)
	}

	// ── Shell picker ──────────────────────────────────────────────────
	{
		m := newBase()
		m.state = stateShellPicker
		m.shellPicker.SetShells([]platform.ShellInfo{
			{Name: "pwsh", Path: `C:\Program Files\PowerShell\7\pwsh.exe`},
			{Name: "powershell", Path: `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`},
			{Name: "cmd", Path: `C:\Windows\System32\cmd.exe`},
			{Name: "bash", Path: `C:\Program Files\Git\bin\bash.exe`},
		}, "pwsh")
		m.recalcLayout()
		addOverlay("shell-picker", m)
	}

	// ── Help overlay ──────────────────────────────────────────────────
	{
		m := newBase()
		m.state = stateHelpOverlay
		m.recalcLayout()
		addOverlay("help-overlay", m)
	}

	// ── Loading state ─────────────────────────────────────────────────
	{
		m := newBase()
		m.state = stateLoading
		m.reindexing = true
		add("loading-state", m)
	}

	// ── Empty state ───────────────────────────────────────────────────
	{
		m := newBase()
		m.sessions = nil
		m.groups = nil
		m.sessionList.SetPivotField(m.pivot)
		m.sessionList.SetGroups(nil)
		m.recalcLayout()
		add("empty-state", m)
	}

	return shots
}

// themeEntry pairs a color scheme with its filesystem-safe directory name.
type themeEntry struct {
	dir    string
	scheme styles.ColorScheme
}

// allThemes lists every built-in scheme with its subfolder name.
var allThemes = []themeEntry{
	{"dispatch-dark", styles.DispatchDark},
	{"dispatch-light", styles.DispatchLight},
	{"campbell", styles.Campbell},
	{"one-half-dark", styles.OneHalfDark},
	{"one-half-light", styles.OneHalfLight},
}

// CaptureScreenshots drives the TUI model through every visual state
// used on the website and returns the rendered ANSI output for each.
func CaptureScreenshots(dbPath string, width, height int) ([]Screenshot, error) {

	// Enable NerdFont icons so the title bar shows the terminal glyph
	// instead of the ⚡ fallback.
	styles.SetNerdFontEnabled(true)

	store, err := data.OpenPath(dbPath)
	if err != nil {
		return nil, err
	}
	defer store.Close()

	// ── Load data ─────────────────────────────────────────────────────
	flatSort := data.SortOptions{Field: data.SortByUpdated, Order: data.Descending}
	flatFilter := data.FilterOptions{Since: timeRangeToSince("7d")}

	sessions, err := store.ListSessions(flatFilter, flatSort, 100)
	if err != nil {
		return nil, err
	}

	folders, _ := store.ListFolders()

	loadGroups := func(pf data.PivotField) []data.SessionGroup {
		groups, err := store.GroupSessions(pf, flatFilter, flatSort, 0)
		if err != nil {
			return nil
		}
		sortGroupsByLatest(groups, data.Descending)
		return groups
	}

	folderGroups := loadGroups(data.PivotByFolder)
	repoGroups := loadGroups(data.PivotByRepo)
	branchGroups := loadGroups(data.PivotByBranch)
	dateGroups := loadGroups(data.PivotByDate)

	var detail *data.SessionDetail
	for _, g := range folderGroups {
		if len(g.Sessions) > 0 {
			if d, err := store.GetSession(g.Sessions[0].ID); err == nil {
				detail = d
			}
			break
		}
	}

	// Build a fake attention map so screenshots show all five status
	// indicators (waiting=purple, active=green, stale=yellow, interrupted=orange ⚡, idle=gray).
	attentionMap := map[string]data.AttentionStatus{
		"fa800b7b-3a24-4e3b-9f2d-a414198b27ab": data.AttentionWaiting,     // Death Star API auth (5 min ago)
		"ses-026":                              data.AttentionActive,      // Fleet dashboard (12 min ago)
		"ses-002":                              data.AttentionActive,      // Superlaser refactor (22 min ago)
		"ses-003":                              data.AttentionStale,       // Warp metrics (38 min ago)
		"ses-004":                              data.AttentionWaiting,     // Cake promise API (52 min ago)
		"ses-005":                              data.AttentionInterrupted, // Sith login fix (95 min ago)
		"ses-006":                              data.AttentionIdle,        // Sorting hat GPT (175 min ago)
		"ses-007":                              data.AttentionInterrupted, // Kimoyo bead sync (340 min ago)
		"ses-027":                              data.AttentionIdle,        // Auth middleware (250 min ago)
	}

	// Build a fake plan map so screenshots show plan indicator dots on a
	// few sessions (matching the demo plan sessions).
	planMap := map[string]bool{
		"fa800b7b-3a24-4e3b-9f2d-a414198b27ab": true, // Waiting — plan dot visible
		"ses-026":                              true, // Active — plan dot visible
		"ses-003":                              true, // Stale — plan dot visible
	}

	ctx := &captureCtx{
		width: width, height: height,
		store:        store,
		sessions:     sessions,
		folders:      folders,
		folderGroups: folderGroups,
		repoGroups:   repoGroups,
		branchGroups: branchGroups,
		dateGroups:   dateGroups,
		detail:       detail,
		flatFilter:   flatFilter,
		flatSort:     flatSort,
		attentionMap: attentionMap,
		planMap:      planMap,
		configVals: components.ConfigValues{
			YoloMode:          false,
			Agent:             "copilot",
			Model:             "claude-sonnet-4",
			LaunchMode:        "in-place",
			Terminal:          "Windows Terminal",
			Shell:             "pwsh",
			Theme:             "Dispatch Dark",
			WorkspaceRecovery: true,
		},
		themeNames: append([]string{"auto"}, styles.BuiltinSchemeNames()...),
	}

	// ── Per-theme subfolders (full feature set for each theme) ────────
	var shots []Screenshot
	for _, tc := range allThemes {
		styles.SetTheme(styles.DeriveTheme(tc.scheme))
		shots = append(shots, ctx.captureFeatures(tc.dir)...)
	}

	return shots, nil
}

// newScreenshotModel creates a minimal Model suitable for off-screen
// rendering. It bypasses config loading and the bubbletea runtime.
func newScreenshotModel(width, height int) *Model {
	m := NewModel()
	m.state = stateSessionList
	m.reindexing = false
	m.width = width
	m.height = height
	return &m
}
