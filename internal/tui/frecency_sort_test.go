package tui

import (
	"testing"
	"time"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
)

func TestSortByFrecency_WrongField(t *testing.T) {
	m := newTestModel()
	m.sort.Field = data.SortByUpdated
	m.cfg.SessionLaunches = map[string]config.SessionLaunch{"s2": {Count: 9, Last: time.Now().Unix()}}
	sessions := []data.Session{{ID: "s1"}, {ID: "s2"}}
	m.sortByFrecency(sessions)
	if sessions[0].ID != "s1" {
		t.Errorf("no-op expected when field != frecency, got %s first", sessions[0].ID)
	}
}

func TestSortByFrecency_RanksMostUsedFirst(t *testing.T) {
	m := newTestModel()
	m.sort.Field = data.SortByFrecency
	m.sort.Order = data.Descending
	now := time.Now().Unix()
	m.cfg.SessionLaunches = map[string]config.SessionLaunch{
		"low":  {Count: 1, Last: now},
		"high": {Count: 10, Last: now},
	}
	sessions := []data.Session{{ID: "low"}, {ID: "high"}}
	m.sortByFrecency(sessions)
	if sessions[0].ID != "high" {
		t.Errorf("most-used should be first, got %s", sessions[0].ID)
	}
}

func TestSortByFrecency_NoHistoryFallbackKeepsOrder(t *testing.T) {
	m := newTestModel()
	m.sort.Field = data.SortByFrecency
	m.sort.Order = data.Descending
	m.cfg.SessionLaunches = map[string]config.SessionLaunch{
		"b": {Count: 4, Last: time.Now().Unix()},
	}
	sessions := []data.Session{{ID: "a"}, {ID: "c"}, {ID: "b"}}
	m.sortByFrecency(sessions)
	if sessions[0].ID != "b" {
		t.Errorf("launched session should be first, got %s", sessions[0].ID)
	}
	if sessions[1].ID != "a" || sessions[2].ID != "c" {
		t.Errorf("no-history order should be stable a,c; got %s,%s", sessions[1].ID, sessions[2].ID)
	}
}

func TestSortByFrecency_RecencyDecayOrders(t *testing.T) {
	m := newTestModel()
	m.sort.Field = data.SortByFrecency
	m.sort.Order = data.Descending
	now := time.Now()
	m.cfg.SessionLaunches = map[string]config.SessionLaunch{
		"stale":  {Count: 3, Last: now.Add(-60 * 24 * time.Hour).Unix()},
		"recent": {Count: 3, Last: now.Unix()},
	}
	sessions := []data.Session{{ID: "stale"}, {ID: "recent"}}
	m.sortByFrecency(sessions)
	if sessions[0].ID != "recent" {
		t.Errorf("recent should outrank stale at equal counts, got %s", sessions[0].ID)
	}
}
