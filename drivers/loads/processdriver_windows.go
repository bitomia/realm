//go:build windows

package loads

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	process "github.com/shirou/gopsutil/v4/process"

	"github.com/bitomia/realm/internal"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procPostMessageW             = user32.NewProc("PostMessageW")
)

// sendWMClose enumerates all top-level windows and posts WM_CLOSE to those
// belonging to the given process. Returns true if at least one window was found.
func sendWMClose(pid int32) bool {
	var found bool

	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		var windowPid uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&windowPid)))
		if int32(windowPid) == int32(lParam) {
			procPostMessageW.Call(hwnd, internal.WM_CLOSE, 0, 0)
			found = true
		}
		return 1 // continue enumeration
	})

	procEnumWindows.Call(cb, uintptr(pid))
	return found
}

func stopProcess(proc *process.Process, stopSignal *int) error {
	if stopSignal != nil && *stopSignal == internal.WM_CLOSE && sendWMClose(proc.Pid) {
		return nil
	}
	return proc.Kill()
}
