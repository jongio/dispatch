package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

func TestIntensityBucket(t *testing.T) {
	tests := []struct {
		count, max, want int
	}{
		{0, 10, 0},
		{5, 0, 0}, // no max -> level 0
		{1, 8, 1}, // ratio 0.125 -> 1
		{2, 8, 1}, // ratio 0.25 -> 1
		{3, 8, 2}, // ratio 0.375 -> 2
		{4, 8, 2}, // ratio 0.5 -> 2
		{5, 8, 3}, // ratio 0.625 -> 3
		{6, 8, 3}, // ratio 0.75 -> 3
		{7, 8, 4}, // ratio 0.875 -> 4
		{8, 8, 4}, // ratio 1.0 -> 4
	}
	for _, tc := range tests {
		if got := intensityBucket(tc.count, tc.max); got != tc.want {
			t.Errorf("intensityBucket(%d, %d) = %d, want %d", tc.count, tc.max, got, tc.want)
		}
	}
}

func TestBuildActivityCalendar_CountsAndGaps(t *testing.T) {
	sessions := []data.Session{
		{CreatedAt: "2026-01-01T09:00:00Z"},
		{CreatedAt: "2026-01-01T18:00:00Z"}, // same day -> count 2
		{CreatedAt: "2026-01-04T10:00:00Z"}, // 2-day gap
	}

	cal := buildActivityCalendar(sessions)

	if cal.MaxCount != 2 {
		t.Errorf("MaxCount = %d, want 2", cal.MaxCount)
	}
	// Jan 1 through Jan 4 inclusive = 4 continuous days.
	if len(cal.Days) != 4 {
		t.Fatalf("len(Days) = %d, want 4 (%+v)", len(cal.Days), cal.Days)
	}

	byDate := map[string]dayCount{}
	for _, d := range cal.Days {
		byDate[d.Date] = d
	}
	if byDate["2026-01-01"].Count != 2 {
		t.Errorf("2026-01-01 count = %d, want 2", byDate["2026-01-01"].Count)
	}
	if byDate["2026-01-01"].Intensity != 4 {
		t.Errorf("2026-01-01 intensity = %d, want 4 (busiest day)", byDate["2026-01-01"].Intensity)
	}
	// Gap days are present with zero count and intensity.
	if d, ok := byDate["2026-01-02"]; !ok || d.Count != 0 || d.Intensity != 0 {
		t.Errorf("2026-01-02 = %+v, want zero-count gap day", d)
	}
	if byDate["2026-01-04"].Count != 1 {
		t.Errorf("2026-01-04 count = %d, want 1", byDate["2026-01-04"].Count)
	}
}

func TestBuildActivityCalendar_Empty(t *testing.T) {
	cal := buildActivityCalendar(nil)
	if len(cal.Days) != 0 || cal.MaxCount != 0 {
		t.Errorf("empty calendar = %+v, want no days and MaxCount 0", cal)
	}

	// Sessions with unparseable dates are skipped and produce an empty grid.
	cal = buildActivityCalendar([]data.Session{{CreatedAt: "not-a-date"}})
	if len(cal.Days) != 0 {
		t.Errorf("unparseable dates should be skipped, got %+v", cal.Days)
	}
}

func TestRunStatsCalendarText(t *testing.T) {
	withStatsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return sampleSessions(), nil
	})

	var buf bytes.Buffer
	if err := runStats(&buf, []string{"stats", "--calendar"}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Activity", "Sun ", "Sat ", "Less", "More"} {
		if !strings.Contains(out, want) {
			t.Errorf("calendar output missing %q\n%s", want, out)
		}
	}
	// The default breakdown must still be present.
	if !strings.Contains(out, "By repository") {
		t.Errorf("default stats output should be unchanged\n%s", out)
	}
}

func TestRunStatsCalendarJSON(t *testing.T) {
	withStatsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return sampleSessions(), nil
	})

	var buf bytes.Buffer
	if err := runStats(&buf, []string{"stats", "--json", "--calendar"}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	var report statsReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if report.Calendar == nil || len(report.Calendar.Days) == 0 {
		t.Fatalf("expected calendar in JSON, got %+v", report.Calendar)
	}
}

func TestRunStatsNoCalendarByDefault(t *testing.T) {
	withStatsList(t, func(data.FilterOptions) ([]data.Session, error) {
		return sampleSessions(), nil
	})

	var buf bytes.Buffer
	if err := runStats(&buf, []string{"stats", "--json"}); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	var report statsReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if report.Calendar != nil {
		t.Errorf("calendar should be omitted without --calendar, got %+v", report.Calendar)
	}
	if strings.Contains(buf.String(), "\"calendar\"") {
		t.Errorf("default JSON should not contain a calendar key\n%s", buf.String())
	}
}

func TestParseStatsArgsCalendar(t *testing.T) {
	opts, err := parseStatsArgs([]string{"stats", "--calendar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.calendar {
		t.Error("expected calendar=true")
	}
}
