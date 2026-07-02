package config

import (
	"testing"
	"time"
)

func TestEffectiveAutoRefreshInterval(t *testing.T) {
	t.Parallel()

	ptr := func(v int) *int { return &v }

	tests := []struct {
		name         string
		seconds      *int
		wantInterval time.Duration
		wantEnabled  bool
	}{
		{"unset uses default", nil, defaultAutoRefreshInterval, true},
		{"zero disables polling", ptr(0), 0, false},
		{"negative falls back to default", ptr(-5), defaultAutoRefreshInterval, true},
		{"positive sets interval", ptr(10), 10 * time.Second, true},
		{"one second is the minimum", ptr(1), minAutoRefreshInterval, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{AutoRefreshSeconds: tc.seconds}
			gotInterval, gotEnabled := cfg.EffectiveAutoRefreshInterval()
			if gotInterval != tc.wantInterval {
				t.Errorf("interval = %v, want %v", gotInterval, tc.wantInterval)
			}
			if gotEnabled != tc.wantEnabled {
				t.Errorf("enabled = %v, want %v", gotEnabled, tc.wantEnabled)
			}
		})
	}
}

func TestAutoRefreshSeconds_JSONRoundTrip(t *testing.T) {
	// Not parallel: withTempConfigDir sets process-wide env vars.
	withTempConfigDir(t)

	// A pointer keeps "unset" and "0" distinct through save/load, so an
	// explicit 0 (disable) survives a restart instead of reverting to default.
	zero := 0
	cfg := Default()
	cfg.AutoRefreshSeconds = &zero
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.AutoRefreshSeconds == nil {
		t.Fatal("AutoRefreshSeconds should persist as 0, got nil")
	}
	if *loaded.AutoRefreshSeconds != 0 {
		t.Errorf("AutoRefreshSeconds = %d, want 0", *loaded.AutoRefreshSeconds)
	}
	if _, enabled := loaded.EffectiveAutoRefreshInterval(); enabled {
		t.Error("interval should be disabled when AutoRefreshSeconds is 0")
	}
}
