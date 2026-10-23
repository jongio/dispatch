//go:build linux

package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type linuxRecoveryProcess struct {
	pid       int
	parentPID int
	startTime uint64
	pidfd     int
}

type linuxRecoveryTree struct {
	rootPID   int
	processes []linuxRecoveryProcess
}

func (t *linuxRecoveryTree) RootPID() int            { return t.rootPID }
func (t *linuxRecoveryTree) ProcessCount() int       { return len(t.processes) }
func (t *linuxRecoveryTree) GracefulSupported() bool { return true }

func InspectRecoveryProcessTree(pid int) (RecoveryProcessTree, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid recovery PID: %d", pid)
	}
	first, err := linuxProcessMap()
	if err != nil {
		return nil, err
	}
	firstIDs := unixDescendantIDs(pid, first)
	second, err := linuxProcessMap()
	if err != nil {
		return nil, err
	}
	secondIDs := unixDescendantIDs(pid, second)
	if !slices.Equal(firstIDs, secondIDs) {
		return nil, errors.New("process tree changed during recovery verification")
	}

	currentUID := os.Getuid()
	tree := &linuxRecoveryTree{rootPID: pid}
	for _, processID := range secondIDs {
		info, ok := second[processID]
		if !ok {
			tree.Close()
			return nil, fmt.Errorf("process %d disappeared during recovery verification", processID)
		}
		if info.uid != currentUID {
			tree.Close()
			return nil, fmt.Errorf("process %d is owned by another user", processID)
		}
		if processID == pid {
			exe, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
			if err != nil {
				tree.Close()
				return nil, fmt.Errorf("read process %d executable: %w", pid, err)
			}
			if filepath.Base(exe) != "copilot" {
				tree.Close()
				return nil, fmt.Errorf("process %d executable is %q, not copilot", pid, filepath.Base(exe))
			}
		}
		pidfd, err := unix.PidfdOpen(processID, 0)
		if err != nil {
			tree.Close()
			return nil, fmt.Errorf("bind process %d identity: %w", processID, err)
		}
		tree.processes = append(tree.processes, linuxRecoveryProcess{
			pid:       processID,
			parentPID: info.parentPID,
			startTime: info.startTime,
			pidfd:     pidfd,
		})
	}
	if err := tree.verifyStable(); err != nil {
		tree.Close()
		return nil, err
	}
	return tree, nil
}

func (t *linuxRecoveryTree) Stop(ctx context.Context, gracePeriod, forceTimeout time.Duration) error {
	if err := t.verifyStable(); err != nil {
		return err
	}
	if err := signalLinuxTree(t.processes, unix.SIGTERM); err != nil {
		return err
	}
	if err := waitLinuxTree(ctx, t.processes, gracePeriod); err == nil {
		return nil
	} else if !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if err := signalLinuxTree(t.processes, unix.SIGKILL); err != nil {
		return err
	}
	if err := waitLinuxTree(ctx, t.processes, forceTimeout); err != nil {
		return fmt.Errorf("confirm verified process-tree exit: %w", err)
	}
	return nil
}

func (t *linuxRecoveryTree) Close() error {
	var firstErr error
	for i := range t.processes {
		if t.processes[i].pidfd < 0 {
			continue
		}
		if err := unix.Close(t.processes[i].pidfd); err != nil && firstErr == nil {
			firstErr = err
		}
		t.processes[i].pidfd = -1
	}
	return firstErr
}

func (t *linuxRecoveryTree) verifyStable() error {
	processMap, err := linuxProcessMap()
	if err != nil {
		return err
	}
	current := unixDescendantIDs(t.rootPID, processMap)
	captured := make([]int, len(t.processes))
	for i, process := range t.processes {
		captured[i] = process.pid
		info, ok := processMap[process.pid]
		if !ok || info.startTime != process.startTime {
			return fmt.Errorf("process %d identity changed during recovery", process.pid)
		}
	}
	slices.Sort(captured)
	if !slices.Equal(current, captured) {
		return errors.New("process tree changed during recovery verification")
	}
	return nil
}

type linuxProcessInfo struct {
	parentPID int
	startTime uint64
	uid       int
}

func linuxProcessMap() (map[int]linuxProcessInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	result := make(map[int]linuxProcessInfo)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 || !entry.IsDir() {
			continue
		}
		info, err := readLinuxProcessInfo(pid)
		if err == nil {
			result[pid] = info
		}
	}
	return result, nil
}

func readLinuxProcessInfo(pid int) (linuxProcessInfo, error) {
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return linuxProcessInfo{}, err
	}
	closeParen := strings.LastIndexByte(string(stat), ')')
	if closeParen < 0 || closeParen+2 >= len(stat) {
		return linuxProcessInfo{}, errors.New("malformed proc stat")
	}
	fields := strings.Fields(string(stat[closeParen+2:]))
	if len(fields) < 20 {
		return linuxProcessInfo{}, errors.New("short proc stat")
	}
	parentPID, err := strconv.Atoi(fields[1])
	if err != nil {
		return linuxProcessInfo{}, err
	}
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return linuxProcessInfo{}, err
	}
	status, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return linuxProcessInfo{}, err
	}
	uid, err := linuxRealUID(string(status))
	if err != nil {
		return linuxProcessInfo{}, err
	}
	return linuxProcessInfo{parentPID: parentPID, startTime: startTime, uid: uid}, nil
}

func linuxRealUID(status string) (int, error) {
	for _, line := range strings.Split(status, "\n") {
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			break
		}
		return strconv.Atoi(fields[1])
	}
	return 0, errors.New("process UID is unavailable")
}

func signalLinuxTree(processes []linuxRecoveryProcess, signal unix.Signal) error {
	for i := len(processes) - 1; i >= 0; i-- {
		process := processes[i]
		if err := unix.PidfdSendSignal(process.pidfd, signal, nil, 0); err != nil && !errors.Is(err, unix.ESRCH) {
			return fmt.Errorf("signal verified process %d: %w", process.pid, err)
		}
	}
	return nil
}

func waitLinuxTree(ctx context.Context, processes []linuxRecoveryProcess, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		allExited := true
		for _, process := range processes {
			poll := []unix.PollFd{{Fd: int32(process.pidfd), Events: unix.POLLIN}}
			if _, err := unix.Poll(poll, 0); err != nil {
				return err
			}
			if poll[0].Revents&unix.POLLIN == 0 {
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

func unixDescendantIDs(root int, processMap map[int]linuxProcessInfo) []int {
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
		depthA := linuxProcessDepth(a, root, processMap)
		depthB := linuxProcessDepth(b, root, processMap)
		if depthA != depthB {
			return depthA - depthB
		}
		return a - b
	})
	return result
}

func linuxProcessDepth(pid, root int, processMap map[int]linuxProcessInfo) int {
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
