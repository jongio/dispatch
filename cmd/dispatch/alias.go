package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jongio/dispatch/internal/config"
)

// aliasResult is the machine-readable output of an alias mutation. Alias is
// empty when the alias was cleared or removed.
type aliasResult struct {
	ID    string `json:"id"`
	Alias string `json:"alias"`
}

// runAlias sets or removes a session alias from the command line, completing
// the CLI parity that `tag` and `notes` already have for their metadata. It
// supports:
//
//	dispatch alias <id> <name>      assign or reassign an alias
//	dispatch alias <id> --clear     remove the alias on a session
//	dispatch alias --remove <name>  remove an alias by its name
//
// <id> accepts the same full ID or short prefix that `dispatch open` does, so
// aliases resolve to exactly one session. Alias names reuse config's
// normalization and uniqueness rules, so `dispatch open <alias>` keeps
// resolving to one session. args[0] is expected to be "alias".
func runAlias(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	sessionArg, name, clearFlag, remove, jsonOut, err := parseAliasArgs(args)
	if err != nil {
		return err
	}

	cfg, err := configLoadFn()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	var (
		sessionID string
		alias     string
		action    string
	)

	switch {
	case remove:
		sessionID = cfg.SessionIDForAlias(name)
		if sessionID == "" {
			return fmt.Errorf("no session uses alias %q", config.NormalizeAlias(name))
		}
		if sErr := cfg.SetAlias(sessionID, ""); sErr != nil {
			return sErr
		}
		action = "cleared"

	case clearFlag:
		resolved, rErr := resolveAliasSession(sessionArg)
		if rErr != nil {
			return rErr
		}
		sessionID = resolved
		if sErr := cfg.SetAlias(sessionID, ""); sErr != nil {
			return sErr
		}
		action = "cleared"

	default:
		resolved, rErr := resolveAliasSession(sessionArg)
		if rErr != nil {
			return rErr
		}
		sessionID = resolved
		if sErr := cfg.SetAlias(sessionID, name); sErr != nil {
			return sErr
		}
		alias = cfg.AliasFor(sessionID)
		action = "set"
	}

	if sErr := configSaveFn(cfg); sErr != nil {
		return sErr
	}

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(aliasResult{ID: sessionID, Alias: alias})
	}

	if action == "cleared" {
		fmt.Fprintf(w, "Cleared alias for %s\n", shortID(sessionID))
	} else {
		fmt.Fprintf(w, "Set alias %q for %s\n", alias, shortID(sessionID))
	}
	return nil
}

// resolveAliasSession resolves a session ID or short prefix to a full session
// ID using the same seam `dispatch open` uses, so prefix matching is identical.
func resolveAliasSession(idArg string) (string, error) {
	if strings.TrimSpace(idArg) == "" {
		return "", errors.New("alias requires a session ID")
	}
	sess, err := openGetSessionFn(idArg)
	if err != nil {
		return "", err
	}
	if sess == nil {
		return "", fmt.Errorf("session %q not found", idArg)
	}
	return sess.ID, nil
}

// parseAliasArgs splits the alias subcommand arguments into the session
// selector, alias name, and mode flags. args[0] is expected to be "alias".
func parseAliasArgs(args []string) (sessionArg, name string, clearFlag, remove, jsonOut bool, err error) {
	rest := args
	if len(rest) > 0 {
		rest = rest[1:] // drop the "alias" token
	}

	var positionals []string
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "--clear":
			clearFlag = true
		case arg == "--remove":
			remove = true
			if i+1 >= len(rest) {
				return "", "", false, false, false, errors.New("--remove requires an alias name")
			}
			i++
			name = rest[i]
		case strings.HasPrefix(arg, "--remove="):
			remove = true
			name = strings.TrimPrefix(arg, "--remove=")
		case strings.HasPrefix(arg, "-"):
			return "", "", false, false, false, fmt.Errorf("unknown flag: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}

	if remove {
		switch {
		case clearFlag:
			return "", "", false, false, false, errors.New("alias --remove cannot be combined with --clear")
		case len(positionals) > 0:
			return "", "", false, false, false, errors.New("alias --remove takes an alias name, not a session ID")
		case strings.TrimSpace(name) == "":
			return "", "", false, false, false, errors.New("--remove requires an alias name")
		}
		return "", name, false, true, jsonOut, nil
	}

	if len(positionals) == 0 {
		return "", "", false, false, false, errors.New("alias requires a session ID")
	}
	sessionArg = positionals[0]

	if clearFlag {
		if len(positionals) > 1 {
			return "", "", false, false, false, errors.New("alias <id> --clear does not take an alias name")
		}
		return sessionArg, "", true, false, jsonOut, nil
	}

	switch len(positionals) {
	case 1:
		return "", "", false, false, false, errors.New("alias <id> requires an alias name (or use --clear)")
	case 2:
		name = positionals[1]
	default:
		return "", "", false, false, false, fmt.Errorf("alias takes a single session ID and a single alias name, got %d arguments", len(positionals))
	}

	if strings.TrimSpace(name) == "" {
		return "", "", false, false, false, errors.New("alias name cannot be empty")
	}
	if strings.ContainsAny(name, " \t") {
		return "", "", false, false, false, errors.New("alias name cannot contain whitespace")
	}
	return sessionArg, name, false, false, jsonOut, nil
}
