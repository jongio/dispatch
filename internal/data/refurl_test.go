package data

import "testing"

func TestRefURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		repo     string
		refType  string
		refValue string
		want     string
		wantOK   bool
	}{
		{"pr slug", "owner/repo", "pr", "42", "https://github.com/owner/repo/pull/42", true},
		{"issue slug", "owner/repo", "issue", "41", "https://github.com/owner/repo/issues/41", true},
		{"commit slug", "owner/repo", "commit", "a1b2c3d", "https://github.com/owner/repo/commit/a1b2c3d", true},
		{"pr with hash", "owner/repo", "pr", "#42", "https://github.com/owner/repo/pull/42", true},
		{"pr prefixed", "owner/repo", "PR", "PR42", "https://github.com/owner/repo/pull/42", true},
		{"https remote", "https://github.com/owner/repo.git", "pr", "7", "https://github.com/owner/repo/pull/7", true},
		{"ssh remote", "git@github.com:owner/repo.git", "issue", "9", "https://github.com/owner/repo/issues/9", true},
		{"github path", "github.com/owner/repo", "commit", "cafebabe", "https://github.com/owner/repo/commit/cafebabe", true},
		{"empty repo", "", "pr", "42", "", false},
		{"repo no slash", "justowner", "pr", "42", "", false},
		{"empty value", "owner/repo", "pr", "", "", false},
		{"pr non-numeric", "owner/repo", "pr", "abc", "", false},
		{"commit non-hex", "owner/repo", "commit", "zzzz", "", false},
		{"unknown type", "owner/repo", "branch", "main", "", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := RefURL(tt.repo, tt.refType, tt.refValue)
			if ok != tt.wantOK {
				t.Fatalf("RefURL(%q,%q,%q) ok = %v, want %v", tt.repo, tt.refType, tt.refValue, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("RefURL(%q,%q,%q) = %q, want %q", tt.repo, tt.refType, tt.refValue, got, tt.want)
			}
		})
	}
}

func TestBestRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		refs      []SessionRef
		wantType  string
		wantValue string
		wantOK    bool
	}{
		{
			name: "prefers pr over issue and commit",
			refs: []SessionRef{
				{RefType: "commit", RefValue: "abc123"},
				{RefType: "issue", RefValue: "10"},
				{RefType: "pr", RefValue: "42"},
			},
			wantType: "pr", wantValue: "42", wantOK: true,
		},
		{
			name: "prefers issue over commit",
			refs: []SessionRef{
				{RefType: "commit", RefValue: "abc123"},
				{RefType: "issue", RefValue: "10"},
			},
			wantType: "issue", wantValue: "10", wantOK: true,
		},
		{
			name:     "commit only",
			refs:     []SessionRef{{RefType: "commit", RefValue: "abc123"}},
			wantType: "commit", wantValue: "abc123", wantOK: true,
		},
		{
			name:   "empty",
			refs:   nil,
			wantOK: false,
		},
		{
			name:   "blank values ignored",
			refs:   []SessionRef{{RefType: "pr", RefValue: "  "}},
			wantOK: false,
		},
		{
			name:   "unknown types ignored",
			refs:   []SessionRef{{RefType: "branch", RefValue: "main"}},
			wantOK: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := BestRef(tt.refs)
			if ok != tt.wantOK {
				t.Fatalf("BestRef() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.RefType != tt.wantType || got.RefValue != tt.wantValue {
				t.Errorf("BestRef() = {%q,%q}, want {%q,%q}", got.RefType, got.RefValue, tt.wantType, tt.wantValue)
			}
		})
	}
}
