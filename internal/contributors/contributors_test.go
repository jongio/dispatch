package contributors

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// parseGitLogOutput
// ---------------------------------------------------------------------------

func TestParseGitLogOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "single author",
			input: "Jon Gallant|jon@example.com\n",
			want:  1,
		},
		{
			name:  "multiple authors",
			input: "Jon Gallant|jon@example.com\nJane Doe|jane@example.com\n",
			want:  2,
		},
		{
			name:  "empty input",
			input: "",
			want:  0,
		},
		{
			name:  "blank lines ignored",
			input: "\nJon Gallant|jon@example.com\n\n\n",
			want:  1,
		},
		{
			name:  "missing separator skipped",
			input: "invalid line\nJon Gallant|jon@example.com\n",
			want:  1,
		},
		{
			name:  "empty name skipped",
			input: "|jon@example.com\n",
			want:  0,
		},
		{
			name:  "empty email skipped",
			input: "Jon Gallant|\n",
			want:  0,
		},
		{
			name:  "windows line endings",
			input: "Jon Gallant|jon@example.com\r\nJane Doe|jane@example.com\r\n",
			want:  2,
		},
		{
			name:  "pipe in remaining fields ignored",
			input: "Jon|jon@example.com|extra\n",
			want:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGitLogOutput(tt.input)
			if len(got) != tt.want {
				t.Errorf("parseGitLogOutput() returned %d contributors, want %d", len(got), tt.want)
			}
		})
	}
}

func TestParseGitLogOutput_Fields(t *testing.T) {
	input := "Jon Gallant|jon@example.com\n"
	got := parseGitLogOutput(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 contributor, got %d", len(got))
	}
	if got[0].Name != "Jon Gallant" {
		t.Errorf("Name = %q, want %q", got[0].Name, "Jon Gallant")
	}
	if got[0].Email != "jon@example.com" {
		t.Errorf("Email = %q, want %q", got[0].Email, "jon@example.com")
	}
	if got[0].Count != 1 {
		t.Errorf("Count = %d, want 1", got[0].Count)
	}
}

// ---------------------------------------------------------------------------
// parseCoAuthoredBy
// ---------------------------------------------------------------------------

func TestParseCoAuthoredBy(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "single co-author",
			input: "Copilot <copilot@github.com>\n",
			want:  1,
		},
		{
			name:  "multiple co-authors",
			input: "Copilot <copilot@github.com>\nJane Doe <jane@example.com>\n",
			want:  2,
		},
		{
			name:  "empty input",
			input: "",
			want:  0,
		},
		{
			name:  "blank lines ignored",
			input: "\nCopilot <copilot@github.com>\n\n\n",
			want:  1,
		},
		{
			name:  "invalid format skipped",
			input: "no angle brackets\nCopilot <copilot@github.com>\n",
			want:  1,
		},
		{
			name:  "windows line endings",
			input: "Copilot <copilot@github.com>\r\nJane <jane@x.com>\r\n",
			want:  2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCoAuthoredBy(tt.input)
			if len(got) != tt.want {
				t.Errorf("parseCoAuthoredBy() returned %d contributors, want %d", len(got), tt.want)
			}
		})
	}
}

func TestParseCoAuthoredBy_Fields(t *testing.T) {
	input := "Copilot <copilot@github.com>\n"
	got := parseCoAuthoredBy(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 contributor, got %d", len(got))
	}
	if got[0].Name != "Copilot" {
		t.Errorf("Name = %q, want %q", got[0].Name, "Copilot")
	}
	if got[0].Email != "copilot@github.com" {
		t.Errorf("Email = %q, want %q", got[0].Email, "copilot@github.com")
	}
	if got[0].Count != 1 {
		t.Errorf("Count = %d, want 1", got[0].Count)
	}
}

// ---------------------------------------------------------------------------
// extractHandle
// ---------------------------------------------------------------------------

func TestExtractHandle(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{
			name:  "numeric prefix noreply",
			email: "12345+jongio@users.noreply.github.com",
			want:  "jongio",
		},
		{
			name:  "plain noreply",
			email: "jongio@users.noreply.github.com",
			want:  "jongio",
		},
		{
			name:  "regular email",
			email: "jon@example.com",
			want:  "",
		},
		{
			name:  "github non-noreply",
			email: "copilot@github.com",
			want:  "",
		},
		{
			name:  "empty email",
			email: "",
			want:  "",
		},
		{
			name:  "not an email",
			email: "not-an-email",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHandle(tt.email)
			if got != tt.want {
				t.Errorf("extractHandle(%q) = %q, want %q", tt.email, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isBot
// ---------------------------------------------------------------------------

func TestIsBot(t *testing.T) {
	tests := []struct {
		name string
		c    Contributor
		want bool
	}{
		{
			name: "github-actions bot",
			c:    Contributor{Name: "github-actions[bot]", Email: "41898282+github-actions[bot]@users.noreply.github.com"},
			want: true,
		},
		{
			name: "dependabot",
			c:    Contributor{Name: "dependabot[bot]", Email: "49699333+dependabot[bot]@users.noreply.github.com"},
			want: true,
		},
		{
			name: "copilot-swe-agent bot",
			c:    Contributor{Name: "copilot-swe-agent[bot]", Email: "copilot-swe-agent[bot]@users.noreply.github.com"},
			want: true,
		},
		{
			name: "regular contributor",
			c:    Contributor{Name: "Jon Gallant", Email: "jon@example.com"},
			want: false,
		},
		{
			name: "copilot co-author kept",
			c:    Contributor{Name: "Copilot", Email: "copilot@github.com"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBot(tt.c)
			if got != tt.want {
				t.Errorf("isBot(%q) = %v, want %v", tt.c.Name, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// filterBots
// ---------------------------------------------------------------------------

func TestFilterBots(t *testing.T) {
	input := []Contributor{
		{Name: "Jon Gallant", Email: "jon@example.com"},
		{Name: "github-actions[bot]", Email: "bot@example.com"},
		{Name: "Copilot", Email: "copilot@github.com"},
		{Name: "dependabot[bot]", Email: "dependabot@example.com"},
	}
	got := filterBots(input)
	if len(got) != 2 {
		t.Fatalf("filterBots returned %d contributors, want 2", len(got))
	}
	if got[0].Name != "Jon Gallant" {
		t.Errorf("got[0].Name = %q, want %q", got[0].Name, "Jon Gallant")
	}
	if got[1].Name != "Copilot" {
		t.Errorf("got[1].Name = %q, want %q", got[1].Name, "Copilot")
	}
}

func TestFilterBots_EmptyInput(t *testing.T) {
	got := filterBots(nil)
	if len(got) != 0 {
		t.Errorf("filterBots(nil) returned %d, want 0", len(got))
	}
}

// ---------------------------------------------------------------------------
// mergeContributors
// ---------------------------------------------------------------------------

func TestMergeContributors_Deduplication(t *testing.T) {
	group1 := []Contributor{
		{Name: "Jon Gallant", Email: "jon@example.com", Count: 1},
		{Name: "Jon Gallant", Email: "jon@example.com", Count: 1},
	}
	group2 := []Contributor{
		{Name: "Jon Gallant", Email: "JON@EXAMPLE.COM", Count: 1},
	}
	got := mergeContributors(group1, group2)
	if len(got) != 1 {
		t.Fatalf("mergeContributors returned %d contributors, want 1", len(got))
	}
	if got[0].Count != 3 {
		t.Errorf("Count = %d, want 3", got[0].Count)
	}
}

func TestMergeContributors_PreservesOrder(t *testing.T) {
	group := []Contributor{
		{Name: "Alice", Email: "alice@example.com", Count: 1},
		{Name: "Bob", Email: "bob@example.com", Count: 1},
		{Name: "Charlie", Email: "charlie@example.com", Count: 1},
	}
	got := mergeContributors(group)
	if len(got) != 3 {
		t.Fatalf("mergeContributors returned %d contributors, want 3", len(got))
	}
	want := []string{"Alice", "Bob", "Charlie"}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("got[%d].Name = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestMergeContributors_ExtractsHandle(t *testing.T) {
	group := []Contributor{
		{Name: "Jon Gallant", Email: "12345+jongio@users.noreply.github.com", Count: 1},
	}
	got := mergeContributors(group)
	if len(got) != 1 {
		t.Fatalf("expected 1 contributor, got %d", len(got))
	}
	if got[0].Handle != "jongio" {
		t.Errorf("Handle = %q, want %q", got[0].Handle, "jongio")
	}
}

func TestMergeContributors_MultipleGroups(t *testing.T) {
	authors := []Contributor{
		{Name: "Alice", Email: "alice@example.com", Count: 1},
	}
	coAuthors := []Contributor{
		{Name: "Alice", Email: "alice@example.com", Count: 1},
		{Name: "Bob", Email: "bob@example.com", Count: 1},
	}
	got := mergeContributors(authors, coAuthors)
	if len(got) != 2 {
		t.Fatalf("expected 2 contributors, got %d", len(got))
	}
	if got[0].Name != "Alice" || got[0].Count != 2 {
		t.Errorf("Alice: Count = %d, want 2", got[0].Count)
	}
	if got[1].Name != "Bob" || got[1].Count != 1 {
		t.Errorf("Bob: Count = %d, want 1", got[1].Count)
	}
}

func TestMergeContributors_Empty(t *testing.T) {
	got := mergeContributors()
	if len(got) != 0 {
		t.Errorf("mergeContributors() returned %d, want 0", len(got))
	}
}

// ---------------------------------------------------------------------------
// DetectFirstTime
// ---------------------------------------------------------------------------

func TestDetectFirstTime(t *testing.T) {
	all := []Contributor{
		{Name: "Alice", Email: "alice@example.com"},
		{Name: "Bob", Email: "bob@example.com"},
	}
	release := []Contributor{
		{Name: "Alice", Email: "alice@example.com"},
		{Name: "Charlie", Email: "charlie@example.com"},
	}
	got := DetectFirstTime(all, release)
	if len(got) != 1 {
		t.Fatalf("DetectFirstTime returned %d contributors, want 1", len(got))
	}
	if got[0].Name != "Charlie" {
		t.Errorf("got[0].Name = %q, want %q", got[0].Name, "Charlie")
	}
}

func TestDetectFirstTime_CaseInsensitive(t *testing.T) {
	all := []Contributor{
		{Name: "Alice", Email: "ALICE@EXAMPLE.COM"},
	}
	release := []Contributor{
		{Name: "Alice", Email: "alice@example.com"},
	}
	got := DetectFirstTime(all, release)
	if len(got) != 0 {
		t.Errorf("DetectFirstTime returned %d, want 0 (case-insensitive match)", len(got))
	}
}

func TestDetectFirstTime_EmptyAll(t *testing.T) {
	release := []Contributor{
		{Name: "Alice", Email: "alice@example.com"},
	}
	got := DetectFirstTime(nil, release)
	if len(got) != 1 {
		t.Errorf("DetectFirstTime with nil all returned %d, want 1", len(got))
	}
}

func TestDetectFirstTime_EmptyRelease(t *testing.T) {
	all := []Contributor{
		{Name: "Alice", Email: "alice@example.com"},
	}
	got := DetectFirstTime(all, nil)
	if len(got) != 0 {
		t.Errorf("DetectFirstTime with nil release returned %d, want 0", len(got))
	}
}

func TestDetectFirstTime_BothEmpty(t *testing.T) {
	got := DetectFirstTime(nil, nil)
	if len(got) != 0 {
		t.Errorf("DetectFirstTime(nil, nil) returned %d, want 0", len(got))
	}
}

// ---------------------------------------------------------------------------
// FormatMarkdown
// ---------------------------------------------------------------------------

func TestFormatMarkdown(t *testing.T) {
	contribs := []Contributor{
		{Name: "Bob", Handle: "bob"},
		{Name: "Alice", Handle: "alice"},
	}
	firstTimers := []Contributor{
		{Name: "Alice", Handle: "alice"},
	}
	got := FormatMarkdown(contribs, firstTimers)

	if !strings.Contains(got, "### Contributors") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "Thanks to the following people") {
		t.Error("missing intro text")
	}

	// Sorted order: Alice before Bob.
	aliceIdx := strings.Index(got, "**Alice**")
	bobIdx := strings.Index(got, "**Bob**")
	if aliceIdx < 0 || bobIdx < 0 {
		t.Fatal("missing contributor names")
	}
	if aliceIdx > bobIdx {
		t.Error("contributors should be sorted by name (Alice before Bob)")
	}

	if !strings.Contains(got, "(@alice)") {
		t.Error("missing Alice's handle")
	}
	if !strings.Contains(got, "(@bob)") {
		t.Error("missing Bob's handle")
	}

	if !strings.Contains(got, "New contributors:") {
		t.Error("missing new contributors line")
	}
	if !strings.Contains(got, "-- welcome!") {
		t.Error("missing welcome text")
	}
}

func TestFormatMarkdown_Empty(t *testing.T) {
	got := FormatMarkdown(nil, nil)
	if got != "" {
		t.Errorf("FormatMarkdown(nil, nil) = %q, want empty", got)
	}
}

func TestFormatMarkdown_NoHandle(t *testing.T) {
	contribs := []Contributor{
		{Name: "Jon Gallant"},
	}
	got := FormatMarkdown(contribs, nil)
	if strings.Contains(got, "(@") {
		t.Error("should not contain handle syntax when handle is empty")
	}
	if !strings.Contains(got, "**Jon Gallant**") {
		t.Error("should contain contributor name")
	}
}

func TestFormatMarkdown_NoFirstTimers(t *testing.T) {
	contribs := []Contributor{
		{Name: "Alice"},
	}
	got := FormatMarkdown(contribs, nil)
	if strings.Contains(got, "New contributors:") {
		t.Error("should not have new contributors section when there are none")
	}
}

// ---------------------------------------------------------------------------
// FormatContributorsFile
// ---------------------------------------------------------------------------

func TestFormatContributorsFile(t *testing.T) {
	contribs := []Contributor{
		{Name: "Alice", Handle: "alice", Count: 5},
		{Name: "Bob", Handle: "bob", Count: 10},
		{Name: "Charlie", Count: 1},
	}
	got := FormatContributorsFile(contribs)

	if !strings.Contains(got, "# Contributors") {
		t.Error("missing main heading")
	}
	if !strings.Contains(got, "## Contributors") {
		t.Error("missing sub-heading")
	}
	if !strings.Contains(got, "mage contributors") {
		t.Error("missing auto-generation note")
	}

	// Sorted by count descending: Bob (10), Alice (5), Charlie (1).
	bobIdx := strings.Index(got, "**Bob**")
	aliceIdx := strings.Index(got, "**Alice**")
	charlieIdx := strings.Index(got, "**Charlie**")
	if bobIdx < 0 || aliceIdx < 0 || charlieIdx < 0 {
		t.Fatal("missing contributor names")
	}
	if bobIdx > aliceIdx {
		t.Error("Bob (10) should appear before Alice (5)")
	}
	if aliceIdx > charlieIdx {
		t.Error("Alice (5) should appear before Charlie (1)")
	}

	if !strings.Contains(got, "-- 10 contributions") {
		t.Error("missing Bob's contribution count")
	}
	if !strings.Contains(got, "-- 5 contributions") {
		t.Error("missing Alice's contribution count")
	}
	if !strings.Contains(got, "-- 1 contribution\n") {
		t.Error("missing singular contribution for Charlie")
	}
	if strings.Contains(got, "-- 1 contributions") {
		t.Error("should use singular 'contribution' for count of 1")
	}
}

func TestFormatContributorsFile_Empty(t *testing.T) {
	got := FormatContributorsFile(nil)
	if !strings.Contains(got, "# Contributors") {
		t.Error("missing heading even with empty contributors")
	}
	if strings.Contains(got, "- **") {
		t.Error("should not contain contributor entries when empty")
	}
}

func TestFormatContributorsFile_SameCountSortedByName(t *testing.T) {
	contribs := []Contributor{
		{Name: "Charlie", Email: "c@x.com", Count: 5},
		{Name: "Alice", Email: "a@x.com", Count: 5},
		{Name: "Bob", Email: "b@x.com", Count: 5},
	}
	got := FormatContributorsFile(contribs)

	aliceIdx := strings.Index(got, "**Alice**")
	bobIdx := strings.Index(got, "**Bob**")
	charlieIdx := strings.Index(got, "**Charlie**")
	if aliceIdx > bobIdx || bobIdx > charlieIdx {
		t.Errorf("same-count contributors should be sorted by name: Alice=%d Bob=%d Charlie=%d",
			aliceIdx, bobIdx, charlieIdx)
	}
}

// ---------------------------------------------------------------------------
// formatEntry
// ---------------------------------------------------------------------------

func TestFormatEntry(t *testing.T) {
	tests := []struct {
		name string
		c    Contributor
		want string
	}{
		{
			name: "with handle",
			c:    Contributor{Name: "Jon Gallant", Handle: "jongio"},
			want: "**Jon Gallant** (@jongio)",
		},
		{
			name: "without handle",
			c:    Contributor{Name: "Jon Gallant"},
			want: "**Jon Gallant**",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatEntry(tt.c)
			if got != tt.want {
				t.Errorf("formatEntry() = %q, want %q", got, tt.want)
			}
		})
	}
}
