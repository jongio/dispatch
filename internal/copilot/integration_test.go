package copilot

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

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
		t.Skip("set DISPATCH_INTEGRATION=1 to run real SDK tests")
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
