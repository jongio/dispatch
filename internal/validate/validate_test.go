package validate

import "testing"

func TestSessionID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		id   string
		want bool
	}{
		{"abc123", true},
		{"a", true},
		{"A1.b-c_d", true},
		{"", false},
		{".leading-dot", false},
		{"-leading-dash", false},
		{"_leading-underscore", false},
		{"has space", false},
		{"has/slash", false},
		{"has\\backslash", false},
		{"has;semicolon", false},
		{string(make([]byte, 129)), false}, // too long (129 chars, max 128)
	}
	for _, tt := range tests {
		got := SessionID(tt.id)
		if got != tt.want {
			t.Errorf("SessionID(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}
