package update

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallersRequireCosignAndVerifyReleaseIdentity(t *testing.T) {
	t.Parallel()

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))

	tests := []struct {
		name           string
		path           string
		cosignRequired string
	}{
		{
			name:           "shell",
			path:           "install.sh",
			cosignRequired: `command -v cosign >/dev/null 2>&1 || fail "cosign is required to verify release authenticity.`,
		},
		{
			name:           "PowerShell",
			path:           "install.ps1",
			cosignRequired: "if (-not (Get-Command cosign -ErrorAction SilentlyContinue))",
		},
	}

	const releaseIdentity = ".github/workflows/release.yml@refs/heads/main"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(filepath.Join(root, tt.path))
			if err != nil {
				t.Fatalf("read %s: %v", tt.path, err)
			}
			source := string(content)
			if !strings.Contains(source, tt.cosignRequired) {
				t.Fatalf("%s must fail closed when cosign is unavailable", tt.path)
			}
			if !strings.Contains(source, releaseIdentity) {
				t.Fatalf("%s must verify the exact release workflow identity", tt.path)
			}
			if !strings.Contains(source, "d3dcc577efe9d6e5e9ed5afa1f9d4be400a6b146a2b559f90f8dd860609a08c4") {
				t.Fatalf("%s must authenticate the transitional v0.14.0 checksums", tt.path)
			}
			if !strings.Contains(source, "bundle") {
				t.Fatalf("%s must verify the published Sigstore bundle", tt.path)
			}
			if strings.Contains(strings.ToLower(source), "skipping signature verification") {
				t.Fatalf("%s must not permit unsigned installation", tt.path)
			}
		})
	}
}
