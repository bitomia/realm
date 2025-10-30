//go:build darwin
// +build darwin

package cpu

import (
	"golang.org/x/sys/unix"
)

func GetMemLimit() float64 {
	memsize, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		// Return max uint64 on error
		return float64(^uint64(0))
	}
	return float64(memsize)
}
