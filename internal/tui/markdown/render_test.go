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
	lines := RenderStatic("", 80)
	if len(lines) == 0 {
		t.Error("expected at least one line from empty input")
	}
}

func TestRenderStatic_ZeroWidth(t *testing.T) {
	lines := RenderStatic("# Test", 0)
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "Test") {
		t.Error("expected fallback width to still render content")
	}
}

func TestRenderStatic_List(t *testing.T) {
	md := "- item one\n- item two\n- item three"
	lines := RenderStatic(md, 60)
	joined := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "item one") || !strings.Contains(joined, "item three") {
		t.Errorf("expected list items, got: %q", joined)
	}
}
