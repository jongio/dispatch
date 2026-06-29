// Package platform provides OS-specific helpers for copilot-dispatch.

package platform

import "regexp"

// redactedPlaceholder is the replacement text for detected secrets.
const redactedPlaceholder = "[redacted]"

// secretPatterns defines compiled regular expressions for common secret
// shapes that should be masked in preview rendering.
var secretPatterns = []*regexp.Regexp{
	// Bearer tokens in Authorization headers (token must be 20+ chars to avoid
	// false positives on natural language like "bearer of bad news").
	regexp.MustCompile(`(?i)(Bearer\s+)\S{20,}`),

	// GitHub personal access tokens (classic and fine-grained).
	regexp.MustCompile(`ghp_[A-Za-z0-9]{36,}`),
	regexp.MustCompile(`gho_[A-Za-z0-9]{36,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{22,}`),

	// Azure connection strings (storage, service bus, etc.).
	regexp.MustCompile(`(?i)((?:AccountKey|SharedAccessKey|SharedAccessKeyName)\s*=\s*)[^\s;]+`),

	// .env style assignments for keys containing sensitive words as standalone
	// segments (preceded by _ or at start, followed by _ or end of name).
	// Avoids false positives on KEYBOARD, MONKEY, TURKEY, etc.
	regexp.MustCompile(`(?im)^([A-Za-z_]*?(?:(?:^|_)(?:TOKEN|SECRET|PASSWORD|KEY))(?:_[A-Za-z_]*)?\s*=\s*).+$`),
}

// replacements maps each pattern index to a replacement function.
// Patterns that have a captured prefix group preserve the prefix and
// replace only the secret portion.
var replacements = []func(match string, submatches []string) string{
	// Bearer: keep "Bearer " prefix, redact the token.
	func(_ string, sub []string) string {
		if len(sub) >= 2 {
			return sub[1] + redactedPlaceholder
		}
		return redactedPlaceholder
	},
	// ghp_ tokens: replace entire match.
	func(_ string, _ []string) string { return redactedPlaceholder },
	// gho_ tokens: replace entire match.
	func(_ string, _ []string) string { return redactedPlaceholder },
	// github_pat_ tokens: replace entire match.
	func(_ string, _ []string) string { return redactedPlaceholder },
	// Azure connection string key=value: keep key=, redact value.
	func(_ string, sub []string) string {
		if len(sub) >= 2 {
			return sub[1] + redactedPlaceholder
		}
		return redactedPlaceholder
	},
	// .env assignments: keep key=, redact value.
	func(_ string, sub []string) string {
		if len(sub) >= 2 {
			return sub[1] + redactedPlaceholder
		}
		return redactedPlaceholder
	},
}

// RedactSecrets replaces common secret patterns in text with a
// [redacted] placeholder. The function is designed for display purposes
// and does not modify the underlying data store.
func RedactSecrets(text string) string {
	for i, pat := range secretPatterns {
		replFunc := replacements[i]
		text = pat.ReplaceAllStringFunc(text, func(match string) string {
			sub := pat.FindStringSubmatch(match)
			return replFunc(match, sub)
		})
	}
	return text
}
