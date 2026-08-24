package data

import (
	"database/sql/driver"
	"log/slog"
	"strings"
	"unicode"
	"unicode/utf8"

	sqlite "modernc.org/sqlite"
)

// normFuncName is a SQLite scalar function that renders text the way the
// session list does before matching against it.
//
// Two things differ between stored text and what a user reads on screen:
//
//   - SQLite's LIKE and lower() fold ASCII only, so "café" never matches
//     stored "CAFÉ".
//   - The list renders summaries through CleanSummary, which collapses every
//     whitespace run to a single space. A summary stored as
//     "## Task\n\nInvoke the skill" is displayed as "## Task Invoke the
//     skill", so a user filtering on the phrase they can see types
//     "Task Invoke" and the raw text never matches.
//
// Users filter and search by what they see, so matching normalizes both sides
// the same way.
const normFuncName = "dispatch_norm"

// normReady reports whether normFuncName was registered with the driver.
// Registration happens once at package load and can only fail if the name is
// already taken, but queries degrade to SQLite's raw matching rather than
// erroring if it ever does.
var normReady bool

func init() {
	err := sqlite.RegisterDeterministicScalarFunction(normFuncName, 1, normScalar)
	if err != nil {
		slog.Warn("match normalization unavailable; filters fall back to raw text matching",
			"function", normFuncName, "error", err)
		return
	}
	normReady = true
}

// normScalar normalizes its argument. NULL and non-text values normalize to
// the empty string so the result is never NULL: these expressions are also
// used negated, where a NULL would drop a row that should have been kept.
func normScalar(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 1 {
		return "", nil
	}
	switch v := args[0].(type) {
	case string:
		return normalizeMatchText(v), nil
	case []byte:
		return normalizeMatchText(string(v)), nil
	default:
		return "", nil
	}
}

// normalizeMatchText lowercases with Unicode awareness and collapses runs of
// whitespace to a single space, mirroring how the session list renders text.
func normalizeMatchText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// needsNormalization reports whether matching term requires the normalizing
// path. Normalization runs a callback per scanned row, so it is only paid for
// when it can change the result:
//
//   - a term containing whitespace has to match across the line breaks that
//     the list collapses for display;
//   - a term containing non-ASCII text needs Unicode folding, which SQLite's
//     LIKE does not do.
//
// A single-word ASCII term is already matched case-insensitively by LIKE and
// stays on the fast path, as does a term that normalizes to nothing.
func needsNormalization(term string) bool {
	if !normReady || normalizeMatchText(term) == "" {
		return false
	}
	if len(term) != utf8.RuneCountInString(term) {
		return true
	}
	return strings.ContainsFunc(term, unicode.IsSpace)
}

// matcher renders the per-column comparison for one search term.
type matcher struct {
	pattern   string // LIKE pattern, normalized when normalize is true
	guard     string // cheap raw-text prefilter; empty when not applicable
	normalize bool
}

// newMatcher builds the comparison for term.
//
// Normalizing every scanned row is expensive: the excluded-word filter is
// negated, so SQLite cannot stop at the first hit and ends up folding whole
// assistant responses. Where possible the normalized test is therefore
// guarded by a plain LIKE over the raw column, which SQLite evaluates first
// and which discards nearly every row without calling into Go.
//
// The guard is sound because normalization only lowercases and collapses
// whitespace: it never reorders or removes a token. So any text whose
// normalized form contains "task invoke" must contain "task", then some
// whitespace, then "invoke" in the raw text, which "%task%invoke%" matches.
// The guard over-matches ("task and then invoke"), and the normalized
// comparison that follows rejects those.
func newMatcher(term string) matcher {
	normalize := needsNormalization(term)
	m := matcher{pattern: likePattern(term, normalize), normalize: normalize}
	if !normalize {
		return m
	}
	// A non-ASCII term cannot be guarded by a raw LIKE, because the guard
	// would itself need the case folding it is trying to avoid.
	if len(term) != utf8.RuneCountInString(term) {
		return m
	}
	fields := strings.Fields(term)
	if len(fields) < 2 {
		return m
	}
	escaped := make([]string, len(fields))
	for i, f := range fields {
		escaped[i] = escapeLIKE(f)
	}
	m.guard = "%" + strings.Join(escaped, "%") + "%"
	return m
}

// columnSQL renders the comparison for one column.
func (m matcher) columnSQL(column string) string {
	const like = ` LIKE ? ESCAPE '\'`
	raw := "COALESCE(" + column + ",'')"
	if !m.normalize {
		return raw + like
	}
	normalized := normFuncName + "(" + raw + ")" + like
	if m.guard == "" {
		return normalized
	}
	return "(" + raw + like + " AND " + normalized + ")"
}

// appendArgs appends one column's bound values, in the order columnSQL
// consumes them.
func (m matcher) appendArgs(dst []any) []any {
	if m.guard != "" {
		dst = append(dst, m.guard)
	}
	return append(dst, m.pattern)
}

// likePattern builds the LIKE pattern for term, normalizing it when the
// comparison is normalized so both sides agree.
func likePattern(term string, normalize bool) string {
	if normalize {
		term = normalizeMatchText(term)
	}
	return "%" + escapeLIKE(term) + "%"
}
