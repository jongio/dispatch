package copilot

import (
	"context"
	"fmt"
	"testing"
)

func TestStreamEventTypes(t *testing.T) {
	// Verify that all stream event types are distinct.
	types := []StreamEventType{
		EventTextDelta,
		EventToolStart,
		EventToolDone,
		EventDone,
		EventError,
	}
	seen := make(map[StreamEventType]bool, len(types))
	for _, typ := range types {
		if seen[typ] {
			t.Errorf("duplicate StreamEventType value: %d", typ)
		}
		seen[typ] = true
	}
}

func TestStreamEventContent(t *testing.T) {
	ev := StreamEvent{Type: EventTextDelta, Content: "hello"}
	if ev.Type != EventTextDelta {
		t.Errorf("expected EventTextDelta, got %d", ev.Type)
	}
	if ev.Content != "hello" {
		t.Errorf("expected content 'hello', got %q", ev.Content)
	}
}

func TestNewClient(t *testing.T) {
	// New with nil store should not panic.
	c := New(nil)
	if c == nil {
		t.Fatal("New returned nil")
	}
	if c.Available() {
		t.Error("new client should not be available before Init")
	}
}

func TestClientInitWithoutStore(t *testing.T) {
	c := New(nil)
	// Available should be false before init.
	if c.Available() {
		t.Error("expected Available() == false before Init")
	}
	// InitError should be nil before any init attempt.
	if c.InitError() != nil {
		t.Errorf("expected nil InitError before Init, got %v", c.InitError())
	}
}

func TestClientCloseSafe(t *testing.T) {
	// Close on uninitialised client should not panic.
	c := New(nil)
	c.Close() // should be a no-op
	if c.Available() {
		t.Error("expected Available() == false after Close")
	}
	// Double close should be safe.
	c.Close()
}

func TestSearchUnavailableClient(t *testing.T) {
	// Search on an uninitialised client should return nil, nil (graceful no-op)
	// when init fails with a non-transport error.
	c := New(nil)
	c.hooks = &testHooks{
		doInit: func(_ context.Context) error {
			return fmt.Errorf("no store configured")
		},
	}
	ids, err := c.Search(context.Background(), "test query")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if ids != nil {
		t.Errorf("expected nil ids, got %v", ids)
	}
}

func TestParseSessionIDs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "valid JSON array",
			input:    `["abc-123", "def-456"]`,
			expected: []string{"abc-123", "def-456"},
		},
		{
			name:     "JSON with surrounding text",
			input:    `Here are the results: ["abc-123", "def-456"] Hope that helps!`,
			expected: []string{"abc-123", "def-456"},
		},
		{
			name:     "markdown fenced JSON",
			input:    "```json\n[\"abc-123\", \"def-456\"]\n```",
			expected: []string{"abc-123", "def-456"},
		},
		{
			name:     "empty array",
			input:    `[]`,
			expected: nil,
		},
		{
			name:     "no JSON",
			input:    "I couldn't find any sessions.",
			expected: nil,
		},
		{
			name:     "duplicates removed",
			input:    `["abc-123", "abc-123", "def-456"]`,
			expected: []string{"abc-123", "def-456"},
		},
		{
			name:     "whitespace in IDs trimmed",
			input:    `[" abc-123 ", "def-456"]`,
			expected: []string{"abc-123", "def-456"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSessionIDs(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d IDs, got %d: %v", len(tt.expected), len(result), result)
			}
			for i := range tt.expected {
				if result[i] != tt.expected[i] {
					t.Errorf("ID[%d]: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}
