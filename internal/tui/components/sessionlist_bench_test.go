package components

import (
	"fmt"
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

// ---------------------------------------------------------------------------
// SetSessions benchmarks
// ---------------------------------------------------------------------------

func BenchmarkSetSessions(b *testing.B) {
	for _, n := range []int{10, 50, 200, 500} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			sessions := makeSessions(n)
			sl := NewSessionList()

			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				sl.SetSessions(sessions)
				benchSinkInt = len(sl.allItems)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SetGroups benchmarks
// ---------------------------------------------------------------------------

func BenchmarkSetGroups(b *testing.B) {
	for _, tc := range []struct {
		folders, perFolder int
	}{
		{5, 10},
		{10, 20},
		{20, 25},
	} {
		name := fmt.Sprintf("folders=%d_per=%d", tc.folders, tc.perFolder)
		b.Run(name, func(b *testing.B) {
			groups := makeGroups(tc.folders, tc.perFolder)
			sl := NewSessionList()

			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				sl.SetGroups(groups)
				benchSinkInt = len(sl.allItems)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// View benchmarks
// ---------------------------------------------------------------------------

func BenchmarkView(b *testing.B) {
	for _, n := range []int{10, 50, 200} {
		b.Run(fmt.Sprintf("flat_n=%d", n), func(b *testing.B) {
			sessions := makeSessions(n)
			sl := NewSessionList()
			sl.SetSessions(sessions)
			sl.SetSize(120, 40)

			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchSinkString = sl.View()
			}
		})
	}
}

func BenchmarkViewGrouped(b *testing.B) {
	groups := makeGroups(10, 20)
	sl := NewSessionList()
	sl.SetGroups(groups)
	sl.SetSize(120, 40)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchSinkString = sl.View()
	}
}

// ---------------------------------------------------------------------------
// Navigation benchmarks
// ---------------------------------------------------------------------------

func BenchmarkMoveDown(b *testing.B) {
	sessions := makeSessions(200)
	sl := NewSessionList()
	sl.SetSessions(sessions)
	sl.SetSize(120, 40)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		sl.cursor = 100
		sl.MoveDown()
		benchSinkInt = sl.cursor
	}
}

func BenchmarkMoveUp(b *testing.B) {
	sessions := makeSessions(200)
	sl := NewSessionList()
	sl.SetSessions(sessions)
	sl.SetSize(120, 40)
	// Move to middle first so MoveUp has room.
	for range 100 {
		sl.MoveDown()
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		sl.cursor = 100
		sl.MoveUp()
		benchSinkInt = sl.cursor
	}
}

func BenchmarkSelected(b *testing.B) {
	sessions := makeSessions(200)
	sl := NewSessionList()
	sl.SetSessions(sessions)
	sl.SetSize(120, 40)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchSinkSession, benchSinkBool = sl.Selected()
	}
}

// ---------------------------------------------------------------------------
// SetHiddenSessions benchmark
// ---------------------------------------------------------------------------

func BenchmarkSetHiddenSessions(b *testing.B) {
	sessions := makeSessions(200)
	sl := NewSessionList()
	sl.SetSessions(sessions)

	// Hide every other session.
	hidden := make(map[string]struct{}, 100)
	for i := 0; i < len(sessions); i += 2 {
		hidden[sessions[i].ID] = struct{}{}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		sl.SetHiddenSessions(hidden)
		benchSinkMap = sl.hiddenSet
	}
}

// ---------------------------------------------------------------------------
// Folder toggle benchmark
// ---------------------------------------------------------------------------

func BenchmarkToggleFolder(b *testing.B) {
	groups := makeGroups(10, 20)
	sl := NewSessionList()
	sl.SetGroups(groups)
	sl.SetSize(120, 40)

	// Position cursor on the first folder header.
	sl.MoveTo(0)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchSinkBool = sl.ToggleFolder()
	}
}

// ---------------------------------------------------------------------------
// ListSessionsByIDs benchmark (data layer, but operates on component data)
// ---------------------------------------------------------------------------

func BenchmarkViewAfterHide(b *testing.B) {
	sessions := makeSessions(200)
	sl := NewSessionList()
	sl.SetSessions(sessions)
	sl.SetSize(120, 40)

	hidden := make(map[string]struct{}, 100)
	for i := 0; i < len(sessions); i += 2 {
		hidden[sessions[i].ID] = struct{}{}
	}
	sl.SetHiddenSessions(hidden)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchSinkString = sl.View()
	}
}

// ---------------------------------------------------------------------------
// SessionCount benchmark
// ---------------------------------------------------------------------------

func BenchmarkSessionCount(b *testing.B) {
	for _, n := range []int{10, 100, 500} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			sessions := makeSessions(n)
			sl := NewSessionList()
			sl.SetSessions(sessions)

			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchSinkInt = sl.SessionCount()
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmark with varying viewport sizes
// ---------------------------------------------------------------------------

func BenchmarkViewSmallViewport(b *testing.B) {
	sessions := makeSessions(200)
	sl := NewSessionList()
	sl.SetSessions(sessions)
	sl.SetSize(80, 20)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchSinkString = sl.View()
	}
}

func BenchmarkViewLargeViewport(b *testing.B) {
	sessions := makeSessions(200)
	sl := NewSessionList()
	sl.SetSessions(sessions)
	sl.SetSize(200, 60)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchSinkString = sl.View()
	}
}

var (
	benchSinkBool    bool
	benchSinkInt     int
	benchSinkMap     map[string]struct{}
	benchSinkSession data.Session
	benchSinkString  string
)
