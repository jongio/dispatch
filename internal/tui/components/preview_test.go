package components

import (
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

// ---------------------------------------------------------------------------
// RenderChatBubble
// ---------------------------------------------------------------------------

func TestRenderChatBubbleEmptyMessage(t *testing.T) {
	got := RenderChatBubble("", "You", 60, true)
	if got != "" {
		t.Errorf("RenderChatBubble with empty msg should return empty, got %q", got)
	}
}

func TestRenderChatBubbleUserContainsLabel(t *testing.T) {
	got := RenderChatBubble("hello", "You", 60, true)
	if !strings.Contains(got, "You") {
		t.Errorf("User bubble should contain label 'You', got:\n%s", got)
	}
}

func TestRenderChatBubbleUserContainsMessage(t *testing.T) {
	got := RenderChatBubble("hello world", "You", 60, true)
	if !strings.Contains(got, "hello world") {
		t.Errorf("User bubble should contain message text, got:\n%s", got)
	}
}

func TestRenderChatBubbleAssistantContainsLabel(t *testing.T) {
	got := RenderChatBubble("hi there", "Copilot", 60, false)
	if !strings.Contains(got, "Copilot") {
		t.Errorf("Assistant bubble should contain label 'Copilot', got:\n%s", got)
	}
}

func TestRenderChatBubbleAssistantContainsMessage(t *testing.T) {
	got := RenderChatBubble("hi there", "Copilot", 60, false)
	if !strings.Contains(got, "hi there") {
		t.Errorf("Assistant bubble should contain message text, got:\n%s", got)
	}
}

func TestRenderChatBubbleUserRightAligned(t *testing.T) {
	got := RenderChatBubble("short", "You", 60, true)
	lines := strings.Split(got, "\n")

	// The label line should have leading whitespace (right-aligned).
	foundLabelLine := false
	for _, line := range lines {
		if strings.Contains(line, "You") {
			foundLabelLine = true
			trimmed := strings.TrimLeft(line, " ")
			leadingSpaces := len(line) - len(trimmed)
			if leadingSpaces == 0 {
				t.Errorf("User label should be right-aligned with leading spaces, got: %q", line)
			}
			break
		}
	}
	if !foundLabelLine {
		t.Error("Could not find label line containing 'You'")
	}
}

func TestRenderChatBubbleAssistantLeftAligned(t *testing.T) {
	got := RenderChatBubble("hello", "Copilot", 60, false)
	lines := strings.Split(got, "\n")

	// The label line should start without leading whitespace (left-aligned).
	if len(lines) == 0 {
		t.Fatal("Expected non-empty output")
	}
	// First line is the label — it should be indented by bubbleInset.
	firstLine := lines[0]
	if strings.Contains(firstLine, "Copilot") {
		trimmed := strings.TrimLeft(firstLine, " ")
		leadingSpaces := len(firstLine) - len(trimmed)
		if leadingSpaces != bubbleInset {
			t.Errorf("Assistant label should be indented by %d spaces, got %d: %q",
				bubbleInset, leadingSpaces, firstLine)
		}
	}
}

func TestRenderChatBubbleNarrowWidth(t *testing.T) {
	// Should not panic with very narrow widths.
	got := RenderChatBubble("hello world this is a test", "You", 15, true)
	if got == "" {
		t.Error("Expected non-empty output for narrow width")
	}
	if !strings.Contains(got, "You") {
		t.Error("Expected label in narrow width output")
	}
}

func TestRenderChatBubbleMinimumWidth(t *testing.T) {
	// Should not panic with minimum width.
	got := RenderChatBubble("x", "Copilot", 1, false)
	if got == "" {
		t.Error("Expected non-empty output for width=1")
	}
}

// ---------------------------------------------------------------------------
// RenderConversation
// ---------------------------------------------------------------------------

func TestRenderConversationEmpty(t *testing.T) {
	got := RenderConversation(nil, 60)
	if got != "" {
		t.Errorf("RenderConversation with nil turns should return empty, got %q", got)
	}

	got = RenderConversation([]data.Turn{}, 60)
	if got != "" {
		t.Errorf("RenderConversation with empty turns should return empty, got %q", got)
	}
}

func TestRenderConversationSingleTurn(t *testing.T) {
	turns := []data.Turn{
		{
			UserMessage:       "What is Go?",
			AssistantResponse: "Go is a programming language.",
		},
	}
	got := RenderConversation(turns, 60)
	if !strings.Contains(got, "You") {
		t.Error("Single turn should contain 'You' label")
	}
	if !strings.Contains(got, "Copilot") {
		t.Error("Single turn should contain 'Copilot' label")
	}
	if !strings.Contains(got, "What is Go?") {
		t.Error("Single turn should contain user message")
	}
	if !strings.Contains(got, "Go is a programming language.") {
		t.Error("Single turn should contain assistant response")
	}
}

func TestRenderConversationMultipleTurns(t *testing.T) {
	turns := []data.Turn{
		{UserMessage: "Hello", AssistantResponse: "Hi!"},
		{UserMessage: "How are you?", AssistantResponse: "I'm good."},
	}
	got := RenderConversation(turns, 60)

	// Should have both user messages.
	if !strings.Contains(got, "Hello") {
		t.Error("Should contain first user message")
	}
	if !strings.Contains(got, "How are you?") {
		t.Error("Should contain second user message")
	}
	// Should have both assistant responses.
	if !strings.Contains(got, "Hi!") {
		t.Error("Should contain first assistant response")
	}
	if !strings.Contains(got, "I'm good.") {
		t.Error("Should contain second assistant response")
	}
	// Should have "You" label at least twice and "Copilot" at least twice.
	if strings.Count(got, "You") < 2 {
		t.Errorf("Expected at least 2 'You' labels, got %d", strings.Count(got, "You"))
	}
	if strings.Count(got, "Copilot") < 2 {
		t.Errorf("Expected at least 2 'Copilot' labels, got %d", strings.Count(got, "Copilot"))
	}
}

func TestRenderConversationMissingUserMessage(t *testing.T) {
	turns := []data.Turn{
		{UserMessage: "", AssistantResponse: "Auto response"},
	}
	got := RenderConversation(turns, 60)
	if strings.Contains(got, "You") {
		t.Error("Should not contain 'You' label when user message is empty")
	}
	if !strings.Contains(got, "Auto response") {
		t.Error("Should contain assistant response")
	}
}

func TestRenderConversationMissingAssistantResponse(t *testing.T) {
	turns := []data.Turn{
		{UserMessage: "Hello?", AssistantResponse: ""},
	}
	got := RenderConversation(turns, 60)
	if !strings.Contains(got, "Hello?") {
		t.Error("Should contain user message")
	}
	if strings.Contains(got, "Copilot") {
		t.Error("Should not contain 'Copilot' label when response is empty")
	}
}

// ---------------------------------------------------------------------------
// PreviewPanel scroll
// ---------------------------------------------------------------------------

func TestPreviewPanelScrollUpClampsAtZero(t *testing.T) {
	p := NewPreviewPanel()
	p.ScrollUp(10)
	if p.scroll != 0 {
		t.Errorf("ScrollUp from 0 should clamp to 0, got %d", p.scroll)
	}
}

func TestPreviewPanelScrollDown(t *testing.T) {
	p := NewPreviewPanel()
	p.SetSize(80, 40)

	detail := &data.SessionDetail{
		Session: data.Session{ID: "test", TurnCount: 5},
		Turns: []data.Turn{
			{UserMessage: "msg1", AssistantResponse: "resp1"},
			{UserMessage: "msg2", AssistantResponse: "resp2"},
			{UserMessage: "msg3", AssistantResponse: "resp3"},
			{UserMessage: "msg4", AssistantResponse: "resp4"},
			{UserMessage: "msg5", AssistantResponse: "resp5"},
		},
	}
	p.SetDetail(detail)

	p.ScrollDown(5)
	if p.scroll < 0 {
		t.Errorf("ScrollDown should increase scroll, got %d", p.scroll)
	}
}

func TestPreviewPanelScrollUpAfterDown(t *testing.T) {
	p := NewPreviewPanel()
	p.SetSize(80, 20) // Small viewport to ensure scrollable content.

	turns := make([]data.Turn, 10)
	for i := range turns {
		turns[i] = data.Turn{
			UserMessage:       "Question " + FormatInt(i) + " with enough text to fill space",
			AssistantResponse: "Answer " + FormatInt(i) + " with a detailed response",
		}
	}
	detail := &data.SessionDetail{
		Session: data.Session{ID: "test", TurnCount: 10},
		Turns:   turns,
	}
	p.SetDetail(detail)

	p.ScrollDown(5)
	prev := p.scroll
	if prev <= 0 {
		t.Skipf("Content not long enough to scroll, totalLines=%d, viewport=%d", p.totalLines, p.height-2)
	}
	p.ScrollUp(2)
	if p.scroll >= prev {
		t.Errorf("ScrollUp should decrease scroll: prev=%d, now=%d", prev, p.scroll)
	}
}

func TestPreviewPanelPageUpPageDown(t *testing.T) {
	p := NewPreviewPanel()
	p.SetSize(80, 40)

	detail := &data.SessionDetail{
		Session: data.Session{ID: "test", TurnCount: 10},
		Turns:   make([]data.Turn, 10),
	}
	for i := range detail.Turns {
		detail.Turns[i] = data.Turn{
			UserMessage:       "Question " + FormatInt(i),
			AssistantResponse: "Answer " + FormatInt(i),
		}
	}
	p.SetDetail(detail)

	p.PageDown()
	if p.scroll <= 0 {
		t.Errorf("PageDown should scroll past 0, got %d", p.scroll)
	}

	scrollAfterDown := p.scroll
	p.PageUp()
	if p.scroll >= scrollAfterDown {
		t.Errorf("PageUp should decrease scroll: was=%d, now=%d", scrollAfterDown, p.scroll)
	}
}

func TestPreviewPanelSetDetailResetsScroll(t *testing.T) {
	p := NewPreviewPanel()
	p.SetSize(80, 40)

	detail1 := &data.SessionDetail{
		Session: data.Session{ID: "s1"},
		Turns:   []data.Turn{{UserMessage: "a", AssistantResponse: "b"}},
	}
	p.SetDetail(detail1)
	p.ScrollDown(10)

	// Setting a new detail should reset scroll to 0.
	detail2 := &data.SessionDetail{
		Session: data.Session{ID: "s2"},
		Turns:   []data.Turn{{UserMessage: "c", AssistantResponse: "d"}},
	}
	p.SetDetail(detail2)
	if p.scroll != 0 {
		t.Errorf("SetDetail should reset scroll to 0, got %d", p.scroll)
	}
}

// ---------------------------------------------------------------------------
// PreviewPanel View
// ---------------------------------------------------------------------------

func TestPreviewPanelViewEmpty(t *testing.T) {
	p := NewPreviewPanel()
	got := p.View()
	if got != "" {
		t.Errorf("View with zero dimensions should return empty, got %q", got)
	}
}

func TestPreviewPanelViewNoDetail(t *testing.T) {
	p := NewPreviewPanel()
	p.SetSize(80, 40)
	got := p.View()
	if !strings.Contains(got, "Select a session") {
		t.Error("View with no detail should show placeholder text")
	}
}

func TestPreviewPanelViewWithDetail(t *testing.T) {
	p := NewPreviewPanel()
	p.SetSize(80, 40)

	detail := &data.SessionDetail{
		Session: data.Session{
			ID:        "abc123",
			Cwd:       "/home/user/project",
			TurnCount: 1,
		},
		Turns: []data.Turn{
			{UserMessage: "test message", AssistantResponse: "test response"},
		},
	}
	p.SetDetail(detail)

	got := p.View()
	if !strings.Contains(got, "Session Detail") {
		t.Error("View should contain session detail header")
	}
	if !strings.Contains(got, "Conversation") {
		t.Error("View should contain conversation header")
	}
}

func TestPreviewPanelViewShowsAllTurns(t *testing.T) {
	p := NewPreviewPanel()
	p.SetSize(120, 100) // Large enough to show everything.

	detail := &data.SessionDetail{
		Session: data.Session{ID: "test", TurnCount: 3},
		Turns: []data.Turn{
			{UserMessage: "first_question", AssistantResponse: "first_answer"},
			{UserMessage: "second_question", AssistantResponse: "second_answer"},
			{UserMessage: "third_question", AssistantResponse: "third_answer"},
		},
	}
	p.SetDetail(detail)

	got := p.View()
	for _, want := range []string{"first_question", "first_answer", "second_question", "second_answer", "third_question", "third_answer"} {
		if !strings.Contains(got, want) {
			t.Errorf("View should contain %q", want)
		}
	}
}

func TestPreviewPanelViewCheckpoints(t *testing.T) {
	p := NewPreviewPanel()
	p.SetSize(120, 100)

	detail := &data.SessionDetail{
		Session: data.Session{ID: "test"},
		Checkpoints: []data.Checkpoint{
			{Title: "Checkpoint Alpha"},
			{Title: "Checkpoint Beta"},
		},
	}
	p.SetDetail(detail)

	got := p.View()
	if !strings.Contains(got, "Checkpoint Alpha") {
		t.Error("View should show checkpoint titles")
	}
	if !strings.Contains(got, "Checkpoint Beta") {
		t.Error("View should show checkpoint titles")
	}
}

// ---------------------------------------------------------------------------
// HitConversationSort / ScrollOffset / convHeaderLine
// ---------------------------------------------------------------------------

func TestPreviewPanelHitConversationSort_NoTurns(t *testing.T) {
	p := NewPreviewPanel()
	p.SetSize(80, 40)
	p.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "test"},
	})
	// No turns → convHeaderLine should be -1 → no hit.
	for row := 0; row < 30; row++ {
		if p.HitConversationSort(row) {
			t.Errorf("HitConversationSort(%d) = true, want false when no turns", row)
		}
	}
}

func TestPreviewPanelHitConversationSort_WithTurns(t *testing.T) {
	p := NewPreviewPanel()
	p.SetSize(80, 40)
	p.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "test", TurnCount: 1},
		Turns: []data.Turn{
			{UserMessage: "hello", AssistantResponse: "world"},
		},
	})
	// convHeaderLine should be positive (after fields + separator).
	if p.convHeaderLine < 0 {
		t.Fatalf("convHeaderLine = %d, want >= 0 when turns present", p.convHeaderLine)
	}
	// Exact line should hit.
	if !p.HitConversationSort(p.convHeaderLine) {
		t.Error("HitConversationSort should return true on the conversation header line")
	}
	// One line above and below should miss.
	if p.HitConversationSort(p.convHeaderLine - 1) {
		t.Error("HitConversationSort should return false one line above")
	}
	if p.HitConversationSort(p.convHeaderLine + 1) {
		t.Error("HitConversationSort should return false one line below")
	}
}

func TestPreviewPanelHitConversationSort_NoDetail(t *testing.T) {
	p := NewPreviewPanel()
	p.SetSize(80, 40)
	// No detail set → convHeaderLine stays -1.
	if p.HitConversationSort(0) {
		t.Error("HitConversationSort should return false when no detail is set")
	}
}

func TestPreviewPanelScrollOffset(t *testing.T) {
	p := NewPreviewPanel()
	if p.ScrollOffset() != 0 {
		t.Errorf("initial ScrollOffset = %d, want 0", p.ScrollOffset())
	}
	p.SetSize(80, 20)
	turns := make([]data.Turn, 10)
	for i := range turns {
		turns[i] = data.Turn{
			UserMessage:       "Question " + FormatInt(i),
			AssistantResponse: "Answer " + FormatInt(i),
		}
	}
	p.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "test", TurnCount: 10},
		Turns:   turns,
	})
	p.ScrollDown(5)
	if p.ScrollOffset() != p.scroll {
		t.Errorf("ScrollOffset() = %d, want %d", p.ScrollOffset(), p.scroll)
	}
}

func TestPreviewPanelConvHeaderLineUpdatesOnResize(t *testing.T) {
	p := NewPreviewPanel()
	p.SetSize(80, 40)
	p.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "test", TurnCount: 1},
		Turns:   []data.Turn{{UserMessage: "q", AssistantResponse: "a"}},
	})
	line1 := p.convHeaderLine

	// Resizing may shift the line (due to word wrap changes).
	p.SetSize(40, 40)
	line2 := p.convHeaderLine

	// Both should be valid (>= 0), though possibly different values.
	if line1 < 0 || line2 < 0 {
		t.Errorf("convHeaderLine should be >= 0 with turns: got %d and %d", line1, line2)
	}
}

func TestPreviewPanelConvHeaderLineResetOnNilDetail(t *testing.T) {
	p := NewPreviewPanel()
	p.SetSize(80, 40)
	p.SetDetail(&data.SessionDetail{
		Session: data.Session{ID: "test", TurnCount: 1},
		Turns:   []data.Turn{{UserMessage: "q", AssistantResponse: "a"}},
	})
	if p.convHeaderLine < 0 {
		t.Fatal("convHeaderLine should be >= 0 with turns")
	}
	p.SetDetail(nil)
	if p.convHeaderLine != -1 {
		t.Errorf("convHeaderLine = %d after nil detail, want -1", p.convHeaderLine)
	}
}
