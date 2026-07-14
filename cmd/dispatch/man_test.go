package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/update"
)

func TestRenderManPage_Structure(t *testing.T) {
	old := manDateFn
	manDateFn = func() string { return "2026-01-02" }
	t.Cleanup(func() { manDateFn = old })

	out := renderManPage()

	// Header line with a pinned date and a section-1 designation.
	if !strings.HasPrefix(out, ".TH DISPATCH 1 \"2026-01-02\"") {
		t.Errorf("man page should start with a .TH header, got:\n%s", firstLine(out))
	}

	wantSections := []string{
		".SH NAME",
		".SH SYNOPSIS",
		".SH DESCRIPTION",
		".SH COMMANDS",
		".SH FLAGS",
		".SH ENVIRONMENT",
		".SH EXAMPLES",
		".SH SEE ALSO",
	}
	for _, s := range wantSections {
		if !strings.Contains(out, s) {
			t.Errorf("man page missing section %q", s)
		}
	}
}

func TestRenderManPage_ListsEveryCommand(t *testing.T) {
	out := renderManPage()
	for _, c := range manCommands {
		if !strings.Contains(out, ".B "+manEscape(c.term)) {
			t.Errorf("man page missing command entry %q", c.term)
		}
	}
	// The man command documents itself.
	if !strings.Contains(out, ".B man") {
		t.Error("man page should document the man command")
	}
}

func TestRunMan_WritesToWriter(t *testing.T) {
	var buf bytes.Buffer
	if err := runMan(&buf); err != nil {
		t.Fatalf("runMan returned error: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("runMan wrote nothing")
	}
	if !strings.Contains(buf.String(), ".TH DISPATCH 1") {
		t.Error("runMan output missing .TH header")
	}
}

func TestRunMan_NilWriter(t *testing.T) {
	if err := runMan(nil); err != nil {
		t.Errorf("runMan(nil) should be a no-op, got error: %v", err)
	}
}

func TestManEscape(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain text unchanged", in: "hello world", want: "hello world"},
		{name: "backslash doubled", in: `a\b`, want: `a\\b`},
		{name: "leading dot guarded", in: ".TP", want: `\&.TP`},
		{name: "leading apostrophe guarded", in: "'quote", want: `\&'quote`},
		{name: "interior dot unchanged", in: "a.b", want: "a.b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := manEscape(tc.in); got != tc.want {
				t.Errorf("manEscape(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestHandleArgs_Man(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	done, _, _, err := handleArgs([]string{"man"}, io.Discard, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true for man")
	}
}

// firstLine returns the first line of s, for readable error messages.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
