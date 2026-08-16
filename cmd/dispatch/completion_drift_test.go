package main

import (
	"regexp"
	"strings"
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

func TestCompletionScriptsCoverResumeFlags(t *testing.T) {
	scripts := []struct {
		shell string
		start string
		end   string
	}{
		{"bash", "    resume)", "    config)"},
		{"zsh", "  resumeflags=(", "  flags=("},
		{"fish", "  complete -c $bin -n '__dispatch_after resume'", "\n"},
		{"powershell", "$script:DispatchResumeFlags = @(", "\n"},
	}
	flags := []string{
		"--json", "--jsonl", "--ids", "--paths", "--commands", "--csv",
		"--table", "--format", "-q", "--query", "--deep", "--repo", "--repository", "--branch",
		"--folder", "--tag", "--host", "--since", "--until", "--sort",
		"--order", "-n", "--limit",
	}

	allScripts := map[string]string{
		"bash":       bashCompletionScript,
		"zsh":        zshCompletionScript,
		"fish":       fishCompletionScript,
		"powershell": powershellCompletionScript,
	}
	for _, script := range scripts {
		content := allScripts[script.shell]
		start := strings.Index(content, script.start)
		if start < 0 {
			t.Fatalf("%s completion script is missing resume section", script.shell)
		}
		content = content[start+len(script.start):]
		end := strings.Index(content, script.end)
		if end < 0 {
			t.Fatalf("%s completion script resume section has no terminator", script.shell)
		}
		resumeSection := content[:end]
		for _, flag := range flags {
			if !wholeWordInScript(resumeSection, flag) {
				t.Errorf("%s completion script is missing resume flag %q", script.shell, flag)
			}
		}
	}
}

func TestCompletionScriptsCoverOpenScopeFlags(t *testing.T) {
	for _, tt := range []struct {
		name   string
		script string
	}{
		{name: "bash", script: bashCompletionScript},
		{name: "zsh", script: zshCompletionScript},
		{name: "fish", script: fishCompletionScript},
		{name: "powershell", script: powershellCompletionScript},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, flag := range []string{"--repo", "--branch", "--folder", "--current"} {
				if !wholeWordInScript(tt.script, flag) {
					t.Errorf("completion script is missing open scope flag %q", flag)
				}
			}
		})
	}
}

func TestCompletionScriptsCoverGlobalRouting(t *testing.T) {
	tests := []struct {
		shell     string
		script    string
		demo      string
		separator string
	}{
		{"bash", bashCompletionScript, `[[ "${COMP_WORDS[1]}" == "--demo" ]] && cmd_index=2`, `--query --"`},
		{"zsh", zshCompletionScript, `[[ ${words[2]} == --demo ]] && cmd_index=3`, `--query --)`},
		{"fish", fishCompletionScript, `test $cmd[2] = --demo; and set index 3`, `--query --'`},
		{"powershell", powershellCompletionScript, `$tokens[1] -eq '--demo'`, `'--query', '--'`},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			if !strings.Contains(tt.script, tt.demo) {
				t.Error("completion script does not account for a leading --demo flag")
			}
			if !strings.Contains(tt.script, tt.separator) {
				t.Error("completion script does not expose the query separator")
			}
		})
	}

	if !strings.Contains(bashCompletionScript, `COMP_CWORD}" -eq $((cmd_index + 1))`) ||
		!strings.Contains(bashCompletionScript, `COMP_WORDS[cmd_index + 1]`) {
		t.Error("bash config completion uses fixed indexes instead of cmd_index")
	}
	if !strings.Contains(fishCompletionScript, "if test (count $cmd) -eq 1\n    return 0") {
		t.Error("fish top-level completion does not preserve the plain command case")
	}
	if !strings.Contains(powershellCompletionScript, `$queryMode = -not ($script:DispatchCommands -contains $command) -and (`) ||
		!strings.Contains(powershellCompletionScript, `($tokens -contains '--' -and $wordToComplete -ne '--')`) ||
		!strings.Contains(powershellCompletionScript, `"''"`) {
		t.Error("PowerShell completion does not suppress fallback in query mode")
	}
}
