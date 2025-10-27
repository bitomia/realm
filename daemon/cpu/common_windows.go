//go:build windows
// +build windows

package cpu

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// MEMORYSTATUSEX structure for GlobalMemoryStatusEx
type memoryStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

func GetMemLimit() float64 {
	var memStatus memoryStatusEx
	memStatus.dwLength = uint32(unsafe.Sizeof(memStatus))

	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	globalMemoryStatusEx := kernel32.NewProc("GlobalMemoryStatusEx")

	ret, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memStatus)))
	if ret == 0 {
		// Return max uint64 on error
		return float64(^uint64(0))
	}

	return float64(memStatus.ullTotalPhys)
}
