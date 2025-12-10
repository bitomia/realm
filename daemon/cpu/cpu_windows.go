//go:build windows
// +build windows

package cpu

import (
	"log/slog"
	"path/filepath"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"golang.org/x/sys/windows"

	"github.com/bitomia/realm/daemon/cruntime"

	"github.com/bitomia/realm/internal/dto"
)

func GetCPUStat(s *cpu.TimesStat) (float64, float64) {
	active := s.User + s.Nice + s.System
	total := active + s.Idle
	return active, total
}

func GetNodeState() (*dto.NodeStateResponse, error) {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	cpuCount, err := cpu.Counts(true)
	if err != nil {
		return nil, err
	}

	var nodeState dto.NodeStateResponse
	nodeState.NumCPU = cpuCount

	cpuStat, cpuUsage, containersState, err := CollectNodeState(ctx, client)
	if err == nil {
		nodeState.Containers = containersState
		nodeState.UserCPU = uint64(cpuStat.User)
		nodeState.IdleCPU = uint64(cpuStat.Idle)
		nodeState.SystemCPU = uint64(cpuStat.System)
		nodeState.TotalCPU = uint64(cpuStat.User + cpuStat.System + cpuStat.Nice + cpuStat.Idle)
		nodeState.UsageCPUPercent = cpuUsage
	} else {
		slog.Error("Error on GetNodeState", "error", err)
	}
	memStat, err := mem.VirtualMemory()
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
