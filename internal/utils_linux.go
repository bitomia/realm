package internal

import (
	"errors"
	"syscall"
)

func PIDExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}

	// Process exists but belongs to another user.
	return errors.Is(err, syscall.EPERM)
}
