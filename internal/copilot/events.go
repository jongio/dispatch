// Package copilot wraps the GitHub Copilot SDK to provide a
// streaming AI chat interface for the Dispatch TUI.
package copilot

// StreamEventType classifies events flowing from the Copilot session
// to the TUI chat panel.
type StreamEventType int

const (
	// EventTextDelta carries an incremental text chunk from the assistant.
	EventTextDelta StreamEventType = iota
	// EventToolStart signals the assistant is invoking a tool.
	EventToolStart
	// EventToolDone signals a tool invocation has completed.
	EventToolDone
	// EventDone signals the assistant has finished its response.
	EventDone
	// EventError carries an error from the Copilot session.
	EventError
)

// StreamEvent is a single event delivered to the TUI via a channel.
type StreamEvent struct {
	Type    StreamEventType
	Content string // text delta, tool name, or error message
}
