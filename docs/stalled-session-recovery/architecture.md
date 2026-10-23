# Stalled Session Recovery Architecture

## Overview

Dispatch must discover Copilot CLI sessions that have stopped producing
observable progress, including sessions missing from Chronicle, and help users
recover them without terminating an unrelated process or starting a duplicate
session.

The feature builds on the existing session-state scanner, attention
classification, file-system watcher, and resume launcher. It adds recovery
evidence, a conservative two-observation classifier, a platform process-control
boundary, and a TUI recovery state machine. Copilot lock and event files remain
read-only.

The restart action is cross-platform and fail-closed. Dispatch performs no
termination when the root identity, executable ownership, descendant set, or
platform capability is ambiguous.

## Architecture

### Session-state snapshots and recovery candidates

`internal/data` owns recovery discovery because it already interprets
`~/.copilot/session-state`.

One bounded `SessionStateSnapshot` becomes the shared read model for attention,
recovery, and event-watcher updates. It contains lock evidence, the first
`session.start` context, the final complete event, and file progress metadata.
This avoids separate readers producing contradictory answers.

A recovery candidate contains:

- Session ID and optional Chronicle metadata.
- Workspace from `data.context.cwd` in the first `session.start` event.
- Live PID and process verification availability.
- Last complete event type, timestamp, and file progress marker.
- Inactivity duration and observation count.
- Classification: interrupted, suspected stalled, or likely stalled.
- Human-readable reason and supported actions.

The scanner extends the existing bounded directory and event-tail reads. It
reads only a capped first-event region for `session.start` context and a capped
tail for progress. It does not index event bodies or read entire event logs.
Session IDs, paths, event types, and rendered reasons are validated and
sanitized before reaching the TUI.

### Progress observation

The first unfinished event that exceeds the configured attention threshold
creates a suspected-stall observation. A candidate becomes likely stalled only
after one additional attention-threshold interval sees the same event progress
marker and the same live process identity. Reusing the configured threshold
avoids a second timing setting and preserves the meaning users already assign
to "running, quiet."

The progress marker uses file identity, size, modification time, and the final
complete event's type and timestamp. Any append, replacement, lock transition,
or process-identity change clears the prior observation. `EventWatcher` remains
the push path; the existing bounded fallback scan supplies elapsed-time
transitions when no file event occurs.

An `internal/data` recovery tracker owns observation history, keyed by session
ID, the unique live lock-owner identity, and progress marker. It resets on
process or lock change, progress, session disappearance, threshold change, and
successful recovery. TUI recreation does not manufacture a second observation;
after restart, candidates begin as suspected until a complete second window is
observed.

`EventWatcher` becomes a session-state change notifier rather than emitting
only an attention projection. Its shared snapshot payload drives both attention
and recovery. It handles create, write, remove, and rename events for lock and
event files. A single bounded fallback scan remains responsible for missed
notifications and time-only state transitions.

### Platform process control

`internal/platform` exposes one cohesive recovery controller with these
operations:

1. Inspect a PID and return an opaque identity-bound target.
2. Verify there is exactly one stable live lock owner, that it belongs to the
   current user, and that its canonical executable basename is `copilot`
   (`copilot.exe` on Windows).
3. Capture and identity-bind the current descendant tree.
4. Re-verify the tree immediately before mutation and refuse on churn.
5. Request graceful stop for the verified tree when a safely addressable
   platform mechanism exists.
6. Wait for confirmed exit, escalate to forceful stop after a deadline, and
   confirm every captured identity exited.

Platform implementations use the strongest available primitives:

- Windows: process handles, creation times, canonical executable paths, owner
  checks, and Toolhelp descendant discovery. Existing terminal processes are
  commonly already job-associated, so job assignment is an optional
  containment enhancement rather than the primary path.
- Linux: pidfds when supported, `/proc` start-time and ownership metadata,
  executable paths, and descendant identities.
- macOS: kernel process metadata, process start time, ownership, executable
  path, and descendant identities.

An opaque target is single-use and carries the captured generation. Operations
re-verify identity before signaling. No implementation falls back to PID-only
termination, command-name matching, shell commands, privilege escalation, or
lock-file deletion.

On Unix, graceful stop sends `SIGTERM` to identity-verified descendants in
leaf-first order and then to the root; forceful escalation uses `SIGKILL` in the
same order. Signals are addressed to verified individual processes because an
external process group can contain unrelated terminal processes. Linux uses
pidfd signaling where available. On Windows, a Ctrl+Break request is attempted
only when the root is verifiably a separately addressable console process-group
leader; otherwise the confirmation states that graceful signaling is
unavailable and the controller proceeds to identity-bound forceful termination
after the grace window. It never broadcasts a console event to an ambiguous
group.

Processes launched outside Dispatch cannot always be retroactively placed in a
durable containment group. The controller therefore defines the verified tree
as the identity-bound root and descendants captured during stable preflight.
It requires exactly one live lock owner and two identical descendant
enumerations before mutation. Multiple live locks, mixed owners, lock-owner
changes, and descendant churn disable restart. If the platform cannot stabilize
and bind that tree, restart is unavailable. This limitation is surfaced to the
user rather than weakened silently.

### Recovery coordinator

The TUI owns orchestration but not process mechanics. A recovery attempt moves
through:

`confirming -> verifying -> stopping -> forcing -> exit-confirmed -> resuming
-> progress-confirmed`

Terminal states are `recovered`, `cancelled`, and `failed`. Failure includes a
specific safe next action. Resume is impossible before `exit-confirmed`.

Each session has a monotonic recovery-attempt generation in the coordinator.
Every asynchronous platform result, timer, and watcher message carries that
generation; older messages are discarded. Verification, graceful stop,
forceful stop, exit confirmation, and post-resume confirmation each have
bounded deadlines and remain cancellable while no mutation is in progress.

Interrupted sessions skip process termination and use the existing resume
launcher. Live sessions require explicit confirmation that names the session,
workspace, PID, last event, inactivity, and the possibility that external tool
side effects may already have occurred.

### Recovery view

A dedicated recovery overlay lists candidates independently of Chronicle
sessions. Indexed candidates enrich their row from `data.Session`; unindexed
candidates remain actionable using session-state metadata.

Actions are state-specific:

- Inspect evidence.
- Wait and continue monitoring.
- Copy the validated resume command.
- Resume an interrupted session.
- Restart a likely stalled live session.
- Focus the existing terminal where supported.

Restart is disabled for suspected stalls and unverifiable targets. Status and
failure text communicate state without relying on color.

### Data flow

1. The bounded scanner discovers a session-state directory and lock owner.
2. Event evidence and the prior observation produce a recovery classification.
3. The watcher publishes progress changes; the model updates or removes the
   candidate independently of Chronicle results.
4. The user selects restart and confirms the evidence-rich warning.
5. Platform preflight binds and re-verifies the process tree.
6. The controller requests graceful stop, escalates if needed, and confirms all
   bound targets exited.
7. A final no-survivor gate rechecks every bound identity, verifies the old lock
   has no live owner, and aborts if a new owner or tree churn appears.
8. The existing external resume launcher starts `copilot --resume=<id>`.
9. A bounded direct probe confirms a new lock or event marker before reporting
   recovery; `EventWatcher` can complete the probe early but is not the sole
   success path.

## Patterns and Decisions

- **Reuse before replacement:** attention scanning, `EventWatcher`, session
  launching, validation, and Bubble Tea command patterns remain authoritative.
- **Evidence model:** recovery status carries facts and reasons rather than
  overloading `AttentionStatus` with process-control state.
- **Opaque platform target:** the TUI cannot manufacture or alter process
  identities, which keeps PID-reuse defenses inside the platform boundary.
- **Fail-closed mutation:** ambiguity removes the restart action; it never
  selects a weaker kill mechanism.
- **Two-phase stop:** graceful request, bounded wait, forceful verified-tree
  stop, bounded confirmation.
- **Generation-aware orchestration:** late messages from an older recovery
  attempt cannot change a newer attempt.
- **No new dependency:** Go and `golang.org/x/sys` already expose the required
  platform APIs; local platform files keep behavior reviewable.

## Gap Remediation

| Gap | Remediation |
|-----|-------------|
| Attention status lacks evidence | Add a recovery candidate and progress marker in `internal/data` |
| Unindexed sessions are absent from the list | Maintain a recovery collection sourced directly from session-state |
| PID liveness is not identity proof | Add opaque platform identities with creation time, owner, and executable verification |
| No tree-wide stop contract | Add platform-specific capture, graceful stop, escalation, and exit confirmation |
| Existing interrupted resume needs Chronicle metadata | Read `data.context.cwd` from the first `session.start` event and permit session-state-only launch |
| No post-resume success signal | Compare lock and event generations through `EventWatcher` |
| No recovery UX | Add an evidence-first overlay and confirmation state |
| Watcher emits only attention status | Publish shared snapshots for create, write, remove, and rename transitions |
| Multiple lock files are ambiguous | Require exactly one stable live owner for live restart |

## Risks and Trade-offs

- **External process containment:** Dispatch did not spawn the target, so strict
  containment may be unavailable. Mitigation: bind every observed identity,
  require a stable tree, and refuse restart when the platform cannot do so.
- **PID reuse and TOCTOU:** a numeric PID can change owners between checks.
  Mitigation: stable handles or kernel identifiers where available, creation
  time and ownership checks elsewhere, and verification immediately before
  every mutation.
- **Escaped descendants:** a process can detach before capture. Mitigation:
  stable repeated enumeration, job assignment on Windows where possible, and
  explicit refusal when tree stability cannot be established.
- **Duplicate side effects:** the last tool may have completed externally
  without writing an event. Mitigation: warn before restart and never replay
  individual tool calls.
- **False stalls:** long-running tools may be quiet. Mitigation: two
  observation windows, unfinished-event evidence, wait as the default action,
  and no restart for merely suspected stalls.
- **Platform parity:** APIs differ materially. Mitigation: one behavioral
  contract, build-tagged implementations, shared contract tests, and
  platform-specific tests.
- **Resumed process stalls:** launching a process does not prove recovery.
  Mitigation: bounded direct progress probes and a failed state when the new
  process produces no lock or event progress.
- **Cross-user lock owner:** existence probes can report permission errors as
  dead on Unix. Mitigation: snapshot evidence distinguishes absent from
  inaccessible, and inaccessible owners are manual-only rather than
  interrupted/resumable.

## Rollout and Rollback

Recovery discovery and evidence are additive. The live restart action is
enabled only when process verification reports support for the selected target.
Existing attention filters and interrupted-session resume remain available.

Rollback removes the recovery overlay and controller wiring without changing
session files or Chronicle schema. Platform controllers are isolated behind the
recovery boundary and do not alter existing launch behavior.

## Decisions Log

| Decision | Resolution |
|----------|------------|
| Release boundary | Ship cross-platform live restart rather than detection-only |
| Stop scope | Stop the verified process tree, not only the root |
| Confirmation | Require explicit user confirmation for every live restart |
| Resume ordering | Resume only after confirmed exit of every bound target |
| Session files | Never modify or delete lock or event files |
| Ambiguous identity | Refuse termination and provide diagnostics/manual guidance |
| Success definition | Require new lock or event progress after resume |
| Observation window | One additional configured attention-threshold interval |
| Lock ownership | Exactly one stable live lock owner is required |
| Executable identity | Canonical basename `copilot` or `copilot.exe`, same user |
| Workspace source | First `session.start` event at `data.context.cwd` |
| Graceful stop | Unix leaf-first SIGTERM; Windows Ctrl+Break only for a verified process group |

## Gut Check

**Greenfield answer:** Copilot CLI would expose a session control API or
supervisor-owned process handle, and Dispatch would request a semantic restart
instead of attaching to an external process tree.

**Reality delta:** That control channel does not exist. Dispatch can observe
session-state and platform process metadata but did not spawn most targets.

**Verdict:** Use the smallest fail-closed recovery coordinator around existing
signals, with platform-specific identity controls and no PID-only fallback.

**Confidence:** Medium. The architecture is appropriate, but externally
launched process trees impose unavoidable platform limitations that must remain
visible in capability checks and tests.
