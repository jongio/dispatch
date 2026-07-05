package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func keysEqual(b key.Binding, want ...string) bool {
	got := b.Keys()
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func warningsContain(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

func TestApplyKeybindingOverrides_NoOverrides(t *testing.T) {
	km, warnings := applyKeybindingOverrides(defaultKeyMap(), nil)
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if !keysEqual(km.Search, "/") {
		t.Errorf("Search keys = %v, want [/]", km.Search.Keys())
	}
	if !keysEqual(km.Quit, "q") {
		t.Errorf("Quit keys = %v, want [q]", km.Quit.Keys())
	}
}

func TestApplyKeybindingOverrides_SimpleRemap(t *testing.T) {
	km, warnings := applyKeybindingOverrides(defaultKeyMap(), map[string]string{
		"search": "/,ctrl+f",
	})
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if !keysEqual(km.Search, "/", "ctrl+f") {
		t.Errorf("Search keys = %v, want [/ ctrl+f]", km.Search.Keys())
	}
	// The default binding must not leak into the returned map's other fields.
	if !keysEqual(km.Quit, "q") {
		t.Errorf("Quit keys = %v, want [q]", km.Quit.Keys())
	}
}

func TestApplyKeybindingOverrides_PreservesDescription(t *testing.T) {
	km, _ := applyKeybindingOverrides(defaultKeyMap(), map[string]string{
		"search": "ctrl+f",
	})
	if got := km.Search.Help().Desc; got != "search" {
		t.Errorf("Search help desc = %q, want %q", got, "search")
	}
	if got := km.Search.Help().Key; got != "ctrl+f" {
		t.Errorf("Search help key = %q, want %q", got, "ctrl+f")
	}
}

func TestApplyKeybindingOverrides_UnknownAction(t *testing.T) {
	km, warnings := applyKeybindingOverrides(defaultKeyMap(), map[string]string{
		"bogus_action": "z",
	})
	if !warningsContain(warnings, "unknown action") {
		t.Errorf("warnings = %v, want an unknown action warning", warnings)
	}
	// Nothing should change.
	if !keysEqual(km.Search, "/") {
		t.Errorf("Search keys = %v, want unchanged [/]", km.Search.Keys())
	}
}

func TestApplyKeybindingOverrides_EmptyValue(t *testing.T) {
	km, warnings := applyKeybindingOverrides(defaultKeyMap(), map[string]string{
		"search": "  ",
	})
	if !warningsContain(warnings, "no valid keys") {
		t.Errorf("warnings = %v, want a no-valid-keys warning", warnings)
	}
	if !keysEqual(km.Search, "/") {
		t.Errorf("Search keys = %v, want unchanged [/]", km.Search.Keys())
	}
}

func TestApplyKeybindingOverrides_ConflictWithDefaultReverts(t *testing.T) {
	// "q" is quit's default; remapping search onto it must revert search.
	km, warnings := applyKeybindingOverrides(defaultKeyMap(), map[string]string{
		"search": "q",
	})
	if !warningsContain(warnings, "conflicts") {
		t.Errorf("warnings = %v, want a conflict warning", warnings)
	}
	if !keysEqual(km.Search, "/") {
		t.Errorf("Search keys = %v, want reverted [/]", km.Search.Keys())
	}
	if !keysEqual(km.Quit, "q") {
		t.Errorf("Quit keys = %v, want [q]", km.Quit.Keys())
	}
}

func TestApplyKeybindingOverrides_SwapWithoutConflict(t *testing.T) {
	// Moving quit off "q" frees it for search in the same batch.
	km, warnings := applyKeybindingOverrides(defaultKeyMap(), map[string]string{
		"quit":   "Q",
		"search": "q",
	})
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if !keysEqual(km.Quit, "Q") {
		t.Errorf("Quit keys = %v, want [Q]", km.Quit.Keys())
	}
	if !keysEqual(km.Search, "q") {
		t.Errorf("Search keys = %v, want [q]", km.Search.Keys())
	}
}

func TestApplyKeybindingOverrides_TwoOverridesConflict(t *testing.T) {
	// Two overrides claiming the same free key both revert to defaults.
	km, warnings := applyKeybindingOverrides(defaultKeyMap(), map[string]string{
		"search": "z",
		"filter": "z",
	})
	if !warningsContain(warnings, "conflicts") {
		t.Errorf("warnings = %v, want conflict warnings", warnings)
	}
	if !keysEqual(km.Search, "/") {
		t.Errorf("Search keys = %v, want reverted [/]", km.Search.Keys())
	}
	if !keysEqual(km.Filter, "f") {
		t.Errorf("Filter keys = %v, want reverted [f]", km.Filter.Keys())
	}
}

func TestApplyKeybindingOverrides_DeduplicatesKeys(t *testing.T) {
	km, warnings := applyKeybindingOverrides(defaultKeyMap(), map[string]string{
		"search": "ctrl+f, ctrl+f",
	})
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if !keysEqual(km.Search, "ctrl+f") {
		t.Errorf("Search keys = %v, want deduped [ctrl+f]", km.Search.Keys())
	}
}

func TestApplyKeybindingOverrides_DoesNotMutateBase(t *testing.T) {
	base := defaultKeyMap()
	_, _ = applyKeybindingOverrides(base, map[string]string{"search": "ctrl+f"})
	if !keysEqual(base.Search, "/") {
		t.Errorf("base Search keys = %v, want unchanged [/]", base.Search.Keys())
	}
}

func TestKeybindingActionNames(t *testing.T) {
	km := defaultKeyMap()
	entries := keybindingEntries(&km)
	seen := make(map[string]struct{})
	for _, e := range entries {
		if _, dup := seen[e.name]; dup {
			t.Errorf("duplicate action name %q", e.name)
		}
		seen[e.name] = struct{}{}
	}
	for _, required := range []string{"search", "quit", "preview", "filter"} {
		if _, ok := seen[required]; !ok {
			t.Errorf("action names missing %q", required)
		}
	}
}

func TestApplyKeybindingOverrides_MatchesRemappedKey(t *testing.T) {
	km, _ := applyKeybindingOverrides(defaultKeyMap(), map[string]string{"search": "u"})
	if !key.Matches(tea.KeyPressMsg{Code: 'u'}, km.Search) {
		t.Error("remapped search binding should match 'u'")
	}
	if key.Matches(tea.KeyPressMsg{Code: '/'}, km.Search) {
		t.Error("search should no longer match its old '/' key")
	}
}

func TestParseKeyList(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"  ", nil},
		{"/", []string{"/"}},
		{"/,ctrl+f", []string{"/", "ctrl+f"}},
		{" a , b ,, c ", []string{"a", "b", "c"}},
		{"x,x", []string{"x"}},
	}
	for _, tt := range tests {
		got := parseKeyList(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("parseKeyList(%q) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseKeyList(%q) = %v, want %v", tt.in, got, tt.want)
				break
			}
		}
	}
}
