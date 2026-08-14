package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	listGetwdFn    = os.Getwd
	listDemoModeFn = func() bool { return os.Getenv("DISPATCH_DEMO") == "1" }
)

// runList is the human-readable, current-directory preset for search.
func runList(w io.Writer, args []string) error {
	opts, err := parseListArgs(args)
	if err != nil {
		return err
	}

	return runSearchOptions(w, opts)
}

// parseListArgs reuses search parsing with list-specific defaults.
func parseListArgs(args []string) (searchOptions, error) {
	opts, err := parseSearchArgsWithDefaults(args, searchOptions{
		sort:   defaultSearchSort(),
		limit:  searchDefaultLimit,
		format: searchFormatTable,
	})
	if err != nil {
		return searchOptions{}, err
	}

	if listDemoModeFn() {
		return opts, nil
	}

	opts.filter.Folder, err = resolveListFolder(opts.filter.Folder)
	if err != nil {
		return searchOptions{}, err
	}

	return opts, nil
}

// resolveListFolder returns an absolute, existing directory for list scoping.
func resolveListFolder(folder string) (string, error) {
	if folder == "" {
		var err error
		folder, err = listGetwdFn()
		if err != nil {
			return "", fmt.Errorf("get current directory: %w", err)
		}
	}

	absolute, err := filepath.Abs(folder)
	if err != nil {
		return "", fmt.Errorf("resolve folder %q: %w", folder, err)
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("folder %q: %w", absolute, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("folder %q is not a directory", absolute)
	}

	return absolute, nil
}
