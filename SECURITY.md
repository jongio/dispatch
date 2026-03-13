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

Copilot Dispatch is a **local-only** tool that reads from your Copilot CLI
session store (`~/.copilot/session-store.db`) in **read-only mode**. It does
not make network requests, except when optionally downloading Nerd Fonts.

### Trust boundaries

- **Configuration file** (platform config directory, e.g.
  `~/.config/dispatch/config.json`): Treated as trusted user
  input. If a `custom_command` is configured, it is executed as-is. Keep your
  config file permissions restricted to your user account.
- **Session data**: Read from the Copilot CLI SQLite database in read-only mode.
  Malformed session data cannot cause code execution.
- **Font downloads**: Optional Nerd Font installation downloads from GitHub
  Releases. Verify checksums if you have security concerns.
