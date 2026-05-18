# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Copilot Dispatch, please report it
responsibly. **Do not open a public GitHub issue.**

Instead, please use [GitHub's private vulnerability reporting](https://github.com/jongio/dispatch/security/advisories/new)
or email the maintainer directly through their GitHub profile.

### What to include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response timeline

- **Acknowledgment**: Within 48 hours
- **Assessment**: Within 1 week
- **Fix**: As soon as practical, depending on severity

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest  | Yes       |

## Security Model

Copilot Dispatch is a **local-first** tool that reads from your Copilot CLI
session store (`~/.copilot/session-store.db`) in **read-only mode**. It makes
network requests in the following scenarios:

- **Self-update**: `dispatch update` contacts the GitHub Releases API
  (`api.github.com`) to check for and download new versions.
- **Copilot SDK**: AI-powered features (work status analysis, semantic search)
  communicate with the GitHub Copilot API. Requires an authenticated Copilot
  session.
- **Nerd Font downloads**: Optional font installation downloads from GitHub
  Releases.

### Network endpoints

The application may contact the following domains:

| Domain | Purpose |
|--------|---------|
| `api.github.com` | Release version checks, asset downloads |
| `github.com` | Release archive downloads |
| `*.githubusercontent.com` | Copilot API communication |

### Trust boundaries

- **Configuration file** (platform config directory, e.g.
  `~/.config/dispatch/config.json`): Treated as trusted user
  input. If a `custom_command` is configured, it is executed as-is. Keep your
  config file permissions restricted to your user account.
- **Session data**: Read from the Copilot CLI SQLite database in read-only mode.
  Malformed session data cannot cause code execution.
- **Font downloads**: Optional Nerd Font installation downloads from GitHub
  Releases. Verify checksums if you have security concerns.
