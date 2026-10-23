//go:build windows

package platform

import (
	"os"
	"slices"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestDescendantIDsOrdersParentsBeforeChildren(t *testing.T) {
	parents := map[uint32]uint32{
		10: 1,
		20: 10,
		30: 20,
		40: 10,
	}
	got := descendantIDs(10, parents)
	if !slices.Equal(got, []uint32{10, 20, 40, 30}) {
		t.Fatalf("descendant order = %v", got)
	}
}

func TestInspectRecoveryProcessTreeRejectsNonCopilotRoot(t *testing.T) {
	tree, err := InspectRecoveryProcessTree(os.Getpid())
	if tree != nil {
		tree.Close()
	}
	if err == nil {
		t.Fatal("current test process must not pass Copilot executable verification")
	}
}

func TestWindowsRecoveryTreeDoesNotClaimGracefulStop(t *testing.T) {
	if (&windowsRecoveryTree{}).GracefulSupported() {
		t.Fatal("Windows recovery must not claim ambiguous Ctrl+Break support")
	}
}

func TestInspectWindowsProcessRejectsOwnerMismatch(t *testing.T) {
	handle, err := windows.OpenProcess(recoveryProcessRights, false, uint32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	otherUser, err := windows.StringToSid("S-1-0-0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inspectWindowsProcess(handle, uint32(os.Getpid()), 0, otherUser, false); err == nil ||
		!strings.Contains(err.Error(), "owned by another user") {
		t.Fatalf("owner mismatch error = %v", err)
	}
}

func TestWindowsRecoveryTreeRejectsChangedProcessIdentity(t *testing.T) {
	handle, err := windows.OpenProcess(recoveryProcessRights, false, uint32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	tree := &windowsRecoveryTree{
		rootPID: os.Getpid(),
		processes: []windowsRecoveryProcess{{
			pid:     uint32(os.Getpid()),
			created: 1,
			handle:  handle,
		}},
	}
	if err := tree.verifyStable(); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("identity mismatch error = %v", err)
	}
}
