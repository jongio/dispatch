package platform

import (
	"strings"
	"testing"
)

// assertContains fails t unless every element of want appears somewhere in
// args. These assertion helpers are shared across the platform package tests
// on every build platform: the tmux tests run on Unix while the WSL tests are
// gated behind //go:build windows, so the helpers must live in an
// unconstrained file to be visible to both.
func assertContains(t *testing.T, args []string, want ...string) {
	t.Helper()
	joined := strings.Join(args, " ")
	for _, w := range want {
		found := false
		for _, a := range args {
			if a == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("args %v should contain %q (full: %s)", args, w, joined)
		}
	}
}

// assertNotContains fails t if any element of excluded appears in args.
func assertNotContains(t *testing.T, args []string, excluded ...string) {
	t.Helper()
	for _, e := range excluded {
		for _, a := range args {
			if a == e {
				t.Errorf("args %v should NOT contain %q", args, e)
			}
		}
	}
}
