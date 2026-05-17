package copilot

import (
"context"
"fmt"
"strings"
"sync/atomic"
"testing"
"time"
)

// ---------------------------------------------------------------------------
// doAnalyze — direct coverage (issue #84)
// ---------------------------------------------------------------------------

// TestCoverage_doAnalyze_sdkNotAvailable verifies that doAnalyze returns
// (nil, nil) when the SDK has not been initialised (no hook, available=false).
func TestCoverage_doAnalyze_sdkNotAvailable(t *testing.T) {
t.Parallel()
c := New(nil) // available=false, sdk=nil, hooks=nil
result, err := c.doAnalyze(context.Background(), "sess-1", "plan")
if err != nil {
t.Errorf("expected nil error, got: %v", err)
}
if result != nil {
t.Error("expected nil result when SDK not available")
}
}

// TestCoverage_doAnalyze_hookPassesArguments confirms the hook receives
// the exact sessionID and planContent the caller provided.
func TestCoverage_doAnalyze_hookPassesArguments(t *testing.T) {
t.Parallel()
var gotSessionID, gotPlan string
c := New(nil)
c.hooks = &testHooks{
doAnalyze: func(_ context.Context, sid, plan string) (*CompletionAnalysis, error) {
gotSessionID = sid
gotPlan = plan
return &CompletionAnalysis{Complete: true, Summary: "ok"}, nil
},
}
_, _ = c.doAnalyze(context.Background(), "my-session", "my-plan")
if gotSessionID != "my-session" {
t.Errorf("sessionID = %q, want %q", gotSessionID, "my-session")
}
if gotPlan != "my-plan" {
t.Errorf("planContent = %q, want %q", gotPlan, "my-plan")
}
}

// TestCoverage_doAnalyze_hookErrorPropagates verifies that an error from
// the hook is returned unmodified to the caller.
func TestCoverage_doAnalyze_hookErrorPropagates(t *testing.T) {
t.Parallel()
c := New(nil)
c.hooks = &testHooks{
doAnalyze: func(_ context.Context, _, _ string) (*CompletionAnalysis, error) {
return nil, fmt.Errorf("synthetic failure")
},
}
result, err := c.doAnalyze(context.Background(), "s", "p")
if err == nil || !strings.Contains(err.Error(), "synthetic failure") {
t.Errorf("expected 'synthetic failure' error, got: %v", err)
}
if result != nil {
t.Error("expected nil result on error")
}
}

// TestCoverage_doAnalyze_hookEmptyPlan verifies the hook is still invoked
// when planContent is empty.
func TestCoverage_doAnalyze_hookEmptyPlan(t *testing.T) {
t.Parallel()
var called bool
c := New(nil)
c.hooks = &testHooks{
doAnalyze: func(_ context.Context, _, plan string) (*CompletionAnalysis, error) {
called = true
if plan != "" {
t.Errorf("expected empty plan, got %q", plan)
}
return &CompletionAnalysis{Complete: true, Summary: "nothing to do"}, nil
},
}
result, err := c.doAnalyze(context.Background(), "s", "")
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if !called {
t.Fatal("hook was not called")
}
if !result.Complete {
t.Error("expected Complete=true")
}
}

// TestCoverage_doAnalyze_hookContextCancelled verifies that a cancelled
// context is forwarded to the hook.
func TestCoverage_doAnalyze_hookContextCancelled(t *testing.T) {
t.Parallel()
c := New(nil)
c.hooks = &testHooks{
doAnalyze: func(ctx context.Context, _, _ string) (*CompletionAnalysis, error) {
return nil, ctx.Err()
},
}
ctx, cancel := context.WithCancel(context.Background())
cancel()
_, err := c.doAnalyze(ctx, "s", "p")
if err == nil {
t.Error("expected error from cancelled context")
}
}

// ---------------------------------------------------------------------------
// AnalyzeCompletion — caller retry & error propagation (issue #84)
// ---------------------------------------------------------------------------

// TestCoverage_AnalyzeCompletion_retrySucceedsOnSecondAttempt exercises the
// retry loop where the first doAnalyze call fails but the second succeeds.
func TestCoverage_AnalyzeCompletion_retrySucceedsOnSecondAttempt(t *testing.T) {
t.Parallel()
var calls int32
c := New(nil)
c.hooks = &testHooks{
doInit: func(_ context.Context) error { return nil },
doAnalyze: func(_ context.Context, _, _ string) (*CompletionAnalysis, error) {
n := atomic.AddInt32(&calls, 1)
if n == 1 {
return nil, fmt.Errorf("transient error")
}
return &CompletionAnalysis{
Complete:       true,
TotalTasks:     2,
CompletedTasks: 2,
Summary:        "retry success",
}, nil
},
}
result, err := c.AnalyzeCompletion(context.Background(), "sess-1", "plan")
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if result == nil || !result.Complete {
t.Error("expected Complete=true after retry")
}
if result != nil && result.Summary != "retry success" {
t.Errorf("Summary = %q, want %q", result.Summary, "retry success")
}
if got := atomic.LoadInt32(&calls); got < 2 {
t.Errorf("expected >= 2 doAnalyze calls, got %d", got)
}
}

// TestCoverage_AnalyzeCompletion_contextCancelledDuringRetryDelay verifies
// that cancelling the context during the retry delay returns (nil, nil).
func TestCoverage_AnalyzeCompletion_contextCancelledDuringRetryDelay(t *testing.T) {
t.Parallel()
c := New(nil)
c.hooks = &testHooks{
doInit: func(_ context.Context) error { return nil },
doAnalyze: func(_ context.Context, _, _ string) (*CompletionAnalysis, error) {
return nil, fmt.Errorf("always fail")
},
}

// Short timeout expires during retry delay (searchRetryDelay = 500ms).
ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
defer cancel()

result, err := c.AnalyzeCompletion(ctx, "sess-1", "plan")
if err != nil {
t.Errorf("expected nil error, got: %v", err)
}
if result != nil {
t.Error("expected nil result")
}
}

// TestCoverage_AnalyzeCompletion_transportErrorInitRetry exercises the
// path where Init returns a transport error, triggering resetSDK + retry.
func TestCoverage_AnalyzeCompletion_transportErrorInitRetry(t *testing.T) {
t.Parallel()
var initCalls int32
c := New(nil)
c.hooks = &testHooks{
doInit: func(_ context.Context) error {
n := atomic.AddInt32(&initCalls, 1)
if n == 1 {
return fmt.Errorf("file already closed")
}
return nil
},
doAnalyze: func(_ context.Context, _, _ string) (*CompletionAnalysis, error) {
return &CompletionAnalysis{Complete: true, Summary: "recovered"}, nil
},
}
result, err := c.AnalyzeCompletion(context.Background(), "sess-1", "plan")
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if result == nil || !result.Complete {
t.Error("expected Complete=true after transport-error recovery")
}
if got := atomic.LoadInt32(&initCalls); got < 2 {
t.Errorf("expected >= 2 init calls, got %d", got)
}
}

// TestCoverage_AnalyzeCompletion_transportErrorInitRetryFails verifies
// graceful degradation when Init transport error + retry-init both fail.
func TestCoverage_AnalyzeCompletion_transportErrorInitRetryFails(t *testing.T) {
t.Parallel()
c := New(nil)
c.hooks = &testHooks{
doInit: func(_ context.Context) error {
return fmt.Errorf("file already closed")
},
}
result, err := c.AnalyzeCompletion(context.Background(), "sess-1", "plan")
if err != nil {
t.Errorf("expected nil error for failed reinit, got: %v", err)
}
if result != nil {
t.Error("expected nil result for failed reinit")
}
}

// TestCoverage_AnalyzeCompletion_contextCancelledAfterInit verifies
// the context check between Init and doAnalyze.
func TestCoverage_AnalyzeCompletion_contextCancelledAfterInit(t *testing.T) {
t.Parallel()
c := New(nil)
ctx, cancel := context.WithCancel(context.Background())
c.hooks = &testHooks{
doInit: func(_ context.Context) error {
cancel()
return nil
},
doAnalyze: func(_ context.Context, _, _ string) (*CompletionAnalysis, error) {
t.Error("doAnalyze should not be called when context cancelled after init")
return nil, nil
},
}
result, err := c.AnalyzeCompletion(ctx, "sess-1", "plan")
if err != nil {
t.Errorf("expected nil error, got: %v", err)
}
if result != nil {
t.Error("expected nil result")
}
}

// TestCoverage_AnalyzeCompletion_contextCancelledAfterDoAnalyze verifies
// results are discarded when context is cancelled during analysis.
func TestCoverage_AnalyzeCompletion_contextCancelledAfterDoAnalyze(t *testing.T) {
t.Parallel()
c := New(nil)
ctx, cancel := context.WithCancel(context.Background())
c.hooks = &testHooks{
doInit: func(_ context.Context) error { return nil },
doAnalyze: func(_ context.Context, _, _ string) (*CompletionAnalysis, error) {
cancel()
return &CompletionAnalysis{Complete: true, Summary: "should be discarded"}, nil
},
}
result, err := c.AnalyzeCompletion(ctx, "sess-1", "plan")
if err != nil {
t.Errorf("expected nil error, got: %v", err)
}
if result != nil {
t.Error("expected nil result when context cancelled after doAnalyze")
}
}

// TestCoverage_AnalyzeCompletion_retryInitFails verifies that when
// doAnalyze fails and retry reinit also fails, the error accumulates.
func TestCoverage_AnalyzeCompletion_retryInitFails(t *testing.T) {
t.Parallel()
var initCalls int32
c := New(nil)
c.hooks = &testHooks{
doInit: func(_ context.Context) error {
n := atomic.AddInt32(&initCalls, 1)
if n == 1 {
return nil
}
return fmt.Errorf("init keeps failing")
},
doAnalyze: func(_ context.Context, _, _ string) (*CompletionAnalysis, error) {
return nil, fmt.Errorf("analysis broke")
},
}
result, err := c.AnalyzeCompletion(context.Background(), "sess-1", "plan")
if err == nil {
t.Fatal("expected error after retries exhausted")
}
if !strings.Contains(err.Error(), "unavailable after") {
t.Errorf("expected 'unavailable after' error, got: %v", err)
}
if result != nil {
t.Error("expected nil result")
}
}

// TestCoverage_AnalyzeCompletion_hookReturnsEdgeValues checks that
// unusual values from the hook flow through AnalyzeCompletion unchanged.
func TestCoverage_AnalyzeCompletion_hookReturnsEdgeValues(t *testing.T) {
t.Parallel()
c := New(nil)
c.hooks = &testHooks{
doInit: func(_ context.Context) error { return nil },
doAnalyze: func(_ context.Context, _, _ string) (*CompletionAnalysis, error) {
return &CompletionAnalysis{
Complete:       false,
TotalTasks:     999,
CompletedTasks: -1,
Summary:        "edge case",
}, nil
},
}
result, err := c.AnalyzeCompletion(context.Background(), "sess-1", "plan")
if err != nil {
t.Fatalf("unexpected error: %v", err)
}
if result == nil {
t.Fatal("expected non-nil result")
}
if result.TotalTasks != 999 {
t.Errorf("TotalTasks = %d, want 999", result.TotalTasks)
}
if result.CompletedTasks != -1 {
t.Errorf("CompletedTasks = %d, want -1", result.CompletedTasks)
}
}

// TestCoverage_AnalyzeCompletion_slowHookTimeout verifies that a slow hook
// with a short caller deadline results in (nil, nil).
func TestCoverage_AnalyzeCompletion_slowHookTimeout(t *testing.T) {
t.Parallel()
c := New(nil)
c.hooks = &testHooks{
doInit: func(_ context.Context) error { return nil },
doAnalyze: func(ctx context.Context, _, _ string) (*CompletionAnalysis, error) {
select {
case <-ctx.Done():
return nil, ctx.Err()
case <-time.After(2 * time.Second):
return &CompletionAnalysis{Complete: true}, nil
}
},
}
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
defer cancel()

result, err := c.AnalyzeCompletion(ctx, "sess-1", "plan")
if err != nil {
t.Errorf("expected nil error, got: %v", err)
}
if result != nil {
t.Error("expected nil result for timed-out analysis")
}
}

// TestCoverage_AnalyzeCompletion_contextCancelledDuringRetryAfterReinit
// exercises the context check between reinit and retry doAnalyze call.
func TestCoverage_AnalyzeCompletion_contextCancelledDuringRetryAfterReinit(t *testing.T) {
t.Parallel()
var analyzeCalls, initCalls int32
ctx, cancel := context.WithCancel(context.Background())
c := New(nil)
c.hooks = &testHooks{
doInit: func(_ context.Context) error {
n := atomic.AddInt32(&initCalls, 1)
if n > 1 {
cancel()
}
return nil
},
doAnalyze: func(_ context.Context, _, _ string) (*CompletionAnalysis, error) {
n := atomic.AddInt32(&analyzeCalls, 1)
if n == 1 {
return nil, fmt.Errorf("first attempt fails")
}
t.Error("doAnalyze should not be called after context cancelled")
return nil, nil
},
}
result, err := c.AnalyzeCompletion(ctx, "sess-1", "plan")
if err != nil {
t.Errorf("expected nil error, got: %v", err)
}
if result != nil {
t.Error("expected nil result")
}
}