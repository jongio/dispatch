package platform

import (
	"testing"
)

func TestBuildTmuxSplitArgs_Direction(t *testing.T) {
	shell := ShellInfo{Path: "/usr/bin/bash"}
	resume := "ghcs --resume s1"

	tests := []struct {
		name    string
		dir     string
		wantDir string // "-h", "-v", or "" for no direction flag
	}{
		{"right maps to -h", "right", "-h"},
		{"left maps to -h", "left", "-h"},
		{"down maps to -v", "down", "-v"},
		{"up maps to -v", "up", "-v"},
		{"auto has no flag", "auto", ""},
		{"empty has no flag", "", ""},
		{"unknown has no flag", "sideways", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := buildTmuxSplitArgs(shell, resume, "/work/dir", tc.dir)

			if len(args) == 0 || args[0] != "split-window" {
				t.Fatalf("args must start with split-window, got %v", args)
			}

			switch tc.wantDir {
			case "-h":
				assertContains(t, args, "-h")
				assertNotContains(t, args, "-v")
			case "-v":
				assertContains(t, args, "-v")
				assertNotContains(t, args, "-h")
			default:
				assertNotContains(t, args, "-h", "-v")
			}

			// Working directory and resume command must always be present.
			assertContains(t, args, "-c", "/work/dir")
			if !containsSeq(args, shell.Path, "-c", resume) {
				t.Errorf("dir %q: missing shell resume command in %v", tc.dir, args)
			}
		})
	}
}

func TestBuildTmuxSplitArgs_NoCwd(t *testing.T) {
	shell := ShellInfo{Path: "/bin/zsh"}
	args := buildTmuxSplitArgs(shell, "cmd", "", "right")

	// With no cwd there is exactly one -c (the shell flag), never a
	// working-directory -c pair.
	if !containsSeq(args, shell.Path, "-c", "cmd") {
		t.Errorf("missing shell resume command in %v", args)
	}
	if containsSeq(args, "-c", "") {
		t.Errorf("unexpected empty working directory flag in %v", args)
	}
}

func TestInsideTmux(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")
		if !insideTmux() {
			t.Error("insideTmux() = false with TMUX set, want true")
		}
	})
	t.Run("empty", func(t *testing.T) {
		t.Setenv("TMUX", "")
		if insideTmux() {
			t.Error("insideTmux() = true with empty TMUX, want false")
		}
	})
}

// containsSeq reports whether seq appears as a contiguous subsequence of s.
func containsSeq(s []string, seq ...string) bool {
	if len(seq) == 0 {
		return true
	}
	for i := 0; i+len(seq) <= len(s); i++ {
		match := true
		for j := range seq {
			if s[i+j] != seq[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
