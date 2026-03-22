package markdown

import (
	"regexp"
	"strings"
	"testing"
)

// stripANSI removes ANSI escape sequences for test assertions.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func TestRenderStatic_BasicMarkdown(t *testing.T) {
	t.Parallel()
	lines := RenderStatic("# Hello World\n\nThis is **bold** text.", 80)
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "Hello World") {
		t.Errorf("expected heading text, got: %q", joined)
	}
	if !strings.Contains(joined, "bold") {
		t.Errorf("expected bold text, got: %q", joined)
	}
}

func TestRenderStatic_FallbackOnEmpty(t *testing.T) {
	t.Parallel()
	lines := RenderStatic("", 80)
	if len(lines) == 0 {
		t.Error("expected at least one line from empty input")
	}
}

func TestRenderStatic_ZeroWidth(t *testing.T) {
	t.Parallel()
	lines := RenderStatic("# Test", 0)
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "Test") {
		t.Error("expected fallback width to still render content")
	}
}

func TestRenderStatic_List(t *testing.T) {
	t.Parallel()
	md := "- item one\n- item two\n- item three"
	lines := RenderStatic(md, 60)
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "item one") || !strings.Contains(joined, "item three") {
		t.Errorf("expected list items, got: %q", joined)
	}
}

// ---------------------------------------------------------------------------
// Additional coverage tests
// ---------------------------------------------------------------------------

func TestRenderStatic_CodeBlock(t *testing.T) {
	t.Parallel()
	md := "```go\nfmt.Println(\"hello\")\n```"
	lines := RenderStatic(md, 80)
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "Println") {
		t.Errorf("expected code content, got: %q", joined)
	}
}

func TestRenderStatic_NegativeWidth(t *testing.T) {
	t.Parallel()
	// Negative width should be treated like zero (fallback to 80)
	lines := RenderStatic("# Test", -1)
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "Test") {
		t.Error("negative width should still render content")
	}
}

func TestRenderStatic_LongContent(t *testing.T) {
	t.Parallel()
	// Content wider than the width should be wrapped
	longLine := strings.Repeat("word ", 100)
	lines := RenderStatic(longLine, 40)
	if len(lines) < 2 {
		t.Errorf("expected wrapping to produce multiple lines, got %d", len(lines))
	}
}

func TestRenderStatic_MultipleHeadings(t *testing.T) {
	t.Parallel()
	md := "# H1\n## H2\n### H3\n#### H4\n##### H5\n###### H6"
	lines := RenderStatic(md, 80)
	joined := stripANSI(strings.Join(lines, "\n"))
	for _, heading := range []string{"H1", "H2", "H3", "H4", "H5", "H6"} {
		if !strings.Contains(joined, heading) {
			t.Errorf("expected %s in output, got: %q", heading, joined)
		}
	}
}

func TestStyle_HeadingPrefixesCleared(t *testing.T) {
	t.Parallel()
	s := Style()
	if s.H2.Prefix != "" {
		t.Errorf("H2 prefix = %q, want empty", s.H2.Prefix)
	}
	if s.H3.Prefix != "" {
		t.Errorf("H3 prefix = %q, want empty", s.H3.Prefix)
	}
	if s.H4.Prefix != "" {
		t.Errorf("H4 prefix = %q, want empty", s.H4.Prefix)
	}
	if s.H5.Prefix != "" {
		t.Errorf("H5 prefix = %q, want empty", s.H5.Prefix)
	}
	if s.H6.Prefix != "" {
		t.Errorf("H6 prefix = %q, want empty", s.H6.Prefix)
	}
}

func TestRenderStatic_SpecialCharacters(t *testing.T) {
	t.Parallel()
	md := "Special chars: < > & \" ' © ® ™ — …"
	lines := RenderStatic(md, 80)
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "Special") {
		t.Error("expected special characters to render")
	}
}

func TestRenderStatic_BlockQuote(t *testing.T) {
	t.Parallel()
	md := "> This is a block quote\n> with multiple lines"
	lines := RenderStatic(md, 80)
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "block quote") {
		t.Errorf("expected block quote content, got: %q", joined)
	}
}

func TestRenderStatic_Table(t *testing.T) {
	t.Parallel()
	md := "| Col1 | Col2 |\n|------|------|\n| A    | B    |"
	lines := RenderStatic(md, 80)
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "Col1") {
		t.Errorf("expected table content, got: %q", joined)
	}
}
