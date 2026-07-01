package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

func TestBuildTimeline_NilDetail(t *testing.T) {
	t.Parallel()
	entries := BuildTimeline(nil)
	if entries != nil {
		t.Errorf("expected nil, got %d entries", len(entries))
	}
}

func TestBuildTimeline_EmptyDetail(t *testing.T) {
	t.Parallel()
	detail := &data.SessionDetail{
		Session: data.Session{ID: "s1", CreatedAt: "2024-01-01T10:00:00Z"},
	}
	entries := BuildTimeline(detail)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty detail, got %d", len(entries))
	}
}

func TestBuildTimeline_Turns(t *testing.T) {
	t.Parallel()
	detail := &data.SessionDetail{
		Session: data.Session{ID: "s1", CreatedAt: "2024-01-01T10:00:00Z"},
		Turns: []data.Turn{
			{TurnIndex: 0, UserMessage: "Hello world", Timestamp: "2024-01-01T10:01:00Z"},
			{TurnIndex: 1, UserMessage: "Second turn", Timestamp: "2024-01-01T10:02:00Z"},
		},
	}
	entries := BuildTimeline(detail)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Category != "turn" {
		t.Errorf("expected category 'turn', got %q", entries[0].Category)
	}
	if entries[0].Description != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", entries[0].Description)
	}
	// Verify chronological order.
	if !entries[0].Time.Before(entries[1].Time) {
		t.Error("entries should be in chronological order")
	}
}

func TestBuildTimeline_Checkpoints(t *testing.T) {
	t.Parallel()
	detail := &data.SessionDetail{
		Session: data.Session{ID: "s1", CreatedAt: "2024-01-01T10:00:00Z"},
		Turns: []data.Turn{
			{TurnIndex: 0, Timestamp: "2024-01-01T10:01:00Z"},
		},
		Checkpoints: []data.Checkpoint{
			{CheckpointNumber: 1, Title: "Initial setup"},
		},
	}
	entries := BuildTimeline(detail)
	found := false
	for _, e := range entries {
		if e.Category == "checkpoint" {
			found = true
			if e.Description != "Initial setup" {
				t.Errorf("expected 'Initial setup', got %q", e.Description)
			}
		}
	}
	if !found {
		t.Error("expected a checkpoint entry")
	}
}

func TestBuildTimeline_Files(t *testing.T) {
	t.Parallel()
	detail := &data.SessionDetail{
		Session: data.Session{ID: "s1", CreatedAt: "2024-01-01T10:00:00Z"},
		Files: []data.SessionFile{
			{FilePath: "src/main.go", ToolName: "edit", FirstSeenAt: "2024-01-01T10:05:00Z"},
		},
	}
	entries := BuildTimeline(detail)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Category != "file" {
		t.Errorf("expected category 'file', got %q", entries[0].Category)
	}
	if !strings.Contains(entries[0].Description, "edit") {
		t.Errorf("expected description to contain 'edit', got %q", entries[0].Description)
	}
}

func TestBuildTimeline_Refs(t *testing.T) {
	t.Parallel()
	detail := &data.SessionDetail{
		Session: data.Session{ID: "s1", CreatedAt: "2024-01-01T10:00:00Z"},
		Refs: []data.SessionRef{
			{RefType: "pr", RefValue: "#42", CreatedAt: "2024-01-01T10:10:00Z"},
			{RefType: "commit", RefValue: "abc123", CreatedAt: "2024-01-01T10:11:00Z"},
		},
	}
	entries := BuildTimeline(detail)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Category != "ref" {
		t.Errorf("expected category 'ref', got %q", entries[0].Category)
	}
}

func TestBuildTimeline_ChronologicalOrder(t *testing.T) {
	t.Parallel()
	detail := &data.SessionDetail{
		Session: data.Session{ID: "s1", CreatedAt: "2024-01-01T10:00:00Z"},
		Turns: []data.Turn{
			{TurnIndex: 0, UserMessage: "first", Timestamp: "2024-01-01T10:05:00Z"},
		},
		Files: []data.SessionFile{
			{FilePath: "a.go", ToolName: "create", FirstSeenAt: "2024-01-01T10:02:00Z"},
		},
		Refs: []data.SessionRef{
			{RefType: "commit", RefValue: "abc", CreatedAt: "2024-01-01T10:08:00Z"},
		},
	}
	entries := BuildTimeline(detail)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// File at 10:02, Turn at 10:05, Ref at 10:08
	if entries[0].Category != "file" {
		t.Errorf("first entry should be file, got %q", entries[0].Category)
	}
	if entries[1].Category != "turn" {
		t.Errorf("second entry should be turn, got %q", entries[1].Category)
	}
	if entries[2].Category != "ref" {
		t.Errorf("third entry should be ref, got %q", entries[2].Category)
	}
}

func TestBuildTimeline_SkipsInvalidTimestamps(t *testing.T) {
	t.Parallel()
	detail := &data.SessionDetail{
		Session: data.Session{ID: "s1", CreatedAt: "2024-01-01T10:00:00Z"},
		Turns: []data.Turn{
			{TurnIndex: 0, UserMessage: "valid", Timestamp: "2024-01-01T10:01:00Z"},
			{TurnIndex: 1, UserMessage: "invalid", Timestamp: "not-a-date"},
		},
	}
	entries := BuildTimeline(detail)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (skipping invalid timestamp), got %d", len(entries))
	}
}

func TestBuildTimeline_TurnWithEmptyMessage(t *testing.T) {
	t.Parallel()
	detail := &data.SessionDetail{
		Session: data.Session{ID: "s1", CreatedAt: "2024-01-01T10:00:00Z"},
		Turns: []data.Turn{
			{TurnIndex: 3, UserMessage: "", Timestamp: "2024-01-01T10:01:00Z"},
		},
	}
	entries := BuildTimeline(detail)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Description != "Turn 3" {
		t.Errorf("expected 'Turn 3' for empty message, got %q", entries[0].Description)
	}
}

func TestRenderTimeline_Empty(t *testing.T) {
	t.Parallel()
	result := RenderTimeline(nil, 80)
	if !strings.Contains(result, "No timeline data") {
		t.Errorf("expected 'No timeline data' message, got %q", result)
	}
}

func TestRenderTimeline_WithEntries(t *testing.T) {
	t.Parallel()
	detail := &data.SessionDetail{
		Session: data.Session{ID: "s1", CreatedAt: "2024-01-01T10:00:00Z"},
		Turns: []data.Turn{
			{TurnIndex: 0, UserMessage: "Hello", Timestamp: "2024-01-01T10:01:00Z"},
		},
		Refs: []data.SessionRef{
			{RefType: "pr", RefValue: "#5", CreatedAt: "2024-01-01T10:02:00Z"},
		},
	}
	entries := BuildTimeline(detail)
	result := RenderTimeline(entries, 80)
	if !strings.Contains(result, "Activity Timeline") {
		t.Error("expected 'Activity Timeline' header")
	}
	if !strings.Contains(result, "Hello") {
		t.Error("expected turn message in output")
	}
	if !strings.Contains(result, "#5") {
		t.Error("expected ref value in output")
	}
}

func TestTruncateMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"", 10, ""},
		{"line1\nline2", 20, "line1"},
		{"this is a very long message that exceeds the limit", 20, "this is a very long…"},
	}
	for _, tc := range tests {
		got := truncateMessage(tc.input, tc.maxLen)
		if got != tc.want {
			t.Errorf("truncateMessage(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
		}
	}
}

func TestAbbreviateFilePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"main.go", "main.go"},
		{"src/main.go", "src/main.go"},
		{"a/b/c/main.go", ".../c/main.go"},
	}
	for _, tc := range tests {
		got := abbreviateFilePath(tc.input)
		if got != tc.want {
			t.Errorf("abbreviateFilePath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestRefIcon(t *testing.T) {
	t.Parallel()
	tests := []struct {
		refType string
		want    string
	}{
		{"commit", "🔨"},
		{"pr", "🔀"},
		{"issue", "📋"},
		{"unknown", "🔗"},
	}
	for _, tc := range tests {
		got := refIcon(tc.refType)
		if got != tc.want {
			t.Errorf("refIcon(%q) = %q, want %q", tc.refType, got, tc.want)
		}
	}
}

func TestPreviewPanel_ToggleTimeline(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 40)

	// Toggle without detail should return false.
	got := p.ToggleTimeline()
	if got {
		t.Error("ToggleTimeline with no detail should return false")
	}
	if p.TimelineMode() {
		t.Error("should not be in timeline mode without detail")
	}

	// Set detail and toggle.
	p.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "test", Cwd: "/tmp", CreatedAt: "2024-01-01T10:00:00Z"},
		Turns: []data.Turn{
			{TurnIndex: 0, UserMessage: "hi", Timestamp: "2024-01-01T10:01:00Z"},
		},
	})

	got = p.ToggleTimeline()
	if !got {
		t.Error("ToggleTimeline should return true when enabling timeline")
	}
	if !p.TimelineMode() {
		t.Error("should be in timeline mode")
	}

	// Toggle back.
	got = p.ToggleTimeline()
	if got {
		t.Error("ToggleTimeline should return false when disabling timeline")
	}
	if p.TimelineMode() {
		t.Error("should not be in timeline mode after second toggle")
	}
}

func TestPreviewPanel_TimelineExitsPlanView(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 40)
	p.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "test", Cwd: "/tmp", CreatedAt: "2024-01-01T10:00:00Z"},
		Turns: []data.Turn{
			{TurnIndex: 0, UserMessage: "hi", Timestamp: "2024-01-01T10:01:00Z"},
		},
	})
	p.SetPlanContent("# Plan")
	p.TogglePlanView()
	if !p.PlanViewMode() {
		t.Fatal("should be in plan view")
	}

	// Toggling timeline should exit plan view.
	p.ToggleTimeline()
	if p.PlanViewMode() {
		t.Error("plan view should be disabled when entering timeline mode")
	}
	if !p.TimelineMode() {
		t.Error("should be in timeline mode")
	}
}

func TestPreviewPanel_TimelineViewRendersContent(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 40)
	p.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "test", Cwd: "/tmp", CreatedAt: "2024-01-01T10:00:00Z"},
		Turns: []data.Turn{
			{TurnIndex: 0, UserMessage: "Hello", Timestamp: "2024-01-01T10:01:00Z"},
		},
	})
	p.ToggleTimeline()

	content := p.Content()
	if !strings.Contains(content, "Activity Timeline") {
		t.Error("timeline content should contain 'Activity Timeline' header")
	}
	if !strings.Contains(content, "Hello") {
		t.Error("timeline content should contain turn message")
	}

	view := p.View()
	if view == "" {
		t.Error("View() should render non-empty content in timeline mode")
	}
}

func TestPreviewPanel_TimelineScrollResets(t *testing.T) {
	t.Parallel()
	p := NewPreviewPanel()
	p.SetSize(80, 40)
	p.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "test", Cwd: "/tmp", CreatedAt: "2024-01-01T10:00:00Z"},
		Turns: []data.Turn{
			{TurnIndex: 0, UserMessage: "hi", Timestamp: "2024-01-01T10:01:00Z"},
		},
	})
	p.ScrollDown(5)

	p.ToggleTimeline()
	if p.ScrollOffset() != 0 {
		t.Errorf("scroll should reset to 0 on timeline toggle, got %d", p.ScrollOffset())
	}
}
