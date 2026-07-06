package config

import (
	"math"
	"time"
)

// frecencyHalfLife is the age at which a session's launch weight decays to
// half. A week strikes a balance between surfacing recent work and keeping
// long-running favorites near the top.
const frecencyHalfLife = 7 * 24 * time.Hour

// SessionLaunch records how many times a session has been launched from
// Dispatch and when it was last launched. It powers the frecency sort.
type SessionLaunch struct {
	// Count is the total number of times the session has been launched.
	Count int `json:"count"`

	// Last is the Unix time (seconds) of the most recent launch.
	Last int64 `json:"last"`
}

// RecordLaunch increments the launch count for a session and stamps the launch
// time. now is passed in so callers and tests control the clock. Empty session
// IDs are ignored so new sessions started without an ID do not pollute stats.
func (c *Config) RecordLaunch(sessionID string, now time.Time) {
	if sessionID == "" {
		return
	}
	if c.SessionLaunches == nil {
		c.SessionLaunches = make(map[string]SessionLaunch)
	}
	st := c.SessionLaunches[sessionID]
	st.Count++
	st.Last = now.Unix()
	c.SessionLaunches[sessionID] = st
}

// FrecencyScore returns a ranking score for a session's launch history. Higher
// scores rank first. The score is the launch count scaled by an exponential
// recency decay, so a session launched often and recently outranks one that is
// launched often but long ago, or recently but only once. A session with no
// launches scores zero, which lets the sort fall back to its incoming order.
func FrecencyScore(st SessionLaunch, now time.Time) float64 {
	if st.Count <= 0 {
		return 0
	}
	age := now.Sub(time.Unix(st.Last, 0))
	if age < 0 {
		age = 0
	}
	decay := math.Exp2(-age.Hours() / frecencyHalfLife.Hours())
	return float64(st.Count) * decay
}
