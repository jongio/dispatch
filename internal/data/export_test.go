package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeFilename(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{
			name: "simple alphanumeric",
			id:   "abc-123",
			want: "abc-123",
		},
		{
			name: "UUID format",
			id:   "550e8400-e29b-41d4-a716-446655440000",
			want: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name: "special characters replaced",
			id:   "hello/world:test",
			want: "hello_world_test",
		},
		{
			name: "spaces replaced",
			id:   "my session id",
			want: "my_session_id",
		},
		{
			name: "truncation at 200 chars",
			id:   strings.Repeat("a", 250),
			want: strings.Repeat("a", 200),
		},
		{
			name: "dots and underscores preserved",
			id:   "file_name.ext",
			want: "file_name.ext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafeFilename(tt.id)
			if got != tt.want {
				t.Errorf("SafeFilename(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestRenderMarkdown_NilDetail(t *testing.T) {
	got := RenderMarkdown(nil)
	if got != "" {
		t.Errorf("RenderMarkdown(nil) = %q, want empty string", got)
	}
}

func TestRenderMarkdown_FullDetail(t *testing.T) {
	detail := &SessionDetail{
		Session: Session{
			ID:           "test-id-123",
			Cwd:          "/home/user/project",
			Repository:   "owner/repo",
			Branch:       "main",
			Summary:      "Implement feature X",
			CreatedAt:    "2025-01-01T10:00:00Z",
			LastActiveAt: "2025-01-01T12:00:00Z",
			TurnCount:    3,
			FileCount:    2,
		},
		Turns: []Turn{
			{TurnIndex: 0, UserMessage: "Please add tests", AssistantResponse: "I'll add the tests now."},
			{TurnIndex: 1, UserMessage: "Looks good", AssistantResponse: "Thanks!"},
		},
		Checkpoints: []Checkpoint{
			{CheckpointNumber: 1, Title: "Initial setup", Overview: "Created project scaffold."},
			{CheckpointNumber: 2, Title: "Added tests", Overview: ""},
		},
		Files: []SessionFile{
			{FilePath: "src/main.go", ToolName: "edit"},
			{FilePath: "src/main_test.go", ToolName: "create"},
			{FilePath: "src/main.go", ToolName: "edit"}, // duplicate
		},
		Refs: []SessionRef{
			{RefType: "commit", RefValue: "abc123"},
			{RefType: "pr", RefValue: "#42"},
			{RefType: "commit", RefValue: "abc123"}, // duplicate
		},
	}

	md := RenderMarkdown(detail)

	// Verify title
	if !strings.Contains(md, "# Session: Implement feature X") {
		t.Error("missing title")
	}

	// Verify metadata table
	if !strings.Contains(md, "| ID | `test-id-123` |") {
		t.Error("missing ID in metadata")
	}
	if !strings.Contains(md, "| Repository | owner/repo |") {
		t.Error("missing repository")
	}
	if !strings.Contains(md, "| Branch | main |") {
		t.Error("missing branch")
	}

	// Verify conversation
	if !strings.Contains(md, "## Conversation") {
		t.Error("missing conversation section")
	}
	if !strings.Contains(md, "Please add tests") {
		t.Error("missing user message")
	}
	if !strings.Contains(md, "I'll add the tests now.") {
		t.Error("missing assistant response")
	}

	// Verify checkpoints
	if !strings.Contains(md, "## Checkpoints") {
		t.Error("missing checkpoints section")
	}
	if !strings.Contains(md, "### 1. Initial setup") {
		t.Error("missing checkpoint title")
	}
	if !strings.Contains(md, "Created project scaffold.") {
		t.Error("missing checkpoint overview")
	}

	// Verify files (deduplication)
	if !strings.Contains(md, "## Files Touched") {
		t.Error("missing files section")
	}
	if strings.Count(md, "src/main.go") != 1 {
		t.Error("file deduplication failed: src/main.go appears more than once")
	}

	// Verify refs (deduplication)
	if !strings.Contains(md, "## References") {
		t.Error("missing references section")
	}
	if strings.Count(md, "abc123") != 1 {
		t.Error("ref deduplication failed: abc123 appears more than once")
	}
}

func TestRenderMarkdown_MinimalDetail(t *testing.T) {
	detail := &SessionDetail{
		Session: Session{
			ID:      "minimal-session",
			Summary: "Empty session",
		},
	}

	md := RenderMarkdown(detail)

	if !strings.Contains(md, "# Session: Empty session") {
		t.Error("missing title")
	}
	// Should not contain optional sections
	if strings.Contains(md, "## Conversation") {
		t.Error("conversation section should be absent with no turns")
	}
	if strings.Contains(md, "## Checkpoints") {
		t.Error("checkpoints section should be absent with no checkpoints")
	}
	if strings.Contains(md, "## Files Touched") {
		t.Error("files section should be absent with no files")
	}
	if strings.Contains(md, "## References") {
		t.Error("references section should be absent with no refs")
	}
}

func TestExportSession(t *testing.T) {
	dir := t.TempDir()

	detail := &SessionDetail{
		Session: Session{
			ID:      "export-test-id",
			Summary: "Test export",
		},
		Turns: []Turn{
			{UserMessage: "hello", AssistantResponse: "world"},
		},
	}

	path, err := ExportSession(detail, dir)
	if err != nil {
		t.Fatalf("ExportSession() error = %v", err)
	}

	// Verify path
	expectedFilename := "export-test-id.md"
	if filepath.Base(path) != expectedFilename {
		t.Errorf("filename = %q, want %q", filepath.Base(path), expectedFilename)
	}

	// Verify file content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading exported file: %v", err)
	}
	if !strings.Contains(string(content), "# Session: Test export") {
		t.Error("exported file missing expected content")
	}

	// Verify file permissions (Unix only; Windows always returns 0o666)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat exported file: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 && os.Getenv("OS") != "Windows_NT" {
		t.Errorf("file permissions too open: %o", info.Mode().Perm())
	}
}

func TestExportSession_NilDetail(t *testing.T) {
	dir := t.TempDir()
	_, err := ExportSession(nil, dir)
	if err == nil {
		t.Error("ExportSession(nil) should return error")
	}
}

func TestExportSession_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "exports")

	detail := &SessionDetail{
		Session: Session{ID: "nested-test", Summary: "nested"},
	}

	path, err := ExportSession(detail, dir)
	if err != nil {
		t.Fatalf("ExportSession() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("exported file not found: %v", err)
	}
}

func TestExportDir(t *testing.T) {
	dir, err := ExportDir()
	if err != nil {
		t.Fatalf("ExportDir() error = %v", err)
	}
	if !strings.HasSuffix(dir, "exports") {
		t.Errorf("ExportDir() = %q, want suffix 'exports'", dir)
	}
}
