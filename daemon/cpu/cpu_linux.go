//go:build linux
// +build linux

package cpu

import (
	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/memory"
	"golang.org/x/sys/unix"

	"github.com/bitomia/realm/daemon/cruntime"
	"github.com/bitomia/realm/internal/dto"
)

func GetCPUStat(s *cpu.Stats) (float64, float64) {
	active := s.User + s.Nice + s.System
	total := active + s.Idle
	return float64(active), float64(total)
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
	var nodeState dto.NodeStateResponse
	nodeState.Containers = containersState
	nodeState.NumCPU = cpuStat.CPUCount
	nodeState.UserCPU = cpuStat.User
	nodeState.IdleCPU = cpuStat.Idle
	nodeState.SystemCPU = cpuStat.System
	nodeState.TotalCPU = cpuStat.Total
	nodeState.UsageCPUPercent = cpuUsage

	memStat, err := memory.Get()
	if err != nil {
		return nil, err
	}
	nodeState.TotalMem = memStat.Total
	nodeState.UsedMem = memStat.Used
	nodeState.FreeMem = memStat.Free
	nodeState.FreeMemPercent = float64(memStat.Available) / float64(memStat.Total) * 100

	var fsStat unix.Statfs_t
	unix.Statfs("/", &fsStat)
	nodeState.FreeStorage = fsStat.Bfree * uint64(fsStat.Bsize)

	return &nodeState, nil
}
