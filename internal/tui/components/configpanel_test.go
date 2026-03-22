package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/platform"
)

// ---------------------------------------------------------------------------
// NewConfigPanel
// ---------------------------------------------------------------------------

func TestNewConfigPanel_Defaults(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	if cp.IsEditing() {
		t.Error("new ConfigPanel should not be in editing mode")
	}
	v := cp.Values()
	if v.YoloMode {
		t.Error("default yoloMode should be false")
	}
	if v.Agent != "" || v.Model != "" || v.LaunchMode != "" || v.Terminal != "" || v.Shell != "" || v.CustomCommand != "" || v.Theme != "" {
		t.Error("default string values should be empty")
	}
}

// ---------------------------------------------------------------------------
// SetValues / Values round-trip
// ---------------------------------------------------------------------------

func TestConfigPanel_SetValues_RoundTrip(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetValues(ConfigValues{
		YoloMode: true, Agent: "myagent", Model: "gpt-4", LaunchMode: "tab",
		Terminal: "Windows Terminal", Shell: "pwsh", CustomCommand: "my-cmd {sessionId}", Theme: "Campbell",
		WorkspaceRecovery: true, PreviewPosition: "bottom",
	})

	v := cp.Values()
	if !v.YoloMode {
		t.Error("yoloMode should be true")
	}
	if v.Agent != "myagent" {
		t.Errorf("agent = %q, want %q", v.Agent, "myagent")
	}
	if v.Model != "gpt-4" {
		t.Errorf("model = %q, want %q", v.Model, "gpt-4")
	}
	if v.LaunchMode != "tab" {
		t.Errorf("launchMode = %q, want %q", v.LaunchMode, "tab")
	}
	if v.Terminal != "Windows Terminal" {
		t.Errorf("terminal = %q, want %q", v.Terminal, "Windows Terminal")
	}
	if v.Shell != "pwsh" {
		t.Errorf("shell = %q, want %q", v.Shell, "pwsh")
	}
	if v.CustomCommand != "my-cmd {sessionId}" {
		t.Errorf("customCommand = %q, want %q", v.CustomCommand, "my-cmd {sessionId}")
	}
	if v.Theme != "Campbell" {
		t.Errorf("theme = %q, want %q", v.Theme, "Campbell")
	}
	if !v.WorkspaceRecovery {
		t.Error("workspaceRecovery should be true")
	}
	if v.PreviewPosition != "bottom" {
		t.Errorf("previewPosition = %q, want %q", v.PreviewPosition, "bottom")
	}
}

// ---------------------------------------------------------------------------
// Navigation
// ---------------------------------------------------------------------------

func TestConfigPanel_MoveUpDown(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	if cp.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", cp.cursor)
	}

	cp.MoveDown()
	if cp.cursor != 1 {
		t.Errorf("after MoveDown cursor = %d, want 1", cp.cursor)
	}

	cp.MoveUp()
	if cp.cursor != 0 {
		t.Errorf("after MoveUp cursor = %d, want 0", cp.cursor)
	}

	// MoveUp at top stays at 0.
	cp.MoveUp()
	if cp.cursor != 0 {
		t.Errorf("MoveUp at top cursor = %d, want 0", cp.cursor)
	}

	// Move to last field.
	for i := 0; i < 20; i++ {
		cp.MoveDown()
	}
	if cp.cursor != cfgFieldCount-1 {
		t.Errorf("MoveDown past end cursor = %d, want %d", cp.cursor, cfgFieldCount-1)
	}
}

func TestConfigPanel_MoveBlockedWhileEditing(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.cursor = cfgAgent
	_ = cp.HandleEnter() // starts editing
	if !cp.IsEditing() {
		t.Fatal("should be editing after HandleEnter on agent field")
	}

	cp.MoveDown()
	if cp.cursor != cfgAgent {
		t.Error("MoveDown should be blocked while editing")
	}
	cp.MoveUp()
	if cp.cursor != cfgAgent {
		t.Error("MoveUp should be blocked while editing")
	}
	cp.CancelEdit()
}

// ---------------------------------------------------------------------------
// HandleEnter
// ---------------------------------------------------------------------------

func TestConfigPanel_HandleEnter_YoloToggle(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.cursor = cfgYoloMode
	cp.HandleEnter()
	if !cp.Values().YoloMode {
		t.Error("HandleEnter on yoloMode should toggle to true")
	}
	cp.HandleEnter()
	if cp.Values().YoloMode {
		t.Error("second HandleEnter on yoloMode should toggle back to false")
	}
}

func TestConfigPanel_HandleEnter_WorkspaceRecoveryToggle(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetValues(ConfigValues{WorkspaceRecovery: true})
	cp.cursor = cfgWorkspaceRecovery
	cp.HandleEnter()
	if cp.Values().WorkspaceRecovery {
		t.Error("HandleEnter on workspaceRecovery should toggle to false")
	}
	cp.HandleEnter()
	if !cp.Values().WorkspaceRecovery {
		t.Error("second HandleEnter on workspaceRecovery should toggle back to true")
	}
}

func TestConfigPanel_HandleEnter_PreviewPositionCycle(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetValues(ConfigValues{PreviewPosition: "right"})
	cp.cursor = cfgPreviewPosition

	// right → bottom
	cp.HandleEnter()
	if pos := cp.Values().PreviewPosition; pos != "bottom" {
		t.Errorf("after cycling from right, pos = %q, want %q", pos, "bottom")
	}

	// bottom → left
	cp.HandleEnter()
	if pos := cp.Values().PreviewPosition; pos != "left" {
		t.Errorf("after cycling from bottom, pos = %q, want %q", pos, "left")
	}

	// left → top
	cp.HandleEnter()
	if pos := cp.Values().PreviewPosition; pos != "top" {
		t.Errorf("after cycling from left, pos = %q, want %q", pos, "top")
	}

	// top → right (wraps around)
	cp.HandleEnter()
	if pos := cp.Values().PreviewPosition; pos != "right" {
		t.Errorf("after cycling from top, pos = %q, want %q", pos, "right")
	}
}

func TestConfigPanel_HandleEnter_LaunchModeCycle(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetValues(ConfigValues{LaunchMode: "window"})
	cp.cursor = cfgLaunchMode
	cp.HandleEnter()
	if mode := cp.Values().LaunchMode; mode != "pane" {
		t.Errorf("after cycling from window, mode = %q, want %q", mode, "pane")
	}
}

func TestConfigPanel_HandleEnter_AgentEditing(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.cursor = cfgAgent
	_ = cp.HandleEnter()
	if !cp.IsEditing() {
		t.Error("HandleEnter on agent should start editing")
	}
	cp.CancelEdit()
}

func TestConfigPanel_HandleEnter_CustomCommandOverrides(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetValues(ConfigValues{CustomCommand: "my-cmd"})

	// With custom command set, yolo/agent/model should be no-ops.
	cp.cursor = cfgYoloMode
	cp.HandleEnter()
	if cp.Values().YoloMode {
		t.Error("yoloMode should not toggle when custom command is set")
	}
}

// ---------------------------------------------------------------------------
// ConfirmEdit / CancelEdit
// ---------------------------------------------------------------------------

func TestConfigPanel_ConfirmEdit(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.cursor = cfgAgent
	_ = cp.HandleEnter() // start editing
	cp.textInput.SetValue("  new-agent  ")
	cp.ConfirmEdit()

	if cp.IsEditing() {
		t.Error("should not be editing after ConfirmEdit")
	}
	if agent := cp.Values().Agent; agent != "new-agent" {
		t.Errorf("agent after confirm = %q, want %q", agent, "new-agent")
	}
}

func TestConfigPanel_CancelEdit(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetValues(ConfigValues{Agent: "original"})
	cp.cursor = cfgAgent
	_ = cp.HandleEnter()
	cp.textInput.SetValue("changed")
	cp.CancelEdit()

	if agent := cp.Values().Agent; agent != "original" {
		t.Errorf("agent after cancel = %q, want %q", agent, "original")
	}
}

// ---------------------------------------------------------------------------
// SetTerminals / SetShellOptions / SetThemeOptions
// ---------------------------------------------------------------------------

func TestConfigPanel_SetTerminals(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetTerminals([]string{"Windows Terminal", "conhost"})
	if len(cp.terminals) != 2 {
		t.Errorf("terminals len = %d, want 2", len(cp.terminals))
	}
}

func TestConfigPanel_SetShellOptions(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	shells := []platform.ShellInfo{
		{Name: "pwsh", Path: "pwsh.exe"},
		{Name: "cmd", Path: "cmd.exe"},
	}
	cp.SetShellOptions(shells)
	if len(cp.shellInfos) != 2 {
		t.Errorf("shellInfos len = %d, want 2", len(cp.shellInfos))
	}
}

func TestConfigPanel_SetThemeOptions(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetThemeOptions([]string{"auto", "Dispatch Dark", "Campbell"})
	if len(cp.themeNames) != 3 {
		t.Errorf("themeNames len = %d, want 3", len(cp.themeNames))
	}
}

// ---------------------------------------------------------------------------
// Terminal/Shell cycling
// ---------------------------------------------------------------------------

func TestConfigPanel_CycleTerminal(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetTerminals([]string{"Windows Terminal", "conhost"})
	cp.SetValues(ConfigValues{})
	cp.cursor = cfgTerminal
	cp.HandleEnter() // cycle from "" → "Windows Terminal"
	if terminal := cp.Values().Terminal; terminal != "Windows Terminal" {
		t.Errorf("after cycle terminal = %q, want %q", terminal, "Windows Terminal")
	}
}

func TestConfigPanel_CycleShell(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetShellOptions([]platform.ShellInfo{
		{Name: "pwsh", Path: "pwsh.exe"},
		{Name: "cmd", Path: "cmd.exe"},
	})
	cp.SetValues(ConfigValues{})
	cp.cursor = cfgShell
	cp.HandleEnter() // cycle from "" → "pwsh"
	if shell := cp.Values().Shell; shell != "pwsh" {
		t.Errorf("after cycle shell = %q, want %q", shell, "pwsh")
	}
}

func TestConfigPanel_CycleTheme(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetThemeOptions([]string{"auto", "Dispatch Dark", "Campbell"})
	cp.SetValues(ConfigValues{Theme: "auto"})
	cp.cursor = cfgTheme
	cp.HandleEnter() // cycle from "auto" → "Dispatch Dark"
	if theme := cp.Values().Theme; theme != "Dispatch Dark" {
		t.Errorf("after cycle theme = %q, want %q", theme, "Dispatch Dark")
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func TestConfigPanel_View_ContainsSettings(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetSize(80, 40)
	view := cp.View()
	if !strings.Contains(view, "Settings") {
		t.Error("View should contain 'Settings' title")
	}
}

func TestConfigPanel_View_ContainsFields(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetSize(80, 40)
	view := cp.View()
	for _, field := range []string{"Yolo Mode", "Agent", "Model", "Launch Mode", "Terminal", "Shell", "Custom Command", "Theme", "Crash Recovery", "Preview Position"} {
		if !strings.Contains(view, field) {
			t.Errorf("View should contain field %q", field)
		}
	}
}

func TestConfigPanel_View_ShowsOverriddenWhenCustomCommand(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetValues(ConfigValues{CustomCommand: "my-cmd"})
	cp.SetSize(80, 40)
	view := cp.View()
	if !strings.Contains(view, "overridden") {
		t.Error("View should show 'overridden' when custom command is set")
	}
}

func TestConfigPanel_View_ShowsEditFooter(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetSize(80, 40)
	cp.cursor = cfgAgent
	_ = cp.HandleEnter()
	view := cp.View()
	if !strings.Contains(view, "Enter confirm") {
		t.Error("View in editing mode should show 'Enter confirm'")
	}
	cp.CancelEdit()
}

func TestConfigPanel_View_CustomCommandHelp(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetSize(80, 40)
	cp.cursor = cfgCustomCommand
	view := cp.View()
	if !strings.Contains(view, "{sessionId}") {
		t.Error("View on Custom Command field should show {sessionId} help")
	}
}

func TestConfigPanel_View_DoesNotPanic(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	_ = cp.View() // zero size
	cp.SetSize(80, 40)
	_ = cp.View()
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func TestBoolDisplay(t *testing.T) {
	t.Parallel()
	on := boolDisplay(true)
	off := boolDisplay(false)
	if !strings.Contains(on, "ON") {
		t.Errorf("boolDisplay(true) = %q, want to contain 'ON'", on)
	}
	if !strings.Contains(off, "OFF") {
		t.Errorf("boolDisplay(false) = %q, want to contain 'OFF'", off)
	}
}

func TestStringDisplay(t *testing.T) {
	t.Parallel()
	empty := stringDisplay("")
	if !strings.Contains(empty, "(none)") {
		t.Errorf("stringDisplay(\"\") = %q, want to contain '(none)'", empty)
	}
	val := stringDisplay("hello")
	if !strings.Contains(val, "hello") {
		t.Errorf("stringDisplay(\"hello\") = %q, want to contain 'hello'", val)
	}
}

func TestLaunchModeDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode string
		want string
	}{
		{"in-place", "In-Place"},
		{"window", "Window"},
		{"tab", "Tab"},
		{"", "Tab"},
	}
	for _, tt := range tests {
		got := launchModeDisplay(tt.mode)
		if !strings.Contains(got, tt.want) {
			t.Errorf("launchModeDisplay(%q) = %q, want to contain %q", tt.mode, got, tt.want)
		}
	}
}

func TestThemeDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		theme string
		want  string
	}{
		{"", "auto"},
		{"auto", "auto"},
		{"Campbell", "Campbell"},
	}
	for _, tt := range tests {
		got := themeDisplay(tt.theme)
		if !strings.Contains(got, tt.want) {
			t.Errorf("themeDisplay(%q) = %q, want to contain %q", tt.theme, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Preview position helpers
// ---------------------------------------------------------------------------

func TestCyclePreviewPosition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		current string
		want    string
	}{
		{"right", "bottom"},
		{"bottom", "left"},
		{"left", "top"},
		{"top", "right"},
		{"", "bottom"},        // default/empty cycles to bottom
		{"invalid", "bottom"}, // invalid cycles to bottom
	}
	for _, tt := range tests {
		got := cyclePreviewPosition(tt.current)
		if got != tt.want {
			t.Errorf("cyclePreviewPosition(%q) = %q, want %q", tt.current, got, tt.want)
		}
	}
}

func TestPreviewPositionDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pos  string
		want string
	}{
		{"right", "Right"},
		{"bottom", "Bottom"},
		{"left", "Left"},
		{"top", "Top"},
		{"", "Right"},        // default shows Right
		{"invalid", "Right"}, // invalid shows Right
	}
	for _, tt := range tests {
		got := previewPositionDisplay(tt.pos)
		if !strings.Contains(got, tt.want) {
			t.Errorf("previewPositionDisplay(%q) = %q, want to contain %q", tt.pos, got, tt.want)
		}
	}
}
