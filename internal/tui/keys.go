package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
)

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
	CopyID            key.Binding
	CopyPath          key.Binding
	CopyResumeCommand key.Binding
	CopyPreview       key.Binding
	ExpandCollapseAll key.Binding
	ScanWorkStatus    key.Binding
	Export            key.Binding
	Note              key.Binding
	ShiftUp           key.Binding
	ShiftDown         key.Binding
	ViewSwitch        key.Binding
	OpenFile          key.Binding
	OpenDir           key.Binding
	Timeline          key.Binding
	Compare           key.Binding
	CmdPalette        key.Binding
}

// ShortHelp returns a compact set of key bindings for the mini help bar.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.LaunchWindow, k.LaunchTab, k.LaunchPane, k.LaunchAll, k.Search, k.Filter, k.Sort, k.Preview, k.ViewPlan, k.Timeline, k.Compare, k.Hide, k.Star, k.Note, k.CopyID, k.CopyPath, k.CopyResumeCommand, k.CopyPreview, k.Export, k.OpenFile, k.OpenDir, k.JumpNextAttention, k.FilterAttention, k.ResumeInterrupted, k.ScanWorkStatus, k.ExpandCollapseAll, k.ViewSwitch, k.CmdPalette, k.Config, k.Help, k.Quit}
}

// FullHelp returns grouped key bindings for the expanded help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.Enter, k.LaunchWindow, k.LaunchTab, k.LaunchPane},
		{k.Space, k.LaunchAll, k.SelectAll, k.DeselectAll, k.ShiftUp, k.ShiftDown},
		{k.Search, k.Escape, k.Filter},
		{k.Sort, k.SortOrder, k.Pivot, k.PivotOrder, k.ExpandCollapseAll},
		{k.Preview, k.PreviewPosition, k.PreviewScrollUp, k.PreviewScrollDown, k.ConversationSort, k.ViewPlan, k.Timeline, k.Compare, k.CopyID, k.CopyPath, k.CopyResumeCommand, k.CopyPreview, k.Export, k.OpenFile, k.OpenDir, k.Reindex, k.ScanWorkStatus, k.ViewSwitch, k.Config},
		{k.Hide, k.ToggleHidden, k.Star, k.Note, k.JumpNextAttention, k.FilterAttention, k.ResumeInterrupted},
		{k.TimeRange1, k.TimeRange2, k.TimeRange3, k.TimeRange4},
		{k.Help, k.CmdPalette, k.Quit},
	}
}

var keys = defaultKeyMap()

// defaultKeyMap returns the built-in key bindings. It is the starting point
// before any user overrides from config are applied.
func defaultKeyMap() keyMap {
	return keyMap{
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
		Reindex:           key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rebuild index")),
		Help:              key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Config:            key.NewBinding(key.WithKeys(","), key.WithHelp(",", "settings")),
		TimeRange1:        key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "1 hour")),
		TimeRange2:        key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "1 day")),
		TimeRange3:        key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "7 days")),
		TimeRange4:        key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "all time")),
		Hide:              key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "hide session")),
		ToggleHidden:      key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "show hidden")),
		Star:              key.NewBinding(key.WithKeys("*"), key.WithHelp("*", "toggle favorite")),
		LaunchWindow:      key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "open in window")),
		LaunchTab:         key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "open in tab")),
		LaunchPane:        key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "open in pane")),
		PreviewScrollUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("PgUp", "preview ↑")),
		PreviewScrollDown: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("PgDn", "preview ↓")),
		JumpNextAttention: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next waiting")),
		FilterAttention:   key.NewBinding(key.WithKeys("!"), key.WithHelp("!", "filter by status")),
		LaunchAll:         key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "launch selected")),
		SelectAll:         key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "select all")),
		DeselectAll:       key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "deselect all")),
		ConversationSort:  key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "conversation order")),
		PreviewPosition:   key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "cycle preview position")),
		ResumeInterrupted: key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "resume interrupted")),
		ViewPlan:          key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view plan")),
		CopyID:            key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy session ID")),
		CopyPath:          key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "copy path")),
		CopyResumeCommand: key.NewBinding(key.WithKeys("Y"), key.WithHelp("Y", "copy resume cmd")),
		CopyPreview:       key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy preview")),
		ExpandCollapseAll: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "expand/collapse all")),
		ScanWorkStatus:    key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "scan work status")),
		Export:            key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "export markdown")),
		Note:              key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "edit note")),
		ShiftUp:           key.NewBinding(key.WithKeys("shift+up"), key.WithHelp("shift+\u2191", "extend select up")),
		ShiftDown:         key.NewBinding(key.WithKeys("shift+down"), key.WithHelp("shift+\u2193", "extend select down")),
		ViewSwitch:        key.NewBinding(key.WithKeys("V"), key.WithHelp("V", "switch view")),
		OpenFile:          key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "open file")),
		OpenDir:           key.NewBinding(key.WithKeys("O"), key.WithHelp("O", "open directory")),
		Timeline:          key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "activity timeline")),
		Compare:           key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "compare selected")),
		CmdPalette:        key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command palette")),
	}
}

// keybindingEntry pairs a config action name with a pointer to the binding it
// controls on a keyMap.
type keybindingEntry struct {
	name    string
	binding *key.Binding
}

// keybindingEntries returns every remappable action in display order, each
// paired with a pointer to its binding on km. The names are the identifiers
// users reference in the "keybindings" config map.
func keybindingEntries(km *keyMap) []keybindingEntry {
	return []keybindingEntry{
		{"up", &km.Up},
		{"down", &km.Down},
		{"left", &km.Left},
		{"right", &km.Right},
		{"enter", &km.Enter},
		{"space", &km.Space},
		{"quit", &km.Quit},
		{"force_quit", &km.ForceQuit},
		{"search", &km.Search},
		{"escape", &km.Escape},
		{"filter", &km.Filter},
		{"sort", &km.Sort},
		{"sort_order", &km.SortOrder},
		{"pivot", &km.Pivot},
		{"pivot_order", &km.PivotOrder},
		{"preview", &km.Preview},
		{"reindex", &km.Reindex},
		{"help", &km.Help},
		{"config", &km.Config},
		{"time_range_1", &km.TimeRange1},
		{"time_range_2", &km.TimeRange2},
		{"time_range_3", &km.TimeRange3},
		{"time_range_4", &km.TimeRange4},
		{"hide", &km.Hide},
		{"toggle_hidden", &km.ToggleHidden},
		{"star", &km.Star},
		{"launch_window", &km.LaunchWindow},
		{"launch_tab", &km.LaunchTab},
		{"launch_pane", &km.LaunchPane},
		{"preview_scroll_up", &km.PreviewScrollUp},
		{"preview_scroll_down", &km.PreviewScrollDown},
		{"jump_next_attention", &km.JumpNextAttention},
		{"filter_attention", &km.FilterAttention},
		{"launch_all", &km.LaunchAll},
		{"select_all", &km.SelectAll},
		{"deselect_all", &km.DeselectAll},
		{"conversation_sort", &km.ConversationSort},
		{"preview_position", &km.PreviewPosition},
		{"resume_interrupted", &km.ResumeInterrupted},
		{"view_plan", &km.ViewPlan},
		{"copy_id", &km.CopyID},
		{"copy_path", &km.CopyPath},
		{"copy_resume_command", &km.CopyResumeCommand},
		{"copy_preview", &km.CopyPreview},
		{"expand_collapse_all", &km.ExpandCollapseAll},
		{"scan_work_status", &km.ScanWorkStatus},
		{"export", &km.Export},
		{"note", &km.Note},
		{"shift_up", &km.ShiftUp},
		{"shift_down", &km.ShiftDown},
		{"view_switch", &km.ViewSwitch},
		{"open_file", &km.OpenFile},
		{"open_dir", &km.OpenDir},
		{"timeline", &km.Timeline},
		{"compare", &km.Compare},
		{"cmd_palette", &km.CmdPalette},
	}
}

// keybindingActionNames returns the remappable action names in display order.
func keybindingActionNames() []string {
	km := defaultKeyMap()
	entries := keybindingEntries(&km)
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	return names
}

// parseKeyList splits a comma-separated key list into trimmed, de-duplicated
// key strings, dropping empty entries while preserving order.
func parseKeyList(csv string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, part := range strings.Split(csv, ",") {
		k := strings.TrimSpace(part)
		if k == "" {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

// applyKeybindingOverrides returns a copy of base with the requested overrides
// applied, plus a list of human-readable warnings. Unknown action names,
// entries with no valid keys, and entries whose keys collide with another
// action are skipped and left at their defaults. base is not modified.
func applyKeybindingOverrides(base keyMap, overrides map[string]string) (keyMap, []string) {
	km := base // value copy; individual bindings are replaced, never mutated in place
	entries := keybindingEntries(&km)

	byName := make(map[string]*key.Binding, len(entries))
	defaults := make(map[string][]string, len(entries))
	desc := make(map[string]string, len(entries))
	order := make([]string, 0, len(entries))
	for _, e := range entries {
		byName[e.name] = e.binding
		defaults[e.name] = append([]string(nil), e.binding.Keys()...)
		desc[e.name] = e.binding.Help().Desc
		order = append(order, e.name)
	}

	var warnings []string

	// Parse requested overrides in sorted order for deterministic warnings.
	requested := make([]string, 0, len(overrides))
	for name := range overrides {
		requested = append(requested, name)
	}
	sort.Strings(requested)

	parsed := make(map[string][]string)
	for _, name := range requested {
		if _, ok := byName[name]; !ok {
			warnings = append(warnings, fmt.Sprintf("keybindings: unknown action %q (ignored)", name))
			continue
		}
		ks := parseKeyList(overrides[name])
		if len(ks) == 0 {
			warnings = append(warnings, fmt.Sprintf("keybindings: action %q has no valid keys (ignored)", name))
			continue
		}
		parsed[name] = ks
	}

	// Resolve conflicts. An override is invalid when any of its keys is also
	// bound by another action in the effective set. Reverting an override to
	// its default is safe because the built-in bindings never collide, so the
	// loop terminates once no override remains in conflict.
	for {
		effective := make(map[string][]string, len(order))
		for _, name := range order {
			if ks, ok := parsed[name]; ok {
				effective[name] = ks
			} else {
				effective[name] = defaults[name]
			}
		}

		owners := make(map[string]map[string]struct{})
		for _, name := range order {
			for _, k := range effective[name] {
				if owners[k] == nil {
					owners[k] = make(map[string]struct{})
				}
				owners[k][name] = struct{}{}
			}
		}

		offending := make(map[string]string) // action -> conflicting key
		for _, name := range order {
			if _, ok := parsed[name]; !ok {
				continue // only overrides can be reverted
			}
			for _, k := range parsed[name] {
				if len(owners[k]) > 1 {
					offending[name] = k
					break
				}
			}
		}
		if len(offending) == 0 {
			break
		}

		revert := make([]string, 0, len(offending))
		for name := range offending {
			revert = append(revert, name)
		}
		sort.Strings(revert)
		for _, name := range revert {
			warnings = append(warnings, fmt.Sprintf("keybindings: action %q key %q conflicts with another action (using default)", name, offending[name]))
			delete(parsed, name)
		}
	}

	// Apply the surviving overrides, keeping each action's help description.
	for name, ks := range parsed {
		b := byName[name]
		*b = key.NewBinding(key.WithKeys(ks...), key.WithHelp(strings.Join(ks, "/"), desc[name]))
	}

	return km, warnings
}
