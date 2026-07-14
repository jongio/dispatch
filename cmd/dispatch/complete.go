package main

import (
	"fmt"
	"io"
	"sort"
)

// completionShells lists the shells the completion scripts support. It is the
// canonical source for the "shells" completion kind.
var completionShells = []string{"bash", "zsh", "fish", "powershell"}

// runComplete backs the hidden "__complete <kind>" command. It prints
// newline-separated completion candidates for the requested kind so the shell
// completion scripts can offer dynamic values (session aliases, config keys)
// that cannot be hardcoded. Unknown kinds print nothing so a completer never
// breaks. The command is intentionally absent from help and usage output.
func runComplete(w io.Writer, args []string) {
	if w == nil {
		w = io.Discard
	}
	if len(args) < 2 {
		return
	}

	var candidates []string
	switch args[1] {
	case "aliases":
		candidates = completeAliases()
	case "config-keys":
		candidates = completeConfigKeys()
	case "shells":
		candidates = completionShells
	default:
		return
	}

	for _, c := range candidates {
		fmt.Fprintln(w, c)
	}
}

// completeAliases returns the configured session aliases in sorted order. A
// config load failure yields no candidates rather than an error so completion
// stays non-fatal.
func completeAliases() []string {
	cfg, err := configLoadFn()
	if err != nil {
		return nil
	}
	aliases := make([]string, 0, len(cfg.SessionAliases))
	for _, alias := range cfg.SessionAliases {
		if alias != "" {
			aliases = append(aliases, alias)
		}
	}
	sort.Strings(aliases)
	return aliases
}

// completeConfigKeys returns the settable config key names in their stable
// display order.
func completeConfigKeys() []string {
	fields := configFields()
	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		keys = append(keys, f.name)
	}
	return keys
}
