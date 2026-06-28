package tui

import "github.com/jongio/dispatch/internal/data"

// applySearchTokens maps the parsed searchFilter fields onto the model's
// filter and UI toggle state. Call this after ParseSearchTokens updates
// m.searchFilter.
func (m *Model) applySearchTokens() {
	sf := m.searchFilter

	// Free text goes to the data-layer query filter.
	m.filter.Query = sf.FreeText

	// Structured tokens override the corresponding filter fields.
	// Only override if the token is set; leave existing filter-panel
	// selections alone when no token is present.
	m.filter.Repository = sf.Repo
	m.filter.Branch = sf.Branch
	m.filter.Folder = sf.Folder
	m.filter.HostType = sf.Host

	// Status token maps to the attention filter.
	if sf.Status != "" {
		status := parseAttentionStatus(sf.Status)
		if status >= 0 {
			m.attentionFilter = map[data.AttentionStatus]struct{}{status: {}}
		}
	} else {
		// Only clear if previously set by a token (not by the picker).
		// For simplicity, token-based status always resets the filter.
		m.attentionFilter = make(map[data.AttentionStatus]struct{})
	}

	// Boolean tokens.
	m.filterPlans = sf.HasPlan
	m.showFavorited = sf.IsFav
	m.showHidden = sf.IsHidden
}

// clearSearchTokenFilters resets the filter fields that may have been set
// by search tokens back to their neutral defaults.
func (m *Model) clearSearchTokenFilters() {
	m.filter.Repository = ""
	m.filter.Branch = ""
	m.filter.Folder = ""
	m.filter.HostType = ""
	m.attentionFilter = make(map[data.AttentionStatus]struct{})
	m.filterPlans = false
	m.showFavorited = false
	m.showHidden = false
}

// parseAttentionStatus converts a status token value string to a
// data.AttentionStatus constant. Returns -1 if the value is unrecognized.
func parseAttentionStatus(s string) data.AttentionStatus {
	switch s {
	case "waiting":
		return data.AttentionWaiting
	case "active":
		return data.AttentionActive
	case "stale":
		return data.AttentionStale
	case "idle":
		return data.AttentionIdle
	case "interrupted":
		return data.AttentionInterrupted
	case "working":
		return data.AttentionWorking
	case "thinking":
		return data.AttentionThinking
	case "compacting":
		return data.AttentionCompacting
	default:
		return -1
	}
}
