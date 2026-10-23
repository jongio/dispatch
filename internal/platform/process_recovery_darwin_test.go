//go:build darwin

package platform

import (
	"os"
	"slices"
	"testing"
)

func TestDarwinDescendantIDsOrdersParentsBeforeChildren(t *testing.T) {
	processes := map[int]darwinProcessInfo{
		10: {parentPID: 1},
		20: {parentPID: 10},
		30: {parentPID: 20},
		40: {parentPID: 10},
	}
	if got := darwinDescendantIDs(10, processes); !slices.Equal(got, []int{10, 20, 40, 30}) {
		t.Fatalf("descendant order = %v", got)
	}
}

func TestDarwinInspectRecoveryProcessTreeRejectsNonCopilotRoot(t *testing.T) {
	tree, err := InspectRecoveryProcessTree(os.Getpid())
	if tree != nil {
		tree.Close()
	}
	if err == nil {
		t.Fatal("current test process must not pass Copilot executable verification")
	}
}
