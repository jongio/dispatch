# Contributing Guide for AI Agents

Instructions for AI coding agents working on this project.

## Build & Test

```bash
go build ./...          # Build — run after every change
go test ./... -count=1  # Test — all packages must pass
go vet ./...            # Lint
mage preflight          # Full CI check (vet + test + build)
```

## Project Structure

- **Language:** Go
- **TUI framework:** [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Key packages:**
  - `cmd/dispatch/` — entry point
  - `internal/tui/` — TUI model, components, styles
  - `internal/data/` — SQLite session store
  - `internal/copilot/` — Copilot SDK client
  - `internal/config/` — user configuration
  - `internal/platform/` — OS-specific helpers
