package config

import "testing"

// enumFieldSetters maps each enum config field to a setter that assigns the
// candidate value to the corresponding Config field. The map key must equal
// both the JSON schema property name and the ValidateConfig diagnostic path.
var enumFieldSetters = map[string]func(*Config, string){
	"default_time_range": func(c *Config, v string) { c.DefaultTimeRange = v },
	"default_sort":       func(c *Config, v string) { c.DefaultSort = v },
	"default_sort_order": func(c *Config, v string) { c.DefaultSortOrder = v },
	"default_pivot":      func(c *Config, v string) { c.DefaultPivot = v },
	"launch_mode":        func(c *Config, v string) { c.LaunchMode = v },
	"pane_direction":     func(c *Config, v string) { c.PaneDirection = v },
	"preview_position":   func(c *Config, v string) { c.PreviewPosition = v },
}

// fieldAccepted reports whether ValidateConfig raises no diagnostic for the
// given field path when the field is set to value.
func fieldAccepted(field, value string) bool {
	cfg := Default()
	enumFieldSetters[field](cfg, value)
	for _, d := range ValidateConfig(cfg) {
		if d.Path == field {
			return false
		}
	}
	return true
}

// TestSchemaEnumsMatchValidatorEnums drives the JSON schema generator and the
// validator against each other: every value the schema advertises as valid
// must be accepted by ValidateConfig, and a value outside the enum must be
// rejected. This keeps schemaForField and ValidateConfig from drifting.
func TestSchemaEnumsMatchValidatorEnums(t *testing.T) {
	schema, err := JSONSchema()
	if err != nil {
		t.Fatalf("JSONSchema: %v", err)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema has no properties object")
	}

	const invalid = "definitely-not-a-valid-enum-value"

	for field := range enumFieldSetters {
		field := field
		t.Run(field, func(t *testing.T) {
			prop, ok := props[field].(map[string]any)
			if !ok {
				t.Fatalf("schema property %q missing", field)
			}
			enum, ok := prop["enum"].([]string)
			if !ok {
				t.Fatalf("schema property %q has no string enum, got %T", field, prop["enum"])
			}
			if len(enum) == 0 {
				t.Fatalf("schema property %q has an empty enum", field)
			}

			for _, v := range enum {
				if !fieldAccepted(field, v) {
					t.Errorf("schema enum value %q for %q is rejected by ValidateConfig", v, field)
				}
			}

			if fieldAccepted(field, invalid) {
				t.Errorf("ValidateConfig accepted %q for %q, which is not in the schema enum", invalid, field)
			}
		})
	}

	// default_sort must advertise frecency; a regression here would silently
	// drop the frecency sort from validation and completion.
	sortProp, ok := props["default_sort"].(map[string]any)
	if !ok {
		t.Fatal("schema property default_sort missing")
	}
	enum, _ := sortProp["enum"].([]string)
	found := false
	for _, v := range enum {
		if v == SortFieldFrecency {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("default_sort enum %v is missing %q", enum, SortFieldFrecency)
	}
}
