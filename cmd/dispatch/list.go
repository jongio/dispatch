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

	sessions, err := loadSearchSessions(opts)
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

	selected, ok, err := listSelectFn(w, sessions)
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
	sessions []data.Session
	rows     []components.SessionPickerRow
	idWidth  int
	cursor   int
	offset   int
	width    int
	height   int
	visible  int
	selected bool
	quitting bool
}

func newListPickerModel(sessions []data.Session) listPickerModel {
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
		visible:  min(len(sessions), listPickerPageSize),
		width:    100,
		height:   24,
	}
}

func (m listPickerModel) Init() tea.Cmd { return nil }

func (m listPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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
			m.showMore()
		case "enter":
			if m.hasMore() && m.cursor == m.visible {
				m.showMore()
				return m, nil
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
		Rows:    m.rows,
		Cursor:  m.cursor,
		Offset:  m.offset,
		Visible: m.visible,
		Width:   m.width,
		Height:  m.height,
		IDWidth: m.idWidth,
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
	return m.visible < len(m.sessions)
}

func (m listPickerModel) selectableCount() int {
	count := m.visible
	if m.hasMore() {
		count++
	}
	return count
}

func (m *listPickerModel) showMore() {
	if !m.hasMore() {
		return
	}
	m.visible = min(len(m.sessions), m.visible+listPickerPageSize)
	m.ensureVisible()
}

func runListPicker(w io.Writer, sessions []data.Session) (data.Session, bool, error) {
	if w == nil {
		w = io.Discard
	}
	program := tea.NewProgram(newListPickerModel(sessions), tea.WithInput(os.Stdin), tea.WithOutput(w))
	result, err := program.Run()
	if err != nil {
		return data.Session{}, false, err
	}
	model, ok := result.(listPickerModel)
	if !ok || !model.selected || model.cursor < 0 || model.cursor >= len(model.sessions) {
		return data.Session{}, false, nil
	}
	return model.sessions[model.cursor], true, nil
}
