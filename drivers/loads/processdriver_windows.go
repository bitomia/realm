package loads

import (
	"os"
	"os/exec"
)

func findProcess(pid int) (*exec.Cmd, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}

	return &exec.Cmd{Process: process}, nil
}
