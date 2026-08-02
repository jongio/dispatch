package config

import (
	"os"
	"strings"
	"testing"
)

// normalizeSchema collapses CRLF to LF and trims trailing whitespace and
// newlines so the committed file (which may carry Windows CRLF endings) can
// be compared against the generator's LF output.
func normalizeSchema(b []byte) string {
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	return strings.TrimRight(s, " \t\n")
}

// TestConfigSchemaFileMatchesGenerated fails if docs/config.schema.json has
// drifted from the generator (JSONSchemaBytes). It also exercises
// JSONSchemaBytes end to end.
func TestConfigSchemaFileMatchesGenerated(t *testing.T) {
	const schemaPath = "../../docs/config.schema.json"

	committed, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("reading %s: %v", schemaPath, err)
	}

	generated, err := JSONSchemaBytes()
	if err != nil {
		t.Fatalf("JSONSchemaBytes: %v", err)
	}

	if normalizeSchema(committed) != normalizeSchema(generated) {
		t.Fatalf("docs/config.schema.json is out of date; regenerate it with:\n\tgo run ./cmd/dispatch config schema > docs/config.schema.json")
	}
}
