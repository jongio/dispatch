package tui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
)

func TestApplyNamedViewAndPickerResetUserState(t *testing.T) {
	t.Run("apply named view", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		view := &config.NamedView{
			Name:          "Focused",
			Search:        "repo:user/repo1 auth",
			TimeRange:     "7d",
			Sort:          "created",
			SortOrder:     config.SortOrderAsc,
			Pivot:         pivotRepo,
			FavoritesOnly: true,
			ShowHidden:    true,
			ExcludedDirs:  []string{filepath.Clean("/tmp/archive")},
		}

		before := time.Now()
		m.applyNamedView(view)

		if m.timeRange != "7d" || m.filter.Since == nil {
			t.Fatalf("time range = %q, since = %v", m.timeRange, m.filter.Since)
		}
		if m.filter.Since.After(before.Add(-6*24*time.Hour)) ||
			m.filter.Since.Before(before.Add(-8*24*time.Hour)) {
			t.Fatalf("7d since value is outside expected range: %v", m.filter.Since)
		}
		if m.sort.Field != data.SortByCreated || m.sort.Order != data.Ascending {
			t.Fatalf("sort = %#v", m.sort)
		}
		if m.pivot != pivotRepo || !m.showFavorited || !m.showHidden {
			t.Fatalf("view flags pivot = %q, favorites = %v, hidden = %v",
				m.pivot, m.showFavorited, m.showHidden)
		}
		if m.filter.Repository != "user/repo1" || m.filter.Query != "auth" {
			t.Fatalf("search filter repo = %q, query = %q", m.filter.Repository, m.filter.Query)
		}
		if len(m.filter.ExcludedDirs) != 1 ||
			m.filter.ExcludedDirs[0] != filepath.Clean("/tmp/archive") {
			t.Fatalf("excluded dirs = %v", m.filter.ExcludedDirs)
		}
	})

	t.Run("picker selects named view", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.cfg.Views = []config.NamedView{{
			Name:      "Focused",
			TimeRange: "1h",
			Sort:      "turns",
			SortOrder: config.SortOrderAsc,
			Pivot:     pivotBranch,
		}}
		m.cfg.ExcludedDirs = []string{"global"}
		m.viewPicker.SetViews(m.cfg.Views)
		m.viewPicker.MoveDown()
		m.searchBar.SetValue("repo:user/old stale")
		m.searchFilter = ParseSearchTokens(m.searchBar.Value())
		m.applySearchTokens()
		m.filter.ExcludedDirs = []string{"previous-view"}
		m.search.deepSearchVersion = 9
		m.search.deepSearchPending = true
		m.filter.DeepSearch = true
		m.sessions = []data.Session{{ID: "current"}}
		m.sessionList.SetSessions(m.sessions)
		m.state = stateViewPicker

		result, cmd := m.handleKey(enterKeyMsg())
		got := result.(Model)

		if got.activeView != "Focused" || got.cfg.ActiveView != "Focused" {
			t.Fatalf("active view model = %q, config = %q", got.activeView, got.cfg.ActiveView)
		}
		if got.timeRange != "1h" || got.sort.Field != data.SortByTurns ||
			got.sort.Order != data.Ascending || got.pivot != pivotBranch {
			t.Fatalf("named view state range = %q, sort = %#v, pivot = %q",
				got.timeRange, got.sort, got.pivot)
		}
		if got.searchBar.Value() != "" || got.filter.Query != "" ||
			got.filter.Repository != "" || got.searchFilter.HasTokens() {
			t.Fatalf("named view inherited previous search state: filter = %#v, tokens = %#v",
				got.filter, got.searchFilter)
		}
		if len(got.filter.ExcludedDirs) != 1 || got.filter.ExcludedDirs[0] != "global" {
			t.Fatalf("named view excluded dirs = %v, want global defaults", got.filter.ExcludedDirs)
		}
		if got.state != stateSessionList || cmd == nil {
			t.Fatalf("picker state = %v, cmd nil = %v", got.state, cmd == nil)
		}
		if got.search.deepSearchVersion != 10 || got.search.deepSearchPending ||
			got.filter.DeepSearch {
			t.Fatalf("deep search state version = %d, pending = %v, enabled = %v",
				got.search.deepSearchVersion, got.search.deepSearchPending, got.filter.DeepSearch)
		}

		staleResult, staleCmd := got.Update(deepSearchResultMsg{
			version:  9,
			sessions: []data.Session{{ID: "stale-result"}},
		})
		afterStale := staleResult.(Model)
		if staleCmd != nil || len(afterStale.sessions) != 1 ||
			afterStale.sessions[0].ID != "current" {
			t.Fatalf("stale deep search replaced named view: %#v", afterStale.sessions)
		}
	})

	t.Run("picker restores defaults", func(t *testing.T) {
		m := newBehavioralFlowModel(t)
		m.cfg.Views = []config.NamedView{{Name: "Focused", TimeRange: "1h"}}
		m.cfg.DefaultTimeRange = "1d"
		m.cfg.DefaultSort = "name"
		m.cfg.DefaultSortOrder = config.SortOrderAsc
		m.cfg.DefaultPivot = pivotFolder
		m.cfg.ExcludedDirs = []string{"ignored"}
		m.activeView = "Focused"
		m.cfg.ActiveView = "Focused"
		m.timeRange = "1h"
		m.showFavorited = true
		m.showHidden = true
		m.searchBar.SetValue("stale")
		m.searchBar.SetSearching(true)
		m.searchFilter = ParseSearchTokens("repo:user/repo1 stale")
		m.applySearchTokens()
		m.filter.DeepSearch = true
		m.search.lastRawInput = "repo:user/repo1 stale"
		m.search.deepSearchVersion = 41
		m.search.deepSearchPending = true
		m.filter.ExcludedDirs = []string{"stale"}
		m.sessions = []data.Session{{ID: "current"}}
		m.sessionList.SetSessions(m.sessions)
		m.viewPicker.SetViews(m.cfg.Views)
		m.viewPicker.SetActiveView("Focused")
		m.viewPicker.MoveUp()
		m.state = stateViewPicker

		result, cmd := m.handleKey(enterKeyMsg())
		got := result.(Model)

		if got.activeView != "" || got.cfg.ActiveView != "" {
			t.Fatalf("default selection left active view %q", got.activeView)
		}
		if got.timeRange != "1d" || got.sort.Field != data.SortByName ||
			got.sort.Order != data.Ascending || got.pivot != pivotFolder {
			t.Fatalf("default state range = %q, sort = %#v, pivot = %q",
				got.timeRange, got.sort, got.pivot)
		}
		if got.showFavorited || got.showHidden || got.searchBar.Value() != "" {
			t.Fatalf("default view did not reset flags or visible search state")
		}
		if got.filter.Query != "" || got.filter.Repository != "" ||
			got.filter.DeepSearch || got.searchFilter.HasTokens() ||
			got.search.lastRawInput != "" {
			t.Fatalf("default view left stale search filters: filter = %#v, tokens = %#v",
				got.filter, got.searchFilter)
		}
		if len(got.filter.ExcludedDirs) != 1 || got.filter.ExcludedDirs[0] != "ignored" {
			t.Fatalf("default excluded dirs = %v", got.filter.ExcludedDirs)
		}
		if got.search.deepSearchVersion != 42 || got.search.deepSearchPending {
			t.Fatalf("deep search state version = %d, pending = %v",
				got.search.deepSearchVersion, got.search.deepSearchPending)
		}
		if cmd == nil {
			t.Fatal("default view selection should reload sessions")
		}

		staleResult, staleCmd := got.Update(deepSearchResultMsg{
			version:  41,
			sessions: []data.Session{{ID: "stale-result"}},
		})
		afterStale := staleResult.(Model)
		if staleCmd != nil || len(afterStale.sessions) != 1 ||
			afterStale.sessions[0].ID != "current" {
			t.Fatalf("stale deep search replaced default view: %#v", afterStale.sessions)
		}
	})
}
