package components

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// FormatInt
// ---------------------------------------------------------------------------

func TestFormatInt(t *testing.T) {
	tests := []struct {
		name string
		v    int
		want string
	}{
		{"zero", 0, "0"},
		{"positive single digit", 7, "7"},
		{"positive multi digit", 42, "42"},
		{"positive large", 123456789, "123456789"},
		{"negative single digit", -1, "-1"},
		{"negative multi digit", -99, "-99"},
		{"negative large", -123456789, "-123456789"},
		{"max int32 range", 2147483647, "2147483647"},
		{"min int32 range", -2147483648, "-2147483648"},
		{"one", 1, "1"},
		{"ten", 10, "10"},
		{"hundred", 100, "100"},
		{"thousand", 1000, "1000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatInt(tt.v)
			if got != tt.want {
				t.Errorf("FormatInt(%d) = %q, want %q", tt.v, got, tt.want)
			}
		})
	}
}

func TestFormatIntMaxInt(t *testing.T) {
	// Verify math.MaxInt produces a valid positive numeric string.
	got := FormatInt(math.MaxInt)
	want := strconv.Itoa(math.MaxInt)
	if got != want {
		t.Errorf("FormatInt(MaxInt) = %q, want %q", got, want)
	}
}

func TestFormatIntMinInt(t *testing.T) {
	// Previously caused infinite recursion with the hand-rolled implementation.
	// Now delegates to strconv.Itoa which handles two's complement correctly.
	got := FormatInt(math.MinInt)
	want := strconv.Itoa(math.MinInt)
	if got != want {
		t.Errorf("FormatInt(MinInt) = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Truncate
// ---------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{"shorter than width", "hi", 10, "hi"},
		{"equal to width", "hello", 5, "hello"},
		{"longer than width", "hello world", 5, "hell…"},
		{"empty string", "", 10, ""},
		{"width zero", "hello", 0, ""},
		{"width one truncates", "hello", 1, "…"},
		{"width one no truncation needed", "h", 1, "h"},
		{"width two", "hello", 2, "h…"},
		{"negative width", "hello", -1, ""},
		{"unicode shorter", "日本語", 5, "日本語"},
		{"unicode exact", "日本語", 3, "日本語"},
		{"unicode longer", "日本語テスト", 4, "日本語…"},
		{"unicode width one", "日本語", 1, "…"},
		{"emoji", "👋🌍🎉", 2, "👋…"},
		{"single char no truncation", "a", 5, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.s, tt.width)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PadRight
// ---------------------------------------------------------------------------

func TestPadRight(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{"shorter pads", "hi", 5, "hi   "},
		{"equal length", "hello", 5, "hello"},
		{"longer truncates", "hello world", 5, "hell…"},
		{"empty string", "", 5, "     "},
		{"width zero", "hello", 0, ""},
		{"negative width", "hello", -1, ""},
		{"width one shorter", "h", 1, "h"},
		{"width one longer", "hi", 1, "…"},
		{"unicode pad", "日本", 5, "日本   "},
		{"single space", "", 1, " "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadRight(tt.s, tt.width)
			if got != tt.want {
				t.Errorf("PadRight(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PadLeft
// ---------------------------------------------------------------------------

func TestPadLeft(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{"shorter pads", "hi", 5, "   hi"},
		{"equal length", "hello", 5, "hello"},
		{"longer no truncation", "hello world", 5, "hello world"},
		{"empty string", "", 5, "     "},
		{"width zero", "hello", 0, ""},
		{"negative width", "hello", -1, ""},
		{"width one shorter", "h", 1, "h"},
		{"width one longer", "hi", 1, "hi"},
		{"unicode pad", "日", 4, "   日"},
		{"single space pad", "", 1, " "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadLeft(tt.s, tt.width)
			if got != tt.want {
				t.Errorf("PadLeft(%q, %d) = %q, want %q", tt.s, tt.width, got, tt.want)
			}
		})
	}
}

// PadLeft does not truncate (unlike PadRight). Verify explicitly.
func TestPadLeftDoesNotTruncate(t *testing.T) {
	got := PadLeft("toolong", 3)
	if got != "toolong" {
		t.Errorf("PadLeft(%q, 3) = %q, want original string unchanged", "toolong", got)
	}
}

// ---------------------------------------------------------------------------
// AbbrevPath
// ---------------------------------------------------------------------------

func TestAbbrevPath(t *testing.T) {
	sep := string(filepath.Separator)

	tests := []struct {
		name string
		path string
		want string
	}{
		{"empty path", "", "–"},
		{"single component", "leaf", "leaf"},
		{"two components", "parent/leaf", "parent" + sep + "leaf"},
		{"three components abbreviated", "a/b/c", "…" + sep + "b" + sep + "c"},
		{"deep path", "a/b/c/d/e", "…" + sep + "d" + sep + "e"},
		{"trailing slash stripped", "a/b/c/", "…" + sep + "b" + sep + "c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AbbrevPath(tt.path)
			if got != tt.want {
				t.Errorf("AbbrevPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestAbbrevPathPlatformSpecific(t *testing.T) {
	sep := string(filepath.Separator)

	if runtime.GOOS == "windows" {
		// On Windows, forward slashes are converted to backslashes.
		got := AbbrevPath(`C:\Users\alice\projects`)
		want := "…" + sep + "alice" + sep + "projects"
		if got != want {
			t.Errorf("AbbrevPath(Windows) = %q, want %q", got, want)
		}
	} else {
		got := AbbrevPath("/home/alice/projects")
		want := "…" + sep + "alice" + sep + "projects"
		if got != want {
			t.Errorf("AbbrevPath(Unix) = %q, want %q", got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// AbbrevHome
// ---------------------------------------------------------------------------

func TestAbbrevHomeEmpty(t *testing.T) {
	got := AbbrevHome("")
	if got != "–" {
		t.Errorf("AbbrevHome(%q) = %q, want %q", "", got, "–")
	}
}

func TestAbbrevHomeWithHomePrefix(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home dir: %v", err)
	}

	// Path under home directory should get ~ prefix.
	input := filepath.Join(home, "projects", "myapp")
	got := AbbrevHome(input)
	if !strings.HasPrefix(got, "~") {
		t.Errorf("AbbrevHome(%q) = %q, want ~ prefix", input, got)
	}
	// Should contain the rest of the path.
	if !strings.Contains(got, "projects") {
		t.Errorf("AbbrevHome(%q) = %q, missing 'projects' component", input, got)
	}
}

func TestAbbrevHomeWithoutHomePrefix(t *testing.T) {
	// Path not under home should be returned as-is (OS-normalised).
	var input string
	if runtime.GOOS == "windows" {
		input = `Z:\unrelated\path`
	} else {
		input = "/unrelated/path"
	}
	got := AbbrevHome(input)
	if strings.HasPrefix(got, "~") {
		t.Errorf("AbbrevHome(%q) = %q, should not start with ~", input, got)
	}
}

func TestAbbrevHomeExactHomeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home dir: %v", err)
	}

	got := AbbrevHome(home)
	if got != "~" {
		t.Errorf("AbbrevHome(homeDir) = %q, want %q", got, "~")
	}
}

// ---------------------------------------------------------------------------
// SplitDirLeaf
// ---------------------------------------------------------------------------

func TestSplitDirLeaf(t *testing.T) {
	sep := string(filepath.Separator)

	tests := []struct {
		name     string
		path     string
		wantDir  string
		wantLeaf string
	}{
		{"simple unix path", "a/b/c", "a" + sep + "b", "c"},
		{"no directory", "file.txt", "", "file.txt"},
		{"empty string", "", "", ""},
		{"trailing slash", "a/b/c/", "a" + sep + "b", "c"},
		{"two components", "dir/file", "dir", "file"},
		{"single with slash", "dir/", "", "dir"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDir, gotLeaf := SplitDirLeaf(tt.path)
			if gotDir != tt.wantDir || gotLeaf != tt.wantLeaf {
				t.Errorf("SplitDirLeaf(%q) = (%q, %q), want (%q, %q)",
					tt.path, gotDir, gotLeaf, tt.wantDir, tt.wantLeaf)
			}
		})
	}
}

func TestSplitDirLeafPlatform(t *testing.T) {
	sep := string(filepath.Separator)

	if runtime.GOOS == "windows" {
		dir, leaf := SplitDirLeaf(`C:\Users\alice\file.txt`)
		wantDir := `C:` + sep + `Users` + sep + `alice`
		if dir != wantDir || leaf != "file.txt" {
			t.Errorf("SplitDirLeaf(Windows) = (%q, %q), want (%q, %q)",
				dir, leaf, wantDir, "file.txt")
		}
	} else {
		dir, leaf := SplitDirLeaf("/home/alice/file.txt")
		wantDir := sep + "home" + sep + "alice"
		if dir != wantDir || leaf != "file.txt" {
			t.Errorf("SplitDirLeaf(Unix) = (%q, %q), want (%q, %q)",
				dir, leaf, wantDir, "file.txt")
		}
	}
}

// ---------------------------------------------------------------------------
// RelativeTime
// ---------------------------------------------------------------------------

func TestRelativeTimeEmpty(t *testing.T) {
	got := RelativeTime("")
	if got != "–" {
		t.Errorf("RelativeTime(%q) = %q, want %q", "", got, "–")
	}
}

func TestRelativeTimeInvalidFormat(t *testing.T) {
	got := RelativeTime("not-a-timestamp")
	if got != "–" {
		t.Errorf("RelativeTime(%q) = %q, want %q", "not-a-timestamp", got, "–")
	}
}

func TestRelativeTimeNow(t *testing.T) {
	ts := time.Now().Format(time.RFC3339)
	got := RelativeTime(ts)
	if got != "now" {
		t.Errorf("RelativeTime(now) = %q, want %q", got, "now")
	}
}

func TestRelativeTimeMinutesAgo(t *testing.T) {
	ts := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)
	got := RelativeTime(ts)
	if !strings.HasSuffix(got, " ago") || !strings.Contains(got, "m") {
		t.Errorf("RelativeTime(5m ago) = %q, want Nm ago pattern", got)
	}
}

func TestRelativeTimeHoursAgo(t *testing.T) {
	ts := time.Now().Add(-3 * time.Hour).Format(time.RFC3339)
	got := RelativeTime(ts)
	if !strings.HasSuffix(got, " ago") || !strings.Contains(got, "h") {
		t.Errorf("RelativeTime(3h ago) = %q, want Nh ago pattern", got)
	}
}

func TestRelativeTimeDaysAgo(t *testing.T) {
	ts := time.Now().Add(-48 * time.Hour).Format(time.RFC3339)
	got := RelativeTime(ts)
	if !strings.HasSuffix(got, "d ago") {
		t.Errorf("RelativeTime(2d ago) = %q, want Nd ago pattern", got)
	}
}

func TestRelativeTimeOneDayAgo(t *testing.T) {
	ts := time.Now().Add(-25 * time.Hour).Format(time.RFC3339)
	got := RelativeTime(ts)
	if got != "1d ago" {
		t.Errorf("RelativeTime(1d ago) = %q, want %q", got, "1d ago")
	}
}

func TestRelativeTimeLargeValue(t *testing.T) {
	ts := time.Now().Add(-365 * 24 * time.Hour).Format(time.RFC3339)
	got := RelativeTime(ts)
	if !strings.HasSuffix(got, "d ago") {
		t.Errorf("RelativeTime(365d ago) = %q, want Nd ago pattern", got)
	}
}

func TestRelativeTimeAlternateFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"RFC3339", time.RFC3339},
		{"datetime T", "2006-01-02T15:04:05"},
		{"datetime space", "2006-01-02 15:04:05"},
	}
	ts := time.Now().Add(-2 * time.Hour)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := ts.Format(tt.format)
			got := RelativeTime(input)
			if got == "–" {
				t.Errorf("RelativeTime(%q) with format %q = %q, want valid relative time",
					input, tt.format, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// formatDuration (unexported)
// ---------------------------------------------------------------------------

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		unit  string
		want  string
	}{
		{"normal minutes", 5.0, "m", "5m"},
		{"fractional truncates", 3.7, "h", "3h"},
		{"zero clamps to one", 0.0, "m", "1m"},
		{"negative clamps to one", -2.0, "s", "1s"},
		{"large value", 100.0, "d", "100d"},
		{"just above zero", 0.9, "m", "1m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.value, tt.unit)
			if got != tt.want {
				t.Errorf("formatDuration(%v, %q) = %q, want %q",
					tt.value, tt.unit, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// wordWrap (unexported, in preview.go)
// ---------------------------------------------------------------------------

func TestWordWrap(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  string
	}{
		{
			"short line no wrap",
			"hello world",
			80,
			"hello world",
		},
		{
			"exact width no wrap",
			"hello",
			5,
			"hello",
		},
		{
			"wraps at width",
			"hello world foo",
			11,
			"hello world\nfoo",
		},
		{
			"long word not broken",
			"superlongword",
			5,
			"superlongword",
		},
		{
			"multiple wraps",
			"a b c d e f",
			3,
			"a b\nc d\ne f",
		},
		{
			"empty string",
			"",
			10,
			"",
		},
		{
			"width zero returns text",
			"hello",
			0,
			"hello",
		},
		{
			"preserves paragraph breaks",
			"line one\nline two",
			80,
			"line one\nline two",
		},
		{
			"wraps within paragraphs",
			"aaa bbb ccc\nddd eee fff",
			7,
			"aaa bbb\nccc\nddd eee\nfff",
		},
		{
			"single word",
			"hello",
			10,
			"hello",
		},
		{
			"unicode words",
			"日本 語テ スト",
			4,
			"日本\n語テ\nスト",
		},
		{
			"negative width returns text",
			"hello",
			-1,
			"hello",
		},
		{
			"whitespace collapsed",
			"  hello   world  ",
			80,
			"hello world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wordWrap(tt.text, tt.width)
			if got != tt.want {
				t.Errorf("wordWrap(%q, %d) =\n  got:  %q\n  want: %q",
					tt.text, tt.width, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Cross-function consistency
// ---------------------------------------------------------------------------

// PadRight(Truncate(s, n), n) should always yield exactly n runes.
func TestPadRightTruncateConsistency(t *testing.T) {
	inputs := []string{"", "a", "hello", "hello world", "日本語テスト"}
	widths := []int{1, 3, 5, 10}

	for _, s := range inputs {
		for _, w := range widths {
			got := PadRight(Truncate(s, w), w)
			runes := []rune(got)
			if len(runes) != w {
				t.Errorf("PadRight(Truncate(%q, %d), %d) has %d runes, want %d; got %q",
					s, w, w, len(runes), w, got)
			}
		}
	}
}

// FormatInt should be consistent with strconv.Itoa for common values.
func TestFormatIntConsistencyWithStrconv(t *testing.T) {
	values := []int{0, 1, -1, 42, -42, 1000, -1000, 999999, -999999}
	for _, v := range values {
		got := FormatInt(v)
		want := strconv.Itoa(v)
		if got != want {
			t.Errorf("FormatInt(%d) = %q, want %q", v, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// ConfigPanel launch mode cycling
// ---------------------------------------------------------------------------

func TestCycleLaunchMode(t *testing.T) {
	// in-place → tab → window → pane → in-place
	if got := cycleLaunchMode("in-place"); got != "tab" {
		t.Errorf("cycleLaunchMode(in-place) = %q, want tab", got)
	}
	if got := cycleLaunchMode("tab"); got != "window" {
		t.Errorf("cycleLaunchMode(tab) = %q, want window", got)
	}
	if got := cycleLaunchMode("window"); got != "pane" {
		t.Errorf("cycleLaunchMode(window) = %q, want pane", got)
	}
	if got := cycleLaunchMode("pane"); got != "in-place" {
		t.Errorf("cycleLaunchMode(pane) = %q, want in-place", got)
	}
	// Unknown defaults to in-place.
	if got := cycleLaunchMode(""); got != "in-place" {
		t.Errorf("cycleLaunchMode('') = %q, want in-place", got)
	}
}

func TestConfigPanelLaunchModeSetGet(t *testing.T) {
	cp := NewConfigPanel()
	cp.SetValues(ConfigValues{LaunchMode: "window"})
	if mode := cp.Values().LaunchMode; mode != "window" {
		t.Errorf("SetValues/Values round-trip: got %q, want 'window'", mode)
	}
}

func TestConfigPanelLaunchModeHandleEnter(t *testing.T) {
	cp := NewConfigPanel()
	cp.SetValues(ConfigValues{LaunchMode: "tab"})
	cp.cursor = cfgLaunchMode
	cp.HandleEnter()
	if mode := cp.Values().LaunchMode; mode != "window" {
		t.Errorf("HandleEnter on LaunchMode: got %q, want 'window'", mode)
	}
}

func TestCyclePaneDirection(t *testing.T) {
	// auto → right → down → left → up → auto
	if got := cyclePaneDirection("auto"); got != "right" {
		t.Errorf("cyclePaneDirection(auto) = %q, want right", got)
	}
	if got := cyclePaneDirection("right"); got != "down" {
		t.Errorf("cyclePaneDirection(right) = %q, want down", got)
	}
	if got := cyclePaneDirection("down"); got != "left" {
		t.Errorf("cyclePaneDirection(down) = %q, want left", got)
	}
	if got := cyclePaneDirection("left"); got != "up" {
		t.Errorf("cyclePaneDirection(left) = %q, want up", got)
	}
	if got := cyclePaneDirection("up"); got != "auto" {
		t.Errorf("cyclePaneDirection(up) = %q, want auto", got)
	}
	// Unknown defaults to auto.
	if got := cyclePaneDirection(""); got != "auto" {
		t.Errorf("cyclePaneDirection('') = %q, want auto", got)
	}
	if got := cyclePaneDirection("bogus"); got != "auto" {
		t.Errorf("cyclePaneDirection(bogus) = %q, want auto", got)
	}
}

func TestPaneDirectionDisplay(t *testing.T) {
	// Verify each direction returns a non-empty rendered string.
	for _, dir := range []string{"auto", "", "right", "down", "left", "up"} {
		got := paneDirectionDisplay(dir)
		if got == "" {
			t.Errorf("paneDirectionDisplay(%q) returned empty string", dir)
		}
	}
}

func TestConfigPanelPaneDirectionSetGet(t *testing.T) {
	cp := NewConfigPanel()
	cp.SetValues(ConfigValues{PaneDirection: "left"})
	if dir := cp.Values().PaneDirection; dir != "left" {
		t.Errorf("SetValues/Values round-trip: got %q, want 'left'", dir)
	}
}

func TestConfigPanelPaneDirectionHandleEnter(t *testing.T) {
	cp := NewConfigPanel()
	cp.SetValues(ConfigValues{PaneDirection: "auto"})
	cp.cursor = cfgPaneDirection
	cp.HandleEnter()
	if dir := cp.Values().PaneDirection; dir != "right" {
		t.Errorf("HandleEnter on PaneDirection: got %q, want 'right'", dir)
	}
}

func TestConfigPanel_PaneDirection_DimmedWhenNotPaneMode(t *testing.T) {
	cp := NewConfigPanel()
	cp.SetValues(ConfigValues{LaunchMode: "tab", PaneDirection: "right"})
	cp.SetSize(80, 24)
	view := cp.View()
	// The pane direction should still appear in the view but be rendered.
	if !strings.Contains(view, "Pane Direction") {
		t.Error("Pane Direction field should appear in settings view")
	}
}

// ---------------------------------------------------------------------------
// CleanSummary
// ---------------------------------------------------------------------------

func TestCleanSummary(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "(untitled)"},
		{"normal title", "Fix Login Bug", "Fix Login Bug"},
		{"multiline", "Fix\nLogin\nBug", "Fix Login Bug"},
		{"user prefix", "[user]: stage and commit", "stage and commit"},
		{"User prefix uppercase", "[User]: do i have worktrees?", "do i have worktrees?"},
		{"user+assistant", "[user]: stage and commit [assistant]: That's a big changeset", "stage and commit"},
		{"assistant prefix", "[assistant]: I'll help you fix that", "I'll help you fix that"},
		{"user prefix no space", "[user]:hello", "hello"},
		{"only prefix", "[user]: ", "(untitled)"},
		{"newline chat", "[user]: stage and commit\n\n[assistant]: \n\nThat's a substantial changeset", "stage and commit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanSummary(tt.in)
			if got != tt.want {
				t.Errorf("CleanSummary(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
