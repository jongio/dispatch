package main

import (
	"regexp"
	"testing"
)

// wholeWordInScript reports whether token appears as a whole word in script.
// Whole-word means the token is not adjacent to another identifier
// character, including a dash, so "path" does not match "--path".
func wholeWordInScript(script, token string) bool {
	re := regexp.MustCompile(`(^|[^\w-])` + regexp.QuoteMeta(token) + `([^\w-]|$)`)
	return re.MatchString(script)
}

// TestCompletionScriptsCoverAllCommands guards against a command being added
// to cliCommands (or a subcommand to configSubcommands) without updating the
// hand-written shell completion scripts.
func TestCompletionScriptsCoverAllCommands(t *testing.T) {
	scripts := []struct {
		shell  string
		script string
	}{
		{"bash", bashCompletionScript},
		{"zsh", zshCompletionScript},
		{"fish", fishCompletionScript},
		{"powershell", powershellCompletionScript},
	}

	for _, s := range scripts {
		for _, cmd := range cliCommands {
			if !wholeWordInScript(s.script, cmd) {
				t.Errorf("%s completion script is missing command %q", s.shell, cmd)
			}
		}
	}

	// Every shell now completes config subcommands (fish gained a
	// "__dispatch_after config" line), so all four must cover them.
	for _, s := range scripts {
		for _, sub := range configSubcommands {
			if !wholeWordInScript(s.script, sub) {
				t.Errorf("%s completion script is missing config subcommand %q", s.shell, sub)
			}
		}
	}
}
