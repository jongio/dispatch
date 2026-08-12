package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveRejectsFutureConfigVersion(t *testing.T) {
	setupTempConfig(t)
	cfg := Default()
	cfg.ConfigVersion = currentConfigVersion + 1

	if err := Save(cfg); err == nil {
		t.Fatal("Save accepted a future config version")
	}
}

func TestSaveRestrictsExistingConfigPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not enforced on Windows")
	}
	setupTempConfig(t)
	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Save(Default()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != configFilePerm {
		t.Fatalf("config permissions = %o, want %o", got, configFilePerm)
	}
}
