//go:build windows
// +build windows

package cpu

import (
	"log/slog"
	"path/filepath"
	"runtime"

	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/memory"
	"golang.org/x/sys/windows"

	"github.com/bitomia/realm/daemon/cruntime"
	"github.com/bitomia/realm/internal/requests"
)

func GetCPUStat(s *cpu.Stats) (float64, float64) {
	active := s.User + s.Nice + s.System
	total := active + s.Idle
	return float64(active), float64(total)
}

func GetNodeState() (*requests.NodeState, error) {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var nodeState requests.NodeState
	nodeState.NumCPU = runtime.NumCPU()

	cpuStat, cpuUsage, containersState, err := GetContainersState(ctx, client)
	if err == nil {
		nodeState.Containers = containersState
		nodeState.UserCPU = cpuStat.User
		nodeState.IdleCPU = cpuStat.Idle
		nodeState.SystemCPU = cpuStat.System
		nodeState.TotalCPU = cpuStat.Total
		nodeState.UsageCPUPercent = cpuUsage
	} else {
		slog.Error("Error on GetContainerState", "error", err)
	}
	memStat, err := memory.Get()
	if err != nil {
		return nil, err
	}
	nodeState.TotalMem = memStat.Total
	nodeState.UsedMem = memStat.Used
	nodeState.FreeMem = memStat.Free
	nodeState.FreeMemPercent = float64(memStat.Used) / float64(memStat.Total) * 100

	// Get free storage for the root drive (C:\)
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	rootPath, err := filepath.Abs("\\")
	if err != nil {
		rootPath = "C:\\"
	}
	pathPtr, err := windows.UTF16PtrFromString(rootPath)
	if err != nil {
		return nil, err
	}

	err = windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes)
	if err != nil {
		return nil, err
	}
	nodeState.FreeStorage = totalFreeBytes

	return &nodeState, nil
}
