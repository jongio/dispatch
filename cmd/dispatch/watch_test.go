package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/update"
)

func withWatchSeams(t *testing.T, attention map[string]data.AttentionStatus, sessions []data.Session) {
	t.Helper()

	prevScan := watchScanAttentionFn
	watchScanAttentionFn = func(time.Duration) map[string]data.AttentionStatus {
		return attention
	}
	t.Cleanup(func() { watchScanAttentionFn = prevScan })

	prevList := watchListSessionsFn
	watchListSessionsFn = func(data.FilterOptions) ([]data.Session, error) {
		return sessions, nil
	}
	t.Cleanup(func() { watchListSessionsFn = prevList })

	prevBell := bellFn
	bellFn = func() {} // suppress bell in tests
	t.Cleanup(func() { bellFn = prevBell })
}

func TestRunWatch_OnceText(t *testing.T) {
	attention := map[string]data.AttentionStatus{
		"ses-1": data.AttentionWaiting,
		"ses-2": data.AttentionIdle,
		"ses-3": data.AttentionWorking,
	}
	sessions := []data.Session{
		{ID: "ses-1", Summary: "Auth work"},
		{ID: "ses-2", Summary: "Old session"},
		{ID: "ses-3", Summary: "Building"},
	}
	withWatchSeams(t, attention, sessions)

	var buf bytes.Buffer
	if err := runWatch(&buf, []string{"watch", "--once"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"waiting", "working", "Total: 3", "Auth work"} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestRunWatch_OnceJSON(t *testing.T) {
	attention := map[string]data.AttentionStatus{
		"ses-1": data.AttentionWaiting,
	}
	sessions := []data.Session{
		{ID: "ses-1", Summary: "Auth work"},
	}
	withWatchSeams(t, attention, sessions)

	var buf bytes.Buffer
	if err := runWatch(&buf, []string{"watch", "--once", "--json"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got watchSnapshot
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Waiting != 1 {
		t.Errorf("waiting = %d, want 1", got.Waiting)
	}
	if got.Total != 1 {
		t.Errorf("total = %d, want 1", got.Total)
	}
}

func TestRunWatch_OnceEmpty(t *testing.T) {
	withWatchSeams(t, map[string]data.AttentionStatus{}, nil)

	var buf bytes.Buffer
	if err := runWatch(&buf, []string{"watch", "--once"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("No sessions found")) {
		t.Errorf("expected empty message, got:\n%s", out)
	}
}

func TestRunWatch_FilteredByRepo(t *testing.T) {
	attention := map[string]data.AttentionStatus{
		"ses-1": data.AttentionWaiting,
		"ses-2": data.AttentionWaiting,
	}
	sessions := []data.Session{
		{ID: "ses-1", Summary: "Auth", Repository: "jongio/dispatch"},
	}
	withWatchSeams(t, attention, sessions)

	var buf bytes.Buffer
	if err := runWatch(&buf, []string{"watch", "--once", "--repo", "jongio/dispatch"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got watchSnapshot
	// Use JSON for precise checking.
	buf.Reset()
	if err := runWatch(&buf, []string{"watch", "--once", "--json", "--repo", "jongio/dispatch"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Total != 1 {
		t.Errorf("total = %d, want 1 (filtered)", got.Total)
	}
}

func TestParseWatchArgs_Interval(t *testing.T) {
	opts, err := parseWatchArgs([]string{"watch", "--interval", "10s"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.interval != 10*time.Second {
		t.Errorf("interval = %v, want 10s", opts.interval)
	}
}

func TestParseWatchArgs_IntervalClamped(t *testing.T) {
	opts, err := parseWatchArgs([]string{"watch", "--interval", "100ms"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.interval < time.Second {
		t.Errorf("interval = %v, should be clamped to at least 1s", opts.interval)
	}
}

func TestParseWatchArgs_UnknownFlag(t *testing.T) {
	_, err := parseWatchArgs([]string{"watch", "--nope"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestParseWatchArgs_MissingIntervalValue(t *testing.T) {
	_, err := parseWatchArgs([]string{"watch", "--interval"})
	if err == nil {
		t.Fatal("expected error for missing interval value")
	}
}

func TestRunWatch_ListSessionsError(t *testing.T) {
	prevScan := watchScanAttentionFn
	watchScanAttentionFn = func(time.Duration) map[string]data.AttentionStatus {
		return map[string]data.AttentionStatus{}
	}
	t.Cleanup(func() { watchScanAttentionFn = prevScan })

	prevList := watchListSessionsFn
	watchListSessionsFn = func(data.FilterOptions) ([]data.Session, error) {
		return nil, errors.New("store error")
	}
	t.Cleanup(func() { watchListSessionsFn = prevList })

	err := runWatch(&bytes.Buffer{}, []string{"watch", "--once"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleArgs_Watch(t *testing.T) {
	withWatchSeams(t, map[string]data.AttentionStatus{}, nil)

	ch := make(chan *update.UpdateInfo, 1)
	ch <- nil

	done, _, _, err := handleArgs([]string{"watch", "--once"}, &bytes.Buffer{}, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true")
	}
}
