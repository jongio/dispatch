package validate

import "testing"

func FuzzSessionID(f *testing.F) {
	for _, seed := range []string{
		"a",
		"session-123",
		"session.with_parts",
		"",
		"-invalid",
		"contains space",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, id string) {
		got := SessionID(id)
		want := validSessionIDCharacters(id)
		if got != want {
			t.Fatalf("SessionID(%q) = %v, want %v", id, got, want)
		}
	})
}

func validSessionIDCharacters(id string) bool {
	if len(id) < 1 || len(id) > 128 || !isASCIIAlphanumeric(id[0]) {
		return false
	}
	for i := 1; i < len(id); i++ {
		if !isASCIIAlphanumeric(id[i]) && id[i] != '.' && id[i] != '_' && id[i] != '-' {
			return false
		}
	}
	return true
}

func isASCIIAlphanumeric(value byte) bool {
	return value >= 'a' && value <= 'z' ||
		value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9'
}
