package platform

import (
	"runtime"
	"testing"
)

func TestTSCodePipePath_Default(t *testing.T) {
	// With no TSCODE_PIPE env var, should return the legacy default.
	t.Setenv("TSCODE_PIPE", "")
	p := tscodePipePath()
	if runtime.GOOS == "windows" {
		if p != tscodePipeWindows {
			t.Errorf("tscodePipePath() = %q, want %q", p, tscodePipeWindows)
		}
	} else {
		if p != tscodePipeUnix {
			t.Errorf("tscodePipePath() = %q, want %q", p, tscodePipeUnix)
		}
	}
}

func TestTSCodePipePath_EnvOverride(t *testing.T) {
	const custom = `\\.\pipe\terminal-studio-code-12345`
	t.Setenv("TSCODE_PIPE", custom)
	p := tscodePipePath()
	if p != custom {
		t.Errorf("tscodePipePath() with TSCODE_PIPE = %q, want %q", p, custom)
	}
}
