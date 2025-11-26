//go:build darwin
// +build darwin

package cpu

import (
	"runtime"

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

func GetNodeState() (*dto.NodeState, error) {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	cpuStat, cpuUsage, containersState, err := GetContainersState(ctx, client)
	if err != nil {
		return nil, err
	}
	var nodeState dto.NodeState
	nodeState.Containers = containersState
	nodeState.NumCPU = runtime.NumCPU()
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
	nodeState.FreeMemPercent = float64(memStat.Free) / float64(memStat.Total) * 100

	var fsStat unix.Statfs_t
	unix.Statfs("/", &fsStat)
	nodeState.FreeStorage = fsStat.Bfree * uint64(fsStat.Bsize)

	return &nodeState, nil
}
