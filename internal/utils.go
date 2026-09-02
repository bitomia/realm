package internal

import (
	"errors"
	"syscall"
)

const bytesToMB = 1024.0 * 1024.0

func ToMB(bytes float64) float64 {
	return bytes / bytesToMB
}

func PIDExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}

	// Process exists but belongs to another user.
	return errors.Is(err, syscall.EPERM)
}
