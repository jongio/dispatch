package main

import (
	"errors"
	"testing"
	"time"

	"github.com/jongio/dispatch/internal/data"
)

func TestScanFilteredScopedLookupFailureReturnsError(t *testing.T) {
	prevScan := watchScanAttentionFn
	watchScanAttentionFn = func(time.Duration) map[string]data.AttentionStatus {
		return map[string]data.AttentionStatus{"unrelated": data.AttentionWaiting}
	}
	t.Cleanup(func() { watchScanAttentionFn = prevScan })

	prevList := watchListSessionsFn
	watchListSessionsFn = func(data.FilterOptions) ([]data.Session, error) {
		return nil, errors.New("metadata unavailable")
	}
	t.Cleanup(func() { watchListSessionsFn = prevList })

	current, err := scanFiltered(watchOptions{repo: "owner/repo"})
	if err == nil {
		t.Fatal("expected scoped metadata error")
	}
	if current != nil {
		t.Fatalf("scoped failure returned unscoped attention: %#v", current)
	}
}
