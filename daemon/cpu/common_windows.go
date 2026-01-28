//go:build windows
// +build windows

package cpu

import (
	"path/filepath"
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
		return float64(^uint64(0))
	}

	return float64(memStatus.ullTotalPhys)
}

func GetFreeStorage() (uint64, error) {
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	rootPath, err := filepath.Abs("\\")
	if err != nil {
		rootPath = "C:\\"
	}
	pathPtr, err := windows.UTF16PtrFromString(rootPath)
	if err != nil {
		return 0, err
	}

	err = windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes)
	if err != nil {
		return 0, err
	}

	return totalFreeBytes, nil
}
