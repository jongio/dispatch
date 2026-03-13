# Copilot Instructions for dispatch

## Build and Deploy After Every Change (MANDATORY)

After every code change (file edit, create, or delete), run:

```
go build ./...
```

This is non-negotiable. Do not batch multiple changes and then build once at the end. Build after EACH change to catch errors immediately.

If the build fails, fix the issue before making any further changes.

After the build passes, run:

```
mage install
```

This runs tests, kills running dispatch processes, builds the dev binary with version info, ensures it's in PATH, and verifies the deployment. Run `mage install` after every change, not just at the end of a task.

## Project Context

- Language: Go
- Build: `go build ./...`
- Test: `go test ./...`
- Lint: `go vet ./...`
- Deploy: `mage install`
- Preflight: `mage preflight`
