package platform

import "testing"

func TestRedactSecrets_BearerToken(t *testing.T) {
	input := "Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.abc123"
	got := RedactSecrets(input)
	want := "Authorization: Bearer [redacted]"
	if got != want {
		t.Errorf("Bearer token:\n got: %q\nwant: %q", got, want)
	}
}

func TestRedactSecrets_BearerCaseInsensitive(t *testing.T) {
	input := "bearer some-opaque-token-value"
	got := RedactSecrets(input)
	want := "bearer [redacted]"
	if got != want {
		t.Errorf("Bearer case-insensitive:\n got: %q\nwant: %q", got, want)
	}
}

func TestRedactSecrets_GitHubPAT(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "ghp_ classic",
			input: "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij",
			want:  "token: [redacted]",
		},
		{
			name:  "gho_ oauth",
			input: "use gho_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij here",
			want:  "use [redacted] here",
		},
		{
			name:  "github_pat_ fine-grained",
			input: "github_pat_11AAAAAA_BBBBBBBBBBBBBBBBBBBBBB",
			want:  "[redacted]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactSecrets(tt.input)
			if got != tt.want {
				t.Errorf("got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestRedactSecrets_AzureConnectionString(t *testing.T) {
	input := "DefaultEndpointsProtocol=https;AccountName=myacct;AccountKey=abc123+def456/ghi==;EndpointSuffix=core.windows.net"
	got := RedactSecrets(input)
	if got == input {
		t.Error("expected AccountKey value to be redacted, got unchanged input")
	}
	// The AccountKey= prefix should remain visible.
	if !contains(got, "AccountKey=") {
		t.Error("expected AccountKey= prefix to remain")
	}
	if !contains(got, "[redacted]") {
		t.Error("expected [redacted] placeholder in output")
	}
}

func TestRedactSecrets_DotEnvSecrets(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "API_TOKEN",
			input: "API_TOKEN=supersecretvalue123",
			want:  "API_TOKEN=[redacted]",
		},
		{
			name:  "DB_PASSWORD",
			input: "DB_PASSWORD=hunter2",
			want:  "DB_PASSWORD=[redacted]",
		},
		{
			name:  "SECRET_KEY with spaces",
			input: "SECRET_KEY = my-secret-value",
			want:  "SECRET_KEY = [redacted]",
		},
		{
			name:  "AZURE_KEY",
			input: "AZURE_KEY=abcdef1234567890",
			want:  "AZURE_KEY=[redacted]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactSecrets(tt.input)
			if got != tt.want {
				t.Errorf("got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestRedactSecrets_NormalTextUnchanged(t *testing.T) {
	inputs := []string{
		"Hello, world!",
		"This is a normal conversation about tokens in NLP.",
		"The password field is required.",
		"func main() { fmt.Println(\"test\") }",
		"git commit -m 'fix key handling'",
		"",
	}
	for _, input := range inputs {
		got := RedactSecrets(input)
		if got != input {
			t.Errorf("normal text should be unchanged:\n input: %q\n   got: %q", input, got)
		}
	}
}

func TestRedactSecrets_MultipleSecrets(t *testing.T) {
	input := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9 and ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"
	got := RedactSecrets(input)
	if !contains(got, "Bearer [redacted]") {
		t.Errorf("expected Bearer to be redacted, got: %q", got)
	}
	// The ghp_ token should also be redacted.
	if contains(got, "ghp_") {
		t.Errorf("expected ghp_ token to be redacted, got: %q", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
