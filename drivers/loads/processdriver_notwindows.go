//go:build !windows

package loads

import (
	"syscall"

	process "github.com/shirou/gopsutil/v4/process"

	"github.com/bitomia/realm/internal"
)

func stopProcess(proc *process.Process, stopSignal *int) error {
	signal := syscall.SIGTERM
	if stopSignal != nil {
		signal = internal.IntToSyscallSignal(*stopSignal)
	}
	return proc.SendSignal(signal)
}
