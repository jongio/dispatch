package tui

import "strings"

// SearchFilter holds structured filter tokens extracted from the search bar input.
// Each field corresponds to a supported token prefix (e.g., "repo:dispatch").
// FreeText contains the remaining query words that did not match any token.
type SearchFilter struct {
	Repo     string // repo:<value>
	Branch   string // branch:<value>
	Folder   string // folder:<value>
	Host     string // host:<value>
	Status   string // status:<value> (waiting, active, stale, idle, interrupted)
	HasPlan  bool   // has:plan
	IsFav    bool   // is:favorite
	IsHidden bool   // is:hidden
	FreeText string // remaining non-token words
}

// HasTokens reports whether any structured token was parsed.
func (sf SearchFilter) HasTokens() bool {
	return sf.Repo != "" ||
		sf.Branch != "" ||
		sf.Folder != "" ||
		sf.Host != "" ||
		sf.Status != "" ||
		sf.HasPlan ||
		sf.IsFav ||
		sf.IsHidden
}

// TokenSummary returns a short description of active tokens suitable for
// display in the badge row. Returns an empty string when no tokens are active.
func (sf SearchFilter) TokenSummary() string {
	var parts []string
	if sf.Repo != "" {
		parts = append(parts, "repo:"+sf.Repo)
	}
	if sf.Branch != "" {
		parts = append(parts, "branch:"+sf.Branch)
	}
	if sf.Folder != "" {
		parts = append(parts, "folder:"+sf.Folder)
	}
	if sf.Host != "" {
		parts = append(parts, "host:"+sf.Host)
	}
	if sf.Status != "" {
		parts = append(parts, "status:"+sf.Status)
	}
	if sf.HasPlan {
		parts = append(parts, "has:plan")
	}
	if sf.IsFav {
		parts = append(parts, "is:favorite")
	}
	if sf.IsHidden {
		parts = append(parts, "is:hidden")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// ParseSearchTokens splits a raw search input string into structured filter
// tokens and remaining free text. Tokens use the format "key:value" with no
// space between key and value. Quoted values (e.g., repo:"my repo") are
// supported for values containing spaces. Unknown tokens are kept as free text.
func ParseSearchTokens(input string) SearchFilter {
	var sf SearchFilter
	var freeWords []string

	words := tokenize(input)
	for _, w := range words {
		key, value, ok := splitToken(w)
		if !ok {
			freeWords = append(freeWords, w)
			continue
		}

		switch key {
		case "repo":
			sf.Repo = value
		case "branch":
			sf.Branch = value
		case "folder":
			sf.Folder = value
		case "host":
			sf.Host = value
		case "status":
			sf.Status = value
		case "has":
			if value == "plan" {
				sf.HasPlan = true
			} else {
				// Unknown has: value; treat as free text.
				freeWords = append(freeWords, w)
			}
		case "is":
			switch value {
			case "favorite", "fav":
				sf.IsFav = true
			case "hidden":
				sf.IsHidden = true
			default:
				freeWords = append(freeWords, w)
			}
		default:
			// Unknown token prefix; keep as free text.
			freeWords = append(freeWords, w)
		}
	}

	sf.FreeText = strings.Join(freeWords, " ")
	return sf
}

// tokenize splits input into words, respecting quoted values attached to
// token prefixes. For example: `repo:"my repo" hello` yields
// ["repo:my repo", "hello"]. Standalone quoted strings are kept as-is.
func tokenize(input string) []string {
	var tokens []string
	i := 0
	n := len(input)

	for i < n {
		// Skip whitespace.
		if input[i] == ' ' || input[i] == '\t' {
			i++
			continue
		}

		start := i
		// Check if this is a token with a quoted value (key:"...").
		colonIdx := -1
		for j := i; j < n && input[j] != ' ' && input[j] != '\t'; j++ {
			if input[j] == ':' && colonIdx == -1 {
				colonIdx = j
			}
			if input[j] == '"' && colonIdx >= 0 && j == colonIdx+1 {
				// Found key:" pattern; read until closing quote.
				key := input[start:colonIdx]
				j++ // skip opening quote
				valueStart := j
				for j < n && input[j] != '"' {
					j++
				}
				value := input[valueStart:j]
				if j < n {
					j++ // skip closing quote
				}
				tokens = append(tokens, key+":"+value)
				i = j
				goto next
			}
		}

		// Regular word (no special quoting).
		for i < n && input[i] != ' ' && input[i] != '\t' {
			i++
		}
		tokens = append(tokens, input[start:i])
	next:
	}
	return tokens
}

// splitToken attempts to split a word on the first colon into key and value.
// Returns false if no colon is found or the value is empty.
func splitToken(word string) (key, value string, ok bool) {
	idx := strings.IndexByte(word, ':')
	if idx <= 0 || idx == len(word)-1 {
		return "", "", false
	}
	return word[:idx], word[idx+1:], true
}
