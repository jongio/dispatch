//go:build screenshots

package main

import (
	"path/filepath"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    options
		wantErr bool
	}{
		{
			name: "defaults to website screenshots directory",
			want: options{outDir: filepath.Clean(defaultOutDir)},
		},
		{
			name: "accepts separated out directory",
			args: []string{"--out", "custom/screens"},
			want: options{outDir: filepath.Clean("custom/screens")},
		},
		{
			name: "accepts equals out directory",
			args: []string{"--out=custom/screens"},
			want: options{outDir: filepath.Clean("custom/screens")},
		},
		{
			name: "accepts check mode",
			args: []string{"--check", "--out", ".screenshots-check"},
			want: options{outDir: filepath.Clean(".screenshots-check"), check: true},
		},
		{
			name: "accepts help",
			args: []string{"--help"},
			want: options{outDir: filepath.Clean(defaultOutDir), help: true},
		},
		{
			name:    "rejects missing out directory",
			args:    []string{"--out"},
			want:    options{outDir: filepath.Clean(defaultOutDir)},
			wantErr: true,
		},
		{
			name:    "rejects empty equals out directory",
			args:    []string{"--out="},
			want:    options{outDir: filepath.Clean(defaultOutDir)},
			wantErr: true,
		},
		{
			name:    "rejects unknown argument",
			args:    []string{"--bogus"},
			want:    options{outDir: filepath.Clean(defaultOutDir)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("parseArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCleanOutputDir(t *testing.T) {
	if got := cleanOutputDir(""); got != filepath.Clean(defaultOutDir) {
		t.Fatalf("cleanOutputDir(\"\") = %q, want %q", got, filepath.Clean(defaultOutDir))
	}
	if got := cleanOutputDir("web/public/screenshots/../screenshots"); got != filepath.Clean("web/public/screenshots") {
		t.Fatalf("cleanOutputDir cleaned path = %q", got)
	}
}
