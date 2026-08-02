package config

import (
	"path/filepath"
	"testing"
)

func TestValidateProjectRoots_Diagnostics(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name     string
		roots    []string
		wantPath string
	}{
		{name: "empty", roots: []string{""}, wantPath: "project_roots[0]"},
		{name: "whitespace only", roots: []string{"   "}, wantPath: "project_roots[0]"},
		{name: "non-absolute", roots: []string{filepath.Join("relative", "dir")}, wantPath: "project_roots[0]"},
		{name: "duplicate", roots: []string{root, root}, wantPath: "project_roots[1]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := validateProjectRoots(tt.roots)
			assertDiagnosticPath(t, diags, tt.wantPath)
		})
	}
}

func TestValidateProjectRoots_Valid(t *testing.T) {
	base := t.TempDir()
	diags := validateProjectRoots([]string{filepath.Join(base, "a"), filepath.Join(base, "b")})
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diags)
	}
}

func TestValidateLaunchSets_Diagnostics(t *testing.T) {
	tests := []struct {
		name     string
		sets     []LaunchSet
		wantPath string
	}{
		{
			name:     "empty session ids",
			sets:     []LaunchSet{{Name: "review", SessionIDs: nil}},
			wantPath: "launch_sets[0].session_ids",
		},
		{
			name:     "missing name",
			sets:     []LaunchSet{{Name: "", SessionIDs: []string{"ses-1"}}},
			wantPath: "launch_sets[0].name",
		},
		{
			name: "duplicate name",
			sets: []LaunchSet{
				{Name: "dup", SessionIDs: []string{"ses-1"}},
				{Name: "dup", SessionIDs: []string{"ses-2"}},
			},
			wantPath: "launch_sets[1].name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := validateLaunchSets(tt.sets)
			assertDiagnosticPath(t, diags, tt.wantPath)
		})
	}
}

func TestValidateLaunchSets_Valid(t *testing.T) {
	sets := []LaunchSet{
		{Name: "a", SessionIDs: []string{"ses-1"}},
		{Name: "b", SessionIDs: []string{"ses-2", "ses-3"}},
	}
	if diags := validateLaunchSets(sets); len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diags)
	}
}
