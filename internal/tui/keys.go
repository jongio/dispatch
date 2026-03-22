package tui

import "charm.land/bubbles/v2/key"

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
	PivotOrder        key.Binding
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
	Star              key.Binding
	FilterFavorites   key.Binding
	LaunchWindow      key.Binding
	LaunchTab         key.Binding
	LaunchPane        key.Binding
	PreviewScrollUp   key.Binding
	PreviewScrollDown key.Binding
	JumpNextAttention key.Binding
	FilterAttention   key.Binding
	LaunchAll         key.Binding
	SelectAll         key.Binding
	DeselectAll       key.Binding
	ConversationSort  key.Binding
	PreviewPosition   key.Binding
	ResumeInterrupted key.Binding
	ViewPlan          key.Binding
	FilterPlans       key.Binding
}

// ShortHelp returns a compact set of key bindings for the mini help bar.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.LaunchWindow, k.LaunchTab, k.LaunchPane, k.LaunchAll, k.Search, k.Filter, k.Sort, k.Preview, k.ViewPlan, k.FilterPlans, k.Hide, k.Star, k.JumpNextAttention, k.FilterAttention, k.ResumeInterrupted, k.Config, k.Help, k.Quit}
}

// FullHelp returns grouped key bindings for the expanded help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.Enter, k.LaunchWindow, k.LaunchTab, k.LaunchPane},
		{k.Space, k.LaunchAll, k.SelectAll, k.DeselectAll},
		{k.Search, k.Escape, k.Filter},
		{k.Sort, k.SortOrder, k.Pivot, k.PivotOrder},
		{k.Preview, k.PreviewPosition, k.PreviewScrollUp, k.PreviewScrollDown, k.ConversationSort, k.ViewPlan, k.Reindex, k.Config},
		{k.Hide, k.ToggleHidden, k.Star, k.FilterFavorites, k.JumpNextAttention, k.FilterAttention, k.FilterPlans, k.ResumeInterrupted},
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
	Space:             key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle select")),
	Quit:              key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	ForceQuit:         key.NewBinding(key.WithKeys("ctrl+c")),
	Search:            key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Escape:            key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back / clear")),
	Filter:            key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter panel")),
	Sort:              key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "cycle sort")),
	SortOrder:         key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "toggle sort order")),
	Pivot:             key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "cycle pivot")),
	PivotOrder:        key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "reverse pivot order")),
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
	Star:              key.NewBinding(key.WithKeys("*"), key.WithHelp("*", "toggle favorite")),
	FilterFavorites:   key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "filter favorites")),
	LaunchWindow:      key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "open in window")),
	LaunchTab:         key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "open in tab")),
	LaunchPane:        key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "open in pane")),
	PreviewScrollUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("PgUp", "preview ↑")),
	PreviewScrollDown: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("PgDn", "preview ↓")),
	JumpNextAttention: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next waiting")),
	FilterAttention:   key.NewBinding(key.WithKeys("!"), key.WithHelp("!", "filter by status")),
	LaunchAll:         key.NewBinding(key.WithKeys("O"), key.WithHelp("O", "open selected")),
	SelectAll:         key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "select all")),
	DeselectAll:       key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "deselect all")),
	ConversationSort:  key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "conversation order")),
	PreviewPosition:   key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "cycle preview position")),
	ResumeInterrupted: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "resume interrupted")),
	ViewPlan:          key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view plan")),
	FilterPlans:       key.NewBinding(key.WithKeys("M"), key.WithHelp("M", "filter plans")),
}
