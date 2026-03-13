package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap holds all key bindings used by the root model.
// It implements help.KeyMap for automatic help text generation.
type keyMap struct {
	Up                key.Binding
	Down              key.Binding
	Left              key.Binding
	Right             key.Binding
	Enter             key.Binding
	Space             key.Binding
	Quit              key.Binding
	ForceQuit         key.Binding
	Search            key.Binding
	Escape            key.Binding
	Filter            key.Binding
	Sort              key.Binding
	SortOrder         key.Binding
	Pivot             key.Binding
	Preview           key.Binding
	Reindex           key.Binding
	Help              key.Binding
	Config            key.Binding
	TimeRange1        key.Binding
	TimeRange2        key.Binding
	TimeRange3        key.Binding
	TimeRange4        key.Binding
	Hide              key.Binding
	ToggleHidden      key.Binding
	LaunchWindow      key.Binding
	LaunchTab         key.Binding
	LaunchPane        key.Binding
	PreviewScrollUp   key.Binding
	PreviewScrollDown key.Binding
}

// ShortHelp returns a compact set of key bindings for the mini help bar.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.LaunchWindow, k.LaunchTab, k.LaunchPane, k.Search, k.Filter, k.Sort, k.Preview, k.Hide, k.Config, k.Help, k.Quit}
}

// FullHelp returns grouped key bindings for the expanded help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.Enter, k.LaunchWindow, k.LaunchTab, k.LaunchPane},
		{k.Search, k.Escape, k.Filter, k.Space},
		{k.Sort, k.SortOrder, k.Pivot},
		{k.Preview, k.PreviewScrollUp, k.PreviewScrollDown, k.Reindex, k.Config},
		{k.Hide, k.ToggleHidden},
		{k.TimeRange1, k.TimeRange2, k.TimeRange3, k.TimeRange4},
		{k.Help, k.Quit},
	}
}

var keys = keyMap{
	Up:                key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:              key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Left:              key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "collapse")),
	Right:             key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "expand")),
	Enter:             key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "launch/toggle")),
	Space:             key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select")),
	Quit:              key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	ForceQuit:         key.NewBinding(key.WithKeys("ctrl+c")),
	Search:            key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Escape:            key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back / clear")),
	Filter:            key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter panel")),
	Sort:              key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "cycle sort")),
	SortOrder:         key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "toggle sort order")),
	Pivot:             key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "cycle pivot")),
	Preview:           key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "toggle preview")),
	Reindex:           key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reindex")),
	Help:              key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Config:            key.NewBinding(key.WithKeys(","), key.WithHelp(",", "settings")),
	TimeRange1:        key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "1 hour")),
	TimeRange2:        key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "1 day")),
	TimeRange3:        key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "7 days")),
	TimeRange4:        key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "all time")),
	Hide:              key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "hide session")),
	ToggleHidden:      key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "show hidden")),
	LaunchWindow:      key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "open in window")),
	LaunchTab:         key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "open in tab")),
	LaunchPane:        key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "open in pane")),
	PreviewScrollUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("PgUp", "preview ↑")),
	PreviewScrollDown: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("PgDn", "preview ↓")),
}
