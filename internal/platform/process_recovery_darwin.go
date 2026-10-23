//go:build darwin

package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type darwinRecoveryProcess struct {
	pid       int
	parentPID int
	startSec  int64
	startUsec int32
}

type darwinRecoveryTree struct {
	rootPID   int
	processes []darwinRecoveryProcess
}

func (t *darwinRecoveryTree) RootPID() int            { return t.rootPID }
func (t *darwinRecoveryTree) ProcessCount() int       { return len(t.processes) }
func (t *darwinRecoveryTree) GracefulSupported() bool { return true }

func InspectRecoveryProcessTree(pid int) (RecoveryProcessTree, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid recovery PID: %d", pid)
	}
	first, err := darwinProcessMap()
	if err != nil {
		return nil, err
	}
	firstIDs := darwinDescendantIDs(pid, first)
	second, err := darwinProcessMap()
	if err != nil {
		return nil, err
	}
	secondIDs := darwinDescendantIDs(pid, second)
	if !slices.Equal(firstIDs, secondIDs) {
		return nil, errors.New("process tree changed during recovery verification")
	}

	tree := &darwinRecoveryTree{rootPID: pid}
	currentUID := uint32(os.Getuid())
	for _, processID := range secondIDs {
		info, ok := second[processID]
		if !ok {
			return nil, fmt.Errorf("process %d disappeared during recovery verification", processID)
		}
		if info.uid != currentUID {
			return nil, fmt.Errorf("process %d is owned by another user", processID)
		}
		if processID == pid && info.command != "copilot" {
			return nil, fmt.Errorf("process %d executable is %q, not copilot", pid, info.command)
		}
		tree.processes = append(tree.processes, darwinRecoveryProcess{
			pid:       processID,
			parentPID: info.parentPID,
			startSec:  info.startSec,
			startUsec: info.startUsec,
		})
	}
	if err := tree.verifyStable(); err != nil {
		return nil, err
	}
	return tree, nil
}

func (t *darwinRecoveryTree) Stop(ctx context.Context, gracePeriod, forceTimeout time.Duration) error {
	if err := t.verifyStable(); err != nil {
		return err
	}
	if err := signalDarwinTree(t.processes, unix.SIGTERM); err != nil {
		return err
	}
	if err := waitDarwinTree(ctx, t.processes, gracePeriod); err == nil {
		return nil
	} else if !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if err := signalDarwinTree(t.processes, unix.SIGKILL); err != nil {
		return err
	}
	if err := waitDarwinTree(ctx, t.processes, forceTimeout); err != nil {
		return fmt.Errorf("confirm verified process-tree exit: %w", err)
	}
	return nil
}

func (t *darwinRecoveryTree) Close() error { return nil }

func (t *darwinRecoveryTree) verifyStable() error {
	processMap, err := darwinProcessMap()
	if err != nil {
		return err
	}
	current := darwinDescendantIDs(t.rootPID, processMap)
	captured := make([]int, len(t.processes))
	for i, process := range t.processes {
		captured[i] = process.pid
		info, ok := processMap[process.pid]
		if !ok || info.startSec != process.startSec || info.startUsec != process.startUsec {
			return fmt.Errorf("process %d identity changed during recovery", process.pid)
		}
	}
	slices.Sort(captured)
	if !slices.Equal(current, captured) {
		return errors.New("process tree changed during recovery verification")
	}
	return nil
}

type darwinProcessInfo struct {
	parentPID int
	startSec  int64
	startUsec int32
	uid       uint32
	command   string
}

func darwinProcessMap() (map[int]darwinProcessInfo, error) {
	processes, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, err
	}
	result := make(map[int]darwinProcessInfo, len(processes))
	for _, process := range processes {
		pid := int(process.Proc.P_pid)
		if pid <= 0 {
			continue
		}
		result[pid] = darwinProcessInfo{
			parentPID: int(process.Eproc.Ppid),
			startSec:  process.Proc.P_starttime.Sec,
			startUsec: process.Proc.P_starttime.Usec,
			uid:       process.Eproc.Ucred.Uid,
			command:   darwinCommand(process.Proc.P_comm[:]),
		}
	}
	return result, nil
}

func darwinCommand(raw []byte) string {
	var builder strings.Builder
	for _, value := range raw {
		if value == 0 {
			break
		}
		builder.WriteByte(value)
	}
	return builder.String()
}

func darwinDescendantIDs(root int, processMap map[int]darwinProcessInfo) []int {
	result := []int{root}
	known := map[int]struct{}{root: {}}
	for changed := true; changed; {
		changed = false
		for pid, info := range processMap {
			if _, ok := known[pid]; ok {
				continue
			}
			if _, ok := known[info.parentPID]; ok {
				known[pid] = struct{}{}
				result = append(result, pid)
				changed = true
			}
		}
	}
	slices.SortFunc(result, func(a, b int) int {
		depthA := darwinProcessDepth(a, root, processMap)
		depthB := darwinProcessDepth(b, root, processMap)
		if depthA != depthB {
			return depthA - depthB
		}
		return a - b
	})
	return result
}

func darwinProcessDepth(pid, root int, processMap map[int]darwinProcessInfo) int {
	depth := 0
	for pid != root && depth < 256 {
		info, ok := processMap[pid]
		if !ok || info.parentPID <= 0 || info.parentPID == pid {
			break
		}
		pid = info.parentPID
		depth++
	}
	return depth
}

func signalDarwinTree(processes []darwinRecoveryProcess, signal unix.Signal) error {
	for i := len(processes) - 1; i >= 0; i-- {
		process := processes[i]
		info, err := unix.SysctlKinfoProc("kern.proc.pid", process.pid)
		if err != nil {
			if errors.Is(err, unix.ESRCH) {
				continue
			}
			return err
		}
		if info.Proc.P_starttime.Sec != process.startSec || info.Proc.P_starttime.Usec != process.startUsec {
			return fmt.Errorf("process %d identity changed before signaling", process.pid)
		}
		if err := unix.Kill(process.pid, signal); err != nil && !errors.Is(err, unix.ESRCH) {
			return fmt.Errorf("signal verified process %d: %w", process.pid, err)
		}
	}
	return nil
}

func waitDarwinTree(ctx context.Context, processes []darwinRecoveryProcess, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		allExited := true
		for _, process := range processes {
			info, err := unix.SysctlKinfoProc("kern.proc.pid", process.pid)
			if err != nil {
				continue
			}
			if info.Proc.P_starttime.Sec == process.startSec && info.Proc.P_starttime.Usec == process.startUsec {
				allExited = false
				break
			}
		}
		if allExited {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return context.DeadlineExceeded
		case <-ticker.C:
		}
	}
}
