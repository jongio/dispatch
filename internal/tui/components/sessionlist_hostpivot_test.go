package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

func TestHostGroupLabel(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"github", "GitHub"},
		{"ado", "Azure DevOps"},
		{"", "No host"},
		{"other", "other"},
	}
	for _, tt := range tests {
		if got := hostGroupLabel(tt.in); got != tt.want {
			t.Errorf("hostGroupLabel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHostPivotRendersFriendlyLabels(t *testing.T) {
	sl := NewSessionList()
	sl.SetPivotField("host")
	sl.SetGroups([]data.SessionGroup{
		{Label: "github", Count: 2, Sessions: []data.Session{
			{ID: "1", HostType: "github"},
			{ID: "2", HostType: "github"},
		}},
		{Label: "", Count: 1, Sessions: []data.Session{
			{ID: "3"},
		}},
	})
	sl.SetSize(80, 20)

	view := sl.View()
	if !strings.Contains(view, "GitHub") {
		t.Errorf("expected view to contain friendly host label 'GitHub'\n%s", view)
	}
	if !strings.Contains(view, "No host") {
		t.Errorf("expected view to contain 'No host' for empty host type\n%s", view)
	}
}
