package loads

import (
	"os"
	"os/exec"
	"syscall"
)

func findProcess(pid int) (*exec.Cmd, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}

	// On Unix systems, FindProcess always succeeds and returns a Process
	// for the given pid, regardless of whether the process exists. To test whether
	// the process actually exists, see whether p.Signal(syscall.Signal(0)) reports
	// an error.
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return nil, err
	}

	return &exec.Cmd{Process: process}, nil
}
