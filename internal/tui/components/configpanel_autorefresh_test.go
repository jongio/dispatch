package components

import (
	"strings"
	"testing"
)

func TestConfigPanel_AutoRefresh_RoundTrip(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetValues(ConfigValues{AutoRefresh: "5"})
	if got := cp.Values().AutoRefresh; got != "5" {
		t.Errorf("AutoRefresh = %q, want %q", got, "5")
	}
}

func TestConfigPanel_AutoRefresh_Edit(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.cursor = cfgAutoRefresh
	if cmd := cp.HandleEnter(); cmd == nil {
		t.Fatal("HandleEnter on Auto Refresh should start text editing")
	}
	if !cp.IsEditing() {
		t.Fatal("expected editing mode after HandleEnter")
	}
	cp.textInput.SetValue("  0  ")
	cp.ConfirmEdit()
	if got := cp.Values().AutoRefresh; got != "0" {
		t.Errorf("AutoRefresh after confirm = %q, want %q", got, "0")
	}
}

func TestAutoRefreshDisplay(t *testing.T) {
	t.Parallel()
	if got := autoRefreshDisplay(""); !strings.Contains(got, "Default") {
		t.Errorf("empty display = %q, want to contain Default", got)
	}
	if got := autoRefreshDisplay("0"); !strings.Contains(got, "Off") {
		t.Errorf("zero display = %q, want to contain Off", got)
	}
	if got := autoRefreshDisplay("5"); !strings.Contains(got, "5s") {
		t.Errorf("positive display = %q, want to contain 5s", got)
	}
}

func TestConfigPanel_View_ShowsAutoRefresh(t *testing.T) {
	t.Parallel()
	cp := NewConfigPanel()
	cp.SetSize(80, 40)
	if !strings.Contains(cp.View(), "Auto Refresh") {
		t.Error("config panel view should list the Auto Refresh field")
	}
}
