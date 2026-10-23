package platform

import (
	"context"
	"time"
)

// RecoveryProcessTree is an identity-bound process tree captured for a
// confirmed stalled-session recovery attempt.
type RecoveryProcessTree interface {
	RootPID() int
	ProcessCount() int
	GracefulSupported() bool
	Stop(context.Context, time.Duration, time.Duration) error
	Close() error
}

// StopRecoveryProcessTree inspects, verifies, stops, and confirms exit of the
// process tree rooted at pid. The tree is always closed before return.
func StopRecoveryProcessTree(
	ctx context.Context,
	pid int,
	gracePeriod time.Duration,
	forceTimeout time.Duration,
) error {
	tree, err := InspectRecoveryProcessTree(pid)
	if err != nil {
		return err
	}
	defer tree.Close()
	return tree.Stop(ctx, gracePeriod, forceTimeout)
}

// CheckRecoveryProcessTree performs the same fail-closed identity and tree
// verification used by StopRecoveryProcessTree without signaling any process.
func CheckRecoveryProcessTree(pid int) (processCount int, graceful bool, err error) {
	tree, err := InspectRecoveryProcessTree(pid)
	if err != nil {
		return 0, false, err
	}
	defer tree.Close()
	return tree.ProcessCount(), tree.GracefulSupported(), nil
}
