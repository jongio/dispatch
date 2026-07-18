package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
)

// tagResolveIDFn resolves a session ID or unique prefix to a full session ID.
// It is a package variable so tests can substitute a fixed resolver.
var tagResolveIDFn = defaultTagResolveID

// defaultTagResolveID resolves an ID or unique prefix to a full session ID via
// the store. It returns ("", nil) when no session matches — including empty
// sessions, which the filtered session list would miss — and an error for an
// ambiguous prefix or a store failure.
func defaultTagResolveID(id string) (string, error) {
	store, err := data.Open()
	if err != nil {
		return "", fmt.Errorf("opening session store: %w", err)
	}
	defer store.Close() //nolint:errcheck // read-only, best-effort close

	fullID, err := store.ResolveIDPrefix(context.Background(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return fullID, nil
}

// tagResult is the JSON output after a tag mutation.
type tagResult struct {
	ID   string   `json:"id"`
	Tags []string `json:"tags"`
}

// runTag manages tags on a single session. args[0] is "tag".
//
//	dispatch tag <id>               list tags for one session
//	dispatch tag <id> --add a,b     add tags
//	dispatch tag <id> --remove a    remove tags
//	dispatch tag <id> --set a,b     replace all tags
//	dispatch tag <id> --json        print result as JSON
func runTag(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	opts, err := parseTagArgs(args)
	if err != nil {
		return err
	}

	cfg, err := configLoadFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Resolve alias to session ID.
	sessionID := opts.id
	if resolved := cfg.SessionIDForAlias(sessionID); resolved != "" {
		sessionID = resolved
	}

	// Resolve the ID or unique prefix to a full session ID. This also confirms
	// the session exists (including empty sessions the filtered list omits).
	fullID, err := tagResolveIDFn(sessionID)
	if err != nil {
		return err
	}
	if fullID == "" {
		return fmt.Errorf("session %q not found", opts.id)
	}
	sessionID = fullID

	// Apply mutation if requested.
	mutated := false
	switch opts.action {
	case tagActionNone:
		// Display-only: no mutation requested.
	case tagActionAdd:
		mutated = true
		addSessionTags(cfg, sessionID, opts.values)
	case tagActionRemove:
		mutated = true
		removeSessionTags(cfg, sessionID, opts.values)
	case tagActionSet:
		mutated = true
		setSessionTags(cfg, sessionID, opts.values)
	}

	if mutated {
		if err := configSaveFn(cfg); err != nil {
			return err
		}
	}

	tags := cfg.SessionTags[sessionID]
	if opts.json {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(tagResult{ID: sessionID, Tags: tags})
	}

	if len(tags) == 0 {
		fmt.Fprintf(w, "Session %s has no tags.\n", sessionID)
	} else {
		fmt.Fprintf(w, "Tags for %s: %s\n", sessionID, strings.Join(tags, ", "))
	}
	return nil
}

// tagAction describes which mutation the user requested.
type tagAction int

const (
	tagActionNone   tagAction = iota
	tagActionAdd              // --add
	tagActionRemove           // --remove
	tagActionSet              // --set
)

// tagOptions holds parsed flags for the tag command.
type tagOptions struct {
	id     string
	action tagAction
	values []string
	json   bool
}

// parseTagArgs reads the tag subcommand flags.
func parseTagArgs(args []string) (tagOptions, error) {
	var opts tagOptions

	rest := args
	if len(rest) > 0 {
		rest = rest[1:] // drop "tag"
	}

	i := 0
	for i < len(rest) {
		arg := rest[i]
		switch {
		case arg == "--json":
			opts.json = true
		case arg == "--add":
			if opts.action != tagActionNone {
				return tagOptions{}, fmt.Errorf("only one of --add, --remove, --set is allowed")
			}
			i++
			if i >= len(rest) {
				return tagOptions{}, fmt.Errorf("--add requires a comma-separated tag list")
			}
			opts.action = tagActionAdd
			opts.values = splitTags(rest[i])
		case arg == "--remove":
			if opts.action != tagActionNone {
				return tagOptions{}, fmt.Errorf("only one of --add, --remove, --set is allowed")
			}
			i++
			if i >= len(rest) {
				return tagOptions{}, fmt.Errorf("--remove requires a comma-separated tag list")
			}
			opts.action = tagActionRemove
			opts.values = splitTags(rest[i])
		case arg == "--set":
			if opts.action != tagActionNone {
				return tagOptions{}, fmt.Errorf("only one of --add, --remove, --set is allowed")
			}
			i++
			if i >= len(rest) {
				return tagOptions{}, fmt.Errorf("--set requires a comma-separated tag list")
			}
			opts.action = tagActionSet
			opts.values = splitTags(rest[i])
		case strings.HasPrefix(arg, "-"):
			return tagOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			if opts.id != "" {
				return tagOptions{}, fmt.Errorf("tag takes one session ID, got extra %q", arg)
			}
			opts.id = arg
		}
		i++
	}

	if opts.id == "" {
		return tagOptions{}, fmt.Errorf("tag requires a session ID")
	}
	return opts, nil
}

// splitTags splits a comma-separated tag list, trims whitespace, and
// removes empty entries.
func splitTags(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// addSessionTags appends tags to a session, deduplicating and sorting.
func addSessionTags(cfg *config.Config, id string, tags []string) {
	if cfg.SessionTags == nil {
		cfg.SessionTags = make(map[string][]string)
	}
	existing := cfg.SessionTags[id]
	seen := make(map[string]bool, len(existing))
	for _, t := range existing {
		seen[t] = true
	}
	for _, t := range tags {
		if !seen[t] {
			existing = append(existing, t)
			seen[t] = true
		}
	}
	sort.Strings(existing)
	cfg.SessionTags[id] = existing
}

// removeSessionTags removes tags from a session.
func removeSessionTags(cfg *config.Config, id string, tags []string) {
	if cfg.SessionTags == nil {
		return
	}
	remove := make(map[string]bool, len(tags))
	for _, t := range tags {
		remove[t] = true
	}
	existing := cfg.SessionTags[id]
	result := make([]string, 0, len(existing))
	for _, t := range existing {
		if !remove[t] {
			result = append(result, t)
		}
	}
	if len(result) == 0 {
		delete(cfg.SessionTags, id)
	} else {
		cfg.SessionTags[id] = result
	}
}

// setSessionTags replaces all tags on a session.
func setSessionTags(cfg *config.Config, id string, tags []string) {
	if cfg.SessionTags == nil {
		cfg.SessionTags = make(map[string][]string)
	}
	if len(tags) == 0 {
		delete(cfg.SessionTags, id)
		return
	}
	deduped := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, t := range tags {
		if !seen[t] {
			deduped = append(deduped, t)
			seen[t] = true
		}
	}
	sort.Strings(deduped)
	cfg.SessionTags[id] = deduped
}
