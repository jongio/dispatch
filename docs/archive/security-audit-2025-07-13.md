# Security Audit Report -- dispatch

**Date**: 2025-07-13
**Auditor**: SecOps Agent (dual-model)
**Target**: `github.com/jongio/dispatch` -- Go 1.26.1 Bubble Tea v2 TUI
**Scope**: Full 9-phase audit (Attack Surface, STRIDE, OWASP, Code, Supply Chain, Infra, Secrets, Compliance, Report)

---

## Executive Summary

The dispatch codebase demonstrates **strong security posture** for a local CLI/TUI application. The developers have proactively addressed most attack vectors with parameterized SQL queries, session ID validation regex, multi-platform shell escaping (POSIX/PowerShell/cmd.exe/AppleScript), zip path-traversal guards, download size limits, HTTPS-only redirect enforcement, config file permission hardening (0o700/0o600), and LD_PRELOAD environment filtering.

Extensive security test suites exist for SQL injection (`store_security_test.go`), command injection (`platform_security_test.go`), and malformed data handling.

The audit found **1 MEDIUM** and **3 LOW** severity findings. No CRITICAL or HIGH vulnerabilities were identified. All findings are defense-in-depth improvements appropriate for a local CLI tool's threat model.

---

## Findings Summary

| # | Severity | CWE | STRIDE | OWASP | File | Title |
|---|----------|-----|--------|-------|------|-------|
| 1 | MEDIUM | CWE-428 | Elevation of Privilege | A08:2021 | `internal/data/chronicle_windows.go:32` | Unquoted binary path in ConPTY command string |
| 2 | MEDIUM | CWE-494 | Tampering | A08:2021 | `internal/platform/fonts.go:59` | Font download has no integrity verification |
| 3 | LOW | CWE-862 | Elevation of Privilege | A01:2021 | `internal/copilot/client.go:159` | SDK PermissionHandler auto-approves all tool requests |
| 4 | LOW | CWE-732 | Tampering | A01:2021 | `internal/platform/fonts.go:244,292,329` | Font files created with default permissions (0o666) |

---

## Detailed Findings

### Finding 1: Unquoted binary path in ConPTY command string [INFORM]

| Field | Value |
|-------|-------|
| **Severity** | MEDIUM |
| **CVSS** | 4.7 (AV:L/AC:H/PR:L/UI:N/S:U/C:N/I:H/A:N) |
| **CWE** | CWE-428: Unquoted Search Path or Element |
| **STRIDE** | Elevation of Privilege |
| **OWASP** | A08:2021 Software and Data Integrity Failures |
| **File** | `internal/data/chronicle_windows.go:32` |

**Description**: The Windows ConPTY `startPTY` function concatenates the binary path directly into the command-line string without quoting:

```go
args := binary + ` --no-auto-update --no-color --no-custom-instructions`
cpty, err := conpty.Start(args, conpty.ConPtyDimensions(ptyDimCols, ptyDimRows))
```

The `binary` path is resolved from `findCopilotBinary()` which constructs paths like:
`C:\Program Files\nodejs\node_modules\@github\copilot\...\copilot.exe`

When ConPTY passes this unquoted string to `CreateProcess` with a NULL `lpApplicationName`, Windows applies its standard path-search algorithm: it tries `C:\Program.exe`, then `C:\Program Files\nodejs\...`, etc. If an attacker can place a `C:\Program.exe` binary, it would execute instead of copilot.exe.

**Exploitability**: LOW -- writing to `C:\` requires admin/elevated privileges on standard Windows configurations. The binary itself also runs at the same privilege level as the user.

**Fix**:

```go
func startPTY(binary string) (io.ReadWriteCloser, error) {
    // Quote the binary path to prevent CWE-428 unquoted search path.
    args := `"` + binary + `" --no-auto-update --no-color --no-custom-instructions`
    cpty, err := conpty.Start(args, conpty.ConPtyDimensions(ptyDimCols, ptyDimRows))
    if err != nil {
        return nil, err
    }
    return &ptyHandle{cpty: cpty}, nil
}
```

---

### Finding 2: Font download has no integrity verification [INFORM]

| Field | Value |
|-------|-------|
| **Severity** | MEDIUM |
| **CVSS** | 4.2 (AV:N/AC:H/PR:N/UI:N/S:U/C:N/I:H/A:N) |
| **CWE** | CWE-494: Download of Code Without Integrity Check |
| **STRIDE** | Tampering |
| **OWASP** | A08:2021 Software and Data Integrity Failures |
| **File** | `internal/platform/fonts.go:59,219-258` |

**Description**: `InstallNerdFont()` downloads a zip archive from GitHub Releases and extracts TTF files into the user's font directory without verifying a SHA-256 checksum or signature. While the download enforces HTTPS and rejects non-HTTPS redirects (good), a supply-chain compromise of the `ryanoasis/nerd-fonts` GitHub repository or a CDN/CA compromise could serve malicious TTF files.

Malicious fonts have been used as attack vectors historically (Windows GDI font parsing CVEs). The font files are installed system-wide and parsed by the OS font renderer.

**Mitigations already present**: HTTPS-only, redirect validation, size limit (256 MiB download, 50 MiB per file), `.ttf` extension filter, path traversal protection.

**Fix**: Add a hardcoded SHA-256 hash for the expected zip and verify after download:

```go
const nerdFontZipSHA256 = "abc123..." // update on version bumps

func downloadFile(dst, url string) error {
    // ... existing download code ...
    // After download, verify integrity:
    hash, err := computeSHA256(dst)
    if err != nil {
        return fmt.Errorf("computing checksum: %w", err)
    }
    if hash != nerdFontZipSHA256 {
        os.Remove(dst)
        return fmt.Errorf("checksum mismatch: expected %s, got %s", nerdFontZipSHA256, hash)
    }
    return nil
}
```

---

### Finding 3: SDK PermissionHandler auto-approves all tool requests [INFORM]

| Field | Value |
|-------|-------|
| **Severity** | LOW |
| **CVSS** | 3.1 (AV:N/AC:H/PR:N/UI:R/S:U/C:L/I:N/A:N) |
| **CWE** | CWE-862: Missing Authorization |
| **STRIDE** | Elevation of Privilege |
| **OWASP** | A01:2021 Broken Access Control |
| **File** | `internal/copilot/client.go:159` |

**Description**: Both `SendMessage()` and `doSearch()` configure the SDK session with `OnPermissionRequest: sdk.PermissionHandler.ApproveAll`. This auto-approves any tool permission the LLM backend requests.

Currently safe because the 4 registered tools (`search_sessions`, `get_session_detail`, `list_repositories`, `search_deep`) are all read-only against the SQLite store. However, if the Copilot SDK introduces built-in tools with write capabilities (file system, network, etc.), they would also be auto-approved.

**Mitigating factors**: Tools are explicitly registered by `defineTools()` and the SDK should only invoke registered tools. The store is opened read-only. Tool results are sanitized via `SanitizeExternalContent()`.

**Fix**: Replace with a permission handler that only approves the registered tool names:

```go
OnPermissionRequest: func(req sdk.PermissionRequest) bool {
    allowed := map[string]bool{
        "search_sessions":    true,
        "get_session_detail": true,
        "list_repositories":  true,
        "search_deep":        true,
    }
    return allowed[req.ToolName]
},
```

---

### Finding 4: Font files created with default permissions [INFORM]

| Field | Value |
|-------|-------|
| **Severity** | LOW |
| **CVSS** | 2.0 (AV:L/AC:H/PR:L/UI:N/S:U/C:N/I:L/A:N) |
| **CWE** | CWE-732: Incorrect Permission Assignment for Critical Resource |
| **STRIDE** | Tampering |
| **OWASP** | A01:2021 Broken Access Control |
| **File** | `internal/platform/fonts.go:244,292,329` |

**Description**: Three uses of `os.Create()` for font files rely on the default mode 0o666 (modified by umask):
- Line 244: `os.Create(dst)` in `downloadFile` (temp zip)
- Line 292: `os.Create(dst)` in `extractTTF` (extracted TTF in temp dir)
- Line 329: `os.Create(dst)` in `copyFile` (final installed font)

On systems with permissive umask (0o000), these files are world-writable. A local attacker could replace an installed font file with a malicious one targeting font parser vulnerabilities.

**Fix**: Use `os.OpenFile` with explicit permissions:

```go
// For temp files:
out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
// For installed fonts (read-only for owner):
out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
```

---

## STRIDE Threat Model

### Component: SQLite Store (`internal/data/store.go`)

| Category | Assessment |
|----------|------------|
| **Spoofing** | PASS -- DB path comes from `~/.copilot/session-store.db` or `DISPATCH_DB` env var. Both are user-controlled at the same privilege level. Read-only mode prevents DB spoofing attacks. |
| **Tampering** | PASS -- Opened with `?mode=ro`. `Maintain()` opens read-write but only runs FTS5 rebuild/optimize (no data modification). SQL injection prevented by parameterized queries. Extensive test coverage proves this. |
| **Repudiation** | N/A -- Read-only session browser, no actions to audit. |
| **Information Disclosure** | PASS -- Error messages include file paths (expected for CLI) but no goroutine IDs or internal state. Verified by `TestErrorMessages_OpenPathNoInternalLeak`. |
| **Denial of Service** | PASS -- Query limits (100 default, 10000 for groups). `isDBBusy` handles lock contention gracefully. Corrupt DB returns clean errors (tested). |
| **Elevation of Privilege** | PASS -- All user input goes through parameterized queries. Sort/pivot enums use switch with safe defaults. |

### Component: Shell/Launch (`internal/platform/shell.go`)

| Category | Assessment |
|----------|------------|
| **Spoofing** | PASS -- Shell binaries resolved via `exec.LookPath` (PATH-based) and `os.Stat` (explicit paths). On Unix, `$SHELL` validated as absolute path to existing non-directory file. |
| **Tampering** | PASS -- `filterEnv()` strips LD_PRELOAD/DYLD_INSERT_LIBRARIES on Unix. |
| **Repudiation** | N/A |
| **Information Disclosure** | PASS -- No secrets in command lines. GITHUB_TOKEN stays in env, not in args. |
| **Denial of Service** | PASS -- No unbounded loops. |
| **Elevation of Privilege** | PASS -- Session IDs validated with `^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`. Shell metacharacters escaped per platform (`shellQuote`, `psQuote`, `cmdEscape`, `escapeAppleScript`). Null bytes stripped (CWE-626). Custom commands validated for embedded newlines. `exec.Command` with separate args (no shell interpretation) for the non-custom path. |

### Component: Font Installation (`internal/platform/fonts.go`)

| Category | Assessment |
|----------|------------|
| **Spoofing** | PASS -- Download URL is a compile-time constant pointing to GitHub HTTPS. |
| **Tampering** | FINDING #2 (no integrity check), FINDING #4 (file permissions). |
| **Repudiation** | N/A |
| **Information Disclosure** | PASS -- No sensitive data involved. |
| **Denial of Service** | PASS -- Download capped at 256 MiB, per-file 50 MiB, total 500 MiB. HTTP timeout 60s. Max 10 redirects. |
| **Elevation of Privilege** | PASS -- Path traversal prevented: `filepath.Base()` + checks for empty/`.`/`..`/separators. Only `.ttf` files extracted. |

### Component: Chronicle Reindex (`internal/data/chronicle.go`)

| Category | Assessment |
|----------|------------|
| **Spoofing** | PASS -- Binary path from hardcoded filesystem locations (no user input). |
| **Tampering** | PASS -- Only sends fixed command strings to PTY. Output is read-only. |
| **Repudiation** | PASS -- Reindex progress streamed to callback (logged in TUI). |
| **Information Disclosure** | PASS -- ANSI stripping prevents escape sequence injection. |
| **Denial of Service** | PASS -- Timeouts on all phases (20s startup, 5s experimental, 120s reindex, 5s exit). Context cancellation supported. |
| **Elevation of Privilege** | FINDING #1 (unquoted path on Windows). |

### Component: Copilot SDK Client (`internal/copilot/client.go`)

| Category | Assessment |
|----------|------------|
| **Spoofing** | PASS -- Auth via GITHUB_TOKEN env var or SDK default auth flow. |
| **Tampering** | PASS -- Tool results sanitized via `SanitizeExternalContent()`. User queries quoted via `QuoteUntrusted()`. |
| **Repudiation** | PASS -- All SDK operations logged via `slog.Debug`. |
| **Information Disclosure** | PASS -- SDK error messages forwarded to TUI but contain no local secrets. `GITHUB_TOKEN` passed directly to SDK, never logged. |
| **Denial of Service** | PASS -- 30s operation timeout. 3 retry limit with delays. Channel buffer bounded (64). |
| **Elevation of Privilege** | FINDING #3 (ApproveAll). |

### Component: Config (`internal/config/config.go`)

| Category | Assessment |
|----------|------------|
| **Spoofing** | PASS -- Config path is platform-standard (`UserConfigDir()/dispatch`). |
| **Tampering** | PASS -- Dir 0o700, file 0o600. JSON deserialization into typed struct (no arbitrary code execution). Default values for missing fields. |
| **Repudiation** | N/A |
| **Information Disclosure** | PASS -- Config contains preferences only, no secrets. |
| **Denial of Service** | PASS -- Config files are small JSON. No size limit needed. |
| **Elevation of Privilege** | PASS -- Config values (shell, terminal, agent, model) are validated or passed as separate exec.Command args (not through shell). Custom command has newline validation + session ID validation. |

---

## OWASP Top 10 Coverage Matrix (2021)

| OWASP Category | Applicable | Status | Notes |
|----------------|-----------|--------|-------|
| **A01: Broken Access Control** | Partially | PASS (with LOW findings) | DB read-only. Config 0o600. Findings #3, #4 are defense-in-depth. |
| **A02: Cryptographic Failures** | Partially | PASS | HTTPS enforced for downloads. No custom crypto. GITHUB_TOKEN handled via env/SDK. |
| **A03: Injection** | Yes | PASS | SQL: parameterized queries, LIKE escaping, tested. Command: session ID regex, shell quoting per platform, null byte stripping, tested. |
| **A04: Insecure Design** | Partially | PASS | Read-only architecture. No write operations on session data. Defense-in-depth throughout. |
| **A05: Security Misconfiguration** | Partially | PASS | Config dir 0o700, file 0o600. Dangerous env vars filtered on Unix. Debug logs only to file (not stderr in TUI mode). |
| **A06: Vulnerable/Outdated Components** | Yes | PASS | All dependencies pinned with hashes in go.sum. No known CVEs in direct deps. |
| **A07: ID and Auth Failures** | Partially | PASS | Auth delegated to Copilot SDK. GITHUB_TOKEN from env, never hardcoded. |
| **A08: Software/Data Integrity** | Yes | MEDIUM (Findings #1, #2) | Font download lacks checksum. ConPTY path unquoted. Installer scripts DO verify checksums. |
| **A09: Logging/Monitoring Failures** | Partially | PASS | slog.Debug for SDK operations. PTY output streamed. Log file 0o600. No secrets in logs. |
| **A10: Server-Side Request Forgery** | No | N/A | No server-side processing. Download URL is a compile-time constant. |

---

## Supply Chain Analysis

| Check | Status | Detail |
|-------|--------|--------|
| **go.sum committed** | PASS | `go.sum` present with integrity hashes for all modules. |
| **No wildcard versions** | PASS | All dependencies pinned to exact versions in go.mod. |
| **Direct dependency CVEs** | PASS | No known CVEs in direct dependencies (modernc.org/sqlite, bubbletea, lipgloss, copilot-sdk, creack/pty, conpty). |
| **Transitive dependencies** | PASS | Indirect deps are well-maintained (golang.org/x/sys, golang.org/x/text, google/uuid). |
| **Typosquatting risk** | PASS | All imports from established orgs (charmbracelet, github, golang, modernc, google). |
| **Abandoned dependencies** | PASS | All deps have recent activity. |
| **CI/CD pinning** | PASS | `.goreleaser.yml` present for release builds. Install scripts verify SHA-256 checksums. |
| **License compatibility** | PASS | MIT license for dispatch. Dependencies use MIT, BSD-3, Apache-2.0 (all compatible). |
| **Install scripts** | PASS | Both `install.ps1` and `install.sh` download checksums and verify SHA-256 before installation. Temp dirs cleaned up. |

---

## Secrets Scan

| Check | Status | Detail |
|-------|--------|--------|
| **Hardcoded credentials** | CLEAN | No passwords, API keys, or tokens in source. |
| **Git history** | CLEAN | No `.env`, `.key`, `.pem`, or `.secret` files ever committed. No `ghp_` patterns found. |
| **Environment variables** | SAFE | `GITHUB_TOKEN` read from env, passed to SDK, never logged or displayed. `DISPATCH_LOG` opens log file with 0o600. `DISPATCH_DB` is read-only path override. |

---

## Regex/ReDoS Analysis

| Regex | File | Risk | Reason |
|-------|------|------|--------|
| `\x1b\[[0-9;]*[a-zA-Z]\|...` | `chronicle.go:48` | SAFE | Applied to 8KB PTY chunks. `.*?\x07` is lazy but bounded by chunk size. |
| `^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$` | `shell.go:54` | SAFE | Anchored, no backtracking, bounded length. |
| `^#[0-9A-Fa-f]{6}$` | `scheme.go:18` | SAFE | Anchored, fixed length, no alternation. |
| `\x1b\[([0-9;]*)m` | `screenshots/main.go:209` | SAFE | Simple character class, no nesting. |

---

## Positive Security Practices (Notable)

These are worth documenting because they show deliberate security engineering:

1. **SQL injection prevention**: Every user-controlled value uses `?` placeholders. `escapeLIKE()` escapes `%`, `_`, `\`. Sort/pivot values use switch statements with hardcoded safe defaults. Comprehensive test suite with 12+ injection payloads.

2. **Command injection prevention**: Strict session ID regex. Platform-specific escaping for 4 shell environments. Null byte stripping (CWE-626). `exec.Command` with separate args avoids shell interpretation. Custom command newline validation.

3. **Path traversal prevention**: `filepath.Base()` plus explicit checks for `.`, `..`, and path separators in zip extraction. Only `.ttf` extension extracted.

4. **Resource exhaustion prevention**: Download 256 MiB cap, per-file 50 MiB, total extraction 500 MiB, HTTP 60s timeout, max 10 redirects, query limits (100/10000), PTY channel buffer (1000), SDK event buffer (64).

5. **LD_PRELOAD stripping**: `filterEnv()` removes LD_PRELOAD, LD_LIBRARY_PATH, LD_AUDIT, DYLD_INSERT_LIBRARIES, DYLD_LIBRARY_PATH, DYLD_FRAMEWORK_PATH from child process environments on Unix.

6. **Prompt injection defense**: `SanitizeExternalContent()` wraps untrusted data in boundary markers with embedded delimiter defusing. `QuoteUntrusted()` uses Go `%q` formatting for single-line values.

7. **Config security**: Directory 0o700, file 0o600. JSON-to-struct deserialization (no eval). Defaults for missing fields. Reset capability.

8. **HTTPS enforcement**: Font download rejects non-HTTPS redirects with explicit error message.

---

## Remediation Roadmap

| Priority | Finding | Effort | Action |
|----------|---------|--------|--------|
| 1 | #1 ConPTY unquoted path | 5 min | Quote binary path in command string |
| 2 | #2 Font integrity | 30 min | Add SHA-256 hash constant and verification after download |
| 3 | #3 ApproveAll handler | 15 min | Implement allowlist-based permission handler |
| 4 | #4 Font file perms | 10 min | Use `os.OpenFile` with explicit 0o644 for installed fonts, 0o600 for temp |

All findings classified as **[INFORM]** -- standard best-practice improvements with clear correct implementations, suitable for batch application.

---

## Conclusion

The dispatch project has a mature security posture with deliberate defense-in-depth across all major attack surfaces. The 4 findings are all defense-in-depth hardening opportunities -- none represent exploitable vulnerabilities in the application's actual threat model (local CLI tool operated by the machine owner). The codebase includes comprehensive security test suites that actively verify injection resistance and input handling. Ready for deployment.
