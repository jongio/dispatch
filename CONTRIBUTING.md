# Contributing to Dispatch

Thank you for your interest in contributing! Here's how to get started.

## Prerequisites

- **Go 1.26.5+** (see `go.mod` for the authoritative version)
- **Git**
- **[Mage](https://magefile.org/)** — Go-based build tool (install: `go install github.com/magefile/mage@v1.17.2`)

### Optional Tools

These are used by `mage preflight`. Install the pinned versions used by CI:

```sh
go install mvdan.cc/gofumpt@v0.11.0
go install golang.org/x/vuln/cmd/govulncheck@v1.6.0
go install golang.org/x/tools/cmd/deadcode@v0.48.0
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
```

## Installing a Release

To install the latest release (or test against a specific version):

```sh
# Latest — Linux / macOS
curl -fsSL https://github.com/jongio/dispatch/releases/latest/download/install.sh | sh

# Specific version — Linux / macOS
version=vX.Y.Z
curl -fsSL "https://github.com/jongio/dispatch/releases/download/$version/install.sh" | sh -s -- "$version"

# Latest — Windows (PowerShell)
irm https://github.com/jongio/dispatch/releases/latest/download/install.ps1 | iex

# Specific version — Windows (PowerShell)
$version="vX.Y.Z"
irm "https://github.com/jongio/dispatch/releases/download/$version/install.ps1" | iex
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
| `mage screenshots` | Regenerate website screenshot PNGs                    |
| `mage screenshotsCheck` | Verify screenshot capture without rendering PNGs |
| `mage preflight` | Full 13-step CI verification (see below)                 |
| `mage vet`       | Run `go vet ./...`                                       |
| `mage lint`      | Run `golangci-lint` (falls back to `go vet`)             |
| `mage fmt`       | Format all Go source files                               |
| `mage clean`     | Remove `bin/` directory                                  |
| `mage testWSL`   | Run tests under WSL for Unix code paths                  |
| `mage coverageReport` | Generate `coverage.html`                            |
| `mage changelogCheck` | Verify CHANGELOG.md has an entry for the latest tag |

## Preflight Steps

`mage preflight` runs these checks in order:

1. `gofmt` formatting
2. `go mod tidy` dependency tidiness
3. `go vet ./...` static analysis
4. `golangci-lint run`
5. WSL/Linux lint (skipped if WSL or its linter is unavailable)
6. `go build ./...` compilation
7. `go test ./... -count=1` unit tests
8. `go test -race ./... -count=1` race detection
9. WSL tests for Unix code paths (skipped if WSL unavailable)
10. `govulncheck ./...` vulnerability scan
11. `gofumpt -l .` strict formatting
12. `deadcode ./...` dead code detection
13. Local install and binary verification

## Project Structure

```text
cmd/dispatch/           Entry point
internal/
  config/               User configuration (JSON, launch modes)
  data/                 SQLite session store, models, filters
  platform/             OS-specific shell/terminal helpers
  tui/                  Bubble Tea model, key bindings, messages
  tui/components/       Reusable TUI components
  tui/styles/           Lipgloss styling and color schemes
  update/               Self-update mechanism (GitHub Releases)
  validate/             Input validation
  version/              Application version metadata
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

### Website Screenshots

When you change TUI visual states, themes, or website docs that reference
screenshots, regenerate them from the repo root:

```sh
npm --prefix web ci
npm --prefix web exec playwright install chromium
npm --prefix web run screenshots
```

The same npm script works from `web/` as `npm run screenshots`. PowerShell users
can run `.\scripts\screenshots.ps1`; macOS/Linux users can run
`./scripts/screenshots.sh`. Use `mage screenshotsCheck` for the fast CI-friendly
capture check, and review PNG changes with `git diff -- web/public/screenshots`.

## Code Style

- Format with `gofumpt` (stricter than `gofmt`).
- Follow standard Go conventions and pass `go vet` and `golangci-lint`.
- Keep functions focused and files under 200 lines when practical.
- Use table-driven tests.
- Use `map[string]struct{}` for set semantics, not `map[string]bool`.
- Extract numeric literals (limits, timeouts, buffer sizes) into named constants.
- Comment only when the code isn't self-explanatory.

## Recognition

All contributors are automatically recognized in:
- The **CHANGELOG.md** entry for each release
- **GitHub Release** notes
- The **CONTRIBUTORS.md** hall of fame

Your first contribution gets a special "New contributor" callout! Run `mage contributors` to see the current contributor list.

## Reporting Issues

- Use GitHub Issues to report bugs or request features.
- Include your OS, Go version, and steps to reproduce.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
