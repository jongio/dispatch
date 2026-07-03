package tui

import "testing"

func TestAutoRefreshFieldValue(t *testing.T) {
	t.Parallel()

	if got := autoRefreshFieldValue(nil); got != "" {
		t.Errorf("nil = %q, want empty string", got)
	}
	zero := 0
	if got := autoRefreshFieldValue(&zero); got != "0" {
		t.Errorf("0 = %q, want \"0\"", got)
	}
	five := 5
	if got := autoRefreshFieldValue(&five); got != "5" {
		t.Errorf("5 = %q, want \"5\"", got)
	}
}

func TestParseAutoRefresh(t *testing.T) {
	t.Parallel()

	if got := parseAutoRefresh(""); got != nil {
		t.Errorf("empty = %v, want nil", got)
	}
	if got := parseAutoRefresh("   "); got != nil {
		t.Errorf("blank = %v, want nil", got)
	}
	if got := parseAutoRefresh("notanumber"); got != nil {
		t.Errorf("invalid = %v, want nil", got)
	}
	if got := parseAutoRefresh("0"); got == nil || *got != 0 {
		t.Errorf("\"0\" = %v, want pointer to 0", got)
	}
	if got := parseAutoRefresh(" 12 "); got == nil || *got != 12 {
		t.Errorf("\" 12 \" = %v, want pointer to 12", got)
	}
}
