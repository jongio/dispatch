package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/tui/components"
)

// openCmdPalette populates the command palette with all available actions
// and switches state to the palette overlay.
func (m *Model) openCmdPalette() {
	hasSelection := func() bool {
		_, ok := m.sessionList.Selected()
		return ok
	}
	hasPreview := func() bool {
		return m.showPreview && m.detail != nil
	}
	hasPath := func() bool {
		if _, ok := m.sessionList.Selected(); ok {
			return true
		}
		return m.sessionList.SelectedFolderCwd() != ""
	}
	alwaysEnabled := func() bool { return true }

	cmds := []components.Command{
		{Name: "Launch", Shortcut: "enter", Description: "open session", Action: "launch", Enabled: hasSelection},
		{Name: "Open in Window", Shortcut: "w", Description: "new window", Action: "launch-window", Enabled: hasSelection},
		{Name: "Open in Tab", Shortcut: "t", Description: "new tab", Action: "launch-tab", Enabled: hasSelection},
		{Name: "Open in Pane", Shortcut: "e", Description: "new pane", Action: "launch-pane", Enabled: hasSelection},
		{Name: "Copy Session ID", Shortcut: "c", Description: "copy to clipboard", Action: "copy-id", Enabled: hasSelection},
		{Name: "Copy Path", Shortcut: "C", Description: "copy working dir", Action: "copy-path", Enabled: hasPath},
		{Name: "Copy Preview", Shortcut: "y", Description: "copy preview text", Action: "copy-preview", Enabled: hasPreview},
		{Name: "Search", Shortcut: "/", Description: "filter sessions", Action: "search", Enabled: alwaysEnabled},
		{Name: "Filter Panel", Shortcut: "f", Description: "directory filter", Action: "filter", Enabled: alwaysEnabled},
		{Name: "Sort", Shortcut: "s", Description: "cycle sort field", Action: "sort", Enabled: alwaysEnabled},
		{Name: "Toggle Preview", Shortcut: "p", Description: "show/hide preview", Action: "preview", Enabled: alwaysEnabled},
		{Name: "Toggle Favorite", Shortcut: "*", Description: "star session", Action: "star", Enabled: hasSelection},
		{Name: "Hide Session", Shortcut: "h", Description: "hide from list", Action: "hide", Enabled: hasSelection},
		{Name: "Export Markdown", Shortcut: "X", Description: "export session", Action: "export", Enabled: hasSelection},
		{Name: "Rebuild Index", Shortcut: "r", Description: "reindex sessions", Action: "reindex", Enabled: func() bool { return !m.reindexing }},
		{Name: "Settings", Shortcut: ",", Description: "open config", Action: "settings", Enabled: alwaysEnabled},
		{Name: "Help", Shortcut: "?", Description: "keyboard shortcuts", Action: "help", Enabled: alwaysEnabled},
		{Name: "Quit", Shortcut: "q", Description: "exit dispatch", Action: "quit", Enabled: alwaysEnabled},
	}

	m.cmdPalette.SetCommands(cmds)
	m.cmdPalette.SetSize(m.width, m.height)
	m.state = stateCmdPalette
}

// handleCmdPaletteAction dispatches the action selected from the command palette.
// Each action maps to the same logic that the original key binding triggers.
func (m Model) handleCmdPaletteAction(msg cmdPaletteActionMsg) (tea.Model, tea.Cmd) {
	switch msg.action {
	case "launch":
		if m.sessionList.IsFolderSelected() {
			return m, m.launchNewInFolder(m.cfg.EffectiveLaunchMode())
		}
		return m, m.launchSelected()

	case "launch-window":
		if m.sessionList.IsFolderSelected() {
			return m, m.launchNewInFolder(config.LaunchModeWindow)
		}
		return m, m.launchWithMode(config.LaunchModeWindow)

	case "launch-tab":
		if m.sessionList.IsFolderSelected() {
			return m, m.launchNewInFolder(config.LaunchModeTab)
		}
		return m, m.launchWithMode(config.LaunchModeTab)

	case "launch-pane":
		if m.sessionList.IsFolderSelected() {
			return m, m.launchNewInFolder(config.LaunchModePane)
		}
		return m, m.launchWithMode(config.LaunchModePane)

	case "copy-id":
		return m.handleCopyID()

	case "copy-path":
		return m.handleCopyPath()

	case "copy-preview":
		return m.handleCopyPreview()

	case "search":
		cmd := m.searchBar.Focus()
		return m, cmd

	case "filter":
		m.state = stateFilterPanel
		return m, loadFilterDataCmd(m.store)

	case "sort":
		m.cycleSort()
		return m, m.loadSessionsCmd()

	case "preview":
		m.showPreview = !m.showPreview
		m.cfg.ShowPreview = m.showPreview
		m.saveConfig()
		m.recalcLayout()
		if m.showPreview {
			m.detailVersion++
			return m, m.loadSelectedDetailCmd()
		}
		return m, nil

	case "star":
		return m.handleToggleFavorite()

	case "hide":
		return m.handleHideSession()

	case "export":
		return m.handleExport()

	case "reindex":
		if !m.reindexing {
			m.reindexing = true
			m.reindexLog = []string{"Starting rebuild index\u2026"}
			m.reindexVP = viewport.New(viewport.WithHeight(reindexOverlayHeight))
			m.updateReindexViewport()
			handle, cmds := components.StartChronicleReindex()
			m.reindexCancel = &handle
			return m, tea.Batch(cmds...)
		}
		return m, nil

	case "settings":
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
			ExcludedWords:     joinExcludedWords(m.cfg.ExcludedWords),
			AutoRefresh:       autoRefreshFieldValue(m.cfg.AutoRefreshSeconds),
			NotifyOnWaiting:   m.cfg.NotifyOnWaiting,
		})
		m.state = stateConfigPanel
		return m, nil

	case "help":
		m.state = stateHelpOverlay
		return m, nil

	case "quit":
		m.closeStore()
		return m, tea.Quit
	}

	return m, nil
}

// joinExcludedWords joins excluded words for config panel display.
func joinExcludedWords(words []string) string {
	return strings.Join(words, ", ")
}
