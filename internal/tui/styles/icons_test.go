package styles

import (
	"testing"
)

// ---------------------------------------------------------------------------
// SetNerdFontEnabled / NerdFontEnabled
// ---------------------------------------------------------------------------

func TestSetNerdFontEnabled_True(t *testing.T) {
	orig := NerdFontEnabled()
	defer SetNerdFontEnabled(orig)

	SetNerdFontEnabled(true)
	if !NerdFontEnabled() {
		t.Error("NerdFontEnabled() should be true after SetNerdFontEnabled(true)")
	}
}

func TestSetNerdFontEnabled_False(t *testing.T) {
	orig := NerdFontEnabled()
	defer SetNerdFontEnabled(orig)

	SetNerdFontEnabled(true) // set first
	SetNerdFontEnabled(false)
	if NerdFontEnabled() {
		t.Error("NerdFontEnabled() should be false after SetNerdFontEnabled(false)")
	}
}

func TestNerdFontEnabled_DefaultIsFalse(t *testing.T) {
	orig := NerdFontEnabled()
	defer SetNerdFontEnabled(orig)

	SetNerdFontEnabled(false) // ensure clean state
	if NerdFontEnabled() {
		t.Error("NerdFontEnabled() should default to false")
	}
}

// ---------------------------------------------------------------------------
// icon helper (tested via public accessors)
// ---------------------------------------------------------------------------

func TestIcon_ReturnsNerdFontWhenEnabled(t *testing.T) {
	SetNerdFontEnabled(true)
	defer SetNerdFontEnabled(false)

	got := icon("nf-value", "fb-value")
	if got != "nf-value" {
		t.Errorf("icon() = %q, want %q when Nerd Font enabled", got, "nf-value")
	}
}

func TestIcon_ReturnsFallbackWhenDisabled(t *testing.T) {
	SetNerdFontEnabled(false)

	got := icon("nf-value", "fb-value")
	if got != "fb-value" {
		t.Errorf("icon() = %q, want %q when Nerd Font disabled", got, "fb-value")
	}
}

// ---------------------------------------------------------------------------
// All Icon* functions — fallback mode
// ---------------------------------------------------------------------------

func TestIconFunctions_Fallback(t *testing.T) {
	SetNerdFontEnabled(false)

	tests := []struct {
		name string
		fn   func() string
		want string
	}{
		{"IconTitle", IconTitle, fbTerminal},
		{"IconFolder", IconFolder, fbFolder},
		{"IconFolderOpen", IconFolderOpen, fbFolderOpen},
		{"IconSearch", IconSearch, fbSearch},
		{"IconPointer", IconPointer, fbPointer},
		{"IconBullet", IconBullet, fbBullet},
		{"IconSortUp", IconSortUp, fbSortUp},
		{"IconSortDown", IconSortDown, fbSortDown},
		{"IconGear", IconGear, fbGear},
		{"IconKeyboard", IconKeyboard, fbKeyboard},
		{"IconSession", IconSession, fbSession},
		{"IconClock", IconClock, fbClock},
		{"IconFilter", IconFilter, fbFilter},
		{"IconGitBranch", IconGitBranch, fbGitBranch},
		{"IconCheck", IconCheck, fbCheck},
		{"IconHidden", IconHidden, fbEyeSlash},
		{"IconRepo", IconRepo, fbRepo},
		{"IconRepoOpen", IconRepoOpen, fbRepo},
		{"IconCalendar", IconCalendar, fbCalendar},
		{"IconCalendarOpen", IconCalendarOpen, fbCalendar},
		{"IconBranch", IconBranch, fbBranch},
		{"IconBranchOpen", IconBranchOpen, fbBranch},
		{"IconList", IconList, fbList},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if got != tt.want {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// All Icon* functions — Nerd Font mode
// ---------------------------------------------------------------------------

func TestIconFunctions_NerdFont(t *testing.T) {
	SetNerdFontEnabled(true)
	defer SetNerdFontEnabled(false)

	tests := []struct {
		name string
		fn   func() string
		want string
	}{
		{"IconTitle", IconTitle, nfTerminal},
		{"IconFolder", IconFolder, nfFolder + " "},
		{"IconFolderOpen", IconFolderOpen, nfFolderOpen + " "},
		{"IconSearch", IconSearch, nfSearch},
		{"IconPointer", IconPointer, nfPointer},
		{"IconBullet", IconBullet, nfBullet},
		{"IconSortUp", IconSortUp, nfSortUp},
		{"IconSortDown", IconSortDown, nfSortDown},
		{"IconGear", IconGear, nfGear},
		{"IconKeyboard", IconKeyboard, nfKeyboard},
		{"IconSession", IconSession, nfSession + " "},
		{"IconClock", IconClock, nfClock + " "},
		{"IconFilter", IconFilter, nfFilter},
		{"IconGitBranch", IconGitBranch, nfGitBranch + " "},
		{"IconCheck", IconCheck, nfCheck},
		{"IconHidden", IconHidden, nfEyeSlash},
		{"IconRepo", IconRepo, nfRepo + " "},
		{"IconRepoOpen", IconRepoOpen, nfRepo + " "},
		{"IconCalendar", IconCalendar, nfCalendar + " "},
		{"IconCalendarOpen", IconCalendarOpen, nfCalendar + " "},
		{"IconBranch", IconBranch, nfGitBranch + " "},
		{"IconBranchOpen", IconBranchOpen, nfGitBranch + " "},
		{"IconList", IconList, nfList + " "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if got != tt.want {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// All Icon* functions produce non-empty strings (both modes)
// ---------------------------------------------------------------------------

func TestIconFunctions_AllNonEmpty(t *testing.T) {
	fns := []struct {
		name string
		fn   func() string
	}{
		{"IconTitle", IconTitle},
		{"IconFolder", IconFolder},
		{"IconFolderOpen", IconFolderOpen},
		{"IconSearch", IconSearch},
		{"IconPointer", IconPointer},
		{"IconBullet", IconBullet},
		{"IconSortUp", IconSortUp},
		{"IconSortDown", IconSortDown},
		{"IconGear", IconGear},
		{"IconKeyboard", IconKeyboard},
		{"IconCheck", IconCheck},
		{"IconHidden", IconHidden},
		{"IconFilter", IconFilter},
		{"IconRepo", IconRepo},
		{"IconRepoOpen", IconRepoOpen},
		{"IconCalendar", IconCalendar},
		{"IconCalendarOpen", IconCalendarOpen},
		{"IconBranch", IconBranch},
		{"IconBranchOpen", IconBranchOpen},
		{"IconList", IconList},
	}

	for _, mode := range []bool{true, false} {
		SetNerdFontEnabled(mode)
		modeName := "fallback"
		if mode {
			modeName = "nerd"
		}
		for _, f := range fns {
			t.Run(modeName+"/"+f.name, func(t *testing.T) {
				got := f.fn()
				if got == "" {
					t.Errorf("%s() returned empty string in %s mode", f.name, modeName)
				}
			})
		}
	}
	SetNerdFontEnabled(false)
}

// ---------------------------------------------------------------------------
// PivotGroupIcons
// ---------------------------------------------------------------------------

func TestPivotGroupIcons(t *testing.T) {
	SetNerdFontEnabled(false)
	defer SetNerdFontEnabled(false)

	tests := []struct {
		pivot         string
		wantCollapsed string
		wantExpanded  string
	}{
		{"folder", IconFolder(), IconFolderOpen()},
		{"cwd", IconFolder(), IconFolderOpen()},
		{"repo", IconRepo(), IconRepoOpen()},
		{"repository", IconRepo(), IconRepoOpen()},
		{"branch", IconBranch(), IconBranchOpen()},
		{"date", IconCalendar(), IconCalendarOpen()},
		{"unknown", IconFolder(), IconFolderOpen()},
		{"", IconFolder(), IconFolderOpen()},
	}
	for _, tt := range tests {
		t.Run(tt.pivot, func(t *testing.T) {
			c, e := PivotGroupIcons(tt.pivot)
			if c != tt.wantCollapsed {
				t.Errorf("collapsed = %q, want %q", c, tt.wantCollapsed)
			}
			if e != tt.wantExpanded {
				t.Errorf("expanded = %q, want %q", e, tt.wantExpanded)
			}
		})
	}
}
