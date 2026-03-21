// Package validate provides shared validation helpers.
package validate

import "regexp"

// SessionIDPattern matches valid Copilot CLI session identifiers.
// Format: starts with alphanumeric, followed by up to 127 alphanumeric/dot/hyphen/underscore chars.
var SessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)

// SessionID reports whether id is a valid session identifier.
func SessionID(id string) bool {
	return SessionIDPattern.MatchString(id)
}
