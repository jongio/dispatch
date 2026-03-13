package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	sdk "github.com/github/copilot-sdk/go"
	"github.com/jongio/dispatch/internal/data"
)

// streamEventBufSize is the buffer capacity for the streaming event channel.
// A moderate buffer prevents backpressure from the TUI render loop from
// blocking the SDK goroutine while keeping memory usage bounded.
const streamEventBufSize = 64

// searchMaxRetries is the maximum number of reinitialisation attempts when the
// SDK transport pipe breaks (e.g. "file already closed").
const searchMaxRetries = 3

// searchRetryDelay is the pause between retry attempts to let the pipe fully
// close and reopen before reinitialising the SDK.
const searchRetryDelay = 500 * time.Millisecond

// searchOperationTimeout is the timeout for individual SDK operations
// (Start, CreateSession, SendAndWait).  These use a non-cancellable
// (background) context to prevent context cancellation from interrupting
// the SDK's JSON-RPC pipe mid-read/write, which causes "file already
// closed" errors.
const searchOperationTimeout = 30 * time.Second

// testHooks allows tests to override internal behaviour without a real
// Copilot SDK process. Only tests in this package set these fields;
// production code leaves hooks nil.
type testHooks struct {
	doSearch func(ctx context.Context, query string) ([]string, error)
	doInit   func(ctx context.Context) error
}

// Client wraps the Copilot SDK to provide search and streaming chat.
// It lazily initialises the SDK client on the first search/message,
// and creates sessions on-demand (no idle background sessions).
type Client struct {
	store *data.Store
	sdk   *sdk.Client

	mu        sync.Mutex
	available bool // true after successful initialisation
	initErr   error

	// searchMu serialises Search() calls so that only one goroutine uses
	// the SDK's JSON-RPC pipe at a time.  Without this, concurrent
	// goroutines can interleave CreateSession/SendAndWait messages on the
	// same pipe, corrupting the transport ("file already closed").
	searchMu sync.Mutex

	hooks *testHooks // nil in production
}

// New creates a new copilot Client backed by the given data store.
// The SDK is not started until Init is called.
func New(store *data.Store) *Client {
	return &Client{store: store}
}

// Init starts the Copilot SDK client. Sessions are created on-demand by
// Search and SendMessage. This is safe to call from a goroutine;
// subsequent calls are no-ops if already initialised.
func (c *Client) Init(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.available {
		return nil // already initialised
	}
	if c.initErr != nil {
		return c.initErr // previous init failed
	}

	// Test hook: allows tests to control init without a real SDK process.
	if c.hooks != nil && c.hooks.doInit != nil {
		if err := c.hooks.doInit(ctx); err != nil {
			// Don't cache context cancellation — it's transient, not a
			// permanent failure.  The next caller with a live context
			// should be allowed to retry.
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				c.initErr = fmt.Errorf("starting Copilot SDK: %w", err)
			}
			return fmt.Errorf("starting Copilot SDK: %w", err)
		}
		c.available = true
		return nil
	}

	opts := &sdk.ClientOptions{
		LogLevel: "error",
	}
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		opts.GitHubToken = tok
	}
	client := sdk.NewClient(opts)
	if err := client.Start(ctx); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			c.initErr = fmt.Errorf("starting Copilot SDK: %w", err)
		}
		return fmt.Errorf("starting Copilot SDK: %w", err)
	}
	c.sdk = client
	c.available = true
	return nil
}

// Available returns true when the Copilot SDK is initialised and ready.
func (c *Client) Available() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.available
}

// InitError returns the error from the last failed Init call, or nil.
func (c *Client) InitError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.initErr
}

// SendMessage sends a user prompt to the Copilot session and returns a
// channel of StreamEvents that the TUI can consume. The channel is
// closed when the response is complete or an error occurs.
//
// The caller should range over the channel to receive events:
//
//	ch, err := client.SendMessage(ctx, "find auth sessions")
//	for ev := range ch {
//	    switch ev.Type { ... }
//	}
func (c *Client) SendMessage(ctx context.Context, prompt string) (<-chan StreamEvent, error) {
	c.mu.Lock()
	if !c.available || c.sdk == nil {
		c.mu.Unlock()
		return nil, errors.New("copilot session not available")
	}
	sdkClient := c.sdk
	store := c.store
	c.mu.Unlock()

	// Create a dedicated streaming session for this message.
	tools := defineTools(store)
	session, err := sdkClient.CreateSession(ctx, &sdk.SessionConfig{
		Model:               "gpt-4.1",
		Streaming:           true,
		Tools:               tools,
		OnPermissionRequest: sdk.PermissionHandler.ApproveAll,
		SystemMessage: &sdk.SystemMessageConfig{
			Content: "You are an AI assistant integrated into the Dispatch session browser. " +
				"Your job is to help users find and explore their Copilot CLI sessions. " +
				"Use the provided tools to search sessions, get details, and list repositories. " +
				"When presenting session results, include the session ID, summary, repository, " +
				"branch, and timestamps. Be concise and helpful.",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating streaming session: %w", err)
	}

	ch := make(chan StreamEvent, streamEventBufSize)

	// done is closed when the goroutine is shutting down the channel.
	// The event handler checks this before sending to avoid a panic
	// from writing to a closed channel (race between late SDK
	// callbacks and the deferred close).
	done := make(chan struct{})
	var closeOnce sync.Once

	// trySend sends an event on ch unless the channel is being closed.
	trySend := func(ev StreamEvent) {
		select {
		case <-done:
			// Channel is closing / closed — drop the event.
		case ch <- ev:
		}
	}

	// Subscribe to session events before sending the message so we
	// don't miss any streaming deltas.
	unsubscribe := session.On(func(event sdk.SessionEvent) {
		switch event.Type {
		case sdk.AssistantMessageDelta:
			if event.Data.DeltaContent != nil {
				trySend(StreamEvent{Type: EventTextDelta, Content: *event.Data.DeltaContent})
			}
		case sdk.ToolExecutionStart:
			name := ""
			if event.Data.ToolName != nil {
				name = *event.Data.ToolName
			}
			trySend(StreamEvent{Type: EventToolStart, Content: name})
		case sdk.ToolExecutionComplete:
			name := ""
			if event.Data.ToolName != nil {
				name = *event.Data.ToolName
			}
			trySend(StreamEvent{Type: EventToolDone, Content: name})
		case sdk.SessionIdle:
			trySend(StreamEvent{Type: EventDone})
		case sdk.SessionError:
			msg := "unknown error"
			if event.Data.Content != nil {
				msg = *event.Data.Content
			}
			trySend(StreamEvent{Type: EventError, Content: msg})
		default:
			// Ignore unhandled SDK event types.
		}
	})

	// Send the message asynchronously. The event handler above will
	// deliver streaming deltas to the channel.
	go func() {
		defer func() {
			unsubscribe()
			_ = session.Disconnect()
			// Signal the done channel first so trySend stops writing,
			// then close ch exactly once.
			closeOnce.Do(func() {
				close(done)
				close(ch)
			})
		}()

		_, err := session.SendAndWait(ctx, sdk.MessageOptions{
			Prompt: prompt,
		})
		if err != nil {
			trySend(StreamEvent{Type: EventError, Content: err.Error()})
		}
	}()

	return ch, nil
}

// Search asks the Copilot SDK to find sessions matching the given query.
// It returns a list of session IDs found by AI-powered search.
// This is non-streaming — it sends the query and waits for the final result.
// If the client is not available, it returns nil, nil (graceful no-op).
//
// Search is self-contained: it calls Init automatically and handles
// transport errors (e.g. "file already closed") by resetting the SDK
// and retrying.  After all retries are exhausted it clears cached
// errors so the next call can attempt a fresh start.
func (c *Client) Search(ctx context.Context, query string) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	searchStart := time.Now()
	slog.Debug("copilot search starting", "query", query)

	// Bail immediately if the caller already cancelled (e.g. a newer
	// search superseded this one while it was queued).
	if err := ctx.Err(); err != nil {
		slog.Debug("copilot search: context already cancelled", "error", err)
		return nil, nil
	}

	// Serialise search calls so only one goroutine uses the SDK pipe at
	// a time.  This prevents the interleaved-JSON-RPC corruption that
	// causes "file already closed".
	c.searchMu.Lock()
	defer c.searchMu.Unlock()

	// Re-check after acquiring the lock — another goroutine may have
	// been holding the lock for seconds.
	if err := ctx.Err(); err != nil {
		slog.Debug("copilot search: context cancelled while waiting for lock", "error", err)
		return nil, nil
	}

	// Ensure the SDK is initialised before searching.
	// Use a non-cancellable context so that cancellation during Start()
	// cannot leave the subprocess half-initialised.
	initCtx, initCancel := context.WithTimeout(context.Background(), searchOperationTimeout)
	defer initCancel()
	if err := c.Init(initCtx); err != nil {
		if isTransportError(err) {
			// Cached transport error from a previous failure — clear and retry.
			slog.Debug("copilot search: cached transport error, resetting", "error", err)
			c.resetSDK()
			if retryErr := c.Init(initCtx); retryErr != nil {
				slog.Debug("copilot search: reinit failed, giving up", "error", retryErr)
				return nil, nil // still can't init — graceful no-op
			}
		} else {
			slog.Debug("copilot search: init failed (non-transport)", "error", err)
			return nil, nil // non-transport init failure — graceful no-op
		}
	}

	// Re-check caller's context after init.
	if ctx.Err() != nil {
		slog.Debug("copilot search: context cancelled after init")
		return nil, nil
	}

	ids, err := c.doSearch(ctx, query)

	// If the context was cancelled, the SDK operations completed cleanly
	// on their own background context — the JSON-RPC pipe is healthy and
	// reusable.  Just discard results; do NOT reset the SDK.
	if ctx.Err() != nil {
		slog.Debug("copilot search: context cancelled, discarding results (pipe healthy)",
			"ctxErr", ctx.Err())
		return nil, nil
	}

	if err == nil {
		slog.Debug("copilot search complete",
			"results", len(ids), "duration", time.Since(searchStart))
		return ids, nil
	}

	// Any doSearch error triggers the retry loop.  After the first
	// successful search, the SDK subprocess can become unstable (e.g.
	// session.Destroy corrupts internal state).  Restarting the
	// subprocess is the only reliable recovery, regardless of whether
	// the error looks like a transport failure.
	for attempt := 1; attempt <= searchMaxRetries; attempt++ {
		slog.Debug("copilot search: error, retrying with fresh SDK",
			"attempt", attempt,
			"maxRetries", searchMaxRetries,
			"error", err)

		// Context-aware delay — exit early if cancelled.
		select {
		case <-ctx.Done():
			slog.Debug("copilot search: cancelled during retry delay")
			return nil, nil
		case <-time.After(searchRetryDelay):
		}

		c.resetSDK()
		if ctx.Err() != nil {
			return nil, nil
		}
		retryInitCtx, retryInitCancel := context.WithTimeout(context.Background(), searchOperationTimeout)
		if initErr := c.Init(retryInitCtx); initErr != nil {
			retryInitCancel()
			err = fmt.Errorf("reinit attempt %d: %w", attempt, initErr)
			continue
		}
		retryInitCancel()

		if ctx.Err() != nil {
			return nil, nil
		}
		ids, err = c.doSearch(ctx, query)
		if ctx.Err() != nil {
			slog.Debug("copilot search: context cancelled during retry, discarding",
				"attempt", attempt)
			return nil, nil
		}
		if err == nil {
			slog.Debug("copilot search: retry succeeded",
				"attempt", attempt, "results", len(ids),
				"duration", time.Since(searchStart))
			return ids, nil
		}
	}

	// All retries exhausted — clear cached errors so the next Search()
	// call can attempt a fresh initialisation instead of returning the
	// stale error immediately.
	c.resetSDK()
	slog.Debug("copilot search: all retries exhausted",
		"duration", time.Since(searchStart), "error", err)
	return nil, fmt.Errorf("search unavailable after %d retries: %w", searchMaxRetries, err)
}

// doSearch performs a single search attempt against the SDK.
//
// SDK operations (CreateSession, SendAndWait) use a non-cancellable
// background context so that context cancellation from the TUI (e.g.
// user typed a new query) cannot interrupt the JSON-RPC pipe mid-
// read/write.  The caller's ctx is checked between operations so we
// can bail early without damaging the SDK's internal state.
func (c *Client) doSearch(ctx context.Context, query string) ([]string, error) {
	// Test hook: allows tests to inject search behaviour.
	if c.hooks != nil && c.hooks.doSearch != nil {
		return c.hooks.doSearch(ctx, query)
	}

	c.mu.Lock()
	if !c.available || c.sdk == nil {
		c.mu.Unlock()
		return nil, nil
	}
	sdkClient := c.sdk
	store := c.store
	c.mu.Unlock()

	// Use a non-cancellable context for SDK pipe operations.  Cancelling
	// mid-SendAndWait corrupts the JSON-RPC pipe and causes "file already
	// closed" errors on subsequent searches.
	sdkCtx, sdkCancel := context.WithTimeout(context.Background(), searchOperationTimeout)
	defer sdkCancel()

	// Create a dedicated session for search with a focused system message.
	tools := defineTools(store)
	session, err := sdkClient.CreateSession(sdkCtx, &sdk.SessionConfig{
		Model:               "gpt-4.1",
		Streaming:           false,
		Tools:               tools,
		OnPermissionRequest: sdk.PermissionHandler.ApproveAll,
		SystemMessage: &sdk.SystemMessageConfig{
			Content: "You are a session search assistant for a coding session browser. " +
				"Given a search query, use the search_sessions and search_deep tools to find " +
				"relevant coding sessions. Think about synonyms, related terms, and alternative " +
				"phrasings of the query. Return ONLY a JSON array of session ID strings that match, " +
				"nothing else. Example: [\"abc-123\", \"def-456\"]",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating search session: %w", err)
	}
	defer func() { _ = session.Disconnect() }()

	// If the caller cancelled while CreateSession was running, skip the
	// expensive SendAndWait.  The session was created cleanly and
	// Destroy() will clean it up normally.
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	resp, err := session.SendAndWait(sdkCtx, sdk.MessageOptions{
		Prompt: "Search for Copilot CLI coding sessions that include: " + QuoteUntrusted(query),
	})
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}

	if resp == nil || resp.Data.Content == nil {
		return nil, nil
	}
	return parseSessionIDs(*resp.Data.Content), nil
}

// isTransportError returns true if the error indicates the SDK's
// underlying transport (JSON-RPC pipe) is broken.
func isTransportError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "file already closed") ||
		strings.Contains(msg, "error reading header") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset")
}

// resetSDK tears down the current SDK client so Init can restart it.
func (c *Client) resetSDK() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sdk != nil {
		_ = c.sdk.Stop()
		c.sdk = nil
	}
	c.available = false
	c.initErr = nil // clear so Init can retry
}

// maxParsedSessionIDs caps the number of session IDs extracted from a
// model response. This prevents memory exhaustion if the model returns
// a huge JSON array (defence-in-depth against malicious/poisoned responses).
const maxParsedSessionIDs = 200

// parseSessionIDs extracts session IDs from the model's response text.
// It expects a JSON array of strings, e.g. ["abc-123","def-456"].
// If parsing fails, it returns nil.
func parseSessionIDs(text string) []string {
	text = strings.TrimSpace(text)

	// Strip markdown fencing if present (```json ... ```).
	if rest, ok := strings.CutPrefix(text, "```"); ok {
		if i := strings.Index(rest, "\n"); i >= 0 {
			text = rest[i+1:]
		}
		if trimmed, ok := strings.CutSuffix(text, "```"); ok {
			text = trimmed
		}
		text = strings.TrimSpace(text)
	}

	// Find the JSON array boundaries.
	start := strings.IndexByte(text, '[')
	end := strings.LastIndexByte(text, ']')
	if start < 0 || end < 0 || end <= start {
		return nil
	}
	text = text[start : end+1]

	var ids []string
	if err := json.Unmarshal([]byte(text), &ids); err != nil {
		return nil
	}

	// Cap before deduplication to limit allocation from oversized responses.
	if len(ids) > maxParsedSessionIDs {
		ids = ids[:maxParsedSessionIDs]
	}

	// Deduplicate while preserving order.
	seen := make(map[string]struct{}, len(ids))
	deduped := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				deduped = append(deduped, id)
			}
		}
	}
	return deduped
}

// Close shuts down the Copilot SDK client. Safe to call
// multiple times or on an uninitialised client.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sdk != nil {
		_ = c.sdk.Stop()
		c.sdk = nil
	}
	c.available = false
}
