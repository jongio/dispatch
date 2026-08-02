package tui

import (
	"sort"
	"testing"

	"github.com/jongio/dispatch/internal/config"
)

// The config package cannot import this package (that would be an import
// cycle), so it mirrors the keybinding action names and their default keys in
// order to generate the JSON Schema and validate config files. These tests
// keep that mirror honest: adding a binding here without updating
// internal/config/schema.go would otherwise make `dispatch config validate`
// reject a legitimate keybinding.

func TestConfigKeybindingActionsMatchKeyMap(t *testing.T) {
	km := defaultKeyMap()

	want := make([]string, 0)
	for _, entry := range keybindingEntries(&km) {
		want = append(want, entry.name)
	}
	got := append([]string(nil), config.KeybindingActions...)

	sort.Strings(want)
	sort.Strings(got)

	if len(want) != len(got) {
		t.Errorf("action count: keys.go has %d, config.KeybindingActions has %d", len(want), len(got))
	}

	inConfig := make(map[string]struct{}, len(got))
	for _, name := range got {
		inConfig[name] = struct{}{}
	}
	for _, name := range want {
		if _, ok := inConfig[name]; !ok {
			t.Errorf("action %q is bound in keys.go but missing from config.KeybindingActions", name)
		}
	}

	inKeyMap := make(map[string]struct{}, len(want))
	for _, name := range want {
		inKeyMap[name] = struct{}{}
	}
	for _, name := range got {
		if _, ok := inKeyMap[name]; !ok {
			t.Errorf("action %q is listed in config.KeybindingActions but is not bound in keys.go", name)
		}
	}
}

func TestConfigDefaultKeybindingsMatchKeyMap(t *testing.T) {
	km := defaultKeyMap()
	defaults := config.DefaultKeybindings()

	inKeyMap := make(map[string]struct{}, len(defaults))
	for _, entry := range keybindingEntries(&km) {
		inKeyMap[entry.name] = struct{}{}
		want := entry.binding.Keys()
		got, ok := defaults[entry.name]
		if !ok {
			t.Errorf("action %q has no default keys in internal/config", entry.name)
			continue
		}
		if len(want) != len(got) {
			t.Errorf("action %q default keys = %v, want %v", entry.name, got, want)
			continue
		}
		for i := range want {
			if want[i] != got[i] {
				t.Errorf("action %q default keys = %v, want %v", entry.name, got, want)
				break
			}
		}
	}

	// Reverse direction: a stale extra action in config.DefaultKeybindings()
	// that no longer exists in the keys.go key map must be caught.
	for action := range defaults {
		if _, ok := inKeyMap[action]; !ok {
			t.Errorf("action %q is in config.DefaultKeybindings() but not bound in keys.go", action)
		}
	}
}
