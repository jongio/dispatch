---
title: Stalled Session Recovery
status: approved
---

# Stalled Session Recovery

## Problem

Copilot CLI sessions can remain alive without producing events. Dispatch can
label indexed sessions as quiet, but it cannot explain the evidence, recover
them safely, or show a recovery candidate that is absent from Chronicle. Users
must inspect processes and run resume commands manually, and a mistaken PID
operation could affect an unrelated process.

## Outcome

Dispatch presents an evidence-based recovery view sourced directly from
Copilot session-state. It distinguishes interrupted, suspected-stalled, and
likely-stalled sessions; clears candidates when progress resumes; safely
resumes interrupted sessions; and offers a confirmed cross-platform restart
only when it can identity-bind and stop the verified process tree.

## User Experience

The recovery view shows the session ID, workspace, PID, last event, inactivity,
classification, reason, and available actions. Unindexed sessions appear with
the same core evidence and actions.

A live restart requires an explicit warning. Dispatch verifies the target tree,
requests graceful stop, escalates if required, waits for confirmed exit, then
resumes the session. It reports success only after a new lock or event marker.
Any identity ambiguity or partial termination produces a clear failure and no
resume.

## Acceptance Criteria

1. Recovery discovery includes valid session-state candidates that have no
   Chronicle row.
2. Every candidate exposes sanitized session ID, workspace, PID, last event,
   inactivity, classification, reason, and supported actions.
3. A quiet unfinished live session is suspected on the first elapsed
   observation and likely stalled only after one additional unchanged
   attention-threshold interval.
4. Event, lock, or process-identity progress immediately clears or downgrades a
   stalled candidate.
5. Interrupted sessions resume through the existing launcher without requiring
   a Chronicle-backed session row.
6. Live restart is unavailable until the session is likely stalled, process
   ownership and canonical Copilot executable identity are verified, exactly
   one stable live lock owner exists, and the descendant tree is stable and
   identity-bound.
7. Live restart requires explicit confirmation, performs graceful then
   forceful verified-tree stop as needed, and never resumes before all bound
   processes have confirmed exit.
8. PID reuse, executable mismatch, owner mismatch, tree churn, unsupported
   platform capability, timeout, and partial-exit paths fail closed without
   modifying session-state files.
9. A resumed session is reported recovered only after a new lock or event
   progress marker found by a watcher or bounded direct probe; timeout is
   reported as a recovery failure.
10. Existing attention states, filters, event watching, focus behavior, and
    standard session launching continue to work.
11. Recovery scanning remains bounded, avoids busy polling, and completes under
    100 ms for a 100-session fixture on supported CI hardware.
12. Recovery states, disabled actions, confirmations, and errors are
    understandable without color.

## Non-Goals

- Sending a message into a frozen Copilot process.
- Replaying or inferring completion of individual tool calls.
- Automatically restarting a session without confirmation.
- Modifying lock files, event logs, or Chronicle schema.
- Searching all raw event bodies as general conversation content.

## Architecture

See [architecture.md](../../stalled-session-recovery/architecture.md).

## Risks

Dispatch does not spawn most target processes, so process-tree containment and
identity capabilities differ by operating system. The action is fail-closed:
targets that cannot satisfy the common safety contract remain inspectable but
not restartable.

<!-- Pipeline tracking (auto-managed, not part of product spec) -->
## Pipeline Status

Phase: VERIFYING
