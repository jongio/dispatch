package copilot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
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

// ---------------------------------------------------------------------------
// sdkContext — unit tests (issue #114)
// ---------------------------------------------------------------------------

// TestSdkContext_ReturnsContextWithTimeout verifies sdkContext creates
// a context with the specified timeout and a valid cancel function.
func TestSdkContext_ReturnsContextWithTimeout(t *testing.T) {
	t.Parallel()
	timeout := 5 * time.Second
	ctx, cancel := sdkContext(timeout)
	defer cancel()

	// Context should not be cancelled yet.
	if ctx.Err() != nil {
		t.Errorf("fresh context should not have error, got: %v", ctx.Err())
	}

	// Deadline should be set and approximately timeout from now.
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("sdkContext should set a deadline")
	}
	remaining := time.Until(deadline)
	if remaining < 4*time.Second || remaining > 6*time.Second {
		t.Errorf("remaining time %v should be close to %v", remaining, timeout)
	}
}

// TestSdkContext_CancelStopsContext verifies the cancel function works.
func TestSdkContext_CancelStopsContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := sdkContext(10 * time.Second)
	cancel()

	if ctx.Err() == nil {
		t.Error("context should be cancelled after cancel()")
	}
}

// TestSdkContext_UsesBackgroundParent verifies the context is derived
// from context.Background() (not any other parent).
func TestSdkContext_UsesBackgroundParent(t *testing.T) {
	t.Parallel()
	ctx, cancel := sdkContext(time.Second)
	defer cancel()

	// A background-derived context should have no values.
	if v := ctx.Value("nonexistent"); v != nil {
		t.Error("sdkContext should be derived from Background (no values)")
	}
}

// ---------------------------------------------------------------------------
// isTransportError — unit tests (issue #114)
// ---------------------------------------------------------------------------

func TestIsTransportError_Nil(t *testing.T) {
	t.Parallel()
	if isTransportError(nil) {
		t.Error("nil error should not be a transport error")
	}
}

func TestIsTransportError_EOF(t *testing.T) {
	t.Parallel()
	if !isTransportError(io.EOF) {
		t.Error("io.EOF should be a transport error")
	}
}

func TestIsTransportError_UnexpectedEOF(t *testing.T) {
	t.Parallel()
	if !isTransportError(io.ErrUnexpectedEOF) {
		t.Error("io.ErrUnexpectedEOF should be a transport error")
	}
}

func TestIsTransportError_OsErrClosed(t *testing.T) {
	t.Parallel()
	if !isTransportError(os.ErrClosed) {
		t.Error("os.ErrClosed should be a transport error")
	}
}

func TestIsTransportError_NetErrClosed(t *testing.T) {
	t.Parallel()
	if !isTransportError(net.ErrClosed) {
		t.Error("net.ErrClosed should be a transport error")
	}
}

func TestIsTransportError_EPIPE(t *testing.T) {
	t.Parallel()
	if !isTransportError(syscall.EPIPE) {
		t.Error("syscall.EPIPE should be a transport error")
	}
}

func TestIsTransportError_ECONNRESET(t *testing.T) {
	t.Parallel()
	if !isTransportError(syscall.ECONNRESET) {
		t.Error("syscall.ECONNRESET should be a transport error")
	}
}

func TestIsTransportError_WrappedEOF(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf("sdk broke: %w", io.EOF)
	if !isTransportError(wrapped) {
		t.Error("wrapped io.EOF should be a transport error")
	}
}

func TestIsTransportError_FileAlreadyClosed(t *testing.T) {
	t.Parallel()
	err := errors.New("something: file already closed")
	if !isTransportError(err) {
		t.Error("'file already closed' string should be a transport error")
	}
}

func TestIsTransportError_ErrorReadingHeader(t *testing.T) {
	t.Parallel()
	err := errors.New("error reading header from pipe")
	if !isTransportError(err) {
		t.Error("'error reading header' string should be a transport error")
	}
}

func TestIsTransportError_BrokenPipe(t *testing.T) {
	t.Parallel()
	err := errors.New("write: broken pipe")
	if !isTransportError(err) {
		t.Error("'broken pipe' string should be a transport error")
	}
}

func TestIsTransportError_ConnectionReset(t *testing.T) {
	t.Parallel()
	err := errors.New("read tcp: connection reset by peer")
	if !isTransportError(err) {
		t.Error("'connection reset' string should be a transport error")
	}
}

func TestIsTransportError_UnrelatedError(t *testing.T) {
	t.Parallel()
	err := errors.New("validation failed: missing field")
	if isTransportError(err) {
		t.Error("unrelated error should not be a transport error")
	}
}

func TestIsTransportError_ContextCancelled(t *testing.T) {
	t.Parallel()
	if isTransportError(context.Canceled) {
		t.Error("context.Canceled should not be a transport error")
	}
}

func TestIsTransportError_DeadlineExceeded(t *testing.T) {
	t.Parallel()
	if isTransportError(context.DeadlineExceeded) {
		t.Error("context.DeadlineExceeded should not be a transport error")
	}
}
