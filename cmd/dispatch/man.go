package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jongio/dispatch/internal/version"
)

// manDateFn returns the date stamped into the man page header. It is a package
// variable so tests can pin it, matching the seam pattern used elsewhere in
// this package (see cli.go and open.go).
var manDateFn = func() string { return time.Now().UTC().Format("2006-01-02") }

// runMan writes a roff-formatted man page for dispatch to w. Packagers can
// redirect it to dispatch.1 for distribution (Homebrew, apt, and so on):
//
//	dispatch man > dispatch.1
//
// The output targets section 1 (user commands) and mirrors the built-in usage
// text so the two stay close.
func runMan(w io.Writer) error {
	if w == nil {
		w = io.Discard
	}
	_, err := io.WriteString(w, renderManPage())
	return err
}

// renderManPage builds the full roff document as a string.
func renderManPage() string {
	var b strings.Builder

	// Header. The double quotes group multi-word fields for the .TH macro.
	fmt.Fprintf(&b, ".TH DISPATCH 1 %q %q %q\n",
		manDateFn(), "dispatch "+version.Version, "User Commands")

	manSection(&b, "NAME")
	b.WriteString("dispatch \\- a terminal UI for browsing and launching GitHub Copilot CLI sessions\n")

	manSection(&b, "SYNOPSIS")
	b.WriteString(".B dispatch\n")
	b.WriteString(".RI [ query ]\n")
	b.WriteString(".br\n")
	b.WriteString(".B dispatch\n")
	b.WriteString(".RI [ command ]\n")
	b.WriteString(".RI [ flags ]\n")

	manSection(&b, "DESCRIPTION")
	b.WriteString("Dispatch reads your local Copilot CLI session store and presents every past\n")
	b.WriteString("session in a searchable, sortable, groupable TUI. Run it with no arguments to\n")
	b.WriteString("open the interactive interface, or pass a query to pre-fill the search box.\n")
	b.WriteString(".PP\n")
	b.WriteString("The subcommands below run without starting the TUI, which makes them useful\n")
	b.WriteString("for scripting and shell integration.\n")

	manSection(&b, "COMMANDS")
	for _, c := range manCommands {
		manItem(&b, c.term, c.desc)
	}

	manSection(&b, "FLAGS")
	for _, f := range manFlags {
		manItem(&b, f.term, f.desc)
	}

	manSection(&b, "ENVIRONMENT")
	for _, e := range manEnv {
		manItem(&b, e.term, e.desc)
	}

	manSection(&b, "EXAMPLES")
	for _, ex := range manExamples {
		b.WriteString(".PP\n")
		b.WriteString(manEscape(ex.desc) + "\n")
		b.WriteString(".PP\n")
		b.WriteString(".RS\n.EX\n")
		b.WriteString(manEscape(ex.cmd) + "\n")
		b.WriteString(".EE\n.RE\n")
	}

	manSection(&b, "SEE ALSO")
	b.WriteString("Project home: https://github.com/jongio/dispatch\n")

	return b.String()
}

// manEntry pairs a term (the tagged item) with its description paragraph.
type manEntry struct {
	term string
	desc string
}

// manCommands mirrors the "Commands" block of the usage banner.
var manCommands = []manEntry{
	{"help", "Show the usage summary."},
	{"version", "Print the version."},
	{"open <id> [--mode M]", "Resume a session by ID or alias (M: inplace, tab, window, pane). Use --last for the most recent session and --print to write the resume command instead of launching."},
	{"new [dir] [--mode M]", "Start a new session in a directory (default: current)."},
	{"completion <shell>", "Print a shell completion script (bash, zsh, fish, powershell)."},
	{"doctor [--json]", "Print environment diagnostics."},
	{"stats [flags]", "Print session totals and breakdowns."},
	{"search [query] [flags]", "Print matching sessions without the TUI."},
	{"tags [--json]", "List tags in use with per-tag session counts."},
	{"config <get|set|list|edit|path>", "Read or change preferences."},
	{"export <id> [flags]", "Export a session as Markdown or JSON."},
	{"man", "Write this man page in roff format to standard output."},
	{"update", "Update dispatch to the latest release."},
}

// manFlags mirrors the top-level flags of the usage banner.
var manFlags = []manEntry{
	{"-h, --help", "Show the usage summary."},
	{"-v, --version", "Print the version."},
	{"--demo", "Launch with demo data."},
	{"--clear-cache", "Reset config to defaults."},
	{"--reindex", "Rebuild the session store index."},
	{"--current", "Filter to the current git repo and branch (from cwd)."},
	{"--cwd <path>", "Filter to sessions under a folder (base dir for --current)."},
	{"--repo <name>", "Filter to a repository (owner/repo)."},
	{"--branch <name>", "Filter to a branch."},
	{"--query <text>", "Pre-fill the search box with free text."},
}

// manEnv mirrors the environment block of the usage banner.
var manEnv = []manEntry{
	{"DISPATCH_DB", "Path to a custom session store database."},
	{"DISPATCH_SESSION_STATE", "Path to a custom session state directory."},
	{"DISPATCH_CONFIG", "Path to a custom config file (overrides the default location)."},
	{"DISPATCH_LOG", "Path to a log file (enables debug logging)."},
	{"DISPATCH_NO_UPDATE_CHECK", "Skip the background release check when set to 1, true, yes, or on."},
}

// manExample gives a described, runnable command line.
type manExample struct {
	desc string
	cmd  string
}

// manExamples gives a few runnable command lines.
var manExamples = []manExample{
	{desc: "Launch the TUI filtered to the current repo and branch:", cmd: "dispatch --current"},
	{desc: "Export a session as JSON to standard output:", cmd: "dispatch export <id> --format json --stdout"},
	{desc: "Run a command whenever a session changes attention state:", cmd: "dispatch watch --exec 'notify-send \"$DISPATCH_SESSION_STATE\"'"},
	{desc: "Install the man page for the current user:", cmd: "dispatch man > ~/.local/share/man/man1/dispatch.1"},
}

// manSection writes a .SH section header.
func manSection(b *strings.Builder, name string) {
	fmt.Fprintf(b, ".SH %s\n", name)
}

// manItem writes a tagged paragraph (.TP): a bold term followed by a wrapped
// description. An empty term is skipped so the description renders on its own.
func manItem(b *strings.Builder, term, desc string) {
	if term == "" {
		b.WriteString(manEscape(desc) + "\n")
		return
	}
	b.WriteString(".TP\n")
	fmt.Fprintf(b, ".B %s\n", manEscape(term))
	b.WriteString(manEscape(desc) + "\n")
}

// manEscape makes a string safe to embed in roff output. It escapes the
// backslash (the roff control character) and a leading dot or apostrophe, which
// roff would otherwise read as a macro at the start of a line.
func manEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	if strings.HasPrefix(s, ".") || strings.HasPrefix(s, "'") {
		s = `\&` + s
	}
	return s
}
