# Stalled Session Recovery Test Plan

## Status: AUTOMATED

## Planned Tests

| ID | Acceptance criterion | Planned automation | Status |
|----|----------------------|--------------------|--------|
| T01 | AC1 | `TestRecoveryTracker_UnindexedSessionPromotesAfterSecondWindow` | automated |
| T02 | AC1 | `TestReadSessionStateSnapshots_SkipsInvalidAndSymlinkedEntries`, `TestEventWatcher_IgnoresInvalidSessionIDs` | automated |
| T03 | AC2 | `TestReadSessionStateSnapshots_ReadsWorkspaceAndSanitizesCandidate`, `TestReadSessionWorkspace_RejectsOversizedFirstEvent` | automated |
| T04 | AC2 | `TestReadSessionStateSnapshots_ReadsWorkspaceAndSanitizesCandidate`, `TestRecoveryView_RendersEvidenceAndConfirmation` | automated |
| T05 | AC3 | `TestRecoveryTracker_UnindexedSessionPromotesAfterSecondWindow` | automated |
| T06 | AC3 | `TestRecoveryTracker_UnindexedSessionPromotesAfterSecondWindow`, `TestRecoveryTracker_ProgressAndIdentityChangesResetObservation` | automated |
| T07 | AC3 | `TestRecoveryTracker_FinishedAndRecentSessionsExcluded`, `TestRecoveryTracker_InterruptedAndAmbiguousLocks` | automated |
| T08 | AC4 | `TestRecoveryTracker_ProgressAndIdentityChangesResetObservation`, `TestEventWatcher_FiresOnEventWrite`, `TestEventWatcher_SchedulesRecoveryScanForLockRemoveAndRename` | automated |
| T09 | AC4 | `TestEventWatcher_FiresOnEventWrite`, `TestEventWatcher_WatchesNewSessionDirs`, `TestEventWatcher_SchedulesRecoveryScanForLockRemoveAndRename` | automated |
| T10 | AC5 | `TestRecoverSessionCmdResumesInterruptedUnindexedSessionAndProbesProgress` | automated |
| T11 | AC5 | `TestRecoverSessionCmdRejectsChangedWorkspaceBeforeLaunch`, existing `platform` resume-command validation tests | automated |
| T12 | AC6 | `TestRecoveryKeyVerifiesBeforeConfirmation`, `TestRecoveryVerifiedOpensConfirmationWithoutMutation`, `TestStopRecoveryProcessTreeStopsVerifiedSubprocess` | automated |
| T13 | AC6 | `TestRecoveryTracker_InterruptedAndAmbiguousLocks`, platform non-Copilot tests, `TestInspectWindowsProcessRejectsOwnerMismatch`, `TestWindowsRecoveryTreeRejectsChangedProcessIdentity`, `TestRecoverSessionCmdStopFailureNeverLaunches` | automated |
| T14 | AC6 | `TestDescendantIDsOrdersParentsBeforeChildren`, `TestUnixDescendantIDsOrdersParentsBeforeChildren`, `TestDarwinDescendantIDsOrdersParentsBeforeChildren`; Linux and macOS platform test packages cross-compile | automated |
| T15 | AC7 | `TestRecoveryKeyVerifiesBeforeConfirmation`, `TestRecoveryVerifiedOpensConfirmationWithoutMutation`, `TestRecoveryView_RendersEvidenceAndConfirmation` | automated |
| T16 | AC7 | `TestStopRecoveryProcessTreeStopsVerifiedSubprocess`, `TestRecoverSessionCmdStopsThenLaunchesAndConfirmsProgress` | automated |
| T17 | AC7 | `TestRecoverSessionCmdStopFailureNeverLaunches`, `TestRecoverSessionCmdSurfacesLaunchFailure`, `TestProbeRecoveryProgressTimesOutWithoutProgress` | automated |
| T17A | AC7 | `TestDescendantIDsOrdersParentsBeforeChildren`, `TestUnixDescendantIDsOrdersParentsBeforeChildren`, `TestDarwinDescendantIDsOrdersParentsBeforeChildren`, `TestWindowsRecoveryTreeDoesNotClaimGracefulStop` | automated |
| T18 | AC8 | `TestReadSessionStateSnapshotsDoesNotModifySessionFiles`, `TestRecoveryVerifiedOpensConfirmationWithoutMutation` | automated |
| T19 | AC8 | `TestStopRecoveryProcessTreeRejectsInvalidPID`, platform non-Copilot rejection tests, `TestRecoveryCompletedFailureDoesNotRemoveCandidate` | automated |
| T20 | AC9 | `TestRecoverSessionCmdStopsThenLaunchesAndConfirmsProgress`, `TestProbeRecoveryProgressTimesOutWithoutProgress` | automated |
| T21 | AC9 | `TestRecoveryCompletedIgnoresOldGeneration` | automated |
| T21A | AC9 | `TestRecoverSessionCmdResumesInterruptedUnindexedSessionAndProbesProgress`, `TestRecoverSessionCmdStopsThenLaunchesAndConfirmsProgress` | automated |
| T22 | AC10 | Full existing `internal/data`, `internal/tui`, and `internal/platform` suites run by `mage install` | automated |
| T23 | AC11 | `TestReadSessionStateSnapshotsBoundsHundredSessionScan` | automated |
| T24 | AC11 | `TestEventWatcher_DebounceCollapses`, `TestEventWatcher_StopIsIdempotent`, `TestProbeRecoveryProgressTimesOutWithoutProgress` | automated |
| T25 | AC12 | `TestRecoveryView_RendersEvidenceAndConfirmation`, `TestRecoveryView_BlockedCandidateCannotConfirm`, `TestRecoveryCompletedFailureDoesNotRemoveCandidate` | automated |
| T26 | AC12 | `TestRecoveryKeyboardNavigationWaitConfirmAndCancel`, `TestRecoveryKeyVerifiesBeforeConfirmation`, existing resume-command and clipboard command tests | automated |
| T27 | AC12 | `TestUpdate_WindowSizeMsgResizesOpenRecoveryView` | automated |

## Functionality Inventory

The functionality inventory will be populated from the implementation diff
during verification. Every introduced function, branch, error path, platform
implementation, and UI state must map to an automated test before this plan can
move to `COVERED`.
