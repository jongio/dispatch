// Package components provides reusable Bubble Tea TUI components for dispatch.
package components

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// ---------------------------------------------------------------------------
// ConfigPanel — overlay for editing Copilot CLI session resume settings.
// ---------------------------------------------------------------------------

// configFieldID identifies a configurable field.
type configFieldID int

const (
	cfgYoloMode configFieldID = iota
	cfgAgent
	cfgModel
	cfgLaunchMode
	cfgPaneDirection
	cfgTerminal
	cfgShell
	cfgCustomCommand
	cfgTheme
	cfgWorkspaceRecovery
	cfgPreviewPosition
	cfgFieldCount
)

// themeAuto is the sentinel value for the automatic/default color scheme.
const themeAuto = "auto"

// ConfigPanel presents an overlay that lets users edit session-resume
// settings (yolo mode, agent, model, launch-in-place, terminal, shell,
// custom command).
type ConfigPanel struct {
	yoloMode          bool
	agent             string
	model             string
	launchMode        string // "in-place", "tab", "window", or "pane"
	paneDirection     string // "auto", "right", "down", "left", "up"
	terminal          string
	shell             string
	customCommand     string
	theme             string // active color scheme name ("auto" or a scheme name)
	workspaceRecovery bool
	previewPosition   string // "right", "bottom", "left", "top"

	// Available options for cycling.
	terminals  []string
	shellInfos []platform.ShellInfo
	themeNames []string // "auto" + built-in + user-defined scheme names

	cursor    configFieldID
	editing   bool
	textInput textinput.Model
	width     int
	height    int
}

// NewConfigPanel returns a ConfigPanel initialised with the provided values.
func NewConfigPanel() ConfigPanel {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 256
	ti.PromptStyle = styles.SearchPromptStyle
	ti.Prompt = "> "

	return ConfigPanel{
		textInput: ti,
	}
}

// ConfigValues bundles the editable fields exchanged between the config
// panel and the rest of the application.
type ConfigValues struct {
	YoloMode          bool
	Agent             string
	Model             string
	LaunchMode        string
	PaneDirection     string
	Terminal          string
	Shell             string
	CustomCommand     string
	Theme             string
	WorkspaceRecovery bool
	PreviewPosition   string
}

// SetValues loads the config panel state from external values.
func (c *ConfigPanel) SetValues(v ConfigValues) {
	c.yoloMode = v.YoloMode
	c.agent = v.Agent
	c.model = v.Model
	c.launchMode = v.LaunchMode
	c.paneDirection = v.PaneDirection
	c.terminal = v.Terminal
	c.shell = v.Shell
	c.customCommand = v.CustomCommand
	c.theme = v.Theme
	c.workspaceRecovery = v.WorkspaceRecovery
	c.previewPosition = v.PreviewPosition
}

// Values returns the current state of all editable fields.
func (c *ConfigPanel) Values() ConfigValues {
	return ConfigValues{
		YoloMode:          c.yoloMode,
		Agent:             c.agent,
		Model:             c.model,
		LaunchMode:        c.launchMode,
		PaneDirection:     c.paneDirection,
		Terminal:          c.terminal,
		Shell:             c.shell,
		CustomCommand:     c.customCommand,
		Theme:             c.theme,
		WorkspaceRecovery: c.workspaceRecovery,
		PreviewPosition:   c.previewPosition,
	}
}

// SetTerminals provides the list of available terminal emulator names.
func (c *ConfigPanel) SetTerminals(names []string) {
	c.terminals = names
}

// SetShellOptions provides the list of available shells.
func (c *ConfigPanel) SetShellOptions(shells []platform.ShellInfo) {
	c.shellInfos = shells
}

// SetThemeOptions provides the list of available scheme names for
// cycling.  The first entry should be "auto".
func (c *ConfigPanel) SetThemeOptions(names []string) {
	c.themeNames = names
}

// SetSize updates the overlay dimensions.
func (c *ConfigPanel) SetSize(w, h int) {
	c.width = w
	c.height = h
}

// MoveUp moves the cursor to the previous field.
func (c *ConfigPanel) MoveUp() {
	if c.editing {
		return
	}
	if c.cursor > 0 {
		c.cursor--
	}
}

// MoveDown moves the cursor to the next field.
func (c *ConfigPanel) MoveDown() {
	if c.editing {
		return
	}
	if c.cursor < cfgFieldCount-1 {
		c.cursor++
	}
}

// HandleEnter toggles a boolean field, cycles a selection, or starts editing
// a text field. Returns a tea.Cmd when a text input needs focus.
func (c *ConfigPanel) HandleEnter() tea.Cmd {
	// When a custom command is set, YOLO/Agent/Model are irrelevant — skip them.
	if c.customCommand != "" {
		switch c.cursor {
		case cfgYoloMode, cfgAgent, cfgModel:
			return nil
		default:
			// Other fields remain interactive with a custom command.
		}
	}

	switch c.cursor {
	case cfgYoloMode:
		c.yoloMode = !c.yoloMode
	case cfgLaunchMode:
		c.launchMode = cycleLaunchMode(c.launchMode)
	case cfgPaneDirection:
		c.paneDirection = cyclePaneDirection(c.paneDirection)
	case cfgAgent:
		c.editing = true
		c.textInput.SetValue(c.agent)
		c.textInput.CharLimit = 64
		return c.textInput.Focus()
	case cfgModel:
		c.editing = true
		c.textInput.SetValue(c.model)
		c.textInput.CharLimit = 64
		return c.textInput.Focus()
	case cfgTerminal:
		c.terminal = c.cycleOption(c.terminal, c.terminals)
	case cfgShell:
		c.shell = c.cycleOption(c.shell, c.shellNames())
	case cfgCustomCommand:
		c.editing = true
		c.textInput.SetValue(c.customCommand)
		c.textInput.CharLimit = 256
		return c.textInput.Focus()
	case cfgTheme:
		c.theme = c.cycleTheme(c.theme)
	case cfgWorkspaceRecovery:
		c.workspaceRecovery = !c.workspaceRecovery
	case cfgPreviewPosition:
		c.previewPosition = cyclePreviewPosition(c.previewPosition)
	default:
		// cfgFieldCount is a sentinel; no action needed.
	}
	return nil
}

// ConfirmEdit saves the current text input value to the appropriate field.
func (c *ConfigPanel) ConfirmEdit() {
	if !c.editing {
		return
	}
	val := strings.TrimSpace(c.textInput.Value())
	switch c.cursor {
	case cfgAgent:
		c.agent = val
	case cfgModel:
		c.model = val
	case cfgCustomCommand:
		c.customCommand = val
	default:
		// Non-editable fields are ignored.
	}
	c.editing = false
	c.textInput.Blur()
}

// CancelEdit discards the current text input without saving.
func (c *ConfigPanel) CancelEdit() {
	c.editing = false
	c.textInput.Blur()
}

// IsEditing returns true when a text field is being edited.
func (c *ConfigPanel) IsEditing() bool {
	return c.editing
}

// Update delegates to the underlying text input when editing.
func (c ConfigPanel) Update(msg tea.Msg) (ConfigPanel, tea.Cmd) {
	if !c.editing {
		return c, nil
	}
	var cmd tea.Cmd
	c.textInput, cmd = c.textInput.Update(msg)
	return c, cmd
}

// View renders the config panel as a centred overlay.
func (c ConfigPanel) View() string {
	title := styles.OverlayTitleStyle.Render(styles.IconGear() + "  Settings")

	// hasCustom indicates that copilot-specific fields (Yolo, Agent, Model)
	// are irrelevant because a custom command overrides them.
	hasCustom := c.customCommand != ""

	type field struct {
		label  string
		value  string
		dimmed bool // when true, the row is rendered as inactive
	}
	fields := []field{
		{"Yolo Mode", boolDisplay(c.yoloMode), hasCustom},
		{"Agent", stringDisplay(c.agent), hasCustom},
		{"Model", stringDisplay(c.model), hasCustom},
		{"Launch Mode", launchModeDisplay(c.launchMode), false},
		{"Pane Direction", paneDirectionDisplay(c.paneDirection), c.launchMode != config.LaunchModePane},
		{"Terminal", stringDisplay(c.terminal), false},
		{"Shell", stringDisplay(c.shell), false},
		{"Custom Command", stringDisplay(c.customCommand), false},
		{"Theme", themeDisplay(c.theme), false},
		{"Crash Recovery", boolDisplay(c.workspaceRecovery), false},
		{"Preview Position", previewPositionDisplay(c.previewPosition), false},
	}

	var body strings.Builder
	body.WriteString(title + "\n\n")

	for i, f := range fields {
		indicator := "  "
		if configFieldID(i) == c.cursor {
			indicator = styles.IconPointer() + " "
		}

		label := styles.ConfigLabelStyle.Render(f.label)

		var val string
		if c.editing && configFieldID(i) == c.cursor {
			val = c.textInput.View()
		} else {
			val = f.value
		}

		line := indicator + label + val

		// Dim the entire row when the field is overridden.
		if f.dimmed {
			line = styles.DimmedStyle.Render(line + "  (overridden)")
		} else if configFieldID(i) == c.cursor && !c.editing {
			line = styles.SelectedStyle.Render(line)
		}

		body.WriteString(line + "\n")
	}

	// Contextual help when the Custom Command field is focused.
	if c.cursor == cfgCustomCommand {
		body.WriteString("\n")
		body.WriteString(styles.DimmedStyle.Render("  {sessionId} is replaced with the session ID at launch.") + "\n")
		body.WriteString(styles.DimmedStyle.Render("  Overrides Yolo, Agent, and Model when set.") + "\n")
		body.WriteString(styles.DimmedStyle.Render("  Example: my-tool --session {sessionId}") + "\n")
	}

	body.WriteString("\n")
	if c.editing {
		body.WriteString(styles.DimmedStyle.Render("Enter confirm · Esc cancel"))
	} else {
		body.WriteString(styles.DimmedStyle.Render("Enter toggle/edit · Esc close"))
	}

	maxW := min(65, c.width-4)
	maxW = max(maxW, 30)

	overlay := styles.OverlayStyle.
		Width(maxW).
		Render(body.String())

	return lipgloss.Place(c.width, c.height, lipgloss.Center, lipgloss.Center, overlay)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// cycleOption cycles through a list of options, including "" (none) as first.
func (c *ConfigPanel) cycleOption(current string, options []string) string {
	if len(options) == 0 {
		return current
	}
	// Prepend empty (none) option.
	all := append([]string{""}, options...)
	for i, opt := range all {
		if opt == current {
			return all[(i+1)%len(all)]
		}
	}
	return all[0]
}

func (c *ConfigPanel) shellNames() []string {
	names := make([]string, 0, len(c.shellInfos))
	for _, sh := range c.shellInfos {
		names = append(names, sh.Name)
	}
	return names
}

func boolDisplay(v bool) string {
	if v {
		return styles.ConfigValueStyle.Render("ON")
	}
	return styles.ConfigDimmedValue.Render("OFF")
}

func stringDisplay(v string) string {
	if v == "" {
		return styles.ConfigDimmedValue.Render("(none)")
	}
	return styles.ConfigValueStyle.Render(v)
}

// cycleLaunchMode cycles through the four launch modes.
func cycleLaunchMode(current string) string {
	switch current {
	case config.LaunchModeInPlace:
		return config.LaunchModeTab
	case config.LaunchModeTab:
		return config.LaunchModeWindow
	case config.LaunchModeWindow:
		return config.LaunchModePane
	default:
		return config.LaunchModeInPlace
	}
}

// launchModeDisplay renders the launch mode as a user-friendly label.
func launchModeDisplay(mode string) string {
	switch mode {
	case config.LaunchModeInPlace:
		return styles.ConfigValueStyle.Render("In-Place")
	case config.LaunchModeWindow:
		return styles.ConfigValueStyle.Render("Window")
	case config.LaunchModePane:
		return styles.ConfigValueStyle.Render("Pane")
	default:
		return styles.ConfigValueStyle.Render("Tab")
	}
}

// themeDisplay renders the theme name as a user-friendly label.
func themeDisplay(theme string) string {
	if theme == "" || theme == themeAuto {
		return styles.ConfigDimmedValue.Render(themeAuto)
	}
	return styles.ConfigValueStyle.Render(theme)
}

// cyclePaneDirection cycles through the five pane directions.
func cyclePaneDirection(current string) string {
	switch current {
	case config.PaneDirectionAuto:
		return config.PaneDirectionRight
	case config.PaneDirectionRight:
		return config.PaneDirectionDown
	case config.PaneDirectionDown:
		return config.PaneDirectionLeft
	case config.PaneDirectionLeft:
		return config.PaneDirectionUp
	default:
		return config.PaneDirectionAuto
	}
}

// paneDirectionDisplay renders the pane direction as a user-friendly label.
func paneDirectionDisplay(dir string) string {
	switch dir {
	case config.PaneDirectionRight:
		return styles.ConfigValueStyle.Render("Right")
	case config.PaneDirectionDown:
		return styles.ConfigValueStyle.Render("Down")
	case config.PaneDirectionLeft:
		return styles.ConfigValueStyle.Render("Left")
	case config.PaneDirectionUp:
		return styles.ConfigValueStyle.Render("Up")
	default:
		return styles.ConfigDimmedValue.Render("Auto")
	}
}

// cycleTheme cycles through available theme names.
func (c *ConfigPanel) cycleTheme(current string) string {
	if len(c.themeNames) == 0 {
		return current
	}
	// Normalise empty to "auto".
	if current == "" {
		current = themeAuto
	}
	for i, name := range c.themeNames {
		if name == current {
			return c.themeNames[(i+1)%len(c.themeNames)]
		}
	}
	return c.themeNames[0]
}

// cyclePreviewPosition cycles through the four preview positions.
func cyclePreviewPosition(current string) string {
	switch current {
	case config.PreviewPositionRight:
		return config.PreviewPositionBottom
	case config.PreviewPositionBottom:
		return config.PreviewPositionLeft
	case config.PreviewPositionLeft:
		return config.PreviewPositionTop
	case config.PreviewPositionTop:
		return config.PreviewPositionRight
	default:
		return config.PreviewPositionBottom
	}
}

// previewPositionDisplay renders the preview position as a user-friendly label.
func previewPositionDisplay(pos string) string {
	switch pos {
	case config.PreviewPositionBottom:
		return styles.ConfigValueStyle.Render("Bottom")
	case config.PreviewPositionLeft:
		return styles.ConfigValueStyle.Render("Left")
	case config.PreviewPositionTop:
		return styles.ConfigValueStyle.Render("Top")
	default:
		return styles.ConfigValueStyle.Render("Right")
	}
}
