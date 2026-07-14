package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/data"
)

func withExportDetail(t *testing.T, fn func(string) (*data.SessionDetail, error)) {
	t.Helper()
	prev := exportGetDetailFn
	exportGetDetailFn = fn
	t.Cleanup(func() { exportGetDetailFn = prev })
}

func sampleDetail() *data.SessionDetail {
	return &data.SessionDetail{
		Session: data.Session{
			ID:         "ses-001",
			Summary:    "Fix the widget",
			Cwd:        "/tmp/project",
			Repository: "jongio/dispatch",
			Branch:     "main",
			CreatedAt:  "2026-01-05T10:00:00Z",
			TurnCount:  2,
			FileCount:  1,
		},
		Turns: []data.Turn{
			{UserMessage: "hello", AssistantResponse: "hi"},
		},
		Refs: []data.SessionRef{
			{RefType: "pr", RefValue: "42"},
		},
	}
}

func TestParseExportArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantID     string
		wantFormat string
		wantStdout bool
		wantOut    string
		wantRedact bool
		wantErr    bool
	}{
		{name: "id only", args: []string{"export", "abc"}, wantID: "abc", wantFormat: "md"},
		{name: "format json", args: []string{"export", "abc", "--format", "json"}, wantID: "abc", wantFormat: "json"},
		{name: "format html", args: []string{"export", "abc", "--format", "html"}, wantID: "abc", wantFormat: "html"},
		{name: "format markdown alias", args: []string{"export", "abc", "--format=markdown"}, wantID: "abc", wantFormat: "md"},
		{name: "short format", args: []string{"export", "-f", "json", "abc"}, wantID: "abc", wantFormat: "json"},
		{name: "stdout", args: []string{"export", "abc", "--stdout"}, wantID: "abc", wantFormat: "md", wantStdout: true},
		{name: "redact", args: []string{"export", "abc", "--redact"}, wantID: "abc", wantFormat: "md", wantRedact: true},
		{name: "out dir", args: []string{"export", "abc", "--out", "/tmp/x"}, wantID: "abc", wantFormat: "md", wantOut: "/tmp/x"},
		{name: "missing id", args: []string{"export"}, wantErr: true},
		{name: "two ids", args: []string{"export", "a", "b"}, wantErr: true},
		{name: "unknown flag", args: []string{"export", "--nope", "a"}, wantErr: true},
		{name: "invalid format", args: []string{"export", "a", "--format", "yaml"}, wantErr: true},
		{name: "format without value", args: []string{"export", "a", "--format"}, wantErr: true},
		{name: "stdout and out", args: []string{"export", "a", "--stdout", "--out", "/tmp/x"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseExportArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", opts)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if opts.id != tc.wantID {
				t.Errorf("id = %q, want %q", opts.id, tc.wantID)
			}
			if opts.format != tc.wantFormat {
				t.Errorf("format = %q, want %q", opts.format, tc.wantFormat)
			}
			if opts.stdout != tc.wantStdout {
				t.Errorf("stdout = %v, want %v", opts.stdout, tc.wantStdout)
			}
			if opts.outDir != tc.wantOut {
				t.Errorf("outDir = %q, want %q", opts.outDir, tc.wantOut)
			}
			if opts.redact != tc.wantRedact {
				t.Errorf("redact = %v, want %v", opts.redact, tc.wantRedact)
			}
		})
	}
}

func TestRunExport_StdoutMarkdown(t *testing.T) {
	withExportDetail(t, func(string) (*data.SessionDetail, error) { return sampleDetail(), nil })

	var buf bytes.Buffer
	if err := runExport(&buf, []string{"export", "ses-001", "--stdout"}); err != nil {
		t.Fatalf("runExport: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "# Session: Fix the widget") {
		t.Errorf("markdown output missing title, got:\n%s", out)
	}
}

func TestRunExport_StdoutJSON(t *testing.T) {
	withExportDetail(t, func(string) (*data.SessionDetail, error) { return sampleDetail(), nil })

	var buf bytes.Buffer
	if err := runExport(&buf, []string{"export", "ses-001", "--stdout", "--format", "json"}); err != nil {
		t.Fatalf("runExport: %v", err)
	}
	var got data.SessionDetail
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got.Session.ID != "ses-001" {
		t.Errorf("json ID = %q, want %q", got.Session.ID, "ses-001")
	}
}

func TestRunExport_RedactsStdout(t *testing.T) {
	detail := sampleDetail()
	detail.Turns = []data.Turn{
		{UserMessage: "Authorization: Bearer abcdefghijklmnopqrstuvwxyz123456", AssistantResponse: "API_TOKEN=super-secret-token"},
	}
	withExportDetail(t, func(string) (*data.SessionDetail, error) { return detail, nil })

	var buf bytes.Buffer
	if err := runExport(&buf, []string{"export", "ses-001", "--stdout", "--redact"}); err != nil {
		t.Fatalf("runExport: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "abcdefghijklmnopqrstuvwxyz123456") || strings.Contains(out, "super-secret-token") {
		t.Fatalf("redacted export leaked a secret:\n%s", out)
	}
	if !strings.Contains(out, "[redacted]") {
		t.Fatalf("redacted export missing placeholder:\n%s", out)
	}
}

func TestRunExport_RedactsJSONWithoutBreakingJSON(t *testing.T) {
	detail := sampleDetail()
	detail.Turns = []data.Turn{
		{UserMessage: "Bearer abcdefghijklmnopqrstuvwxyz123456", AssistantResponse: "safe"},
	}
	withExportDetail(t, func(string) (*data.SessionDetail, error) { return detail, nil })

	var buf bytes.Buffer
	if err := runExport(&buf, []string{"export", "ses-001", "--stdout", "--format", "json", "--redact"}); err != nil {
		t.Fatalf("runExport: %v", err)
	}
	var got data.SessionDetail
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("redacted output is not valid JSON: %v\n%s", err, buf.String())
	}
	if strings.Contains(buf.String(), "abcdefghijklmnopqrstuvwxyz123456") {
		t.Fatalf("redacted JSON leaked a token:\n%s", buf.String())
	}
}

func TestRunExport_WritesFile(t *testing.T) {
	withExportDetail(t, func(string) (*data.SessionDetail, error) { return sampleDetail(), nil })

	dir := t.TempDir()
	var buf bytes.Buffer
	if err := runExport(&buf, []string{"export", "ses-001", "--out", dir, "--format", "json"}); err != nil {
		t.Fatalf("runExport: %v", err)
	}
	path := filepath.Join(dir, "ses-001.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected export file at %s: %v", path, err)
	}
	if !strings.Contains(buf.String(), path) {
		t.Errorf("output should report the path %q, got %q", path, buf.String())
	}
}

func TestRunExport_StdoutHTML(t *testing.T) {
	withExportDetail(t, func(string) (*data.SessionDetail, error) { return sampleDetail(), nil })

	var buf bytes.Buffer
	if err := runExport(&buf, []string{"export", "ses-001", "--stdout", "--format", "html"}); err != nil {
		t.Fatalf("runExport: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"<!DOCTYPE html>", "<title>Session: Fix the widget</title>", "<style>"} {
		if !strings.Contains(out, want) {
			t.Errorf("html output missing %q, got:\n%s", want, out)
		}
	}
}

func TestRunExport_HTMLEscapesContent(t *testing.T) {
	detail := sampleDetail()
	detail.Session.Summary = "Fix <script>alert(1)</script>"
	detail.Turns = []data.Turn{{UserMessage: "run <b>bold</b> & stuff", AssistantResponse: "ok"}}
	withExportDetail(t, func(string) (*data.SessionDetail, error) { return detail, nil })

	var buf bytes.Buffer
	if err := runExport(&buf, []string{"export", "ses-001", "--stdout", "--format", "html"}); err != nil {
		t.Fatalf("runExport: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Errorf("html output must escape raw script tags, got:\n%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("html output missing escaped summary, got:\n%s", out)
	}
	if !strings.Contains(out, "run &lt;b&gt;bold&lt;/b&gt; &amp; stuff") {
		t.Errorf("html output missing escaped message body, got:\n%s", out)
	}
}

func TestRunExport_WritesHTMLFile(t *testing.T) {
	withExportDetail(t, func(string) (*data.SessionDetail, error) { return sampleDetail(), nil })

	dir := t.TempDir()
	var buf bytes.Buffer
	if err := runExport(&buf, []string{"export", "ses-001", "--out", dir, "--format", "html"}); err != nil {
		t.Fatalf("runExport: %v", err)
	}
	path := filepath.Join(dir, "ses-001.html")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected export file at %s: %v", path, err)
	}
	if !strings.Contains(buf.String(), path) {
		t.Errorf("output should report the path %q, got %q", path, buf.String())
	}
}

func TestRunExport_NotFound(t *testing.T) {
	withExportDetail(t, func(string) (*data.SessionDetail, error) { return nil, nil })

	err := runExport(&bytes.Buffer{}, []string{"export", "missing"})
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}

func TestRunExport_LoadError(t *testing.T) {
	sentinel := errors.New("boom")
	withExportDetail(t, func(string) (*data.SessionDetail, error) { return nil, sentinel })

	err := runExport(&bytes.Buffer{}, []string{"export", "abc"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
}
