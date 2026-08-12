package platform

import (
	"strings"
	"testing"
)

func TestWTEscapeArgEscapesCommandSeparators(t *testing.T) {
	t.Parallel()

	got := wtEscapeArg(`C:\work;new-tab -- powershell`)
	if got != `C:\work\;new-tab -- powershell` {
		t.Fatalf("wtEscapeArg() = %q", got)
	}
	if strings.Contains(strings.ReplaceAll(got, `\;`, ""), ";") {
		t.Fatalf("wtEscapeArg() left an unescaped semicolon: %q", got)
	}
}

func TestBuildWSLWTArgsEscapesAllDynamicSemicolons(t *testing.T) {
	t.Parallel()

	args := buildWSLWTArgs(
		ShellInfo{Path: `/bin/bash;new-tab`},
		`copilot;new-tab`,
		`C:\work;new-tab`,
		`Ubuntu;new-tab`,
		"",
		"",
	)
	for _, arg := range args {
		if strings.Contains(strings.ReplaceAll(arg, `\;`, ""), ";") {
			t.Fatalf("argument contains an unescaped WT command separator: %q", arg)
		}
	}
}
