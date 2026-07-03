package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/update"
	"github.com/jongio/dispatch/internal/version"
)

// handleArgs processes CLI arguments and executes early-exit subcommands
// (help, version, update, clear-cache, reindex). It returns done=true when
// the caller should exit without starting the TUI. When --demo is among
// the arguments, cleanup is non-nil and the caller must defer it. Errors
// indicate a failing subcommand; the error message is already printed to
// stderr.
//
// Function variables (below) allow test substitution of external calls.
var (
	chronicleReindexFn = data.ChronicleReindex
	maintainFn         = data.Maintain
	runUpdateFn        = update.RunUpdate
	configResetFn      = config.Reset
)

func handleArgs(args []string, origStderr io.Writer, updateCh <-chan *update.UpdateInfo) (done bool, cleanup func(), err error) {
	for _, arg := range args {
		switch arg {
		case "--help", "-h", "help":
			printUsage()
			showUpdateNotification(origStderr, updateCh)
			return true, cleanup, nil

		case "--version", "-v", "version":
			fmt.Println(version.Version)
			showUpdateNotification(origStderr, updateCh)
			return true, cleanup, nil

		case "update":
			if uErr := runUpdateFn(context.Background(), version.Version); uErr != nil {
				fmt.Fprintf(os.Stderr, "update: %v\n", uErr)
				return true, cleanup, uErr
			}
			return true, cleanup, nil

		case "completion":
			if len(args) < 2 {
				err := errors.New("completion requires a shell: bash, zsh, or powershell")
				fmt.Fprintf(os.Stderr, "completion: %v\n", err)
				return true, cleanup, err
			}
			if cErr := runCompletion(os.Stdout, args[1]); cErr != nil {
				fmt.Fprintf(os.Stderr, "completion: %v\n", cErr)
				return true, cleanup, cErr
			}
			return true, cleanup, nil

		case "doctor":
			if slices.Contains(args, "--json") {
				if jErr := runDoctorJSON(os.Stdout); jErr != nil {
					fmt.Fprintf(os.Stderr, "doctor: %v\n", jErr)
					return true, cleanup, jErr
				}
			} else {
				runDoctor(os.Stdout)
			}
			showUpdateNotification(origStderr, updateCh)
			return true, cleanup, nil

		case "open":
			if oErr := runOpen(os.Stdout, args); oErr != nil {
				fmt.Fprintf(os.Stderr, "open: %v\n", oErr)
				return true, cleanup, oErr
			}
			return true, cleanup, nil

		case "stats":
			if sErr := runStats(os.Stdout, args); sErr != nil {
				fmt.Fprintf(os.Stderr, "stats: %v\n", sErr)
				return true, cleanup, sErr
			}
			return true, cleanup, nil

		case "--demo":
			c, demoErr := setupDemo()
			if demoErr != nil {
				fmt.Fprintf(os.Stderr, "demo: %v\n", demoErr)
				return true, cleanup, demoErr
			}
			cleanup = c

		case "--clear-cache":
			if cErr := configResetFn(); cErr != nil {
				fmt.Fprintf(os.Stderr, "clear-cache: %v\n", cErr)
				return true, cleanup, cErr
			}
			fmt.Println("Config reset to defaults.")
			return true, cleanup, nil

		case "--reindex":
			fmt.Println("Reindexing session store via Copilot CLI…")
			rErr := chronicleReindexFn(context.Background(), func(line string) {
				fmt.Println(line)
			})
			if rErr != nil {
				if errors.Is(rErr, data.ErrCopilotNotFound) {
					fmt.Println("Copilot CLI not found, running index maintenance…")
					if mErr := maintainFn(context.Background()); mErr != nil {
						fmt.Fprintf(os.Stderr, "reindex: %v\n", mErr)
						return true, cleanup, mErr
					}
				} else {
					fmt.Fprintf(os.Stderr, "reindex: %v\n", rErr)
					return true, cleanup, rErr
				}
			}
			// Post-reindex maintenance (WAL checkpoint + FTS5 optimize).
			if mErr := maintainFn(context.Background()); mErr != nil {
				fmt.Fprintf(os.Stderr, "warning: post-reindex maintenance: %v\n", mErr)
			}
			fmt.Println("Done.")
			return true, cleanup, nil

		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", arg)
			printUsage()
			return true, cleanup, fmt.Errorf("unknown flag: %s", arg)
		}
	}
	return false, cleanup, nil
}

func runCompletion(w io.Writer, shell string) error {
	if w == nil {
		w = io.Discard
	}
	switch strings.ToLower(shell) {
	case "bash":
		fmt.Fprint(w, bashCompletionScript)
	case "zsh":
		fmt.Fprint(w, zshCompletionScript)
	case "powershell", "pwsh":
		fmt.Fprint(w, powershellCompletionScript)
	default:
		return fmt.Errorf("unsupported shell %q (want bash, zsh, or powershell)", shell)
	}
	return nil
}

const bashCompletionScript = `# bash completion for dispatch
_dispatch_completion() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local commands="help version open doctor update completion stats"
  local flags="-h --help -v --version --demo --clear-cache --reindex"

  if [[ "${COMP_CWORD}" -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "${commands} ${flags}" -- "${cur}") )
    return 0
  fi

  if [[ "${COMP_WORDS[1]}" == "completion" ]]; then
    COMPREPLY=( $(compgen -W "bash zsh powershell" -- "${cur}") )
    return 0
  fi
}
complete -F _dispatch_completion dispatch disp
`

const zshCompletionScript = `#compdef dispatch disp
_dispatch_completion() {
  local -a commands shells flags
  commands=(help version open doctor update completion stats)
  shells=(bash zsh powershell)
  flags=(-h --help -v --version --demo --clear-cache --reindex)

  if (( CURRENT == 2 )); then
    _describe -t commands 'dispatch command' commands || _describe -t flags 'dispatch flag' flags
    return
  fi

  if [[ ${words[2]} == completion ]]; then
    _describe -t shells 'shell' shells
    return
  fi
}
_dispatch_completion "$@"
`

const powershellCompletionScript = `# PowerShell completion for dispatch
$script:DispatchCommands = @('help', 'version', 'open', 'doctor', 'update', 'completion', 'stats')
$script:DispatchFlags = @('-h', '--help', '-v', '--version', '--demo', '--clear-cache', '--reindex')
$script:DispatchShells = @('bash', 'zsh', 'powershell')

Register-ArgumentCompleter -Native -CommandName dispatch, disp -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $tokens = @($commandAst.CommandElements | ForEach-Object { $_.ToString() })
    $values = if ($tokens.Count -ge 2 -and $tokens[1] -eq 'completion') {
        $script:DispatchShells
    } else {
        $script:DispatchCommands + $script:DispatchFlags
    }
    $values |
        Where-Object { $_ -like "$wordToComplete*" } |
        ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) }
}
`

// doctorStatus values describe the result of a single diagnostic path check.
const (
	statusFound     = "found"
	statusMissing   = "missing"
	statusWrongType = "wrong_type"
	statusError     = "error"
)

// doctorEntry is the diagnostic result for one path. The err field is used
// only by the text renderer and is not serialized to JSON.
type doctorEntry struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	err    error
}

// doctorReport is the full set of diagnostics gathered by the doctor command.
// Both the text and JSON renderers consume this struct so their outputs stay
// in sync.
type doctorReport struct {
	Version      string      `json:"version"`
	OS           string      `json:"os"`
	Config       doctorEntry `json:"config"`
	SessionStore doctorEntry `json:"session_store"`
	SessionState doctorEntry `json:"session_state"`
	CopilotCLI   doctorEntry `json:"copilot_cli"`
}

// collectDoctorReport gathers the environment diagnostics once so they can be
// rendered as text or JSON without drifting apart.
func collectDoctorReport() doctorReport {
	r := doctorReport{
		Version: version.Version,
		OS:      runtime.GOOS + "/" + runtime.GOARCH,
	}

	if p, err := config.ConfigPath(); err != nil {
		r.Config = doctorEntry{Status: statusError, err: err}
	} else {
		r.Config = doctorEntry{Path: p, Status: pathStatus(p, false)}
	}

	if p, err := platform.SessionStorePath(); err != nil {
		r.SessionStore = doctorEntry{Status: statusError, err: err}
	} else {
		r.SessionStore = doctorEntry{Path: p, Status: pathStatus(p, false)}
	}

	if p := data.SessionStatePath(); p == "" {
		r.SessionState = doctorEntry{Status: statusMissing}
	} else {
		r.SessionState = doctorEntry{Path: p, Status: pathStatus(p, true)}
	}

	if p := platform.FindCLIBinary(); p == "" {
		r.CopilotCLI = doctorEntry{Status: statusMissing}
	} else {
		r.CopilotCLI = doctorEntry{Path: p, Status: statusFound}
	}

	return r
}

// pathStatus stats a path and reports whether it is found, missing, or the
// wrong type (a file where a directory is expected, or vice versa).
func pathStatus(path string, wantDir bool) string {
	info, err := os.Stat(path)
	if err != nil {
		return statusMissing
	}
	if wantDir != info.IsDir() {
		return statusWrongType
	}
	return statusFound
}

func runDoctor(w io.Writer) {
	if w == nil {
		w = io.Discard
	}

	r := collectDoctorReport()

	fmt.Fprintf(w, "Dispatch doctor\n")
	fmt.Fprintf(w, "Version: %s\n", r.Version)
	fmt.Fprintf(w, "OS: %s\n", r.OS)
	fmt.Fprintf(w, "\n")

	writeDoctorLine(w, "Config", r.Config, false)
	writeDoctorLine(w, "Session store", r.SessionStore, false)
	writeDoctorLine(w, "Session state", r.SessionState, true)
	writeDoctorLine(w, "Copilot CLI", r.CopilotCLI, false)
}

// runDoctorJSON writes the diagnostics as a single JSON object followed by a
// newline.
func runDoctorJSON(w io.Writer) error {
	if w == nil {
		w = io.Discard
	}
	b, err := json.MarshalIndent(collectDoctorReport(), "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "%s\n", b)
	return nil
}

// writeDoctorLine renders one diagnostic entry as human-readable text.
func writeDoctorLine(w io.Writer, label string, e doctorEntry, wantDir bool) {
	if e.err != nil {
		fmt.Fprintf(w, "%s: error: %v\n", label, e.err)
		return
	}
	switch e.Status {
	case statusMissing:
		if e.Path == "" {
			fmt.Fprintf(w, "%s: missing\n", label)
		} else {
			fmt.Fprintf(w, "%s: missing (%s)\n", label, e.Path)
		}
	case statusWrongType:
		if wantDir {
			fmt.Fprintf(w, "%s: wrong type, expected directory (%s)\n", label, e.Path)
		} else {
			fmt.Fprintf(w, "%s: wrong type, expected file (%s)\n", label, e.Path)
		}
	default:
		fmt.Fprintf(w, "%s: found (%s)\n", label, e.Path)
	}
}

// setupLogRedirect opens the log file (if configured via DISPATCH_LOG) and
// redirects stderr to it. When no log file is configured, stderr is sent to
// os.DevNull to keep Bubble Tea's alt-screen clean. Returns the writer for
// structured logging and a cleanup function that closes the redirect target.
func setupLogRedirect() (io.Writer, func()) {
	logFile := openLogFile(os.Getenv("DISPATCH_LOG"))
	if logFile != nil {
		redirectStderr(logFile)
		return logFile, func() { logFile.Close() } //nolint:errcheck // best-effort
	}
	if devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0); err == nil {
		redirectStderr(devNull)
		return io.Discard, func() { devNull.Close() } //nolint:errcheck // best-effort
	}
	return io.Discard, func() {}
}

// openLogFile opens a log file for writing at the given path. The path
// must be absolute and must not be a UNC path (to prevent outbound SMB
// authentication on Windows). Returns nil if the path is empty, invalid,
// or cannot be opened.
func openLogFile(logPath string) *os.File {
	if logPath == "" {
		return nil
	}
	cleaned := filepath.Clean(logPath)
	if !filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, `\\`) {
		return nil
	}
	f, err := os.OpenFile(cleaned, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil
	}
	return f
}
