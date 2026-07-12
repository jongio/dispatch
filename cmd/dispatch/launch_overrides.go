package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jongio/dispatch/internal/config"
)

// launchOverrides holds per-invocation overrides for the agent, model, and
// yolo settings parsed from the open and new subcommand flags. A nil pointer
// means the flag was not given, so the saved config value is used for that
// setting. These overrides are never written back to the config file.
type launchOverrides struct {
	agent *string
	model *string
	yolo  *bool
}

// apply copies any set overrides onto cfg. cfg is the freshly loaded config for
// a single launch and is never persisted, so this only affects the current
// invocation. Callers apply overrides before building a platform.ResumeConfig
// so every launch mode and --print see the same values.
func (o launchOverrides) apply(cfg *config.Config) {
	if o.agent != nil {
		cfg.Agent = *o.agent
	}
	if o.model != nil {
		cfg.Model = *o.model
	}
	if o.yolo != nil {
		cfg.YoloMode = *o.yolo
	}
}

// matchLaunchOverride interprets rest[i] as one of the shared --agent, --model,
// or --yolo override flags used by both the open and new subcommands. It
// returns matched=true when the token is one of those flags, the index the
// caller should continue from (next), and an error for a missing or invalid
// value. When matched is false the caller handles the token itself. Both
// "--flag value" and "--flag=value" forms are accepted; --yolo also accepts a
// bare form (implies true) and an explicit --yolo=true/false value.
func matchLaunchOverride(rest []string, i int, ov *launchOverrides) (matched bool, next int, err error) {
	name, inline, hasInline := splitFlag(rest[i])

	switch name {
	case "--agent", "--model":
		value, ni, vErr := takeOverrideValue(rest, i, name, inline, hasInline)
		if vErr != nil {
			return true, i, vErr
		}
		if name == "--agent" {
			ov.agent = &value
		} else {
			ov.model = &value
		}
		return true, ni, nil
	case "--yolo":
		if !hasInline {
			enabled := true
			ov.yolo = &enabled
			return true, i, nil
		}
		enabled, pErr := strconv.ParseBool(strings.TrimSpace(inline))
		if pErr != nil {
			return true, i, fmt.Errorf("invalid value %q for --yolo (want true or false)", inline)
		}
		ov.yolo = &enabled
		return true, i, nil
	default:
		return false, i, nil
	}
}

// takeOverrideValue resolves the value for a value-taking override flag,
// accepting both "--flag value" and "--flag=value" forms. It returns the index
// the caller should continue from: unchanged for the inline form, advanced past
// the consumed value for the space-separated form.
func takeOverrideValue(rest []string, i int, name, inline string, hasInline bool) (value string, next int, err error) {
	if hasInline {
		if inline == "" {
			return "", i, fmt.Errorf("%s requires a value", name)
		}
		return inline, i, nil
	}
	if i+1 >= len(rest) {
		return "", i, fmt.Errorf("%s requires a value", name)
	}
	return rest[i+1], i + 1, nil
}
