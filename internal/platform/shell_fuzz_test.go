package platform

import (
	"strings"
	"testing"
)

func FuzzShellQuoteRoundTrip(f *testing.F) {
	for _, seed := range []string{
		"copilot",
		"space separated",
		"single'quote",
		`double"quote`,
		"$HOME; rm -rf /",
		"nul\x00suffix",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		sanitized := strings.ReplaceAll(input, "\x00", "")
		quoted := shellQuote(input)
		if strings.ContainsRune(quoted, '\x00') {
			t.Fatalf("shellQuote(%q) retained a null byte", input)
		}

		roundTripped, ok := decodePOSIXSingleQuote(quoted)
		if !ok {
			if quoted != sanitized {
				t.Fatalf("unquoted shell value = %q, want %q", quoted, sanitized)
			}
			return
		}
		if roundTripped != sanitized {
			t.Fatalf("decoded shell quote = %q, want %q", roundTripped, sanitized)
		}
	})
}

func FuzzCmdQuoteEscapesExpansionAndNulls(f *testing.F) {
	for _, seed := range []string{"", "plain", `%PATH%`, `say "hello"`, "nul\x00suffix"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		quoted := cmdQuote(input)
		if !strings.HasPrefix(quoted, `"`) || !strings.HasSuffix(quoted, `"`) {
			t.Fatalf("cmdQuote(%q) = %q, want surrounding quotes", input, quoted)
		}
		if strings.ContainsRune(quoted, '\x00') {
			t.Fatalf("cmdQuote(%q) retained a null byte", input)
		}
		if strings.Contains(quoted, `%`) && strings.Count(quoted, `%`)%2 != 0 {
			t.Fatalf("cmdQuote(%q) left an unmatched percent in %q", input, quoted)
		}
	})
}

func decodePOSIXSingleQuote(value string) (string, bool) {
	if len(value) < 2 || value[0] != '\'' || value[len(value)-1] != '\'' {
		return "", false
	}
	inner := value[1 : len(value)-1]
	return strings.ReplaceAll(inner, `'\''`, `'`), true
}
