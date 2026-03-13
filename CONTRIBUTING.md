# Contributing to Copilot Dispatch

Thank you for your interest in contributing! Here's how to get started.

## Prerequisites

- **Go 1.26.1+**
- **Git**
- **[Mage](https://magefile.org/)** — Go-based build tool (install: `go install github.com/magefile/mage@latest`)

### Optional Tools

These are used by `mage preflight` but skipped gracefully if not installed:

```sh
go install mvdan.cc/gofumpt@latest           # Strict formatting
go install golang.org/x/vuln/cmd/govulncheck@latest  # Vulnerability scanner
go install golang.org/x/tools/cmd/deadcode@latest    # Dead code detection
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest  # Linter
```

## Development Setup

1. **Clone and build**:
   ```sh
   git clone https://github.com/jongio/dispatch.git
   cd dispatch
   go build ./...
   ```

2. **Run tests**:
   ```sh
   go test ./... -count=1
   ```

3. **Install locally** (test, build, add to PATH, verify):
   ```sh
   mage install
   ```

4. **Run full CI check** before submitting a PR:
   ```sh
   mage preflight
   ```

## Build Targets

| Target           | Description                                              |
|------------------|----------------------------------------------------------|
| `mage install`   | Test, kill stale processes, build dev binary, update PATH |
| `mage test`      | Run tests with race detector and shuffle                 |
| `mage build`     | Compile dev binary with version info                     |
| `mage preflight` | Full 11-step CI verification (see below)                 |
| `mage vet`       | Run `go vet ./...`                                       |
| `mage lint`      | Run `golangci-lint` (falls back to `go vet`)             |
| `mage fmt`       | Format all Go source files                               |
| `mage clean`     | Remove `bin/` directory                                  |
| `mage testWSL`   | Run tests under WSL for Unix code paths                  |
| `mage coverageReport` | Generate `coverage.html`                            |

## Preflight Steps

`mage preflight` runs these checks in order:

1. `gofmt` formatting
2. `go mod tidy` dependency tidiness
3. `go vet ./...` static analysis
4. `golangci-lint run` (skipped if not installed)
5. `go build ./...` compilation
6. `go test ./... -count=1` unit tests
7. `go test -race ./... -count=1` race detection
8. WSL tests for Unix code paths (skipped if WSL unavailable)
9. `govulncheck ./...` vulnerability scan (skipped if not installed)
10. `gofumpt -l .` strict formatting (skipped if not installed)
11. `deadcode ./...` dead code detection (skipped if not installed)

## Project Structure

```
cmd/dispatch/           Entry point
internal/
  config/               User configuration (JSON, launch modes)
  copilot/              Copilot SDK client (streaming AI chat)
  data/                 SQLite session store, models, filters
  platform/             OS-specific shell/terminal helpers
  tui/                  Bubble Tea model, key bindings, messages
  tui/components/       Reusable TUI components
  tui/styles/           Lipgloss styling and color schemes
web/                    Project website (Astro)
scripts/                Screenshot generation
```

## Making Changes

1. Fork the repository and create a feature branch.
2. Make your changes, keeping commits focused and well-described.
3. **Build after every change**: `go build ./...`
4. Add or update tests for any new functionality.
5. Run `mage preflight` to verify everything passes.
6. Open a pull request with a clear description of what changed and why.

## Code Style

- Format with `gofumpt` (stricter than `gofmt`).
- Follow standard Go conventions and pass `go vet` and `golangci-lint`.
- Keep functions focused and files under 200 lines when practical.
- Use table-driven tests.
- Use `map[string]struct{}` for set semantics, not `map[string]bool`.
- Extract numeric literals (limits, timeouts, buffer sizes) into named constants.
- Comment only when the code isn't self-explanatory.

## Reporting Issues

- Use GitHub Issues to report bugs or request features.
- Include your OS, Go version, and steps to reproduce.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
