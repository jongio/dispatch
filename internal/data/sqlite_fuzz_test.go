package data

import (
	"net/url"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReadOnlySQLiteDSNRejectsMalformedPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "invalid UTF-8", path: string([]byte{0xde, ':', '0'})},
	}
	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name string
			path string
		}{name: "non-absolute pseudo-volume", path: "ް:"})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := readOnlySQLiteDSN(tt.path); err == nil {
				t.Fatal("readOnlySQLiteDSN() error = nil, want malformed path error")
			}
		})
	}
}

func FuzzReadOnlySQLiteDSN(f *testing.F) {
	for _, seed := range []string{
		"session-store.db",
		"space # percent %.db",
		`C:\Users\test\session store.db`,
		"../relative/session.db",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, path string) {
		dsn, err := readOnlySQLiteDSN(path)
		if err != nil {
			return
		}

		parsed, err := url.Parse(dsn)
		if err != nil {
			t.Fatalf("url.Parse(%q): %v", dsn, err)
		}
		if parsed.Scheme != "file" {
			t.Fatalf("scheme = %q, want file", parsed.Scheme)
		}
		if parsed.Host != "" {
			t.Fatalf("host = %q, want empty", parsed.Host)
		}
		if parsed.Fragment != "" {
			t.Fatalf("fragment = %q, want empty", parsed.Fragment)
		}
		if parsed.Query().Get("mode") != "ro" {
			t.Fatalf("mode = %q, want ro", parsed.Query().Get("mode"))
		}
		if parsed.Query().Get("_query_only") != "1" {
			t.Fatalf("_query_only = %q, want 1", parsed.Query().Get("_query_only"))
		}

		absolute, err := filepath.Abs(path)
		if err != nil {
			t.Fatalf("filepath.Abs(%q): %v", path, err)
		}
		wantPath := filepath.ToSlash(absolute)
		if filepath.VolumeName(absolute) != "" {
			wantPath = "/" + wantPath
		}
		if parsed.Path != wantPath {
			t.Fatalf("path = %q, want %q", parsed.Path, wantPath)
		}
	})
}
