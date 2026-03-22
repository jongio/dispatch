# Security Audit: Contributor Recognition Feature

**Date**: 2026-03-21
**Branch**: `feature/contributor-recognition`
**Scope**: `internal/contributors/`, `cmd/contributors/`, `.github/workflows/release.yml`, `.goreleaser.yml`
**Auditor**: SecOps Agent (STRIDE + OWASP Top 10 + CWE-mapped)
**Status**: PASS -- all findings remediated or risk-accepted

---

## Executive Summary

Full threat-model-driven security audit of the contributor recognition feature.
The feature extracts contributor data from git history via `exec.CommandContext`,
formats it as markdown, and integrates into the CI/CD release pipeline.

**0 CRITICAL** | **0 HIGH** | **1 MEDIUM (fixed)** | **2 LOW (fixed)** | **1 INFO (accepted)**

All AUTO-FIX and INFORM items have been applied. Build and full test suite pass.

---

## Phase 1: Attack Surface Enumeration

| Entry Point | Trust Boundary | Data Flow |
|---|---|---|
| `ExtractContributors(repoDir, fromTag, toTag)` | Library API -> OS subprocess | CLI args -> git command args |
| `ExtractContributorsUpTo(repoDir, ref)` | Library API -> OS subprocess | CLI args -> git command args |
| `ExtractAllContributors(repoDir)` | Library API -> OS subprocess | CWD -> git working directory |
| `gitOutput(repoDir, args...)` | Go process -> git subprocess | String args -> exec.Command |
| `parseGitLogOutput(output)` | Git stdout -> Go string parsing | Pipe-delimited text -> structs |
| `parseCoAuthoredBy(output)` | Git stdout -> Go string parsing | Trailer text -> structs |
| `formatEntry(c)` / `FormatMarkdown(...)` | Contributor data -> Markdown output | Struct fields -> rendered text |
| `runAll(repoDir)` | CLI -> filesystem write | Formatted text -> CONTRIBUTORS.md |
| CI `release.yml` step "Generate contributor notes" | GitHub Actions -> git/gh CLI | Tag names -> shell commands |
| CI `release.yml` step "Add contributor notes to release" | File content -> gh release edit | Markdown -> GitHub release body |

---

## Phase 2: STRIDE Threat Model

### Component: `gitOutput` (subprocess execution)

| Category | Threat | Mitigated? | Control |
|---|---|---|---|
| **Spoofing** | Attacker impersonates git binary via PATH | Yes | `exec.Command("git", ...)` uses PATH lookup; CI uses pinned runner images |
| **Tampering** | Malicious ref args alter git behavior | **Yes (SEC-01 fix)** | `validateRef()` rejects refs starting with `-` |
| **Repudiation** | Git commands not logged | Partial | Errors returned but not logged; acceptable for CLI tool |
| **Information Disclosure** | `--all` flag injection leaks cross-branch data | **Yes (SEC-01 fix)** | `validateRef()` blocks `-`-prefixed args |
| **Denial of Service** | Hung git subprocess blocks forever | **Yes (SEC-02 fix)** | `context.WithTimeout(60s)` kills stalled processes |
| **Elevation of Privilege** | N/A | N/A | No privilege changes in subprocess |

### Component: `formatEntry` / `FormatMarkdown` (output formatting)

| Category | Threat | Mitigated? | Control |
|---|---|---|---|
| **Spoofing** | Crafted names impersonate other contributors | **Yes (SEC-03 fix)** | `sanitizeMD()` strips markdown link/formatting chars |
| **Tampering** | Markdown injection alters document structure | **Yes (SEC-03 fix)** | `sanitizeMD()` strips `*[]()<>\`\n\r` |
| **Repudiation** | N/A | N/A | |
| **Information Disclosure** | N/A | N/A | Names are already public git metadata |
| **Denial of Service** | Extremely long names cause OOM | No (LOW risk) | Go string ops are memory-bound but git names have practical limits |
| **Elevation of Privilege** | N/A | N/A | |

### Component: CI/CD `release.yml`

| Category | Threat | Mitigated? | Control |
|---|---|---|---|
| **Spoofing** | Workflow triggered by unauthorized actor | Yes | `workflow_dispatch` requires repo write access |
| **Tampering** | Contributor data injected into release notes | Yes | `sanitizeMD()` + shell variables double-quoted |
| **Repudiation** | Release actions not attributable | Yes | GitHub Actions audit log + bot identity |
| **Information Disclosure** | GITHUB_TOKEN leaked in logs | Yes | GitHub masks secrets in logs automatically |
| **Denial of Service** | Concurrent releases corrupt state | Yes | `concurrency: group: release, cancel-in-progress: false` |
| **Elevation of Privilege** | Push to main bypasses branch protection | **Accepted (SEC-05)** | Standard CI pattern; branch protection settings control this |

### Component: CLI argument parsing (`cmd/contributors/main.go`)

| Category | Threat | Mitigated? | Control |
|---|---|---|---|
| **Spoofing** | N/A | N/A | Local CLI, user is the operator |
| **Tampering** | Malicious tag names passed via args | **Yes (SEC-01 fix)** | `validateRef()` in library layer |
| **Repudiation** | N/A | N/A | Local tool |
| **Information Disclosure** | Error messages leak internals | Yes | Errors wrap context, don't expose file paths |
| **Denial of Service** | N/A | N/A | Local CLI, user controls own resources |
| **Elevation of Privilege** | N/A | N/A | No privilege operations |

---

## Phase 3: OWASP Top 10 (2021) Coverage

| # | Category | Status | Notes |
|---|---|---|---|
| A01 | Broken Access Control | PASS | No auth required (local CLI); CI uses `contents: write` (minimal) |
| A02 | Cryptographic Failures | N/A | No encryption, no secrets handled by this feature |
| A03 | Injection | **FIXED** | SEC-01 (git arg injection), SEC-03 (markdown injection) |
| A04 | Insecure Design | PASS | `exec.Command` (no shell), input validation, output sanitization |
| A05 | Security Misconfiguration | PASS | CI actions pinned to SHA, concurrency controls, minimal permissions |
| A06 | Vulnerable Components | PASS | No new dependencies; stdlib only |
| A07 | Auth Failures | N/A | No authentication in scope |
| A08 | Software/Data Integrity | PASS | Cosign signing, SBOM generation, SHA-pinned actions |
| A09 | Logging/Monitoring | PASS | CLI tool, errors returned to caller; CI has Actions audit log |
| A10 | SSRF | N/A | No network requests made by this feature |

---

## Phase 4: Findings Detail

### SEC-01: Git Argument Injection via Unsanitized Rev Range [FIXED]

| Field | Value |
|---|---|
| **Severity** | MEDIUM |
| **CWE** | CWE-88: Improper Neutralization of Argument Delimiters in a Command |
| **CVSS** | 5.3 (AV:L/AC:L/PR:N/UI:N/S:U/C:L/I:L/A:N) |
| **STRIDE** | Tampering |
| **OWASP** | A03:2021 Injection |
| **Classification** | [AUTO-FIX] |
| **Status** | Fixed |

**Attack**: Tag/ref arguments (`fromTag`, `toTag`, `ref`) passed directly to `git log` as positional
arguments. A ref starting with `-` (e.g., `--all`, `--remotes`) would be interpreted as a git flag,
altering the command behavior. In CI, `PREV_TAG` comes from `git describe` which reads tag names
from the repository -- a malicious tag named `--all` would inject into the git log command.

**Vector**: `exec.CommandContext("git", "log", "--all", "--format=%aN|%aE")` instead of the intended
`exec.CommandContext("git", "log", "v1.0..v2.0", "--format=%aN|%aE")`.

**Impact**: Information disclosure (cross-branch contributor data), inaccurate release notes.
Not RCE because `exec.Command` does not invoke a shell.

**Fix**: Added `validateRef()` function that rejects any ref starting with `-`.
Called in `ExtractContributors()` (both `fromTag` and `toTag`) and `ExtractContributorsUpTo()`.
Empty strings are allowed (used for "all history" mode).

```go
func validateRef(ref string) error {
    if strings.HasPrefix(ref, "-") {
        return fmt.Errorf("invalid git ref %q: must not start with '-'", ref)
    }
    return nil
}
```

---

### SEC-02: Missing Execution Timeout on Git Subprocesses [FIXED]

| Field | Value |
|---|---|
| **Severity** | LOW |
| **CWE** | CWE-400: Uncontrolled Resource Consumption |
| **CVSS** | 3.1 (AV:L/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:L) |
| **STRIDE** | Denial of Service |
| **OWASP** | N/A |
| **Classification** | [AUTO-FIX] |
| **Status** | Fixed |

**Attack**: `gitOutput()` used `context.Background()` which never times out. A corrupted repository,
network-mounted filesystem, or adversarial git configuration could cause the subprocess to hang
indefinitely, blocking the caller.

**Fix**: Replaced with `context.WithTimeout(context.Background(), 60*time.Second)`. The 60-second
timeout is generous for any reasonable repository but prevents indefinite hangs.

```go
const gitTimeout = 60 * time.Second

func gitOutput(repoDir string, args ...string) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
    defer cancel()
    cmd := exec.CommandContext(ctx, "git", args...)
    // ...
}
```

---

### SEC-03: Unsanitized Contributor Names in Markdown Output [FIXED]

| Field | Value |
|---|---|
| **Severity** | LOW |
| **CWE** | CWE-79: Improper Neutralization of Input During Web Page Generation |
| **CVSS** | 3.5 (AV:N/AC:L/PR:L/UI:R/S:U/C:N/I:L/A:N) |
| **STRIDE** | Spoofing |
| **OWASP** | A03:2021 Injection |
| **Classification** | [INFORM] |
| **Status** | Fixed |

**Attack**: Git commit author names and co-author trailer names are user-controlled. A contributor
with a crafted name containing markdown syntax (`**`, `[link](url)`, `<script>`, backticks, or
newlines) could alter the rendered appearance of CONTRIBUTORS.md or GitHub release notes. While
GitHub sanitizes raw HTML in markdown rendering (preventing XSS), markdown structure injection
(fake links, spoofed formatting, document structure breaks via newlines) remains possible.

**Fix**: Added `sanitizeMD()` function that strips markdown-significant characters from contributor
names and handles before embedding in output. Stripped characters: `*`, `[`, `]`, `(`, `)`, `` ` ``,
`<`, `>`, `\n`, `\r`.

```go
var mdReplacer = strings.NewReplacer(
    "*", "", "[", "", "]", "", "(", "", ")", "",
    "`", "", "<", "", ">", "", "\n", " ", "\r", "",
)
func sanitizeMD(s string) string { return mdReplacer.Replace(s) }
```

---

### SEC-05: CI Workflow Pushes Directly to Main Branch [ACCEPTED]

| Field | Value |
|---|---|
| **Severity** | INFO |
| **CWE** | CWE-284: Improper Access Control |
| **CVSS** | 2.0 |
| **STRIDE** | Elevation of Privilege |
| **OWASP** | A01:2021 Broken Access Control |
| **Classification** | [INFORM] |
| **Status** | Risk Accepted |

**Observation**: The release workflow commits CONTRIBUTORS.md and pushes directly to `main` via
`git push origin HEAD:main`. This bypasses branch protection rules (status checks, required
reviews). The `github-actions[bot]` token with `contents: write` permission can push directly
unless branch protection is configured to block this.

**Risk acceptance rationale**: This is a standard pattern for CI automation. The push is
auto-generated content (contributor list) with no code changes. Branch protection settings
at the repository level control whether this is allowed. No code change required.

---

## Phase 5: Supply Chain Analysis

| Check | Status | Detail |
|---|---|---|
| New dependencies | CLEAN | No new deps; `internal/contributors` uses only stdlib |
| Lockfile integrity | CLEAN | `go.sum` committed, no modifications needed |
| Known CVEs | CLEAN | No vulnerable packages in contributor feature |
| Typosquatting | N/A | No new imports to evaluate |
| Abandoned deps | N/A | No new imports |
| License compliance | CLEAN | All stdlib (BSD-3-Clause) |
| CI actions pinned | CLEAN | All actions pinned to full SHA with version comments |
| Install scripts | CLEAN | No install scripts in contributor feature |

---

## Phase 6: Infrastructure Review

| Check | Status | Detail |
|---|---|---|
| CI permissions | PASS | `contents: write` + `id-token: write` (minimal for release) |
| CI concurrency | PASS | `cancel-in-progress: false` prevents race conditions |
| Action pinning | PASS | All 4 actions use full SHA: checkout, setup-go, cosign, sbom |
| goreleaser version | PASS | Pinned to `v2.9.0` |
| Signing | PASS | Cosign OIDC with GitHub Actions issuer |
| SBOM | PASS | Syft generates SBOMs for archives |

---

## Phase 7: Secrets Scan

| Check | Status | Detail |
|---|---|---|
| Hardcoded credentials | CLEAN | No passwords, keys, tokens, or API keys in changed files |
| Git history | CLEAN | No `.env`, `.key`, `.pem`, or credential files in git log |
| Environment variables | CLEAN | Only `GITHUB_TOKEN` used, from `secrets.GITHUB_TOKEN` |
| Secret in logs/errors | CLEAN | Error messages contain git stderr, no secrets |

---

## Phase 8: Regex/ReDoS Analysis

| Pattern | Location | Safe? | Analysis |
|---|---|---|---|
| `^(?:\d+\+)?([^@]+)@users\.noreply\.github\.com$` | `noreplyPattern` | Yes | No overlapping quantifiers; `[^@]+` is bounded by `@` |
| `^(.+?)\s*<([^>]+)>$` | `coAuthorPattern` | Yes | Non-greedy `.+?` + `[^>]+` bounded by `>`; no catastrophic backtracking |

---

## Phase 9: Compliance Notes

| Standard | Applicability | Status |
|---|---|---|
| GDPR | LOW | Git author emails are processed; no PII storage beyond what git provides |
| SOC 2 | N/A | Local CLI tool, no service component |
| HIPAA | N/A | No health data |
| PCI-DSS | N/A | No payment data |

---

## Remediation Summary

| ID | Fix | Lines Changed | Verified |
|---|---|---|---|
| SEC-01 | `validateRef()` + calls in public API | +12 lines | `go build` + `go test` PASS |
| SEC-02 | `context.WithTimeout(60s)` in `gitOutput()` | +4 lines | `go build` + `go test` PASS |
| SEC-03 | `sanitizeMD()` in `formatEntry()` | +12 lines | `go build` + `go test` PASS |

**Build**: `go build ./...` -- PASS
**Tests**: `go test ./... -count=1` -- 12/12 packages PASS

---

## Verdict

**PASS** -- Ready for DevOps. No CRITICAL or HIGH findings. All MEDIUM/LOW findings fixed.
One INFO finding (SEC-05) risk-accepted with documented rationale.
