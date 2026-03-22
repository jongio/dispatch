# Security Audit: Contributor Recognition -- MQ Wave 4 Hack (Red Team)

**Date**: 2026-03-21
**Scope**: `internal/contributors/`, `cmd/contributors/`, `.github/workflows/release.yml`
**Methodology**: STRIDE threat modeling, OWASP Top 10, 5-persona adversarial red team
**Verdict**: PASS -- 1 MEDIUM fixed, 0 CRITICAL, 0 HIGH remaining

---

## Attack Surface Map

```
Entry Points:
  1. CLI args (--all, --release <from> <to>)  --> cmd/contributors/main.go
  2. Git history (author names, emails)        --> git log --format=%aN|%aE
  3. Git trailers (Co-authored-by)             --> git log --format=%(trailers:...)
  4. Git tags (revision ranges)                --> validateRef() gate
  5. CI/CD pipeline (release.yml)              --> workflow_dispatch only

Trust Boundaries:
  [Untrusted] Git commit metadata (names, emails, trailers)
  [Trusted]   CLI arguments (local user), CI workflow inputs (enum-validated)
  [Trusted]   Git binary, Go stdlib

Data Flow:
  git log --> parseGitLogOutput/parseCoAuthoredBy --> mergeContributors
    --> filterBots --> formatEntry(sanitizeMD) --> markdown output
    --> CI: contrib-notes.md --> gh release edit --notes-file -
```

---

## STRIDE Threat Model

| Component | S | T | I | R | E | D | Notes |
|-----------|---|---|---|---|---|---|-------|
| CLI arg parsing | -- | -- | -- | -- | -- | -- | Simple flag parsing, no auth |
| validateRef() | -- | PASS | -- | -- | -- | -- | Blocks `-` prefix injection (CWE-88) |
| gitOutput() | -- | PASS | -- | -- | -- | PASS | exec.CommandContext array args (no shell), 60s timeout |
| parseGitLogOutput() | -- | PASS | -- | -- | -- | -- | SplitN on `\|`, skips malformed |
| parseCoAuthoredBy() | -- | PASS | -- | -- | -- | -- | Regex match, skips no-match |
| sanitizeMD() | -- | **FIXED** | -- | -- | -- | -- | Was missing `~`, `\`, `\|` -- now stripped |
| formatEntry() | -- | PASS | -- | -- | -- | -- | Wraps sanitized output in `**...**` |
| filterBots() | -- | PASS | -- | -- | -- | -- | Case-sensitive `[bot]` suffix |
| release.yml | -- | **FIXED** | -- | -- | -- | PASS | Added `set -euo pipefail`, `--notes-file -` |

Legend: S=Spoofing, T=Tampering, I=Info Disclosure, R=Repudiation, E=Elevation, D=DoS

---

## Findings

### F-01: Incomplete markdown sanitization in sanitizeMD [FIXED]

- **Severity**: MEDIUM (CVSS 4.3 -- AV:N/AC:L/PR:L/UI:R/S:U/C:N/I:L/A:N)
- **CWE**: CWE-79 (Improper Neutralization of Input During Web Page Generation)
- **STRIDE**: Tampering
- **OWASP**: A03:2021 Injection
- **Classification**: [AUTO-FIX]

**Attack**: A contributor crafts a git author name containing `~~legitimate~~` (strikethrough),
`\` (escape breaking bold markers), or `|` (table cell injection). The `sanitizeMD` function
did not strip these characters, allowing markdown formatting manipulation in CONTRIBUTORS.md
and release notes.

**Proof of concept**: Git author name `~~Alice~~` would render with strikethrough in the
contributor list, enabling reputation manipulation. A `\` at end of name produces `**\**`
which breaks bold formatting for subsequent entries in some parsers.

**Fix applied**: Added `~`, `\`, `|` to `mdReplacer` in `contributors.go` line 338.
Added 11 new test cases covering all three characters individually and in combination.

**Verification**: `go test ./internal/contributors/... -count=1` -- all pass.

---

### F-02: CI shell steps missing strict error handling [FIXED]

- **Severity**: LOW (CVSS 2.1 -- AV:N/AC:H/PR:H/UI:N/S:U/C:N/I:L/A:N)
- **CWE**: CWE-390 (Detection of Error Condition Without Action)
- **STRIDE**: Tampering
- **OWASP**: A05:2021 Security Misconfiguration
- **Classification**: [AUTO-FIX]

**Attack**: Without `set -euo pipefail`, a failing intermediate command (e.g., `go run`
producing partial output) could be silently ignored, resulting in corrupted or incomplete
contributor notes in the release.

**Fix applied**: Added `set -euo pipefail` to both shell steps in release.yml.

---

### F-03: CI `gh release edit --notes "$VAR"` subject to ARG_MAX limits [FIXED]

- **Severity**: LOW (CVSS 2.1 -- AV:N/AC:H/PR:H/UI:N/S:U/C:N/I:N/A:L)
- **CWE**: CWE-400 (Uncontrolled Resource Consumption)
- **STRIDE**: Denial of Service
- **OWASP**: A05:2021 Security Misconfiguration
- **Classification**: [AUTO-FIX]

**Attack**: For repositories with very large contributor lists, passing release notes as a
CLI argument could exceed POSIX ARG_MAX (~128KB per string), causing silent truncation or
command failure.

**Fix applied**: Changed from `--notes "$NEW_BODY"` to `--notes-file -` with stdin pipe.
This eliminates the argument length limit entirely.

---

### F-04: Bot filter is case-sensitive (informational)

- **Severity**: INFO (CVSS 0.0)
- **CWE**: N/A
- **STRIDE**: N/A
- **OWASP**: N/A
- **Classification**: [INFORM]

**Observation**: `isBot()` matches only exact `[bot]` suffix. `[BOT]` and `[Bot]` variants
pass through. This matches GitHub's convention (all GitHub-managed bots use lowercase `[bot]`).
Documented behavior, not a vulnerability.

---

### F-05: HTML entity bypass in sanitizeMD (accepted risk)

- **Severity**: LOW (CVSS 2.0 -- AV:N/AC:H/PR:L/UI:R/S:U/C:N/I:L/A:N)
- **CWE**: CWE-79
- **STRIDE**: Tampering
- **OWASP**: A03:2021 Injection
- **Classification**: [INFORM]

**Observation**: `&lt;script&gt;` passes through `sanitizeMD` (only raw `<>` are stripped).
In GitHub's markdown rendering, HTML entities ARE decoded but script tags are sanitized by
GitHub's server-side DOMPurify equivalent. Risk only applies to third-party renderers lacking
XSS protection. Stripping `&` would damage legitimate names (e.g., "Smith & Jones").

**Risk acceptance**: GitHub rendering is safe. Third-party rendering is out of scope.
Defense in depth provided by stripping raw `<>`.

---

## Attack Persona Results

### 1. Script Kiddie -- Git Author Name Injection
- **Tested**: `<script>alert(1)</script>`, `$(whoami)`, `` `rm -rf /` ``
- **Result**: BLOCKED. `sanitizeMD` strips `<>`, backticks. Shell metacharacters stored
  verbatim (never executed -- no shell involved). 100+ existing security tests confirm.

### 2. Black Hat -- Git Argument Injection
- **Tested**: `--exec=evil`, `--upload-pack=evil`, `-n`, `--`
- **Result**: BLOCKED. `validateRef()` rejects all refs starting with `-`.
  `exec.CommandContext` uses array args (no shell expansion). 12 test cases confirm.

### 3. Bug Bounty Hunter -- Regex Pattern Bypass
- **Tested**: ReDoS with 100K char strings, nested angle brackets, subdomain injection
  in noreply pattern, unicode lookalikes, numeric prefix edge cases
- **Result**: No bypass found. `noreplyPattern` anchored with `^...$`, `[^@]+` prevents
  backtracking. `coAuthorPattern` uses non-greedy `(.+?)`. Tested with 50K+ char inputs
  under 20ms. 15+ regex edge case tests exist.

### 4. Red Team (APT) -- Supply Chain via Malicious Git History
- **Tested**: Fork repo, craft commits with injection payloads in author names/emails,
  submit PR that gets merged, trigger release
- **Result**: MITIGATED. `sanitizeMD` (now hardened) strips formatting payloads.
  `exec.CommandContext` prevents command injection. CI uses `workflow_dispatch` (manual
  trigger, write access required). Actions pinned to SHA. No third-party dependencies
  in contributor package (stdlib only).

### 5. AI-Automated Attacker -- Systematic Fuzzing
- **Tested**: Null bytes, tab characters, Windows line endings, 100K-line outputs,
  10K-char names, zero-width characters, CJK/RTL/emoji, combining diacritics
- **Result**: All handled gracefully. No panics. Parser skips malformed lines.
  Unicode preserved correctly. Performance linear. 30+ fuzz-style tests exist.

---

## OWASP Top 10 (2021) Coverage Matrix

| # | Category | Status | Notes |
|---|----------|--------|-------|
| A01 | Broken Access Control | N/A | CLI tool, no auth system |
| A02 | Cryptographic Failures | N/A | No crypto operations |
| A03 | Injection | PASS | sanitizeMD (now hardened), exec array args, validateRef |
| A04 | Insecure Design | PASS | Minimal attack surface, stdlib only |
| A05 | Security Misconfiguration | PASS | CI pipefail added, actions pinned to SHA |
| A06 | Vulnerable Components | PASS | No third-party deps in contributor package |
| A07 | Auth Failures | N/A | No authentication |
| A08 | Software/Data Integrity | PASS | cosign signing, SBOM generation, SHA-pinned actions |
| A09 | Logging/Monitoring Failures | N/A | CLI tool, no persistent logging |
| A10 | SSRF | N/A | No outbound HTTP calls |

---

## Supply Chain Assessment

- **Dependencies**: Zero third-party deps in `internal/contributors/` (stdlib only)
- **CI Actions**: All pinned to full SHA (checkout@34e114..., setup-go@40f158..., etc.)
- **Release signing**: cosign with OIDC issuer, SBOM via syft
- **Lockfile**: go.sum committed, `go mod tidy` enforced in CI
- **Vulnerability scanning**: govulncheck runs in CI pipeline

---

## Secrets Scan

- **Codebase**: No hardcoded secrets, tokens, or API keys found
- **Git history**: No deleted `.env`, `.key`, `.pem` files. No AWS access keys (AKIA pattern)
- **CI/CD**: `GITHUB_TOKEN` used via `${{ secrets.GITHUB_TOKEN }}` (not hardcoded)
- **Environment**: No secrets in environment variables or build outputs

---

## Fixes Applied

| File | Change | Lines |
|------|--------|-------|
| `internal/contributors/contributors.go` | Added `~`, `\`, `\|` to `mdReplacer` | ~3 lines |
| `internal/contributors/contributors_test.go` | 11 new security test cases | ~95 lines |
| `.github/workflows/release.yml` | `set -euo pipefail` + `--notes-file -` | ~8 lines |

## Verification

```
go build ./...              # EXIT 0
go test ./... -count=1      # ALL PASS (13 packages)
```
