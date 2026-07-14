package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/update"
)

func TestRunComplete_ConfigKeys(t *testing.T) {
	var buf bytes.Buffer
	runComplete(&buf, []string{"__complete", "config-keys"})
	lines := splitNonEmptyLines(buf.String())

	want := len(configFields())
	if len(lines) != want {
		t.Fatalf("got %d keys, want %d:\n%s", len(lines), want, buf.String())
	}
	for _, key := range []string{"agent", "model", "yoloMode", "theme"} {
		if !containsLine(lines, key) {
			t.Errorf("config-keys output missing %q", key)
		}
	}
}

func TestRunComplete_Aliases(t *testing.T) {
	cfg := config.Default()
	cfg.SessionAliases = map[string]string{"s2": "bugfix", "s1": "authfix"}
	withConfigSeams(t, cfg)

	var buf bytes.Buffer
	runComplete(&buf, []string{"__complete", "aliases"})
	// Aliases are sorted regardless of map iteration order.
	if got := buf.String(); got != "authfix\nbugfix\n" {
		t.Errorf("aliases output = %q, want sorted list", got)
	}
}

func TestRunComplete_AliasesEmpty(t *testing.T) {
	withConfigSeams(t, config.Default())

	var buf bytes.Buffer
	runComplete(&buf, []string{"__complete", "aliases"})
	if got := buf.String(); got != "" {
		t.Errorf("aliases output = %q, want empty", got)
	}
}

func TestRunComplete_AliasesLoadErrorIsSilent(t *testing.T) {
	prev := configLoadFn
	configLoadFn = func() (*config.Config, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { configLoadFn = prev })

	var buf bytes.Buffer
	runComplete(&buf, []string{"__complete", "aliases"})
	if got := buf.String(); got != "" {
		t.Errorf("aliases output = %q, want empty on load error", got)
	}
}

func TestRunComplete_Shells(t *testing.T) {
	var buf bytes.Buffer
	runComplete(&buf, []string{"__complete", "shells"})
	if got := buf.String(); got != "bash\nzsh\nfish\npowershell\n" {
		t.Errorf("shells output = %q", got)
	}
}

func TestRunComplete_UnknownKindIsSilent(t *testing.T) {
	var buf bytes.Buffer
	runComplete(&buf, []string{"__complete", "bogus"})
	if got := buf.String(); got != "" {
		t.Errorf("unknown kind output = %q, want empty", got)
	}
}

func TestRunComplete_NoKindIsSilent(t *testing.T) {
	var buf bytes.Buffer
	runComplete(&buf, []string{"__complete"})
	if got := buf.String(); got != "" {
		t.Errorf("no-kind output = %q, want empty", got)
	}
}

func TestHandleArgs_Complete(t *testing.T) {
	ch := make(chan *update.UpdateInfo, 1)

	done, cleanup, _, err := handleArgs([]string{"__complete", "shells"}, io.Discard, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("expected done=true for __complete")
	}
	if cleanup != nil {
		t.Error("expected cleanup=nil for __complete")
	}
}

func TestPrintUsage_HidesCompleteCommand(t *testing.T) {
	var buf bytes.Buffer
	prev := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	printUsage()

	w.Close()
	os.Stdout = prev
	<-done

	if strings.Contains(buf.String(), "__complete") {
		t.Errorf("printUsage must not advertise the hidden __complete command:\n%s", buf.String())
	}
}

func splitNonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func containsLine(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}
