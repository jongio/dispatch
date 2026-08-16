package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/platform"
	"github.com/jongio/dispatch/internal/update"
	"github.com/jongio/dispatch/internal/version"
)

// handleArgs processes CLI arguments and executes early-exit subcommands
// (help, version, update, clear-cache, reindex). It returns done=true when
// the caller should exit without starting the TUI. Bare tokens that are not a
// known subcommand and do not start with "-" are treated as an initial search
// query; the startup filter flags (--current, --cwd, --repo, --branch,
// --query) seed structured filters. Both are returned in startup so the caller
// can seed the TUI. When --demo is among the arguments, cleanup is non-nil and
// the caller must defer it. Errors indicate a failing subcommand, an unknown
// flag, a bad path, or a non-git directory; the error message is already
// printed to stderr.
//
// Function variables (below) allow test substitution of external calls.
var (
	chronicleReindexFn = data.ChronicleReindex
	maintainFn         = data.Maintain
	runUpdateFn        = update.RunUpdate
	configResetFn      = config.Reset

	// doctorCopilotVersionFn resolves the Copilot CLI version string for the
	// doctor report; doctorSessionCountFn counts stored sessions. Both are
	// seams so tests can substitute them without touching the environment.
	doctorCopilotVersionFn = defaultCopilotVersion
	doctorSessionCountFn   = defaultSessionCount
	doctorWorkspacesFn     = defaultDoctorWorkspaces
)

type versionOutput struct {
	Version string `json:"version"`
}

func runVersion(w io.Writer, args []string) error {
	if w == nil {
		w = io.Discard
	}

	jsonOut := false
	rest := args
	if len(rest) > 0 {
		rest = rest[1:]
	}
	for _, arg := range rest {
		switch arg {
		case "--json":
			jsonOut = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown flag: %s", arg)
			}
			return fmt.Errorf("version does not take positional arguments, got %q", arg)
		}
	}

	if jsonOut {
		return json.NewEncoder(w).Encode(versionOutput{Version: version.Version})
	}
	_, err := fmt.Fprintln(w, version.Version)
	return err
}

func handleArgs(args []string, origStderr io.Writer, updateCh <-chan *update.UpdateInfo) (done bool, cleanup func(), startup startupOptions, err error) {
	args, demo := extractDemoFlag(args)
	if demo {
		c, demoErr := setupDemo()
		if demoErr != nil {
			fmt.Fprintf(os.Stderr, "demo: %v\n", demoErr)
			return true, nil, startupOptions{}, demoErr
		}
		cleanup = c
	}

	var flags startupFlags
	positionalQueryStarted := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if positionalQueryStarted && !strings.HasPrefix(arg, "-") {
			flags.queryParts = append(flags.queryParts, arg)
			continue
		}
		switch arg {
		case "--":
			flags.queryParts = append(flags.queryParts, args[i+1:]...)
			i = len(args)

		case "--help", "-h", "help":
			printUsage()
			showUpdateNotification(origStderr, updateCh)
			return true, cleanup, startupOptions{}, nil

		case "--version", "-v", "version":
			if vErr := runVersion(os.Stdout, args[i:]); vErr != nil {
				fmt.Fprintf(os.Stderr, "version: %v\n", vErr)
				return true, cleanup, startupOptions{}, vErr
			}
			showUpdateNotification(origStderr, updateCh)
			return true, cleanup, startupOptions{}, nil

		case "update":
			if uErr := runUpdateFn(context.Background(), version.Version); uErr != nil {
				fmt.Fprintf(os.Stderr, "update: %v\n", uErr)
				return true, cleanup, startupOptions{}, uErr
			}
			return true, cleanup, startupOptions{}, nil

		case "completion":
			commandArgs := args[i:]
			if len(commandArgs) < 2 {
				err := errors.New("completion requires a shell: bash, zsh, fish, or powershell")
				fmt.Fprintf(os.Stderr, "completion: %v\n", err)
				return true, cleanup, startupOptions{}, err
			}
			if cErr := runCompletion(os.Stdout, commandArgs[1]); cErr != nil {
				fmt.Fprintf(os.Stderr, "completion: %v\n", cErr)
				return true, cleanup, startupOptions{}, cErr
			}
			return true, cleanup, startupOptions{}, nil

		case "doctor":
			opts, dErr := parseDoctorArgs(args[i:])
			if dErr != nil {
				fmt.Fprintf(os.Stderr, "doctor: %v\n", dErr)
				return true, cleanup, startupOptions{}, dErr
			}
			var report doctorReport
			if opts.JSON {
				var jErr error
				report, jErr = runDoctorJSON(os.Stdout)
				if jErr != nil {
					fmt.Fprintf(os.Stderr, "doctor: %v\n", jErr)
					return true, cleanup, startupOptions{}, jErr
				}
			} else {
				report = runDoctor(os.Stdout)
			}
			if opts.Strict && !report.OK {
				err := errors.New("health checks failed")
				fmt.Fprintf(os.Stderr, "doctor: %v\n", err)
				return true, cleanup, startupOptions{}, err
			}
			showUpdateNotification(origStderr, updateCh)
			return true, cleanup, startupOptions{}, nil

		case "open":
			if oErr := runOpen(os.Stdout, args[i:]); oErr != nil {
				fmt.Fprintf(os.Stderr, "open: %v\n", oErr)
				return true, cleanup, startupOptions{}, oErr
			}
			return true, cleanup, startupOptions{}, nil

		case "new":
			if nErr := runNew(os.Stdout, args[i:]); nErr != nil {
				fmt.Fprintf(os.Stderr, "new: %v\n", nErr)
				return true, cleanup, startupOptions{}, nErr
			}
			return true, cleanup, startupOptions{}, nil

		case "stats":
			if sErr := runStats(os.Stdout, args[i:]); sErr != nil {
				fmt.Fprintf(os.Stderr, "stats: %v\n", sErr)
				return true, cleanup, startupOptions{}, sErr
			}
			return true, cleanup, startupOptions{}, nil

		case "search":
			if sErr := runSearch(os.Stdout, args[i:]); sErr != nil {
				fmt.Fprintf(os.Stderr, "search: %v\n", sErr)
				return true, cleanup, startupOptions{}, sErr
			}
			return true, cleanup, startupOptions{}, nil

		case "resume":
			if lErr := runList(os.Stdout, args[i:]); lErr != nil {
				fmt.Fprintf(os.Stderr, "resume: %v\n", lErr)
				return true, cleanup, startupOptions{}, lErr
			}
			return true, cleanup, startupOptions{}, nil

		case "tags":
			if tErr := runTags(os.Stdout, args[i:]); tErr != nil {
				fmt.Fprintf(os.Stderr, "tags: %v\n", tErr)
				return true, cleanup, startupOptions{}, tErr
			}
			return true, cleanup, startupOptions{}, nil

		case "notes":
			if nErr := runNotes(os.Stdout, args[i:]); nErr != nil {
				fmt.Fprintf(os.Stderr, "notes: %v\n", nErr)
				return true, cleanup, startupOptions{}, nErr
			}
			return true, cleanup, startupOptions{}, nil

		case "views":
			if vErr := runViews(os.Stdout, args[i:]); vErr != nil {
				fmt.Fprintf(os.Stderr, "views: %v\n", vErr)
				return true, cleanup, startupOptions{}, vErr
			}
			return true, cleanup, startupOptions{}, nil

		case "config":
			if cErr := runConfig(os.Stdout, args[i:]); cErr != nil {
				fmt.Fprintf(os.Stderr, "config: %v\n", cErr)
				return true, cleanup, startupOptions{}, cErr
			}
			return true, cleanup, startupOptions{}, nil

		case "export":
			if eErr := runExport(os.Stdout, args[i:]); eErr != nil {
				fmt.Fprintf(os.Stderr, "export: %v\n", eErr)
				return true, cleanup, startupOptions{}, eErr
			}
			return true, cleanup, startupOptions{}, nil

		case "info":
			if iErr := runInfo(os.Stdout, args[i:]); iErr != nil {
				fmt.Fprintf(os.Stderr, "info: %v\n", iErr)
				return true, cleanup, startupOptions{}, iErr
			}
			return true, cleanup, startupOptions{}, nil

		case "path":
			if pErr := runPath(os.Stdout, args[i:]); pErr != nil {
				fmt.Fprintf(os.Stderr, "path: %v\n", pErr)
				return true, cleanup, startupOptions{}, pErr
			}
			return true, cleanup, startupOptions{}, nil

		case "compare":
			if cErr := runCompare(os.Stdout, args[i:]); cErr != nil {
				fmt.Fprintf(os.Stderr, "compare: %v\n", cErr)
				return true, cleanup, startupOptions{}, cErr
			}
			return true, cleanup, startupOptions{}, nil

		case "aliases":
			if aErr := runAliases(os.Stdout, args[i:]); aErr != nil {
				fmt.Fprintf(os.Stderr, "aliases: %v\n", aErr)
				return true, cleanup, startupOptions{}, aErr
			}
			return true, cleanup, startupOptions{}, nil

		case "alias":
			if aErr := runAlias(os.Stdout, args[i:]); aErr != nil {
				fmt.Fprintf(os.Stderr, "alias: %v\n", aErr)
				return true, cleanup, startupOptions{}, aErr
			}
			return true, cleanup, startupOptions{}, nil

		case "prune":
			if pErr := runPrune(os.Stdout, args[i:]); pErr != nil {
				fmt.Fprintf(os.Stderr, "prune: %v\n", pErr)
				return true, cleanup, startupOptions{}, pErr
			}
			return true, cleanup, startupOptions{}, nil

		case "tag":
			if tErr := runTag(os.Stdout, args[i:]); tErr != nil {
				fmt.Fprintf(os.Stderr, "tag: %v\n", tErr)
				return true, cleanup, startupOptions{}, tErr
			}
			return true, cleanup, startupOptions{}, nil

		case "watch":
			if wErr := runWatch(os.Stdout, args[i:]); wErr != nil {
				fmt.Fprintf(os.Stderr, "watch: %v\n", wErr)
				return true, cleanup, startupOptions{}, wErr
			}
			return true, cleanup, startupOptions{}, nil

		case "man":
			if mErr := runMan(os.Stdout); mErr != nil {
				fmt.Fprintf(os.Stderr, "man: %v\n", mErr)
				return true, cleanup, startupOptions{}, mErr
			}
			return true, cleanup, startupOptions{}, nil

		case "__complete":
			// Hidden helper used by the shell completion scripts to fetch
			// dynamic candidates. Deliberately omitted from help and usage.
			runComplete(os.Stdout, args[i:])
			return true, cleanup, startupOptions{}, nil
		case "--clear-cache":
			if cErr := configResetFn(); cErr != nil {
				fmt.Fprintf(os.Stderr, "clear-cache: %v\n", cErr)
				return true, cleanup, startupOptions{}, cErr
			}
			fmt.Println("Config reset to defaults.")
			return true, cleanup, startupOptions{}, nil

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
						return true, cleanup, startupOptions{}, mErr
					}
				} else {
					fmt.Fprintf(os.Stderr, "reindex: %v\n", rErr)
					return true, cleanup, startupOptions{}, rErr
				}
			}
			// Post-reindex maintenance (WAL checkpoint + FTS5 optimize).
			if mErr := maintainFn(context.Background()); mErr != nil {
				fmt.Fprintf(os.Stderr, "warning: post-reindex maintenance: %v\n", mErr)
			}
			fmt.Println("Done.")
			return true, cleanup, startupOptions{}, nil

		case "--current":
			flags.current = true
			positionalQueryStarted = true

		case "--repo", "--branch", "--cwd", "--query":
			value, next, ok := flagValue(args, i, arg)
			if !ok {
				fmt.Fprintf(os.Stderr, "%s requires a value\n", arg)
				printUsage()
				return true, cleanup, startupOptions{}, fmt.Errorf("%s requires a value", arg)
			}
			i = next
			switch arg {
			case "--repo":
				flags.repo = value
			case "--branch":
				flags.branch = value
			case "--cwd":
				flags.cwd = value
			case "--query":
				flags.query = value
			}
			positionalQueryStarted = true

		default:
			// Handle the inline forms (--repo=foo) and unknown flags.
			if strings.HasPrefix(arg, "-") {
				if key, value, ok := splitInlineFlag(arg); ok {
					switch key {
					case "--repo":
						flags.repo = value
					case "--branch":
						flags.branch = value
					case "--cwd":
						flags.cwd = value
					case "--query":
						flags.query = value
					default:
						return true, cleanup, startupOptions{}, unknownFlag(arg)
					}
					positionalQueryStarted = true
					continue
				}
				return true, cleanup, startupOptions{}, unknownFlag(arg)
			}
			flags.queryParts = append(flags.queryParts, arg)
			positionalQueryStarted = true
		}
	}

	startup, err = resolveStartupOptions(flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return true, cleanup, startupOptions{}, err
	}
	return false, cleanup, startup, nil
}

func extractDemoFlag(args []string) ([]string, bool) {
	cleaned := make([]string, 0, len(args))
	demo := false
	command := ""
	for i, arg := range args {
		if arg == "--" {
			cleaned = append(cleaned, args[i:]...)
			break
		}
		if command == "" && !strings.HasPrefix(arg, "-") {
			command = arg
		}
		if arg == "--demo" {
			if command != "" && command != "resume" && command != "search" {
				cleaned = append(cleaned, arg)
				continue
			}
			if i > 0 && searchFlagConsumesValue(args[i-1]) {
				cleaned = append(cleaned, arg)
				continue
			}
			demo = true
			continue
		}
		cleaned = append(cleaned, arg)
	}
	return cleaned, demo
}

func searchFlagConsumesValue(arg string) bool {
	if strings.Contains(arg, "=") {
		return false
	}
	switch arg {
	case "--branch", "--folder", "--format", "--host", "--limit", "--order",
		"--query", "--repo", "--repository", "--since", "--sort", "--tag",
		"--until", "-n", "-q":
		return true
	default:
		return false
	}
}

// unknownFlag prints the usage banner and returns the unknown-flag error. It is
// shared by the direct and inline flag parsing paths.
func unknownFlag(arg string) error {
	fmt.Fprintf(os.Stderr, "unknown flag: %s\n", arg)
	printUsage()
	return fmt.Errorf("unknown flag: %s", arg)
}

// flagValue returns the value for a value-taking flag at index i. It supports
// both "--flag value" (consuming args[i+1]) and "--flag=value" forms. The
// returned next index is the position the caller should advance i to.
func flagValue(args []string, i int, flag string) (value string, next int, ok bool) {
	if v, found := strings.CutPrefix(args[i], flag+"="); found {
		return v, i, v != ""
	}
	if i+1 < len(args) {
		return args[i+1], i + 1, true
	}
	return "", i, false
}

// splitInlineFlag splits a "--flag=value" argument into its flag and value.
// It returns ok=false when the argument has no "=".
func splitInlineFlag(arg string) (flag, value string, ok bool) {
	idx := strings.IndexByte(arg, '=')
	if idx <= 0 {
		return "", "", false
	}
	return arg[:idx], arg[idx+1:], true
}

// cliCommands is the canonical list of top-level subcommands accepted by
// handleArgs. Several surfaces mirror this list — the usage banner in
// main.go, the manCommands table in man.go, and the four shell completion
// scripts below. Those mirrors are hand-written on purpose (the completion
// scripts must stay readable as shell code), so drift tests in this package
// assert every command here appears in each of them.
var cliCommands = []string{
	"help", "version", "open", "new", "doctor", "update", "completion",
	"stats", "search", "resume", "tags", "notes", "views", "aliases", "alias",
	"compare", "prune", "tag", "watch", "config", "export", "info",
	"path", "man",
}

// configSubcommands is the canonical list of `dispatch config` subcommands
// accepted by runConfig. Mirrored by the completion scripts and the man
// page synopsis; guarded by the same drift tests as cliCommands.
var configSubcommands = []string{
	"list", "get", "set", "unset", "edit", "path",
	"validate", "schema", "export", "import",
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
	case "fish":
		fmt.Fprint(w, fishCompletionScript)
	case "powershell", "pwsh":
		fmt.Fprint(w, powershellCompletionScript)
	default:
		return fmt.Errorf("unsupported shell %q (want bash, zsh, fish, or powershell)", shell)
	}
	return nil
}

const bashCompletionScript = `# bash completion for dispatch
_dispatch_completion() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local bin="${COMP_WORDS[0]}"
  local commands="help version open new doctor update completion stats search resume tags notes views aliases alias compare prune tag watch config export info path man"
  local flags="-h --help -v --version --demo --clear-cache --reindex --current --cwd --repo --branch --query --"
  local cmd_index=1
  [[ "${COMP_WORDS[1]}" == "--demo" ]] && cmd_index=2

  if [[ "${COMP_CWORD}" -eq "${cmd_index}" ]]; then
    COMPREPLY=( $(compgen -W "${commands} ${flags}" -- "${cur}") )
    return 0
  fi

  case "${COMP_WORDS[cmd_index]}" in
    completion)
      COMPREPLY=( $(compgen -W "$("${bin}" __complete shells)" -- "${cur}") )
      return 0
      ;;
    open)
      COMPREPLY=( $(compgen -W "$("${bin}" __complete aliases) --mode --last --print --agent --model --yolo --repo --branch --folder --current" -- "${cur}") )
      return 0
      ;;
    new)
      COMPREPLY=( $(compgen -W "--mode --agent --model --yolo" -- "${cur}") )
      return 0
      ;;
    resume)
      COMPREPLY=( $(compgen -W "--json --jsonl --ids --paths --commands --csv --table --format -q --query --deep --repo --repository --branch --folder --tag --host --since --until --sort --order -n --limit" -- "${cur}") )
      return 0
      ;;
    config)
      if [[ "${COMP_CWORD}" -eq $((cmd_index + 1)) ]]; then
        COMPREPLY=( $(compgen -W "list get set unset edit path validate schema export import" -- "${cur}") )
      elif [[ "${COMP_WORDS[cmd_index + 1]}" == "get" || "${COMP_WORDS[cmd_index + 1]}" == "set" || "${COMP_WORDS[cmd_index + 1]}" == "unset" ]]; then
        COMPREPLY=( $(compgen -W "$("${bin}" __complete config-keys)" -- "${cur}") )
      fi
      return 0
      ;;
  esac
}
complete -F _dispatch_completion dispatch disp
`

const zshCompletionScript = `#compdef dispatch disp
_dispatch_completion() {
  local -a commands flags configsubs shells aliases configkeys openflags newflags resumeflags
  local bin=${words[1]}
  local cmd_index=2
  [[ ${words[2]} == --demo ]] && cmd_index=3
  commands=(help version open new doctor update completion stats search resume tags notes views aliases alias compare prune tag watch config export info path man)
  configsubs=(list get set unset edit path validate schema export import)
  openflags=(--mode --last --print --agent --model --yolo --repo --branch --folder --current)
  newflags=(--mode --agent --model --yolo)
  resumeflags=(--json --jsonl --ids --paths --commands --csv --table --format -q --query --deep --repo --repository --branch --folder --tag --host --since --until --sort --order -n --limit)
  flags=(-h --help -v --version --demo --clear-cache --reindex --current --cwd --repo --branch --query --)

  if (( CURRENT == cmd_index )); then
    _describe -t commands 'dispatch command' commands || _describe -t flags 'dispatch flag' flags
    return
  fi

  if [[ ${words[cmd_index]} == completion ]]; then
    shells=(${(f)"$($bin __complete shells)"})
    _describe -t shells 'shell' shells
    return
  fi

  if [[ ${words[cmd_index]} == open ]]; then
    aliases=(${(f)"$($bin __complete aliases)"})
    _describe -t aliases 'session alias' aliases
    _describe -t openflags 'open flag' openflags
    return
  fi

  if [[ ${words[cmd_index]} == config ]]; then
    if (( CURRENT == cmd_index + 1 )); then
      _describe -t configsubs 'config subcommand' configsubs
    elif [[ ${words[cmd_index + 1]} == get || ${words[cmd_index + 1]} == set || ${words[cmd_index + 1]} == unset ]]; then
      configkeys=(${(f)"$($bin __complete config-keys)"})
      _describe -t configkeys 'config key' configkeys
    fi
    return
  fi

  if [[ ${words[cmd_index]} == new ]]; then
    _describe -t newflags 'new flag' newflags
    return
  fi

  if [[ ${words[cmd_index]} == resume ]]; then
    _describe -t resumeflags 'resume flag' resumeflags
    return
  fi
}
_dispatch_completion "$@"
`

const fishCompletionScript = `# fish completion for dispatch and disp
function __dispatch_needs_command
  set -l cmd (commandline -opc)
  if test (count $cmd) -eq 1
    return 0
  end
  test (count $cmd) -eq 2; and test $cmd[2] = --demo
end

function __dispatch_after
  set -l cmd (commandline -opc)
  set -l index 2
  test (count $cmd) -ge 2; and test $cmd[2] = --demo; and set index 3
  test (count $cmd) -ge $index; and test $cmd[$index] = $argv[1]
end

function __dispatch_config_key
  set -l cmd (commandline -opc)
  set -l index 2
  test (count $cmd) -ge 2; and test $cmd[2] = --demo; and set index 3
  test (count $cmd) -ge (math $index + 1); and test $cmd[$index] = config; and contains -- $cmd[(math $index + 1)] get set unset
end

function __dispatch_using_subcommand
  set -l cmd (commandline -opc)
  set -l index 2
  test (count $cmd) -ge 2; and test $cmd[2] = --demo; and set index 3
  test (count $cmd) -ge $index; and test $cmd[$index] = $argv[1]
end

for bin in dispatch disp
  complete -c $bin -f
  complete -c $bin -n '__dispatch_needs_command' -a 'help version open new doctor update completion stats search resume tags notes views aliases alias compare prune tag watch config export info path man'
  complete -c $bin -n '__dispatch_needs_command' -a '-h --help -v --version --demo --clear-cache --reindex --current --cwd --repo --branch --query --'
  complete -c $bin -n '__dispatch_after completion' -a "($bin __complete shells)"
  complete -c $bin -n '__dispatch_after open' -a "($bin __complete aliases)"
  complete -c $bin -n '__dispatch_after open' -a '--mode --last --print --agent --model --yolo --repo --branch --folder --current'
  complete -c $bin -n '__dispatch_after new' -a '--mode --agent --model --yolo'
  complete -c $bin -n '__dispatch_after resume' -a '--json --jsonl --ids --paths --commands --csv --table --format -q --query --deep --repo --repository --branch --folder --tag --host --since --until --sort --order -n --limit'
  complete -c $bin -n '__dispatch_after config' -a 'list get set unset edit path validate schema export import'
  complete -c $bin -n '__dispatch_config_key' -a "($bin __complete config-keys)"
end
`

const powershellCompletionScript = `# PowerShell completion for dispatch
$script:DispatchCommands = @('help', 'version', 'open', 'new', 'doctor', 'update', 'completion', 'stats', 'search', 'resume', 'tags', 'notes', 'views', 'aliases', 'alias', 'compare', 'prune', 'tag', 'watch', 'config', 'export', 'info', 'path', 'man')
$script:DispatchFlags = @('-h', '--help', '-v', '--version', '--demo', '--clear-cache', '--reindex', '--current', '--cwd', '--repo', '--branch', '--query', '--')
$script:DispatchConfigSubcommands = @('list', 'get', 'set', 'unset', 'edit', 'path', 'validate', 'schema', 'export', 'import')
$script:DispatchOpenFlags = @('--mode', '--last', '--print', '--agent', '--model', '--yolo', '--repo', '--branch', '--folder', '--current')
$script:DispatchNewFlags = @('--mode', '--agent', '--model', '--yolo')
$script:DispatchResumeFlags = @('--json', '--jsonl', '--ids', '--paths', '--commands', '--csv', '--table', '--format', '-q', '--query', '--deep', '--repo', '--repository', '--branch', '--folder', '--tag', '--host', '--since', '--until', '--sort', '--order', '-n', '--limit')

Register-ArgumentCompleter -Native -CommandName dispatch, disp -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $tokens = @($commandAst.CommandElements | ForEach-Object { $_.ToString() })
    $bin = $tokens[0]
    $commandIndex = if ($tokens.Count -ge 2 -and $tokens[1] -eq '--demo') { 2 } else { 1 }
    $command = if ($tokens.Count -gt $commandIndex) { $tokens[$commandIndex] } else { '' }
    $queryMode = -not ($script:DispatchCommands -contains $command) -and (
        ($tokens -contains '--' -and $wordToComplete -ne '--') -or
        ($tokens | Where-Object { $_ -match '^--(?:current|cwd(?:=|$)|repo(?:=|$)|branch(?:=|$)|query(?:=|$))' })
    )
    if ($queryMode) {
        if ([string]::IsNullOrEmpty($wordToComplete)) { "''" } else { $wordToComplete }
        return
    }
    $values = if ($command -eq 'completion') {
        & $bin __complete shells
    } elseif ($command -eq 'open') {
        (& $bin __complete aliases) + $script:DispatchOpenFlags
    } elseif ($command -eq 'config') {
        if ($tokens.Count -gt ($commandIndex + 1) -and @('get', 'set', 'unset') -contains $tokens[$commandIndex + 1]) {
            & $bin __complete config-keys
        } else {
            $script:DispatchConfigSubcommands
        }
    } elseif ($command -eq 'new') {
        $script:DispatchNewFlags
    } elseif ($command -eq 'resume') {
        $script:DispatchResumeFlags
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
	Version        string          `json:"version"`
	OS             string          `json:"os"`
	OK             bool            `json:"ok"`
	Config         doctorEntry     `json:"config"`
	SessionStore   doctorEntry     `json:"session_store"`
	SessionState   doctorEntry     `json:"session_state"`
	CopilotCLI     doctorEntry     `json:"copilot_cli"`
	CopilotVersion string          `json:"copilot_version"`
	SessionCount   int             `json:"session_count"`
	Workspaces     workspaceReport `json:"workspaces"`
}

// workspaceReport summarizes whether stored session working directories still
// exist on disk.
type workspaceReport struct {
	Total   int      `json:"total"`
	Missing int      `json:"missing"`
	Samples []string `json:"samples,omitempty"`
	Error   string   `json:"error,omitempty"`
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
		r.CopilotVersion = doctorCopilotVersionFn(p)
	}

	r.SessionCount = doctorSessionCountFn()
	r.Workspaces = doctorWorkspacesFn()
	r.OK = doctorEntryOK(r.Config) &&
		doctorEntryOK(r.SessionStore) &&
		doctorEntryOK(r.SessionState) &&
		doctorEntryOK(r.CopilotCLI)

	return r
}

type doctorOptions struct {
	JSON   bool
	Strict bool
}

func parseDoctorArgs(args []string) (doctorOptions, error) {
	var opts doctorOptions
	rest := args
	if len(rest) > 0 {
		rest = rest[1:]
	}
	for _, arg := range rest {
		switch arg {
		case "--json":
			opts.JSON = true
		case "--strict":
			opts.Strict = true
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("unknown flag: %s", arg)
			}
			return opts, fmt.Errorf("doctor does not take positional arguments, got %q", arg)
		}
	}
	return opts, nil
}

func doctorEntryOK(e doctorEntry) bool {
	return e.err == nil && e.Status == statusFound
}

// defaultCopilotVersion runs the Copilot CLI binary with --version and returns
// the first line of its output. It returns "unknown" when the binary cannot be
// run, keeping the doctor command non-fatal when the CLI misbehaves.
func defaultCopilotVersion(binary string) string {
	if binary == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "--version").Output()
	if err != nil {
		return "unknown"
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	if line == "" {
		return "unknown"
	}
	return line
}

// defaultSessionCount returns the number of stored sessions, degrading to 0
// when the store cannot be opened or queried so the doctor command stays
// non-fatal.
func defaultSessionCount() int {
	store, err := data.Open()
	if err != nil {
		return 0
	}
	defer store.Close() //nolint:errcheck // read-only, best-effort close

	n, err := store.CountSessions(context.Background())
	if err != nil {
		return 0
	}
	return n
}

func defaultDoctorWorkspaces() workspaceReport {
	store, err := data.Open()
	if err != nil {
		return workspaceReport{Error: err.Error()}
	}
	defer store.Close() //nolint:errcheck // read-only, best-effort close

	folders, err := store.ListFolders(context.Background())
	if err != nil {
		return workspaceReport{Error: err.Error()}
	}

	r := workspaceReport{Total: len(folders)}
	for _, folder := range folders {
		if strings.TrimSpace(folder) == "" {
			continue
		}
		if _, err := os.Stat(folder); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				r.Missing++
				if len(r.Samples) < 5 {
					r.Samples = append(r.Samples, folder)
				}
			}
		}
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

func runDoctor(w io.Writer) doctorReport {
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

	if r.CopilotVersion != "" {
		fmt.Fprintf(w, "Copilot CLI version: %s\n", r.CopilotVersion)
	} else {
		fmt.Fprintf(w, "Copilot CLI version: not detected\n")
	}
	fmt.Fprintf(w, "Stored sessions: %d\n", r.SessionCount)
	writeWorkspaceLine(w, r.Workspaces)
	fmt.Fprintf(w, "OK: %t\n", r.OK)
	return r
}

// runDoctorJSON writes the diagnostics as a single JSON object followed by a
// newline.
func runDoctorJSON(w io.Writer) (doctorReport, error) {
	if w == nil {
		w = io.Discard
	}
	r := collectDoctorReport()
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return r, err
	}
	fmt.Fprintf(w, "%s\n", b)
	return r, nil
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

func writeWorkspaceLine(w io.Writer, r workspaceReport) {
	if r.Error != "" {
		fmt.Fprintf(w, "Missing workspaces: unknown (%s)\n", r.Error)
		return
	}
	fmt.Fprintf(w, "Missing workspaces: %d of %d folders\n", r.Missing, r.Total)
	for _, sample := range r.Samples {
		fmt.Fprintf(w, "  - %s\n", sample)
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
