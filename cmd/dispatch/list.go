package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/components"
)

var (
	listGetwdFn    = os.Getwd
	listDemoModeFn = func() bool { return os.Getenv("DISPATCH_DEMO") == "1" }
	listSelectFn   = runListPicker
)

const listPickerPageSize = components.DefaultSessionPickerBatchSize

type listPageLoader func(count int) ([]data.Session, bool, error)

type listPageMsg struct {
	sessions []data.Session
	hasMore  bool
	err      error
}

// runList opens an interactive picker for sessions under the selected folder.
// Explicit output flags retain the non-interactive search renderers.
func runList(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	opts, err := parseListArgs(args)
	if err != nil {
		return err
	}

	if opts.formatExplicit {
		return runSearchOptions(w, opts)
	}

	initialCount := listPickerPageSize
	if opts.limit > 0 {
		initialCount = min(initialCount, opts.limit)
	}
	sessions, hasMore, err := loadListPage(opts, initialCount)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		_, err := fmt.Fprintln(w, "No sessions found.")
		return err
	}
	if msg := checkTerminalCompat(); msg != "" {
		return errors.New(msg)
	}

	loader := func(count int) ([]data.Session, bool, error) {
		return loadListPage(opts, count)
	}
	selected, ok, err := listSelectFn(w, sessions, hasMore, opts.limit, loader)
	if err != nil || !ok {
		return err
	}

	cfg, err := openLoadConfigFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	mode := resolveOpenMode("", cfg)
	return openInteractiveLaunchFn(w, cfg, &selected, mode)
}

func loadListPage(opts searchOptions, count int) ([]data.Session, bool, error) {
	if count <= 0 {
		return nil, false, nil
	}

	cappedCount := count
	if opts.limit > 0 {
		cappedCount = min(cappedCount, opts.limit)
	}
	probeForMore := opts.limit == 0 || cappedCount < opts.limit
	queryLimit := cappedCount
	if probeForMore {
		queryLimit++
	}

	if opts.tag != "" {
		return loadTaggedListPage(opts, cappedCount, queryLimit, probeForMore)
	}

	pageOpts := opts
	pageOpts.limit = queryLimit
	sessions, err := loadSearchSessions(pageOpts)
	if err != nil {
		return nil, false, err
	}

	hasMore := probeForMore && len(sessions) > cappedCount
	if hasMore {
		sessions = sessions[:cappedCount]
	}
	return sessions, hasMore, nil
}

func loadTaggedListPage(
	opts searchOptions,
	count int,
	queryLimit int,
	probeForMore bool,
) ([]data.Session, bool, error) {
	cfg, err := configLoadFn()
	if err != nil {
		return nil, false, fmt.Errorf("loading config: %w", err)
	}

	maxScan := searchAllLimit
	scanLimit := min(maxScan, max(queryLimit, listPickerPageSize+1))

	for {
		sessions, err := searchListSessionsFn(opts.filter, opts.sort, scanLimit)
		if err != nil {
			return nil, false, err
		}
		filtered := filterSessionsByTag(sessions, cfg, opts.tag)
		if len(filtered) >= queryLimit || len(sessions) < scanLimit || scanLimit >= maxScan {
			hasMore := probeForMore && len(filtered) > count
			if len(filtered) > count {
				filtered = filtered[:count]
			}
			return filtered, hasMore, nil
		}
		scanLimit = min(maxScan, scanLimit*2)
	}
}

// parseListArgs reuses search parsing with list-specific defaults.
func parseListArgs(args []string) (searchOptions, error) {
	opts, err := parseSearchArgsWithDefaults(args, searchOptions{
		sort:   defaultSearchSort(),
		limit:  0,
		format: searchFormatTable,
	})
	if err != nil {
		return searchOptions{}, err
	}

	if listDemoModeFn() {
		return opts, nil
	}

	opts.filter.Folder, err = resolveListFolder(opts.filter.Folder)
	if err != nil {
		return searchOptions{}, err
	}

	return opts, nil
}

// resolveListFolder returns an absolute, existing directory for list scoping.
func resolveListFolder(folder string) (string, error) {
	if folder == "" {
		var err error
		folder, err = listGetwdFn()
		if err != nil {
			return "", fmt.Errorf("get current directory: %w", err)
		}
	}

	absolute, err := filepath.Abs(folder)
	if err != nil {
		return "", fmt.Errorf("resolve folder %q: %w", folder, err)
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("folder %q: %w", absolute, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("folder %q is not a directory", absolute)
	}

	return absolute, nil
}

type listPickerModel struct {
	sessions  []data.Session
	rows      []components.SessionPickerRow
	idWidth   int
	cursor    int
	offset    int
	width     int
	height    int
	visible   int
	limit     int
	selected  bool
	quitting  bool
	loading   bool
	loadErr   error
	hasNext   bool
	loader    listPageLoader
	selectNew bool
}

func newPagedListPickerModel(
	sessions []data.Session,
	hasMore bool,
	limit int,
	loader listPageLoader,
) listPickerModel {
	rows := make([]components.SessionPickerRow, len(sessions))
	idWidth := 0
	for i, session := range sessions {
		rows[i] = components.SessionPickerRow{
			ID:         searchTableCell(session.ID),
			Repository: searchTableCell(session.Repository),
			Branch:     searchTableCell(session.Branch),
			Summary:    searchTableCell(session.Summary),
		}
		idWidth = max(idWidth, ansi.StringWidth(rows[i].ID))
	}
	return listPickerModel{
		sessions: sessions,
		rows:     rows,
		idWidth:  idWidth,
		visible:  len(sessions),
		limit:    limit,
		width:    100,
		height:   24,
		hasNext:  hasMore,
		loader:   loader,
	}
}

func (m listPickerModel) Init() tea.Cmd { return nil }

func (m listPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureVisible()
	case listPageMsg:
		width := m.width
		height := m.height
		previousCursor := m.cursor
		previousCount := len(m.sessions)
		selectNew := m.selectNew
		m.loading = false
		if msg.err != nil {
			m.loadErr = msg.err
			m.quitting = true
			return m, tea.Quit
		}
		if len(msg.sessions) == 0 {
			m.sessions = nil
			m.rows = nil
			m.visible = 0
			m.cursor = 0
			m.offset = 0
			m.hasNext = false
			return m, nil
		}
		m = newPagedListPickerModel(msg.sessions, msg.hasMore, m.limit, m.loader)
		m.width = width
		m.height = height
		m.cursor = previousCursor
		if selectNew {
			m.cursor = min(len(m.sessions)-1, previousCount)
		}
		m.ensureVisible()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
		case "down", "j":
			if m.cursor < m.selectableCount()-1 {
				m.cursor++
				m.ensureVisible()
			}
		case "home", "g":
			m.cursor = 0
			m.ensureVisible()
		case "end", "G":
			m.cursor = m.selectableCount() - 1
			m.ensureVisible()
		case "m":
			return m, m.showMore(false)
		case "enter":
			if m.hasMore() && m.cursor == m.visible {
				return m, m.showMore(true)
			}
			m.selected = m.cursor >= 0 && m.cursor < m.visible
			m.quitting = true
			return m, tea.Quit
		case "esc", "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m listPickerModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}

	content := components.SessionPickerView{
		Rows:      m.rows,
		Cursor:    m.cursor,
		Offset:    m.offset,
		Visible:   m.visible,
		Width:     m.width,
		Height:    m.height,
		IDWidth:   m.idWidth,
		HasMore:   m.hasMore(),
		Loading:   m.loading,
		MoreCount: m.moreCount(),
	}.Render()
	return tea.NewView(content)
}

func (m listPickerModel) visibleRows() int {
	return components.SessionPickerVisibleRows(m.height)
}

func (m *listPickerModel) ensureVisible() {
	rows := m.visibleRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
}

func (m listPickerModel) hasMore() bool {
	return m.hasNext
}

func (m listPickerModel) selectableCount() int {
	count := m.visible
	if m.hasMore() {
		count++
	}
	return count
}

func (m *listPickerModel) showMore(selectNew bool) tea.Cmd {
	if !m.hasMore() || m.loading || m.loader == nil {
		return nil
	}
	m.loading = true
	m.selectNew = selectNew
	count := len(m.sessions) + listPickerPageSize
	if m.limit > 0 {
		count = min(count, m.limit)
	}
	loader := m.loader
	return func() tea.Msg {
		sessions, hasMore, err := loader(count)
		return listPageMsg{sessions: sessions, hasMore: hasMore, err: err}
	}
}

func (m listPickerModel) moreCount() int {
	count := listPickerPageSize
	if m.limit > 0 {
		count = min(count, m.limit-len(m.sessions))
	}
	return max(0, count)
}

func runListPicker(
	w io.Writer,
	sessions []data.Session,
	hasMore bool,
	limit int,
	loader listPageLoader,
) (data.Session, bool, error) {
	if w == nil {
		w = io.Discard
	}
	model := newPagedListPickerModel(sessions, hasMore, limit, loader)
	program := tea.NewProgram(model, tea.WithInput(os.Stdin), tea.WithOutput(w))
	result, err := program.Run()
	if err != nil {
		return data.Session{}, false, err
	}
	model, ok := result.(listPickerModel)
	if !ok || !model.selected || model.cursor < 0 || model.cursor >= len(model.sessions) {
		if ok && model.loadErr != nil {
			return data.Session{}, false, model.loadErr
		}
		return data.Session{}, false, nil
	}
	return model.sessions[model.cursor], true, nil
}
