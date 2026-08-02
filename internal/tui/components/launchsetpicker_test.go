package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/config"
)

func TestLaunchSetPicker_SetLaunchSetsMarksMissing(t *testing.T) {
	t.Parallel()
	p := NewLaunchSetPicker()
	p.SetLaunchSets([]config.LaunchSet{
		{Name: "Feature", SessionIDs: []string{"s1", "s2"}},
	}, map[string]struct{}{"s1": {}})

	missing := p.MissingIDs("Feature")
	if len(missing) != 1 || missing[0] != "s2" {
		t.Fatalf("MissingIDs = %v, want [s2]", missing)
	}
}

func TestLaunchSetPicker_SelectedAndNavigation(t *testing.T) {
	t.Parallel()
	p := NewLaunchSetPicker()
	p.SetLaunchSets([]config.LaunchSet{
		{Name: "A", SessionIDs: []string{"s1"}},
		{Name: "B", SessionIDs: []string{"s2"}},
	}, map[string]struct{}{"s1": {}, "s2": {}})

	p.MoveDown()
	set, ok := p.Selected()
	if !ok || set.Name != "B" {
		t.Fatalf("Selected = (%q, %v), want B true", set.Name, ok)
	}
	p.MoveDown()
	set, _ = p.Selected()
	if set.Name != "B" {
		t.Fatalf("MoveDown should clamp at B, got %q", set.Name)
	}
	p.MoveUp()
	set, _ = p.Selected()
	if set.Name != "A" {
		t.Fatalf("MoveUp selected %q, want A", set.Name)
	}
}

func TestLaunchSetPicker_SaveRenameDeleteModes(t *testing.T) {
	t.Parallel()
	p := NewLaunchSetPicker()
	p.SetLaunchSets([]config.LaunchSet{{Name: "A", SessionIDs: []string{"s1"}}}, map[string]struct{}{"s1": {}})

	p.BeginSave("New")
	if !p.Saving() || p.Value() != "New" {
		t.Fatalf("BeginSave mode/value = %v/%q, want saving/New", p.Saving(), p.Value())
	}
	p.CancelMode()
	if p.Saving() {
		t.Fatal("CancelMode should leave save mode")
	}

	p.BeginRename()
	if !p.Renaming() || p.Value() != "A" {
		t.Fatalf("BeginRename mode/value = %v/%q, want renaming/A", p.Renaming(), p.Value())
	}
	p.CancelMode()
	p.BeginDelete()
	if !p.Deleting() {
		t.Fatal("BeginDelete should enter delete mode")
	}
}

func TestLaunchSetPicker_ViewShowsMissing(t *testing.T) {
	t.Parallel()
	p := NewLaunchSetPicker()
	p.SetLaunchSets([]config.LaunchSet{
		{Name: "Feature", SessionIDs: []string{"s1", "s2"}},
	}, map[string]struct{}{"s1": {}})
	p.SetSize(100, 30)

	out := p.View()
	for _, want := range []string{"Launch Sets", "Feature", "1 missing", "missing: s2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("View() missing %q in:\n%s", want, out)
		}
	}
}
