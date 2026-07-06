package config

import "testing"

func TestColumnVisible(t *testing.T) {
	c := &Config{}
	for _, key := range ToggleableColumns {
		if !c.ColumnVisible(key) {
			t.Errorf("empty HiddenColumns: column %q should be visible", key)
		}
	}

	c.HiddenColumns = []string{ColumnRepo, ColumnTurns}
	if c.ColumnVisible(ColumnRepo) {
		t.Error("repo column should be hidden")
	}
	if c.ColumnVisible(ColumnTurns) {
		t.Error("turns column should be hidden")
	}
	if !c.ColumnVisible(ColumnFolder) {
		t.Error("folder column should remain visible")
	}
	if !c.ColumnVisible(ColumnHost) {
		t.Error("host column should remain visible")
	}
}
