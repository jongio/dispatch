package contributors

import (
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// parseGitLogOutput
// ---------------------------------------------------------------------------

func TestParseGitLogOutput(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			got := parseGitLogOutput(tt.input)
			if len(got) != tt.want {
				t.Errorf("parseGitLogOutput() returned %d contributors, want %d", len(got), tt.want)
			}
		})
	}
}

func TestParseGitLogOutput_Fields(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
			got := parseCoAuthoredBy(tt.input)
			if len(got) != tt.want {
				t.Errorf("parseCoAuthoredBy() returned %d contributors, want %d", len(got), tt.want)
			}
		})
	}
}

func TestParseCoAuthoredBy_Fields(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	got := filterBots(nil)
	if len(got) != 0 {
		t.Errorf("filterBots(nil) returned %d, want 0", len(got))
	}
}

// ---------------------------------------------------------------------------
// mergeContributors
// ---------------------------------------------------------------------------

func TestMergeContributors_Deduplication(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	got := mergeContributors()
	if len(got) != 0 {
		t.Errorf("mergeContributors() returned %d, want 0", len(got))
	}
}

// ---------------------------------------------------------------------------
// DetectFirstTime
// ---------------------------------------------------------------------------

func TestDetectFirstTime(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	release := []Contributor{
		{Name: "Alice", Email: "alice@example.com"},
	}
	got := DetectFirstTime(nil, release)
	if len(got) != 1 {
		t.Errorf("DetectFirstTime with nil all returned %d, want 1", len(got))
	}
}

func TestDetectFirstTime_EmptyRelease(t *testing.T) {
	t.Parallel()
	all := []Contributor{
		{Name: "Alice", Email: "alice@example.com"},
	}
	got := DetectFirstTime(all, nil)
	if len(got) != 0 {
		t.Errorf("DetectFirstTime with nil release returned %d, want 0", len(got))
	}
}

func TestDetectFirstTime_BothEmpty(t *testing.T) {
	t.Parallel()
	got := DetectFirstTime(nil, nil)
	if len(got) != 0 {
		t.Errorf("DetectFirstTime(nil, nil) returned %d, want 0", len(got))
	}
}

// ---------------------------------------------------------------------------
// FormatMarkdown
// ---------------------------------------------------------------------------

func TestFormatMarkdown(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	got := FormatMarkdown(nil, nil)
	if got != "" {
		t.Errorf("FormatMarkdown(nil, nil) = %q, want empty", got)
	}
}

func TestFormatMarkdown_NoHandle(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	got := FormatContributorsFile(nil)
	if !strings.Contains(got, "# Contributors") {
		t.Error("missing heading even with empty contributors")
	}
	if strings.Contains(got, "- **") {
		t.Error("should not contain contributor entries when empty")
	}
}

func TestFormatContributorsFile_SameCountSortedByName(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
			got := formatEntry(tt.c)
			if got != tt.want {
				t.Errorf("formatEntry() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ===========================================================================
// Security Tests
// ===========================================================================

// ---------------------------------------------------------------------------
// SEC-00: validateRef — git flag injection prevention (CWE-88)
// ---------------------------------------------------------------------------

func TestValidateRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		{name: "valid tag", ref: "v1.0.0", wantErr: false},
		{name: "valid branch", ref: "main", wantErr: false},
		{name: "valid slash ref", ref: "release/2.0", wantErr: false},
		{name: "valid SHA prefix", ref: "abc123", wantErr: false},
		{name: "empty ref allowed", ref: "", wantErr: false},
		{name: "single dash", ref: "-", wantErr: true},
		{name: "double dash flag", ref: "--exec=malicious", wantErr: true},
		{name: "dash option", ref: "-n", wantErr: true},
		{name: "dash-dash", ref: "--", wantErr: true},
		{name: "flag with value", ref: "--upload-pack=evil", wantErr: true},
		{name: "tilde ref", ref: "HEAD~1", wantErr: false},
		{name: "caret ref", ref: "v1.0^{}", wantErr: false},
		{name: "ref with dots", ref: "v1.0.0..v2.0.0", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateRef(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRef(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SEC-01: Malformed git output — command injection payloads, binary, edge cases
// ---------------------------------------------------------------------------

func TestParseGitLogOutput_SecurityMalformed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "only pipe separator",
			input: "|\n",
			want:  0,
		},
		{
			name:  "null bytes in name",
			input: "John\x00Doe|john@example.com\n",
			want:  1,
		},
		{
			name:  "tab characters in name",
			input: "John\tDoe|john@example.com\n",
			want:  1,
		},
		{
			name:  "whitespace-only name",
			input: "   |john@example.com\n",
			want:  0,
		},
		{
			name:  "whitespace-only email",
			input: "John|   \n",
			want:  0,
		},
		{
			name:  "shell metacharacters in name",
			input: "$(whoami)|evil@example.com\n",
			want:  1,
		},
		{
			name:  "semicolons and pipes in name",
			input: "foo;bar&&baz|evil@example.com\n",
			want:  1,
		},
		{
			name:  "backtick injection in name",
			input: "`rm -rf /`|evil@example.com\n",
			want:  1,
		},
		{
			name:  "git flag injection in name",
			input: "--exec=malicious|evil@example.com\n",
			want:  1,
		},
		{
			name:  "thousands of blank lines",
			input: strings.Repeat("\n", 10000),
			want:  0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseGitLogOutput(tt.input)
			if len(got) != tt.want {
				t.Errorf("parseGitLogOutput() returned %d contributors, want %d", len(got), tt.want)
			}
		})
	}
}

func TestParseGitLogOutput_ShellInjectionPreserved(t *testing.T) {
	t.Parallel()
	// Verify shell metacharacter payloads are stored verbatim, not executed.
	// The parse function should not interpret these — they are just strings.
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantEmail string
	}{
		{
			name:      "dollar sign command substitution",
			input:     "$(whoami)|$(id)@evil.com\n",
			wantName:  "$(whoami)",
			wantEmail: "$(id)@evil.com",
		},
		{
			name:      "backtick command substitution",
			input:     "`id`|`whoami`@evil.com\n",
			wantName:  "`id`",
			wantEmail: "`whoami`@evil.com",
		},
		{
			name:      "pipe and redirect",
			input:     "name;cat /etc/passwd|email>&2@x.com\n",
			wantName:  "name;cat /etc/passwd",
			wantEmail: "email>&2@x.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseGitLogOutput(tt.input)
			if len(got) != 1 {
				t.Fatalf("expected 1 contributor, got %d", len(got))
			}
			if got[0].Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got[0].Name, tt.wantName)
			}
			if got[0].Email != tt.wantEmail {
				t.Errorf("Email = %q, want %q", got[0].Email, tt.wantEmail)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SEC-02: Bot filtering robustness — partial suffix, case, edge cases
// ---------------------------------------------------------------------------

func TestIsBot_SecurityEdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		c    Contributor
		want bool
	}{
		{
			name: "partial suffix [bo]",
			c:    Contributor{Name: "something[bo]"},
			want: false,
		},
		{
			name: "partial suffix bot] no open bracket",
			c:    Contributor{Name: "somethingbot]"},
			want: false,
		},
		{
			name: "open bracket without close [bot",
			c:    Contributor{Name: "something[bot"},
			want: false,
		},
		{
			name: "uppercase [BOT]",
			c:    Contributor{Name: "mybot[BOT]"},
			want: false, // HasSuffix is case-sensitive
		},
		{
			name: "mixed case [Bot]",
			c:    Contributor{Name: "mybot[Bot]"},
			want: false,
		},
		{
			name: "bot in middle of name",
			c:    Contributor{Name: "my[bot]name"},
			want: false,
		},
		{
			name: "just the suffix alone",
			c:    Contributor{Name: "[bot]"},
			want: true,
		},
		{
			name: "empty name",
			c:    Contributor{Name: ""},
			want: false,
		},
		{
			name: "spaces before suffix",
			c:    Contributor{Name: "bot name [bot]"},
			want: true,
		},
		{
			name: "similar ending robot",
			c:    Contributor{Name: "robot"},
			want: false,
		},
		{
			name: "double bot suffix",
			c:    Contributor{Name: "[bot][bot]"},
			want: true,
		},
		{
			name: "trailing whitespace after suffix",
			c:    Contributor{Name: "name[bot] "},
			want: false, // trailing space means suffix doesn't match
		},
		{
			name: "unicode lookalike brackets",
			c:    Contributor{Name: "name\uff3bbot\uff3d"},
			want: false, // fullwidth brackets are not ASCII [bot]
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isBot(tt.c)
			if got != tt.want {
				t.Errorf("isBot(%q) = %v, want %v", tt.c.Name, got, tt.want)
			}
		})
	}
}

func TestFilterBots_CaseSensitivity(t *testing.T) {
	t.Parallel()
	// Verify that [BOT] and [Bot] variants are NOT filtered out.
	// This documents current behavior: only exact lowercase [bot] is filtered.
	input := []Contributor{
		{Name: "real[bot]", Email: "a@x.com"},
		{Name: "fake[BOT]", Email: "b@x.com"},
		{Name: "fake[Bot]", Email: "c@x.com"},
	}
	got := filterBots(input)
	if len(got) != 2 {
		t.Fatalf("filterBots returned %d, want 2 (only exact [bot] filtered)", len(got))
	}
	if got[0].Name != "fake[BOT]" {
		t.Errorf("got[0].Name = %q, want %q", got[0].Name, "fake[BOT]")
	}
	if got[1].Name != "fake[Bot]" {
		t.Errorf("got[1].Name = %q, want %q", got[1].Name, "fake[Bot]")
	}
}

// ---------------------------------------------------------------------------
// SEC-03: Regex edge cases — ReDoS, boundary conditions, crafted emails
// ---------------------------------------------------------------------------

func TestExtractHandle_SecurityEdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{
			name:  "very long username in noreply",
			email: strings.Repeat("a", 10000) + "@users.noreply.github.com",
			want:  strings.Repeat("a", 10000),
		},
		{
			name:  "special chars in username",
			email: "user+name@users.noreply.github.com",
			want:  "user+name",
		},
		{
			name:  "dots in username",
			email: "user.name@users.noreply.github.com",
			want:  "user.name",
		},
		{
			name:  "uppercase domain not matched",
			email: "user@users.noreply.github.COM",
			want:  "", // regex is case-sensitive, .COM != .com
		},
		{
			name:  "nested noreply domain",
			email: "12345+user@users.noreply.github.com@users.noreply.github.com",
			want:  "", // extra @ breaks the pattern
		},
		{
			name:  "unicode in local part",
			email: "\u00e9\u00e8@users.noreply.github.com",
			want:  "\u00e9\u00e8",
		},
		{
			name:  "empty local part",
			email: "@users.noreply.github.com",
			want:  "", // [^@]+ requires at least one char
		},
		{
			name:  "numeric prefix with empty username after plus",
			email: "12345+@users.noreply.github.com",
			want:  "12345+", // optional group doesn't match, [^@]+ captures 12345+
		},
		{
			name:  "ReDoS attempt long non-matching string",
			email: strings.Repeat("a", 100000) + "@other.domain.com",
			want:  "",
		},
		{
			name:  "subdomain injection",
			email: "user@evil.users.noreply.github.com",
			want:  "", // [^@]+ captures user, but @evil.users... doesn't match @users.
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractHandle(tt.email)
			if got != tt.want {
				t.Errorf("extractHandle(%q) = %q, want %q", tt.email, got, tt.want)
			}
		})
	}
}

func TestParseCoAuthoredBy_SecurityEdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "nested angle brackets",
			input: "Name <<nested>>@email.com>\n",
			want:  0, // no valid <email> split due to embedded >
		},
		{
			name:  "empty name with angle brackets",
			input: " <email@example.com>\n",
			want:  0, // after trim starts with <, no name captured
		},
		{
			name:  "null bytes in name",
			input: "Na\x00me <null@example.com>\n",
			want:  1,
		},
		{
			name:  "shell metacharacters in email",
			input: "Name <$(whoami)@evil.com>\n",
			want:  1,
		},
		{
			name:  "markdown injection in name",
			input: "**bold** <inject@example.com>\n",
			want:  1,
		},
		{
			name:  "thousands of empty lines",
			input: strings.Repeat("\n", 10000),
			want:  0,
		},
		{
			name:  "ReDoS attempt — long name no brackets",
			input: strings.Repeat("a ", 50000) + "\n",
			want:  0,
		},
		{
			name:  "multiple angle bracket pairs",
			input: "Name <a> <b@c.com>\n",
			want:  1, // regex captures Name <a> as name, b@c.com as email
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseCoAuthoredBy(tt.input)
			if len(got) != tt.want {
				t.Errorf("parseCoAuthoredBy() returned %d contributors, want %d", len(got), tt.want)
			}
		})
	}
}

func TestParseCoAuthoredBy_ShellPayloadsStoredVerbatim(t *testing.T) {
	t.Parallel()
	input := "Name <$(whoami)@evil.com>\n"
	got := parseCoAuthoredBy(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 contributor, got %d", len(got))
	}
	if got[0].Email != "$(whoami)@evil.com" {
		t.Errorf("Email = %q, want verbatim %q", got[0].Email, "$(whoami)@evil.com")
	}
}

// ---------------------------------------------------------------------------
// SEC-04: Unicode handling — CJK, emoji, RTL, combining chars, zero-width
// ---------------------------------------------------------------------------

func TestParseGitLogOutput_Unicode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantName  string
	}{
		{
			name:      "CJK characters",
			input:     "\u5f20\u4e09|zhang@example.com\n",
			wantCount: 1,
			wantName:  "\u5f20\u4e09",
		},
		{
			name:      "emoji in name",
			input:     "\U0001f680 Rocket|rocket@example.com\n",
			wantCount: 1,
			wantName:  "\U0001f680 Rocket",
		},
		{
			name:      "RTL Arabic characters",
			input:     "\u0645\u062d\u0645\u062f|arabic@example.com\n",
			wantCount: 1,
			wantName:  "\u0645\u062d\u0645\u062f",
		},
		{
			name:      "combining diacritics",
			input:     "Jose\u0301|jose@example.com\n",
			wantCount: 1,
			wantName:  "Jose\u0301",
		},
		{
			name:      "zero-width joiner in name",
			input:     "John\u200bDoe|john@example.com\n",
			wantCount: 1,
			wantName:  "John\u200bDoe",
		},
		{
			name:      "mixed scripts",
			input:     "\u0410lice \u5f20|mixed@example.com\n",
			wantCount: 1,
			wantName:  "\u0410lice \u5f20",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseGitLogOutput(tt.input)
			if len(got) != tt.wantCount {
				t.Fatalf("parseGitLogOutput() returned %d contributors, want %d", len(got), tt.wantCount)
			}
			if tt.wantCount > 0 && got[0].Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got[0].Name, tt.wantName)
			}
		})
	}
}

func TestFormatMarkdown_Unicode(t *testing.T) {
	t.Parallel()
	contribs := []Contributor{
		{Name: "\u5f20\u4e09", Handle: "zhangsan"},
		{Name: "\U0001f680 Rocket"},
	}
	got := FormatMarkdown(contribs, nil)
	if !strings.Contains(got, "**\u5f20\u4e09**") {
		t.Error("missing CJK contributor name in output")
	}
	if !strings.Contains(got, "**\U0001f680 Rocket**") {
		t.Error("missing emoji contributor name in output")
	}
}

func TestFormatContributorsFile_Unicode(t *testing.T) {
	t.Parallel()
	contribs := []Contributor{
		{Name: "\u5f20\u4e09", Count: 5},
		{Name: "Jose\u0301", Count: 3},
	}
	got := FormatContributorsFile(contribs)
	if !strings.Contains(got, "**\u5f20\u4e09**") {
		t.Error("missing CJK name")
	}
	if !strings.Contains(got, "**Jose\u0301**") {
		t.Error("missing combining diacritics name")
	}
}

// ---------------------------------------------------------------------------
// SEC-05: Empty/nil inputs — comprehensive cross-function coverage
// ---------------------------------------------------------------------------

func TestMergeContributors_SecurityNilVariants(t *testing.T) {
	t.Parallel()
	// Multiple nil groups must not panic.
	got := mergeContributors(nil, nil, nil)
	if len(got) != 0 {
		t.Errorf("mergeContributors(nil, nil, nil) returned %d, want 0", len(got))
	}

	// Mixed empty and populated must work.
	got = mergeContributors(
		[]Contributor{},
		[]Contributor{{Name: "Alice", Email: "a@x.com", Count: 1}},
		nil,
	)
	if len(got) != 1 {
		t.Errorf("mixed nil/empty returned %d, want 1", len(got))
	}
}

func TestDetectFirstTime_AllSameContributors(t *testing.T) {
	t.Parallel()
	// When release has identical contributors as all-time, no first-timers.
	contribs := []Contributor{
		{Name: "Alice", Email: "alice@example.com"},
		{Name: "Bob", Email: "bob@example.com"},
	}
	got := DetectFirstTime(contribs, contribs)
	if len(got) != 0 {
		t.Errorf("all same contributors should produce 0 first-timers, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// SEC-06: Oversized input — performance and no panics under load
// ---------------------------------------------------------------------------

func TestParseGitLogOutput_OversizedInput(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	const lineCount = 100000
	for i := 0; i < lineCount; i++ {
		fmt.Fprintf(&b, "User%d|user%d@example.com\n", i, i)
	}
	got := parseGitLogOutput(b.String())
	if len(got) != lineCount {
		t.Errorf("parseGitLogOutput with %d lines returned %d, want %d", lineCount, len(got), lineCount)
	}
}

func TestParseCoAuthoredBy_OversizedInput(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	const lineCount = 100000
	for i := 0; i < lineCount; i++ {
		fmt.Fprintf(&b, "User%d <user%d@example.com>\n", i, i)
	}
	got := parseCoAuthoredBy(b.String())
	if len(got) != lineCount {
		t.Errorf("parseCoAuthoredBy with %d lines returned %d, want %d", lineCount, len(got), lineCount)
	}
}

func TestParseGitLogOutput_VeryLongFields(t *testing.T) {
	t.Parallel()
	longName := strings.Repeat("A", 10000)
	longEmail := strings.Repeat("a", 10000) + "@example.com"
	input := longName + "|" + longEmail + "\n"
	got := parseGitLogOutput(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 contributor, got %d", len(got))
	}
	if got[0].Name != longName {
		t.Errorf("Name length = %d, want %d", len(got[0].Name), len(longName))
	}
	if got[0].Email != longEmail {
		t.Errorf("Email length = %d, want %d", len(got[0].Email), len(longEmail))
	}
}

func TestMergeContributors_LargeDedup(t *testing.T) {
	t.Parallel()
	// 10000 entries for the same email should collapse into one with count 10000.
	group := make([]Contributor, 10000)
	for i := range group {
		group[i] = Contributor{Name: "Same Person", Email: "same@example.com", Count: 1}
	}
	got := mergeContributors(group)
	if len(got) != 1 {
		t.Fatalf("expected 1 deduplicated contributor, got %d", len(got))
	}
	if got[0].Count != 10000 {
		t.Errorf("Count = %d, want 10000", got[0].Count)
	}
}

// ---------------------------------------------------------------------------
// SEC-07: Markdown injection — names that could break output formatting
// ---------------------------------------------------------------------------

func TestFormatMarkdown_MarkdownInjection(t *testing.T) {
	t.Parallel()
	contribs := []Contributor{
		{Name: "**bold**", Handle: "hacker"},
		{Name: "Name with `backticks`"},
		{Name: "Name <script>alert('xss')</script>"},
		{Name: "Name with [link](http://evil.com)"},
	}
	got := FormatMarkdown(contribs, nil)
	// Function must not panic and must produce valid output.
	if got == "" {
		t.Error("FormatMarkdown should produce output for non-empty contributors")
	}
	if !strings.Contains(got, "### Contributors") {
		t.Error("missing heading")
	}
	// sanitizeMD strips *, [], (), <>, backticks. Verify sanitized names appear.
	sanitizedNames := []string{
		"bold",                          // **bold** -> bold
		"Name with backticks",           // backticks stripped
		"Name scriptalert'xss'/script",  // < > ( ) stripped
		"Name with linkhttp://evil.com", // [ ] ( ) stripped
	}
	for _, name := range sanitizedNames {
		if !strings.Contains(got, "**"+name+"**") {
			t.Errorf("missing sanitized contributor name %q in output", name)
		}
	}
}

func TestFormatContributorsFile_MarkdownInjection(t *testing.T) {
	t.Parallel()
	contribs := []Contributor{
		{Name: "**bold**", Handle: "hacker", Count: 5},
		{Name: "Name with\nnewline", Count: 3},
		{Name: "-- 999 contributions", Count: 1},
	}
	got := FormatContributorsFile(contribs)
	if got == "" {
		t.Error("FormatContributorsFile should produce output")
	}
	if !strings.Contains(got, "# Contributors") {
		t.Error("missing heading")
	}
}

func TestFormatEntry_MarkdownInjection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		c    Contributor
		want string
	}{
		{
			name: "name with bold markers sanitized",
			c:    Contributor{Name: "**bold**"},
			want: "**bold**", // sanitizeMD strips *, "**bold**" -> "bold", then "**"+"bold"+"**"
		},
		{
			name: "name with parentheses sanitized",
			c:    Contributor{Name: "Name (@fake)"},
			want: "**Name @fake**", // ( ) stripped by sanitizeMD
		},
		{
			name: "handle with special chars preserved",
			c:    Contributor{Name: "Name", Handle: "user;rm -rf"},
			want: "**Name** (@user;rm -rf)", // semicolons not stripped by sanitizeMD
		},
		{
			name: "XSS in name stripped",
			c:    Contributor{Name: "<img src=x onerror=alert(1)>"},
			want: "**img src=x onerror=alert1**", // < > ( ) stripped
		},
		{
			name: "newline in name replaced with space",
			c:    Contributor{Name: "Name\nwith\nnewlines"},
			want: "**Name with newlines**",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatEntry(tt.c)
			if got != tt.want {
				t.Errorf("formatEntry() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SEC-07b: Extended sanitizeMD — tilde, backslash, pipe stripping
// ---------------------------------------------------------------------------

func TestSanitizeMD_ExtendedChars(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "tilde strikethrough stripped",
			input: "~~deleted~~ text",
			want:  "deleted text",
		},
		{
			name:  "backslash escape stripped",
			input: `Name\with\backslashes`,
			want:  "Namewithbackslashes",
		},
		{
			name:  "pipe table separator stripped",
			input: "cell1 | cell2 | cell3",
			want:  "cell1  cell2  cell3",
		},
		{
			name:  "all extended chars combined",
			input: "~~strike~~ \\escape | pipe",
			want:  "strike escape  pipe",
		},
		{
			name:  "backslash at end of name",
			input: `Name\`,
			want:  "Name",
		},
		{
			name:  "multiple tildes (GFM strikethrough)",
			input: "~~~code~~~ text",
			want:  "code text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeMD(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeMD(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatEntry_ExtendedInjection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		c    Contributor
		want string
	}{
		{
			name: "strikethrough in name stripped",
			c:    Contributor{Name: "~~crossed-out~~"},
			want: "**crossed-out**",
		},
		{
			name: "backslash breaking bold markers stripped",
			c:    Contributor{Name: `\`},
			want: "****",
		},
		{
			name: "pipe injection in name stripped",
			c:    Contributor{Name: "Name | fake-column"},
			want: "**Name  fake-column**",
		},
		{
			name: "handle with tilde stripped",
			c:    Contributor{Name: "Name", Handle: "user~~strike"},
			want: "**Name** (@userstrike)",
		},
		{
			name: "combined extended attack payload",
			c:    Contributor{Name: "~~evil~~ \\escape | table"},
			want: "**evil escape  table**",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatEntry(tt.c)
			if got != tt.want {
				t.Errorf("formatEntry() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SEC-08: CoAuthoredBy regex captures validated correctly
// ---------------------------------------------------------------------------

func TestParseCoAuthoredBy_FieldVerification(t *testing.T) {
	t.Parallel()
	// Verify that fields are captured and trimmed correctly for edge-case inputs.
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantEmail string
	}{
		{
			name:      "extra whitespace around name",
			input:     "  Jon Gallant   <jon@example.com>\n",
			wantName:  "Jon Gallant",
			wantEmail: "jon@example.com",
		},
		{
			name:      "email with plus addressing",
			input:     "Name <user+tag@example.com>\n",
			wantName:  "Name",
			wantEmail: "user+tag@example.com",
		},
		{
			name:      "name with numbers",
			input:     "User123 <user123@example.com>\n",
			wantName:  "User123",
			wantEmail: "user123@example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseCoAuthoredBy(tt.input)
			if len(got) != 1 {
				t.Fatalf("expected 1 contributor, got %d", len(got))
			}
			if got[0].Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got[0].Name, tt.wantName)
			}
			if got[0].Email != tt.wantEmail {
				t.Errorf("Email = %q, want %q", got[0].Email, tt.wantEmail)
			}
		})
	}
}
