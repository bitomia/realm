//go:build linux
// +build linux

package cpu

import (
	"time"

	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/memory"
	"golang.org/x/sys/unix"

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

	statsATime := time.Now()
	statsA, err := GetContainerState(ctx, client)
	if err != nil {
		return nil, err
	}
	cpuStatA, err := cpu.Get()
	if err != nil {
		return nil, err
	}

	time.Sleep(500 * time.Millisecond)
	timeDelta := time.Since(statsATime)

	cpuStatB, err := cpu.Get()
	if err != nil {
		return nil, err
	}

	statsB, err := GetContainerState(ctx, client)
	if err != nil {
		return nil, err
	}

	for container, statB := range statsB {
		statA := statsA[container]
		cpuUsage := (statB.CPUUsage*1000 - statA.CPUUsage*1000) / float64(timeDelta.Nanoseconds()) * 100
		cpuSystem := (statB.CPUSystem*1000 - statA.CPUSystem*1000) / float64(timeDelta.Nanoseconds()) * 100
		cpuUser := (statB.CPUUser*1000 - statA.CPUUser*1000) / float64(timeDelta.Nanoseconds()) * 100

		stat := requests.ContainerState{
			ContainerID:   container,
			CPUUsage:      cpuUsage,
			CPUSystem:     cpuSystem,
			CPUUser:       cpuUser,
			MemoryUsage:   statB.MemoryUsage,
			MemoryLimit:   statB.MemoryLimit,
			MemoryPercent: statB.MemoryPercent,
		}
		nodeState.Containers = append(nodeState.Containers, stat)
	}

	nodeState.NumCPU = cpuStatB.CPUCount
	nodeState.UserCPU = cpuStatB.User
	nodeState.IdleCPU = cpuStatB.Idle
	nodeState.SystemCPU = cpuStatB.System
	nodeState.TotalCPU = cpuStatB.Total

	cpuAActive, cpuATotal := GetCPUStat(cpuStatA)
	cpuBActive, cpuBTotal := GetCPUStat(cpuStatB)
	cpuUsage := ((cpuBActive - cpuAActive) / (cpuBTotal - cpuATotal)) * 100
	nodeState.UsageCPUPercent = cpuUsage

	memStat, err := memory.Get()
	if err != nil {
		return nil, err
	}
	nodeState.TotalMem = memStat.Total
	nodeState.UsedMem = memStat.Used
	nodeState.InactiveMem = memStat.Inactive
	nodeState.CachedMem = memStat.Cached
	nodeState.FreeMem = memStat.Free
	nodeState.AvailableMem = memStat.Available
	nodeState.FreeMemPercent = float64(memStat.Available) / float64(memStat.Total) * 100

	var fsStat unix.Statfs_t
	unix.Statfs("/", &fsStat)
	nodeState.FreeStorage = fsStat.Bfree * uint64(fsStat.Bsize)

	return &nodeState, nil
}
