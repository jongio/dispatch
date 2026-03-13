//go:build !windows

package platform

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// filterEnv — dangerous environment variable filtering (HACK-09)
// ---------------------------------------------------------------------------

func TestFilterEnv_RemovesDangerousVars(t *testing.T) {
	env := []string{
		"HOME=/home/user",
		"PATH=/usr/bin",
		"LD_PRELOAD=/evil/lib.so",
		"LD_LIBRARY_PATH=/evil/lib",
		"LD_AUDIT=/evil/audit.so",
		"DYLD_INSERT_LIBRARIES=/evil/lib.dylib",
		"DYLD_LIBRARY_PATH=/evil/path",
		"DYLD_FRAMEWORK_PATH=/evil/frameworks",
		"SHELL=/bin/bash",
	}

	filtered := filterEnv(env)

	// Verify dangerous vars are removed
	for _, e := range filtered {
		name := e
		if idx := strings.IndexByte(e, '='); idx >= 0 {
			name = e[:idx]
		}
		for _, dangerous := range dangerousEnvPrefixes {
			if strings.EqualFold(name, dangerous) {
				t.Errorf("filterEnv should have removed %q", e)
			}
		}
	}

	// Verify safe vars are preserved
	found := map[string]bool{}
	for _, e := range filtered {
		if strings.HasPrefix(e, "HOME=") {
			found["HOME"] = true
		}
		if strings.HasPrefix(e, "PATH=") {
			found["PATH"] = true
		}
		if strings.HasPrefix(e, "SHELL=") {
			found["SHELL"] = true
		}
	}
	if !found["HOME"] || !found["PATH"] || !found["SHELL"] {
		t.Errorf("filterEnv removed safe env vars; kept: %v", found)
	}
}

func TestFilterEnv_CaseInsensitive(t *testing.T) {
	env := []string{
		"ld_preload=/evil/lib.so",
		"Ld_Preload=/evil/lib2.so",
		"HOME=/home/user",
	}

	filtered := filterEnv(env)

	if len(filtered) != 1 {
		t.Errorf("expected 1 remaining env var, got %d: %v", len(filtered), filtered)
	}
	if filtered[0] != "HOME=/home/user" {
		t.Errorf("expected HOME to survive, got %q", filtered[0])
	}
}

func TestFilterEnv_EmptyEnv(t *testing.T) {
	filtered := filterEnv([]string{})
	if len(filtered) != 0 {
		t.Errorf("filterEnv of empty slice should return empty, got %v", filtered)
	}
}

func TestFilterEnv_NoDangerousVars(t *testing.T) {
	env := []string{
		"HOME=/home/user",
		"PATH=/usr/bin",
		"TERM=xterm",
	}

	filtered := filterEnv(env)
	if len(filtered) != len(env) {
		t.Errorf("filterEnv should preserve all safe vars; got %d, want %d", len(filtered), len(env))
	}
}
