package platform

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows         = user32.NewProc("EnumWindows")
	procGetWindowThreadPID  = user32.NewProc("GetWindowThreadProcessId")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procShowWindow          = user32.NewProc("ShowWindow")

	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procCreateToolhelp32 = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First   = kernel32.NewProc("Process32FirstW")
	procProcess32Next    = kernel32.NewProc("Process32NextW")
)

const (
	thSnapProcess = 0x00000002
	swRestore     = 9
)

// processEntry32 is the PROCESSENTRY32W structure.
type processEntry32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [windows.MAX_PATH]uint16
}

// FocusSessionWindow brings the terminal window hosting the given PID to the
// foreground. It walks the process tree upward from the target PID to find a
// visible top-level window owned by an ancestor process (typically wt.exe or
// conhost.exe), then calls SetForegroundWindow.
func FocusSessionWindow(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}

	// Build the set of ancestor PIDs (the target + its parent chain).
	ancestors := buildAncestorSet(uint32(pid))
	if len(ancestors) == 0 {
		return fmt.Errorf("could not build process tree for PID %d", pid)
	}

	// Find a visible top-level window owned by any ancestor.
	hwnd := findWindowForPIDs(ancestors)
	if hwnd == 0 {
		return fmt.Errorf("no visible window found for PID %d or its ancestors", pid)
	}

	// Restore if minimized, then bring to foreground.
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	if visible == 0 {
		procShowWindow.Call(hwnd, uintptr(swRestore))
	}
	ret, _, _ := procSetForegroundWindow.Call(hwnd)
	if ret == 0 {
		return fmt.Errorf("SetForegroundWindow failed for HWND %v", hwnd)
	}
	return nil
}

// buildAncestorSet returns a set of PIDs from the target up through its
// parent chain. Stops after 32 hops to avoid infinite loops.
func buildAncestorSet(targetPID uint32) map[uint32]struct{} {
	snapshot, _, _ := procCreateToolhelp32.Call(uintptr(thSnapProcess), 0)
	if snapshot == uintptr(windows.InvalidHandle) {
		return nil
	}
	defer windows.CloseHandle(windows.Handle(snapshot))

	// Build a child->parent map.
	parentMap := make(map[uint32]uint32)
	var entry processEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32First.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return nil
	}

	for {
		parentMap[entry.ProcessID] = entry.ParentProcessID
		entry.Size = uint32(unsafe.Sizeof(entry))
		ret, _, _ = procProcess32Next.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	// Walk up from target.
	result := make(map[uint32]struct{})
	current := targetPID
	for i := 0; i < 32; i++ {
		result[current] = struct{}{}
		parent, ok := parentMap[current]
		if !ok || parent == 0 || parent == current {
			break
		}
		current = parent
	}
	return result
}

// findWindowForPIDs enumerates top-level windows and returns the first
// visible one owned by a PID in the given set.
func findWindowForPIDs(pids map[uint32]struct{}) uintptr {
	var found uintptr

	// The callback receives each top-level window handle.
	cb := windows.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		var winPID uint32
		procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&winPID)))
		if _, ok := pids[winPID]; ok {
			visible, _, _ := procIsWindowVisible.Call(hwnd)
			if visible != 0 {
				found = hwnd
				return 0 // stop enumeration
			}
		}
		return 1 // continue
	})

	procEnumWindows.Call(cb, 0)
	return found
}
