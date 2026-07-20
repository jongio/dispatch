package main

import (
	"fmt"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
)

func parseSingleTagFilter(value, flag string) (string, error) {
	tags := config.ParseTags(value)
	if len(tags) != 1 {
		return "", fmt.Errorf("%s requires exactly one tag", flag)
	}
	return tags[0], nil
}

func loadAndFilterSessionsByTag(sessions []data.Session, tag string) ([]data.Session, error) {
	if tag == "" {
		return sessions, nil
	}
	cfg, err := configLoadFn()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return filterSessionsByTag(sessions, cfg, tag), nil
}

func filterSessionsByTag(sessions []data.Session, cfg *config.Config, tag string) []data.Session {
	if cfg == nil || tag == "" {
		return sessions
	}
	filtered := make([]data.Session, 0, len(sessions))
	for _, s := range sessions {
		if cfg.HasTag(s.ID, tag) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}
