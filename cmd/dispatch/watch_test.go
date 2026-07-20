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

// TestRunWatch_InterruptedNotIdle verifies an interrupted session is counted as
// interrupted, not silently bucketed into the idle total (regression).
func TestRunWatch_InterruptedNotIdle(t *testing.T) {
	attention := map[string]data.AttentionStatus{
		"ses-1": data.AttentionInterrupted,
		"ses-2": data.AttentionIdle,
	}
	sessions := []data.Session{
		{ID: "ses-1", Summary: "Crashed"},
		{ID: "ses-2", Summary: "Quiet"},
	}
	withWatchSeams(t, attention, sessions)

	var buf bytes.Buffer
	if err := runWatch(&buf, []string{"watch", "--once", "--json"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got watchSnapshot
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if got.Interrupted != 1 {
		t.Errorf("Interrupted = %d, want 1", got.Interrupted)
	}
	if got.Idle != 1 {
		t.Errorf("Idle = %d, want 1 (only the truly idle session)", got.Idle)
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

func TestRunWatch_FilteredByStatus(t *testing.T) {
	attention := map[string]data.AttentionStatus{
		"ses-1": data.AttentionWaiting,
		"ses-2": data.AttentionIdle,
		"ses-3": data.AttentionWaiting,
	}
	sessions := []data.Session{
		{ID: "ses-1", Summary: "Auth"},
		{ID: "ses-2", Summary: "Quiet"},
		{ID: "ses-3", Summary: "Review"},
	}
	withWatchSeams(t, attention, sessions)

	var buf bytes.Buffer
	if err := runWatch(&buf, []string{"watch", "--once", "--json", "--status", "waiting"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got watchSnapshot
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Total != 2 || got.Waiting != 2 || got.Idle != 0 {
		t.Fatalf("snapshot = %+v, want two waiting sessions only", got)
	}
	if len(got.Sessions) != 2 {
		t.Fatalf("sessions len = %d, want 2", len(got.Sessions))
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

func TestParseWatchArgs_Status(t *testing.T) {
	opts, err := parseWatchArgs([]string{"watch", "--status", "Interrupted"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.hasStatus || opts.status != data.AttentionInterrupted {
		t.Fatalf("status filter = %v %v, want interrupted", opts.hasStatus, opts.status)
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

func TestParseWatchArgs_StatusErrors(t *testing.T) {
	for _, args := range [][]string{
		{"watch", "--status"},
		{"watch", "--status", "blocked"},
	} {
		if _, err := parseWatchArgs(args); err == nil {
			t.Fatalf("parseWatchArgs(%v) returned nil error", args)
		}
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
