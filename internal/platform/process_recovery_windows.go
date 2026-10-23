//go:build windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const recoveryProcessRights = windows.PROCESS_QUERY_LIMITED_INFORMATION |
	windows.PROCESS_TERMINATE |
	windows.SYNCHRONIZE

type windowsRecoveryProcess struct {
	pid       uint32
	parentPID uint32
	created   uint64
	handle    windows.Handle
}

type windowsRecoveryTree struct {
	rootPID   int
	processes []windowsRecoveryProcess
}

func (t *windowsRecoveryTree) RootPID() int            { return t.rootPID }
func (t *windowsRecoveryTree) ProcessCount() int       { return len(t.processes) }
func (t *windowsRecoveryTree) GracefulSupported() bool { return false }

// InspectRecoveryProcessTree captures two identical descendant snapshots and
// opens stable handles for every process before returning.
func InspectRecoveryProcessTree(pid int) (RecoveryProcessTree, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid recovery PID: %d", pid)
	}

	first, err := windowsProcessMap()
	if err != nil {
		return nil, err
	}
	firstIDs := descendantIDs(uint32(pid), first)
	second, err := windowsProcessMap()
	if err != nil {
		return nil, err
	}
	secondIDs := descendantIDs(uint32(pid), second)
	if !slices.Equal(firstIDs, secondIDs) {
		return nil, errors.New("process tree changed during recovery verification")
	}

	currentUser, err := currentWindowsUser()
	if err != nil {
		return nil, fmt.Errorf("resolve current process owner: %w", err)
	}
	tree := &windowsRecoveryTree{rootPID: pid}
	for _, processID := range secondIDs {
		h, err := windows.OpenProcess(recoveryProcessRights, false, processID)
		if err != nil {
			tree.Close()
			return nil, fmt.Errorf("open process %d for recovery: %w", processID, err)
		}
		process, err := inspectWindowsProcess(h, processID, second[processID], currentUser, processID == uint32(pid))
		if err != nil {
			windows.CloseHandle(h)
			tree.Close()
			return nil, err
		}
		tree.processes = append(tree.processes, process)
	}

	if err := tree.verifyStable(); err != nil {
		tree.Close()
		return nil, err
	}
	return tree, nil
}

func (t *windowsRecoveryTree) Stop(ctx context.Context, gracePeriod, forceTimeout time.Duration) error {
	if err := t.verifyStable(); err != nil {
		return err
	}

	if err := waitWindowsTree(ctx, t.processes, gracePeriod); err == nil {
		return nil
	} else if !errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	for i := len(t.processes) - 1; i >= 0; i-- {
		process := t.processes[i]
		if exited, err := windowsProcessExited(process.handle); err != nil {
			return err
		} else if exited {
			continue
		}
		if err := windows.TerminateProcess(process.handle, 1); err != nil {
			return fmt.Errorf("terminate verified process %d: %w", process.pid, err)
		}
	}
	if err := waitWindowsTree(ctx, t.processes, forceTimeout); err != nil {
		return fmt.Errorf("confirm verified process-tree exit: %w", err)
	}
	return nil
}

func (t *windowsRecoveryTree) Close() error {
	var firstErr error
	for i := range t.processes {
		if t.processes[i].handle == 0 {
			continue
		}
		if err := windows.CloseHandle(t.processes[i].handle); err != nil && firstErr == nil {
			firstErr = err
		}
		t.processes[i].handle = 0
	}
	return firstErr
}

func (t *windowsRecoveryTree) verifyStable() error {
	processMap, err := windowsProcessMap()
	if err != nil {
		return err
	}
	currentIDs := descendantIDs(uint32(t.rootPID), processMap)
	capturedIDs := make([]uint32, len(t.processes))
	for i, process := range t.processes {
		capturedIDs[i] = process.pid
		var creation, exit, kernel, user windows.Filetime
		if err := windows.GetProcessTimes(process.handle, &creation, &exit, &kernel, &user); err != nil {
			return fmt.Errorf("re-verify process %d: %w", process.pid, err)
		}
		if filetimeUint64(creation) != process.created {
			return fmt.Errorf("process %d identity changed during recovery", process.pid)
		}
	}
	slices.Sort(capturedIDs)
	if !slices.Equal(currentIDs, capturedIDs) {
		return errors.New("process tree changed during recovery verification")
	}
	return nil
}

func inspectWindowsProcess(
	handle windows.Handle,
	pid uint32,
	parentPID uint32,
	currentUser *windows.SID,
	root bool,
) (windowsRecoveryProcess, error) {
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return windowsRecoveryProcess{}, fmt.Errorf("read process %d creation time: %w", pid, err)
	}
	token, err := processWindowsToken(handle)
	if err != nil {
		return windowsRecoveryProcess{}, fmt.Errorf("read process %d owner: %w", pid, err)
	}
	defer token.Close()
	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return windowsRecoveryProcess{}, fmt.Errorf("read process %d owner SID: %w", pid, err)
	}
	if !tokenUser.User.Sid.Equals(currentUser) {
		return windowsRecoveryProcess{}, fmt.Errorf("process %d is owned by another user", pid)
	}
	if root {
		path, err := windowsProcessPath(handle)
		if err != nil {
			return windowsRecoveryProcess{}, fmt.Errorf("read process %d executable: %w", pid, err)
		}
		if !strings.EqualFold(filepath.Base(path), "copilot.exe") {
			return windowsRecoveryProcess{}, fmt.Errorf("process %d executable is %q, not copilot.exe", pid, filepath.Base(path))
		}
	}
	return windowsRecoveryProcess{
		pid:       pid,
		parentPID: parentPID,
		created:   filetimeUint64(creation),
		handle:    handle,
	}, nil
}

func currentWindowsUser() (*windows.SID, error) {
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return nil, err
	}
	return user.User.Sid.Copy()
}

func processWindowsToken(handle windows.Handle) (windows.Token, error) {
	var token windows.Token
	err := windows.OpenProcessToken(handle, windows.TOKEN_QUERY, &token)
	return token, err
}

func windowsProcessPath(handle windows.Handle) (string, error) {
	buf := make([]uint16, 32768)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:size]), nil
}

func windowsProcessMap() (map[uint32]uint32, error) {
	snapshot, _, callErr := procCreateToolhelp32.Call(uintptr(thSnapProcess), 0)
	if snapshot == uintptr(windows.InvalidHandle) {
		return nil, fmt.Errorf("create process snapshot: %w", callErr)
	}
	defer windows.CloseHandle(windows.Handle(snapshot))

	result := make(map[uint32]uint32)
	var entry processEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	ret, _, callErr := procProcess32First.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return nil, fmt.Errorf("read process snapshot: %w", callErr)
	}
	for {
		result[entry.ProcessID] = entry.ParentProcessID
		entry.Size = uint32(unsafe.Sizeof(entry))
		ret, _, _ = procProcess32Next.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}
	return result, nil
}

func descendantIDs(root uint32, parents map[uint32]uint32) []uint32 {
	result := []uint32{root}
	known := map[uint32]struct{}{root: {}}
	for changed := true; changed; {
		changed = false
		for pid, parent := range parents {
			if _, ok := known[pid]; ok {
				continue
			}
			if _, ok := known[parent]; ok {
				known[pid] = struct{}{}
				result = append(result, pid)
				changed = true
			}
		}
	}
	slices.SortFunc(result, func(a, b uint32) int {
		depthA := windowsProcessDepth(a, root, parents)
		depthB := windowsProcessDepth(b, root, parents)
		if depthA != depthB {
			return depthA - depthB
		}
		return int(a) - int(b)
	})
	return result
}

func windowsProcessDepth(pid, root uint32, parents map[uint32]uint32) int {
	depth := 0
	for pid != root && depth < 256 {
		parent, ok := parents[pid]
		if !ok || parent == 0 || parent == pid {
			break
		}
		pid = parent
		depth++
	}
	return depth
}

func filetimeUint64(value windows.Filetime) uint64 {
	return uint64(value.HighDateTime)<<32 | uint64(value.LowDateTime)
}

func windowsProcessExited(handle windows.Handle) (bool, error) {
	status, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return false, err
	}
	return status == windows.WAIT_OBJECT_0, nil
}

func waitWindowsTree(ctx context.Context, processes []windowsRecoveryProcess, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		allExited := true
		for _, process := range processes {
			exited, err := windowsProcessExited(process.handle)
			if err != nil {
				return err
			}
			if !exited {
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
