//go:build darwin
// +build darwin

package cpu

import (
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"golang.org/x/sys/unix"

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

	cpuStat, cpuUsage, containersState, err := CollectNodeState(ctx, client)
	if err != nil {
		return nil, err
	}

	cpuCount, err := cpu.Counts(true)
	if err != nil {
		return nil, err
	}

	var nodeState dto.NodeStateResponse
	nodeState.Containers = containersState
	nodeState.NumCPU = cpuCount
	nodeState.UserCPU = uint64(cpuStat.User)
	nodeState.IdleCPU = uint64(cpuStat.Idle)
	nodeState.SystemCPU = uint64(cpuStat.System)
	nodeState.TotalCPU = uint64(cpuStat.User + cpuStat.System + cpuStat.Nice + cpuStat.Idle)
	nodeState.UsageCPUPercent = cpuUsage

	memStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	nodeState.TotalMem = memStat.Total
	nodeState.UsedMem = memStat.Used
	nodeState.FreeMem = memStat.Free
	nodeState.FreeMemPercent = float64(memStat.Free) / float64(memStat.Total) * 100

	var fsStat unix.Statfs_t
	unix.Statfs("/", &fsStat)
	nodeState.FreeStorage = fsStat.Bfree * uint64(fsStat.Bsize)

	return &nodeState, nil
}
