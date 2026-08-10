package version

import "testing"

func TestVersionCanBeOverriddenAtBuildTime(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })

	if Version == "" {
		t.Fatal("Version should have a non-empty development default")
	}

	Version = "v1.2.3"
	if Version != "v1.2.3" {
		t.Fatalf("Version = %q, want v1.2.3", Version)
	}
}
