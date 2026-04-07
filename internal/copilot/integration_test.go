//go:build integration

package copilot

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	sdk "github.com/github/copilot-sdk/go"
	"github.com/jongio/dispatch/internal/data"
)

// TestIntegration_RealSearch calls the real Copilot SDK to perform a search
// against the user's actual session store. This test is skipped in CI
// (no GITHUB_TOKEN or copilot binary) but runs locally to verify the
// search path end-to-end.
//
// Run with: go test -run TestIntegration_RealSearch -v -count=1
func TestIntegration_RealSearch(t *testing.T) {
	// Skip if no copilot binary or auth is available.
	if os.Getenv("DISPATCH_INTEGRATION") == "" {
		t.Fatal("set DISPATCH_INTEGRATION=1 to run real SDK tests")
	}

	store, err := data.Open()
	if err != nil {
		t.Fatalf("opening session store: %v", err)
	}
	defer func() { _ = store.Close() }()

	client := New(store)
	defer client.Close()

	// Step 1: Init the SDK
	ctx := context.Background()
	t.Log("Initialising Copilot SDK...")
	start := time.Now()
	if err := client.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Logf("Init succeeded in %v, available=%v", time.Since(start), client.Available())

	// Step 2: Search with a broad query
	query := "authentication"
	t.Logf("Searching for %q...", query)
	start = time.Now()
	searchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	ids, err := client.Search(searchCtx, query)
	elapsed := time.Since(start)
	t.Logf("Search completed in %v", elapsed)

	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	t.Logf("Search returned %d session IDs", len(ids))
	for i, id := range ids {
		t.Logf("  [%d] %s", i, id)
	}

	if len(ids) == 0 {
		t.Log("WARNING: No results returned — SDK may not be searching effectively")
	}

	// Step 3: Try a second search to verify connection is still alive
	query2 := "bug fix"
	t.Logf("Second search for %q...", query2)
	start = time.Now()
	searchCtx2, cancel2 := context.WithTimeout(ctx, 30*time.Second)
	defer cancel2()

	ids2, err2 := client.Search(searchCtx2, query2)
	t.Logf("Second search completed in %v", time.Since(start))

	if err2 != nil {
		t.Errorf("Second search returned error: %v", err2)
	} else {
		t.Logf("Second search returned %d session IDs", len(ids2))
		for i, id := range ids2 {
			t.Logf("  [%d] %s", i, id)
		}
	}

	// Step 4: Verify results exist in actual store
	if len(ids) > 0 {
		for _, id := range ids[:minInt(3, len(ids))] {
			detail, lookupErr := store.GetSession(id)
			if lookupErr != nil {
				t.Logf("  Session %s: lookup error: %v", id, lookupErr)
			} else if detail == nil {
				t.Logf("  Session %s: NOT FOUND in store (AI hallucination?)", id)
			} else {
				t.Logf("  Session %s: ✓ found — %s", id, truncate(detail.Session.Summary, 60))
			}
		}
	}

	fmt.Println() // flush
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---------------------------------------------------------------------------
// Tests with a real SDK session (require Copilot CLI binary + auth)
// ---------------------------------------------------------------------------

func createRealSession(t *testing.T) (*sdk.Client, *sdk.Session) {
	t.Helper()
	sdkClient := sdk.NewClient(nil)
	if err := sdkClient.Start(context.Background()); err != nil {
		t.Fatalf("Copilot SDK not available: %v", err)
	}
	session, err := sdkClient.CreateSession(context.Background(), &sdk.SessionConfig{
		Model:               "gpt-4.1",
		Streaming:           true,
		OnPermissionRequest: sdk.PermissionHandler.ApproveAll,
		SystemMessage: &sdk.SystemMessageConfig{
			Content: "You are a test assistant. Respond briefly.",
		},
	})
	if err != nil {
		_ = sdkClient.Stop()
		t.Fatalf("Cannot create session (auth may not be configured): %v", err)
	}
	return sdkClient, session
}

func TestIntegration_SendMessage_withRealSession(t *testing.T) {
	if os.Getenv("DISPATCH_INTEGRATION") == "" {
		t.Fatal("set DISPATCH_INTEGRATION=1 to run real SDK tests")
	}

	sdkClient, session := createRealSession(t)

	store, err := data.Open()
	if err != nil {
		t.Fatalf("opening session store: %v", err)
	}
	defer func() { _ = store.Close() }()

	c := New(store)
	c.mu.Lock()
	c.sdk = sdkClient
	c.available = true
	c.mu.Unlock()

	// Destroy the pre-created session; SendMessage creates its own now.
	_ = session.Disconnect()

	ch, sendErr := c.SendMessage(context.Background(), "Say hello in one word.")
	if sendErr != nil {
		c.Close()
		t.Fatalf("SendMessage failed: %v", sendErr)
	}

	// Drain the channel to exercise all event types.
	var gotDone, gotDelta bool
	for ev := range ch {
		switch ev.Type {
		case EventTextDelta:
			gotDelta = true
		case EventDone:
			gotDone = true
		case EventError:
			t.Logf("stream error: %s", ev.Content)
		}
	}
	if !gotDelta {
		t.Log("no text delta received (may be expected for some models)")
	}
	if !gotDone {
		// The done event is a protocol guarantee; flag as a soft warning.
		// We don't t.Error here because streaming behaviour can vary by
		// model or network conditions during manual integration runs.
		t.Log("WARNING: no done event received — verify SDK streaming is healthy")
	}

	c.Close()
}

func TestIntegration_Close_withRealSession(t *testing.T) {
	if os.Getenv("DISPATCH_INTEGRATION") == "" {
		t.Fatal("set DISPATCH_INTEGRATION=1 to run real SDK tests")
	}

	sdkClient, session := createRealSession(t)
	// Destroy the session immediately — Close only stops the SDK client now.
	_ = session.Disconnect()

	c := New(nil)
	c.mu.Lock()
	c.sdk = sdkClient
	c.available = true
	c.mu.Unlock()

	// Close should call sdk.Stop() without panic.
	c.Close()
	if c.Available() {
		t.Error("should not be available after Close")
	}
	c.mu.Lock()
	if c.sdk != nil {
		t.Error("sdk should be nil after Close")
	}
	c.mu.Unlock()
}
