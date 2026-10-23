# Product Requirements: Stalled Session Recovery

**Author:** Jon Gallant | **Date:** 2026-08-21 | **Status:** Approved

---

## Problem Statement

Copilot CLI sessions can stop producing events while their process and lock
remain alive. Dispatch currently labels these sessions "Running, quiet," but it
does not explain why they are suspicious or help users recover them. Sessions
that have not yet been indexed into `session-store.db` can also disappear from
the searchable session list precisely when recovery is most important.

## Goals

1. Surface live sessions that have stopped making progress without depending on
   Chronicle indexing.
2. Distinguish a possibly slow operation from a likely stalled operation using
   explicit evidence and conservative thresholds.
3. Let users inspect, wait for, or restart a selected stalled session through a
   safe, confirmed workflow.
4. Verify that a resumed session actually produces new events.
5. Preserve Dispatch's read-only treatment of Copilot session-state files.

## User Personas

**Multi-session developer** - Runs several Copilot sessions concurrently and
needs to know which terminal silently stopped progressing.

**Repository maintainer** - Needs enough evidence to recover valuable work
without killing an unrelated process or replaying an unsafe operation.

**Dispatch operator** - Wants a concise recovery queue that includes active
sessions missing from the Chronicle database.

## Use Cases

1. **Inspect a suspected stall** - Dispatch observes an unchanged unfinished
   event beyond the attention threshold and shows the PID, workspace, last
   event, and inactivity duration.
2. **Wait for a slow session** - The user dismisses recovery and keeps watching;
   new event activity automatically clears the recovery state.
3. **Resume an interrupted session** - A session has a dead PID and recent
   unfinished work; Dispatch launches `copilot --resume <id>`.
4. **Restart a live stalled session** - After explicit confirmation, Dispatch
   binds the root and its descendants to platform-specific process identities,
   stops the verified process tree, confirms every bound process exited,
   resumes the session, and watches for new activity.
5. **Find an unindexed session** - A session-state directory has recovery
   evidence but no usable Chronicle row; it still appears in the recovery view.
6. **Handle ambiguous evidence** - If process identity cannot be verified,
   Dispatch refuses termination and offers diagnostics or a copyable command.

## Functional Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| FR-001 | Build recovery candidates directly from Copilot session-state directories | Must | Must not depend on `session-store.db` |
| FR-002 | Report session ID, PID, workspace, last event, inactivity, status, and reason | Must | Evidence must be user-visible |
| FR-003 | Classify live quiet sessions as suspected rather than definitively hung | Must | Avoid false certainty |
| FR-004 | Escalate to likely stalled only after unchanged unfinished activity survives a second observation window | Must | Use hysteresis |
| FR-005 | Clear recovery state immediately when event progress resumes | Must | EventWatcher-driven |
| FR-006 | Present recovery candidates even when no Chronicle session row exists | Must | Covers the observed logo-session failure |
| FR-007 | Resume interrupted sessions through the existing resume launcher | Must | Reuse `copilot --resume` |
| FR-008 | Require explicit confirmation before stopping a live process | Must | No automatic termination |
| FR-009 | Bind the root and every observed descendant to a platform-specific identity before termination | Must | Refuse on ambiguity or tree churn |
| FR-010 | Never delete or modify Copilot lock or event files | Must | Dispatch remains read-only |
| FR-011 | Verify post-resume progress and report success or failure | Must | New lock/event activity |
| FR-012 | Allow wait, inspect, copy recovery command, resume, and restart actions as applicable | Should | State-specific actions |
| FR-013 | Stop the complete verified process tree and confirm every bound process exited before resume | Must | Never resume after partial or unconfirmed termination |
| FR-014 | Use a graceful-stop window before forceful tree termination when the platform can address the target safely | Must | Unsupported graceful signaling is disclosed before confirmation |

## Non-Functional Requirements

| ID | Requirement | Target | Notes |
|----|-------------|--------|-------|
| NFR-001 | Recovery scan latency | Under 100 ms for 100 sessions | Bounded tail reads only |
| NFR-002 | Idle CPU impact | No busy polling; existing watcher plus bounded fallback scans | Preserve current architecture |
| NFR-003 | False-positive safety | No automatic process termination | User confirmation required |
| NFR-004 | Process safety | Zero termination attempts when identity verification fails | Fail closed |
| NFR-005 | Cross-platform behavior | Detection and identity-bound restart on Windows, macOS, Linux | Unsupported or unverifiable targets are disabled clearly |
| NFR-006 | Accessibility | Recovery state and actions communicated with text, not color alone | Terminal UI |
| NFR-007 | Compatibility | Existing attention states, filters, and interrupted-session resume continue to work | Additive evolution |

## Success Metrics

| Metric | Baseline | Target | Measurement |
|--------|----------|--------|-------------|
| Observed stalled session discoverability | Missing when unindexed | 100% of valid session-state candidates shown | Automated fixtures |
| Unsafe termination attempts | Not applicable | 0 | Verification tests |
| Recovery confirmation | Manual external workflow | New event progress detected after successful resume | Integration tests |
| Existing attention regressions | 0 expected | 0 | Full Go test suite |
| Candidate scan performance | Existing attention scan under 50 ms typical | Under 100 ms for 100 sessions | Benchmark/test |

## Out of Scope

- Modifying Copilot CLI to emit heartbeats or expose a control channel.
- Sending messages into a currently frozen Copilot process.
- Automatically killing or restarting sessions without confirmation.
- Deleting stale lock files or rewriting `events.jsonl`.
- Replaying tool calls or determining whether external side effects completed.
- Replacing Chronicle or indexing all raw event content for general search.

## Decisions

- Recovery candidates originate from Copilot session-state, including sessions
  absent from Chronicle.
- Live sessions are described as "suspected stalled" or "likely stalled," never
  definitively "hung."
- Restart is always user-confirmed and operates on a verified process tree.
- Dispatch resumes only after every bound process has a confirmed exit.
- Windows uses process handles and job containment where available; Linux uses
  pidfds and process identity metadata; macOS uses kernel process metadata and
  creation-time identity tokens.
- A platform or target that cannot satisfy the identity checks gets diagnostics
  and copyable manual recovery guidance, not a weaker termination path.
