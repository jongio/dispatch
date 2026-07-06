package config

import (
	"testing"
	"time"
)

func TestRecordLaunch(t *testing.T) {
	c := &Config{}
	base := time.Unix(1_000_000, 0)
	c.RecordLaunch("s1", base)
	if got := c.SessionLaunches["s1"].Count; got != 1 {
		t.Fatalf("Count = %d, want 1", got)
	}
	if got := c.SessionLaunches["s1"].Last; got != base.Unix() {
		t.Errorf("Last = %d, want %d", got, base.Unix())
	}
	later := base.Add(time.Hour)
	c.RecordLaunch("s1", later)
	if got := c.SessionLaunches["s1"].Count; got != 2 {
		t.Errorf("Count = %d, want 2", got)
	}
	if got := c.SessionLaunches["s1"].Last; got != later.Unix() {
		t.Errorf("Last = %d, want %d", got, later.Unix())
	}
}

func TestRecordLaunchIgnoresEmptyID(t *testing.T) {
	c := &Config{}
	c.RecordLaunch("", time.Now())
	if len(c.SessionLaunches) != 0 {
		t.Errorf("empty ID should not be recorded, got %d entries", len(c.SessionLaunches))
	}
}

func TestFrecencyScoreZeroWithoutLaunches(t *testing.T) {
	if s := FrecencyScore(SessionLaunch{}, time.Now()); s != 0 {
		t.Errorf("score = %v, want 0 for no launches", s)
	}
}

func TestFrecencyScoreRanksMoreLaunchesHigher(t *testing.T) {
	now := time.Now()
	last := now.Unix()
	few := FrecencyScore(SessionLaunch{Count: 1, Last: last}, now)
	many := FrecencyScore(SessionLaunch{Count: 5, Last: last}, now)
	if many <= few {
		t.Errorf("more launches should score higher: many=%v few=%v", many, few)
	}
}

func TestFrecencyScoreRanksRecentHigher(t *testing.T) {
	now := time.Now()
	recent := FrecencyScore(SessionLaunch{Count: 3, Last: now.Unix()}, now)
	old := FrecencyScore(SessionLaunch{Count: 3, Last: now.Add(-30 * 24 * time.Hour).Unix()}, now)
	if recent <= old {
		t.Errorf("more recent should score higher: recent=%v old=%v", recent, old)
	}
}

func TestFrecencyScoreHalfLifeDecay(t *testing.T) {
	now := time.Now()
	fresh := FrecencyScore(SessionLaunch{Count: 4, Last: now.Unix()}, now)
	oneHalfLife := FrecencyScore(SessionLaunch{Count: 4, Last: now.Add(-frecencyHalfLife).Unix()}, now)
	ratio := oneHalfLife / fresh
	if ratio < 0.45 || ratio > 0.55 {
		t.Errorf("half-life decay ratio = %v, want ~0.5", ratio)
	}
}

func TestSessionLaunchesRoundTrip(t *testing.T) {
	withTempConfigDir(t)
	original := &Config{
		SessionLaunches: map[string]SessionLaunch{
			"s1": {Count: 3, Last: 1_700_000_000},
		},
	}
	if err := Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := loaded.SessionLaunches["s1"]
	if got.Count != 3 || got.Last != 1_700_000_000 {
		t.Errorf("round-trip = %+v, want {Count:3 Last:1700000000}", got)
	}
}
