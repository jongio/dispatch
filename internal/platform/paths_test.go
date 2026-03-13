package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// SessionStorePath
// ---------------------------------------------------------------------------

func TestSessionStorePathNonEmpty(t *testing.T) {
	path, err := SessionStorePath()
	if err != nil {
		t.Fatalf("SessionStorePath: %v", err)
	}
	if path == "" {
		t.Fatal("SessionStorePath returned empty string")
	}
}

func TestSessionStorePathEndsWith(t *testing.T) {
	path, err := SessionStorePath()
	if err != nil {
		t.Fatalf("SessionStorePath: %v", err)
	}
	want := filepath.Join(".copilot", "session-store.db")
	if !strings.HasSuffix(path, want) {
		t.Errorf("SessionStorePath() = %q, want suffix %q", path, want)
	}
}

func TestSessionStorePathAbsolute(t *testing.T) {
	path, err := SessionStorePath()
	if err != nil {
		t.Fatalf("SessionStorePath: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("SessionStorePath() = %q, want absolute path", path)
	}
}

func TestSessionStorePathContainsHomeDir(t *testing.T) {
	path, err := SessionStorePath()
	if err != nil {
		t.Fatalf("SessionStorePath: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home dir: %v", err)
	}
	if !strings.HasPrefix(path, home) {
		t.Errorf("SessionStorePath() = %q, expected prefix %q", path, home)
	}
}

// ---------------------------------------------------------------------------
// ConfigDir
// ---------------------------------------------------------------------------

func TestConfigDirNonEmpty(t *testing.T) {
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	if dir == "" {
		t.Fatal("ConfigDir returned empty string")
	}
}

func TestConfigDirEndsWithDispatch(t *testing.T) {
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	if filepath.Base(dir) != "dispatch" {
		t.Errorf("ConfigDir() = %q, want base 'dispatch'", dir)
	}
}

func TestConfigDirAbsolute(t *testing.T) {
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("ConfigDir() = %q, want absolute path", dir)
	}
}

func TestConfigDirUsesOSConfigBase(t *testing.T) {
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}

	base, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("cannot determine user config dir: %v", err)
	}

	want := filepath.Join(base, "dispatch")
	if dir != want {
		t.Errorf("ConfigDir() = %q, want %q", dir, want)
	}
}

// ---------------------------------------------------------------------------
// Constants verification
// ---------------------------------------------------------------------------

func TestAppNameConstant(t *testing.T) {
	// The appName constant is used to build the config directory path.
	// Verify indirectly: ConfigDir must end with "dispatch".
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	if filepath.Base(dir) != appName {
		t.Errorf("ConfigDir base = %q, expected appName = %q", filepath.Base(dir), appName)
	}
	if appName != "dispatch" {
		t.Errorf("appName = %q, want 'dispatch'", appName)
	}
}

func TestSessionStoreRelConstant(t *testing.T) {
	if sessionStoreRel != ".copilot/session-store.db" {
		t.Errorf("sessionStoreRel = %q, want '.copilot/session-store.db'", sessionStoreRel)
	}
}

// ---------------------------------------------------------------------------
// Path consistency
// ---------------------------------------------------------------------------

func TestPathsAreConsistentAcrossCalls(t *testing.T) {
	// Calling the same function twice should return the same path.
	p1, err := SessionStorePath()
	if err != nil {
		t.Fatalf("first SessionStorePath: %v", err)
	}
	p2, err := SessionStorePath()
	if err != nil {
		t.Fatalf("second SessionStorePath: %v", err)
	}
	if p1 != p2 {
		t.Errorf("SessionStorePath not consistent: %q != %q", p1, p2)
	}

	d1, err := ConfigDir()
	if err != nil {
		t.Fatalf("first ConfigDir: %v", err)
	}
	d2, err := ConfigDir()
	if err != nil {
		t.Fatalf("second ConfigDir: %v", err)
	}
	if d1 != d2 {
		t.Errorf("ConfigDir not consistent: %q != %q", d1, d2)
	}
}
