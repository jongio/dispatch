package main

import (
	"testing"

	"github.com/jongio/dispatch/internal/config"
)

func TestParseOpenArgs_Overrides(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantID    string
		wantMode  string
		wantAgent *string
		wantModel *string
		wantYolo  *bool
		wantErr   bool
	}{
		{name: "agent space", args: []string{"open", "abc", "--agent", "coder"}, wantID: "abc", wantAgent: ptr("coder")},
		{name: "agent equals", args: []string{"open", "abc", "--agent=coder"}, wantID: "abc", wantAgent: ptr("coder")},
		{name: "model space", args: []string{"open", "abc", "--model", "gpt-5"}, wantID: "abc", wantModel: ptr("gpt-5")},
		{name: "model equals", args: []string{"open", "abc", "--model=gpt-5"}, wantID: "abc", wantModel: ptr("gpt-5")},
		{name: "yolo bare", args: []string{"open", "abc", "--yolo"}, wantID: "abc", wantYolo: ptr(true)},
		{name: "yolo true", args: []string{"open", "abc", "--yolo=true"}, wantID: "abc", wantYolo: ptr(true)},
		{name: "yolo false", args: []string{"open", "abc", "--yolo=false"}, wantID: "abc", wantYolo: ptr(false)},
		{
			name:      "all overrides before id",
			args:      []string{"open", "--agent", "coder", "--model", "gpt-5", "--yolo", "abc"},
			wantID:    "abc",
			wantAgent: ptr("coder"),
			wantModel: ptr("gpt-5"),
			wantYolo:  ptr(true),
		},
		{name: "override with mode", args: []string{"open", "abc", "--agent", "coder", "--mode", "tab"}, wantID: "abc", wantMode: "tab", wantAgent: ptr("coder")},
		{name: "agent missing value", args: []string{"open", "abc", "--agent"}, wantErr: true},
		{name: "model missing value at end", args: []string{"open", "abc", "--model"}, wantErr: true},
		{name: "agent empty inline", args: []string{"open", "abc", "--agent="}, wantErr: true},
		{name: "yolo invalid", args: []string{"open", "abc", "--yolo=maybe"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, mode, _, _, ov, err := parseOpenArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got id=%q mode=%q", id, mode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tc.wantID {
				t.Errorf("id = %q, want %q", id, tc.wantID)
			}
			if mode != tc.wantMode {
				t.Errorf("mode = %q, want %q", mode, tc.wantMode)
			}
			assertStrPtr(t, "agent", ov.agent, tc.wantAgent)
			assertStrPtr(t, "model", ov.model, tc.wantModel)
			assertBoolPtr(t, "yolo", ov.yolo, tc.wantYolo)
		})
	}
}

func TestParseNewArgs_Overrides(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantDir   string
		wantMode  string
		wantAgent *string
		wantModel *string
		wantYolo  *bool
		wantErr   bool
	}{
		{name: "agent only", args: []string{"new", "--agent", "coder"}, wantAgent: ptr("coder")},
		{name: "dir plus overrides", args: []string{"new", "/tmp/proj", "--model=gpt-5", "--yolo"}, wantDir: "/tmp/proj", wantModel: ptr("gpt-5"), wantYolo: ptr(true)},
		{name: "override with mode", args: []string{"new", "--mode", "window", "--agent", "coder"}, wantMode: "window", wantAgent: ptr("coder")},
		{name: "model missing value", args: []string{"new", "--model"}, wantErr: true},
		{name: "yolo invalid", args: []string{"new", "--yolo=nope"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir, mode, ov, err := parseNewArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got dir=%q mode=%q", dir, mode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dir != tc.wantDir {
				t.Errorf("dir = %q, want %q", dir, tc.wantDir)
			}
			if mode != tc.wantMode {
				t.Errorf("mode = %q, want %q", mode, tc.wantMode)
			}
			assertStrPtr(t, "agent", ov.agent, tc.wantAgent)
			assertStrPtr(t, "model", ov.model, tc.wantModel)
			assertBoolPtr(t, "yolo", ov.yolo, tc.wantYolo)
		})
	}
}

func TestLaunchOverridesApply(t *testing.T) {
	t.Run("empty overrides leave config unchanged", func(t *testing.T) {
		cfg := &config.Config{Agent: "base", Model: "base-model", YoloMode: true}
		launchOverrides{}.apply(cfg)
		if cfg.Agent != "base" || cfg.Model != "base-model" || !cfg.YoloMode {
			t.Fatalf("empty overrides mutated config: %+v", cfg)
		}
	})

	t.Run("set overrides replace config values", func(t *testing.T) {
		cfg := &config.Config{Agent: "base", Model: "base-model", YoloMode: false}
		launchOverrides{agent: ptr("coder"), model: ptr("gpt-5"), yolo: ptr(true)}.apply(cfg)
		if cfg.Agent != "coder" {
			t.Errorf("agent = %q, want %q", cfg.Agent, "coder")
		}
		if cfg.Model != "gpt-5" {
			t.Errorf("model = %q, want %q", cfg.Model, "gpt-5")
		}
		if !cfg.YoloMode {
			t.Error("yolo = false, want true")
		}
	})

	t.Run("yolo false override disables enabled config", func(t *testing.T) {
		cfg := &config.Config{YoloMode: true}
		launchOverrides{yolo: ptr(false)}.apply(cfg)
		if cfg.YoloMode {
			t.Error("yolo = true, want false")
		}
	})
}

func TestMatchLaunchOverride_NonOverrideFlag(t *testing.T) {
	var ov launchOverrides
	matched, next, err := matchLaunchOverride([]string{"--mode", "tab"}, 0, &ov)
	if matched {
		t.Fatal("expected --mode to not match a launch override")
	}
	if next != 0 || err != nil {
		t.Fatalf("unexpected next=%d err=%v", next, err)
	}
	if ov.agent != nil || ov.model != nil || ov.yolo != nil {
		t.Errorf("overrides mutated on non-match: %+v", ov)
	}
}

func ptr[T any](v T) *T { return &v }

func assertStrPtr(t *testing.T, field string, got, want *string) {
	t.Helper()
	switch {
	case want == nil && got != nil:
		t.Errorf("%s = %q, want unset", field, *got)
	case want != nil && got == nil:
		t.Errorf("%s unset, want %q", field, *want)
	case want != nil && got != nil && *got != *want:
		t.Errorf("%s = %q, want %q", field, *got, *want)
	}
}

func assertBoolPtr(t *testing.T, field string, got, want *bool) {
	t.Helper()
	switch {
	case want == nil && got != nil:
		t.Errorf("%s = %v, want unset", field, *got)
	case want != nil && got == nil:
		t.Errorf("%s unset, want %v", field, *want)
	case want != nil && got != nil && *got != *want:
		t.Errorf("%s = %v, want %v", field, *got, *want)
	}
}
