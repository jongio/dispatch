package tui

import (
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

func TestParseSearchTokens_FreeTextOnly(t *testing.T) {
	sf := ParseSearchTokens("hello world")
	if sf.FreeText != "hello world" {
		t.Errorf("expected free text %q, got %q", "hello world", sf.FreeText)
	}
	if sf.HasTokens() {
		t.Error("expected no tokens")
	}
}

func TestParseSearchTokens_SingleRepoToken(t *testing.T) {
	sf := ParseSearchTokens("repo:dispatch")
	if sf.Repo != "dispatch" {
		t.Errorf("expected repo %q, got %q", "dispatch", sf.Repo)
	}
	if sf.FreeText != "" {
		t.Errorf("expected empty free text, got %q", sf.FreeText)
	}
}

func TestParseSearchTokens_AllTokens(t *testing.T) {
	input := "repo:myrepo branch:main folder:/home/user host:github status:waiting has:plan is:favorite is:hidden leftover"
	sf := ParseSearchTokens(input)

	if sf.Repo != "myrepo" {
		t.Errorf("Repo = %q, want %q", sf.Repo, "myrepo")
	}
	if sf.Branch != "main" {
		t.Errorf("Branch = %q, want %q", sf.Branch, "main")
	}
	if sf.Folder != "/home/user" {
		t.Errorf("Folder = %q, want %q", sf.Folder, "/home/user")
	}
	if sf.Host != "github" {
		t.Errorf("Host = %q, want %q", sf.Host, "github")
	}
	if sf.Status != "waiting" {
		t.Errorf("Status = %q, want %q", sf.Status, "waiting")
	}
	if !sf.HasPlan {
		t.Error("expected HasPlan = true")
	}
	if !sf.IsFav {
		t.Error("expected IsFav = true")
	}
	if !sf.IsHidden {
		t.Error("expected IsHidden = true")
	}
	if sf.FreeText != "leftover" {
		t.Errorf("FreeText = %q, want %q", sf.FreeText, "leftover")
	}
}

func TestParseSearchTokens_MixedTokensAndText(t *testing.T) {
	input := "some text repo:dispatch more text branch:feature/auth"
	sf := ParseSearchTokens(input)

	if sf.Repo != "dispatch" {
		t.Errorf("Repo = %q, want %q", sf.Repo, "dispatch")
	}
	if sf.Branch != "feature/auth" {
		t.Errorf("Branch = %q, want %q", sf.Branch, "feature/auth")
	}
	if sf.FreeText != "some text more text" {
		t.Errorf("FreeText = %q, want %q", sf.FreeText, "some text more text")
	}
}

func TestParseSearchTokens_QuotedValue(t *testing.T) {
	input := `repo:"my cool repo" hello`
	sf := ParseSearchTokens(input)

	if sf.Repo != "my cool repo" {
		t.Errorf("Repo = %q, want %q", sf.Repo, "my cool repo")
	}
	if sf.FreeText != "hello" {
		t.Errorf("FreeText = %q, want %q", sf.FreeText, "hello")
	}
}

func TestParseSearchTokens_UnknownToken(t *testing.T) {
	sf := ParseSearchTokens("unknown:value text")
	if sf.HasTokens() {
		t.Error("unknown token should not set any filter")
	}
	if sf.FreeText != "unknown:value text" {
		t.Errorf("FreeText = %q, want %q", sf.FreeText, "unknown:value text")
	}
}

func TestParseSearchTokens_UnknownHasValue(t *testing.T) {
	sf := ParseSearchTokens("has:coffee")
	if sf.HasPlan {
		t.Error("has:coffee should not set HasPlan")
	}
	if sf.FreeText != "has:coffee" {
		t.Errorf("FreeText = %q, want %q", sf.FreeText, "has:coffee")
	}
}

func TestParseSearchTokens_UnknownIsValue(t *testing.T) {
	sf := ParseSearchTokens("is:unknown")
	if sf.IsFav || sf.IsHidden {
		t.Error("is:unknown should not set IsFav or IsHidden")
	}
	if sf.FreeText != "is:unknown" {
		t.Errorf("FreeText = %q, want %q", sf.FreeText, "is:unknown")
	}
}

func TestParseSearchTokens_IsFavShorthand(t *testing.T) {
	sf := ParseSearchTokens("is:fav")
	if !sf.IsFav {
		t.Error("is:fav should set IsFav = true")
	}
}

func TestParseSearchTokens_EmptyInput(t *testing.T) {
	sf := ParseSearchTokens("")
	if sf.HasTokens() {
		t.Error("empty input should have no tokens")
	}
	if sf.FreeText != "" {
		t.Errorf("FreeText = %q, want empty", sf.FreeText)
	}
}

func TestParseSearchTokens_ColonWithoutValue(t *testing.T) {
	// "repo:" with no value should stay as free text.
	sf := ParseSearchTokens("repo:")
	if sf.Repo != "" {
		t.Errorf("Repo should be empty for bare colon, got %q", sf.Repo)
	}
	if sf.FreeText != "repo:" {
		t.Errorf("FreeText = %q, want %q", sf.FreeText, "repo:")
	}
}

func TestParseSearchTokens_DuplicateTokenLastWins(t *testing.T) {
	sf := ParseSearchTokens("repo:first repo:second")
	if sf.Repo != "second" {
		t.Errorf("Repo = %q, want %q (last wins)", sf.Repo, "second")
	}
}

func TestSearchFilter_TokenSummary(t *testing.T) {
	sf := SearchFilter{Repo: "dispatch", HasPlan: true}
	got := sf.TokenSummary()
	want := "repo:dispatch has:plan"
	if got != want {
		t.Errorf("TokenSummary() = %q, want %q", got, want)
	}
}

func TestSearchFilter_TokenSummaryEmpty(t *testing.T) {
	sf := SearchFilter{FreeText: "hello"}
	got := sf.TokenSummary()
	if got != "" {
		t.Errorf("TokenSummary() = %q, want empty", got)
	}
}

func TestApplySearchTokens_SetsFilterFields(t *testing.T) {
	m := newTestModel()
	m.searchFilter = ParseSearchTokens("repo:dispatch branch:main folder:/home hello world")
	m.applySearchTokens()

	if m.filter.Repository != "dispatch" {
		t.Errorf("filter.Repository = %q, want %q", m.filter.Repository, "dispatch")
	}
	if m.filter.Branch != "main" {
		t.Errorf("filter.Branch = %q, want %q", m.filter.Branch, "main")
	}
	if m.filter.Folder != "/home" {
		t.Errorf("filter.Folder = %q, want %q", m.filter.Folder, "/home")
	}
	if m.filter.Query != "hello world" {
		t.Errorf("filter.Query = %q, want %q", m.filter.Query, "hello world")
	}
}

func TestApplySearchTokens_StatusSetsAttentionFilter(t *testing.T) {
	m := newTestModel()
	m.searchFilter = ParseSearchTokens("status:waiting")
	m.applySearchTokens()

	if len(m.attentionFilter) != 1 {
		t.Fatalf("expected 1 attention filter, got %d", len(m.attentionFilter))
	}
	if _, ok := m.attentionFilter[data.AttentionWaiting]; !ok {
		t.Error("expected attentionFilter to contain AttentionWaiting")
	}
}

func TestApplySearchTokens_BooleanTokens(t *testing.T) {
	m := newTestModel()
	m.searchFilter = ParseSearchTokens("has:plan is:favorite is:hidden")
	m.applySearchTokens()

	if !m.filterPlans {
		t.Error("expected filterPlans = true")
	}
	if !m.showFavorited {
		t.Error("expected showFavorited = true")
	}
	if !m.showHidden {
		t.Error("expected showHidden = true")
	}
}

func TestClearSearchTokenFilters_ResetsAll(t *testing.T) {
	m := newTestModel()
	m.filter.Repository = "test"
	m.filter.Branch = "dev"
	m.filter.Folder = "/tmp"
	m.filter.HostType = "github"
	m.filterPlans = true
	m.showFavorited = true
	m.showHidden = true

	m.clearSearchTokenFilters()

	if m.filter.Repository != "" {
		t.Errorf("Repository should be empty, got %q", m.filter.Repository)
	}
	if m.filter.Branch != "" {
		t.Errorf("Branch should be empty, got %q", m.filter.Branch)
	}
	if m.filter.Folder != "" {
		t.Errorf("Folder should be empty, got %q", m.filter.Folder)
	}
	if m.filter.HostType != "" {
		t.Errorf("HostType should be empty, got %q", m.filter.HostType)
	}
	if m.filterPlans {
		t.Error("filterPlans should be false")
	}
	if m.showFavorited {
		t.Error("showFavorited should be false")
	}
	if m.showHidden {
		t.Error("showHidden should be false")
	}
}

func TestApplySearchTokens_HostToken(t *testing.T) {
	m := newTestModel()
	m.searchFilter = ParseSearchTokens("host:github")
	m.applySearchTokens()

	if m.filter.HostType != "github" {
		t.Errorf("filter.HostType = %q, want %q", m.filter.HostType, "github")
	}
}

func TestParseSearchTokens_WhitespaceHandling(t *testing.T) {
	sf := ParseSearchTokens("  repo:dispatch   hello   ")
	if sf.Repo != "dispatch" {
		t.Errorf("Repo = %q, want %q", sf.Repo, "dispatch")
	}
	if sf.FreeText != "hello" {
		t.Errorf("FreeText = %q, want %q", sf.FreeText, "hello")
	}
}

func TestParseSearchTokens_StatusValues(t *testing.T) {
	statuses := []string{"waiting", "active", "stale", "idle", "interrupted", "working", "thinking"}
	for _, s := range statuses {
		sf := ParseSearchTokens("status:" + s)
		if sf.Status != s {
			t.Errorf("status:%s: Status = %q, want %q", s, sf.Status, s)
		}
	}
}
