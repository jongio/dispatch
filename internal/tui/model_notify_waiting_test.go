package tui

import (
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

// TestNotifyWaiting_FirstScanNoBell verifies the initial scan only records a
// baseline and never rings the bell, even when a session is already waiting.
func TestNotifyWaiting_FirstScanNoBell(t *testing.T) {
	m := newTestModel()
	m.cfg.NotifyOnWaiting = true

	cmd := m.notifyWaiting(map[string]data.AttentionStatus{"s1": data.AttentionWaiting})

	if cmd != nil {
		t.Error("first scan should not ring the bell")
	}
	if !m.attentionScanned {
		t.Error("first scan should mark attentionScanned")
	}
	if m.statusInfo != "" {
		t.Errorf("statusInfo = %q, want empty on first scan", m.statusInfo)
	}
	if _, ok := m.waitingNotified["s1"]; !ok {
		t.Error("first scan should record already-waiting sessions as baseline")
	}
}

// TestNotifyWaiting_TransitionRingsBell verifies a session entering waiting
// after the baseline rings the bell and sets a footer message.
func TestNotifyWaiting_TransitionRingsBell(t *testing.T) {
	m := newTestModel()
	m.cfg.NotifyOnWaiting = true
	m.attentionScanned = true

	cmd := m.notifyWaiting(map[string]data.AttentionStatus{"s1": data.AttentionWaiting})

	if cmd == nil {
		t.Fatal("transition into waiting should ring the bell")
	}
	if m.statusInfo != "1 session is waiting" {
		t.Errorf("statusInfo = %q, want %q", m.statusInfo, "1 session is waiting")
	}
}

// TestNotifyWaiting_Disabled verifies no bell fires when the setting is off.
func TestNotifyWaiting_Disabled(t *testing.T) {
	m := newTestModel()
	m.cfg.NotifyOnWaiting = false
	m.attentionScanned = true

	cmd := m.notifyWaiting(map[string]data.AttentionStatus{"s1": data.AttentionWaiting})

	if cmd != nil {
		t.Error("bell should not ring when notify_on_waiting is disabled")
	}
	if m.statusInfo != "" {
		t.Errorf("statusInfo = %q, want empty when disabled", m.statusInfo)
	}
}

// TestNotifyWaiting_SteadyStateNoSecondBell verifies a session that stays in
// the waiting state only rings the bell once.
func TestNotifyWaiting_SteadyStateNoSecondBell(t *testing.T) {
	m := newTestModel()
	m.cfg.NotifyOnWaiting = true
	m.attentionScanned = true

	statuses := map[string]data.AttentionStatus{"s1": data.AttentionWaiting}
	if cmd := m.notifyWaiting(statuses); cmd == nil {
		t.Fatal("first transition should ring the bell")
	}
	m.statusInfo = ""
	if cmd := m.notifyWaiting(statuses); cmd != nil {
		t.Error("steady waiting state should not ring the bell again")
	}
}

// TestNotifyWaiting_ReentryRingsAgain verifies leaving and re-entering the
// waiting state rings the bell a second time.
func TestNotifyWaiting_ReentryRingsAgain(t *testing.T) {
	m := newTestModel()
	m.cfg.NotifyOnWaiting = true
	m.attentionScanned = true

	if cmd := m.notifyWaiting(map[string]data.AttentionStatus{"s1": data.AttentionWaiting}); cmd == nil {
		t.Fatal("first transition should ring the bell")
	}
	// Leave the waiting state.
	if cmd := m.notifyWaiting(map[string]data.AttentionStatus{"s1": data.AttentionWorking}); cmd != nil {
		t.Error("leaving waiting should not ring the bell")
	}
	if _, ok := m.waitingNotified["s1"]; ok {
		t.Error("session should be dropped from waitingNotified after leaving waiting")
	}
	// Re-enter waiting.
	if cmd := m.notifyWaiting(map[string]data.AttentionStatus{"s1": data.AttentionWaiting}); cmd == nil {
		t.Error("re-entering waiting should ring the bell again")
	}
}

// TestNotifyWaiting_MultipleSessionsSingleBell verifies several sessions
// entering waiting in one scan ring the bell once with a pluralized message.
func TestNotifyWaiting_MultipleSessionsSingleBell(t *testing.T) {
	m := newTestModel()
	m.cfg.NotifyOnWaiting = true
	m.attentionScanned = true

	cmd := m.notifyWaiting(map[string]data.AttentionStatus{
		"s1": data.AttentionWaiting,
		"s2": data.AttentionWaiting,
	})

	if cmd == nil {
		t.Fatal("multiple sessions entering waiting should ring the bell")
	}
	if m.statusInfo != "2 sessions are waiting" {
		t.Errorf("statusInfo = %q, want %q", m.statusInfo, "2 sessions are waiting")
	}
}

// TestNotifyWaiting_ForgetsDisappearedSessions verifies sessions that vanish
// from the scan are dropped from the notified set.
func TestNotifyWaiting_ForgetsDisappearedSessions(t *testing.T) {
	m := newTestModel()
	m.cfg.NotifyOnWaiting = true
	m.attentionScanned = true

	m.notifyWaiting(map[string]data.AttentionStatus{"s1": data.AttentionWaiting})
	m.notifyWaiting(map[string]data.AttentionStatus{})
	if _, ok := m.waitingNotified["s1"]; ok {
		t.Error("disappeared session should be forgotten")
	}
}

// TestBellCmd_RingsBell verifies bellCmd invokes the bell function and returns
// a nil message.
func TestBellCmd_RingsBell(t *testing.T) {
	called := 0
	orig := bellFn
	bellFn = func() { called++ }
	t.Cleanup(func() { bellFn = orig })

	msg := bellCmd()()
	if called != 1 {
		t.Errorf("bellFn called %d times, want 1", called)
	}
	if msg != nil {
		t.Errorf("bellCmd message = %v, want nil", msg)
	}
}

// TestHandleAttentionScanned_NotifiesOnTransition verifies the handler wiring:
// the first scan is a baseline and a later transition sets the footer message.
func TestHandleAttentionScanned_NotifiesOnTransition(t *testing.T) {
	m := newTestModelWithSize(120, 30)
	m.cfg.NotifyOnWaiting = true

	// First scan: baseline, no notification.
	m1, _ := m.handleAttentionScanned(attentionScannedMsg{
		statuses: map[string]data.AttentionStatus{"s1": data.AttentionActive},
	})
	if m1.statusInfo != "" {
		t.Errorf("statusInfo = %q, want empty after baseline scan", m1.statusInfo)
	}

	// Second scan: s1 enters waiting.
	m2, _ := m1.handleAttentionScanned(attentionScannedMsg{
		statuses: map[string]data.AttentionStatus{"s1": data.AttentionWaiting},
	})
	if m2.statusInfo != "1 session is waiting" {
		t.Errorf("statusInfo = %q, want %q", m2.statusInfo, "1 session is waiting")
	}
}
