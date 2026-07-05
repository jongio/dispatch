package config

import (
	"sort"
	"strings"
)

// ParseTags splits a comma-separated tag string into a normalized,
// de-duplicated slice. Tags are lowercased and trimmed; blank entries are
// dropped. The result is sorted so persistence and display stay stable.
func ParseTags(input string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, part := range strings.Split(input, ",") {
		t := strings.ToLower(strings.TrimSpace(part))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// SetTags stores the tags for a session. Passing an empty slice removes the
// entry so the config stays compact.
func (c *Config) SetTags(sessionID string, tags []string) {
	if sessionID == "" {
		return
	}
	if len(tags) == 0 {
		delete(c.SessionTags, sessionID)
		return
	}
	if c.SessionTags == nil {
		c.SessionTags = make(map[string][]string)
	}
	c.SessionTags[sessionID] = tags
}

// TagsFor returns the tags for a session, or nil when it has none.
func (c *Config) TagsFor(sessionID string) []string {
	return c.SessionTags[sessionID]
}

// HasTag reports whether a session carries the given tag. The comparison is
// case-insensitive.
func (c *Config) HasTag(sessionID, tag string) bool {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" {
		return false
	}
	for _, t := range c.SessionTags[sessionID] {
		if t == tag {
			return true
		}
	}
	return false
}
