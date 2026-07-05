package config

import (
	"fmt"
	"strings"
)

// NormalizeAlias trims and lowercases an alias and reduces it to its first
// whitespace-delimited token so aliases stay usable as a single CLI argument.
func NormalizeAlias(input string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		s = s[:i]
	}
	return s
}

// SetAlias assigns an alias to a session. An empty alias clears any existing
// alias for the session. It returns an error when the alias is already used by
// a different session so aliases stay unique.
func (c *Config) SetAlias(sessionID, alias string) error {
	if sessionID == "" {
		return nil
	}
	alias = NormalizeAlias(alias)
	if alias == "" {
		delete(c.SessionAliases, sessionID)
		return nil
	}
	for id, a := range c.SessionAliases {
		if a == alias && id != sessionID {
			return fmt.Errorf("alias %q is already used by another session", alias)
		}
	}
	if c.SessionAliases == nil {
		c.SessionAliases = make(map[string]string)
	}
	c.SessionAliases[sessionID] = alias
	return nil
}

// AliasFor returns the alias for a session, or an empty string when none.
func (c *Config) AliasFor(sessionID string) string {
	return c.SessionAliases[sessionID]
}

// SessionIDForAlias returns the session ID mapped to the given alias, or an
// empty string when no session uses it. The lookup is case-insensitive.
func (c *Config) SessionIDForAlias(alias string) string {
	alias = NormalizeAlias(alias)
	if alias == "" {
		return ""
	}
	for id, a := range c.SessionAliases {
		if a == alias {
			return id
		}
	}
	return ""
}
