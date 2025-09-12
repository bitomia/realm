//go:build linux
// +build linux

package cpu

import (
	"time"

	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/memory"
	"golang.org/x/sys/unix"

	"github.com/bitomia/realm/daemon/cruntime"
)

func GetCPUStat(s *cpu.Stats) (float64, float64) {
	active := s.User + s.Nice + s.System
	total := active + s.Idle
	return float64(active), float64(total)
}

func GetHostStatus() (*HostStatus, error) {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var hostStatus HostStatus

	statsATime := time.Now()
	statsA, err := GetContainerStats(ctx, client)
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

	statsB, err := GetContainerStats(ctx, client)
	if err != nil {
		return nil, err
	}

	for container, statB := range statsB {
		statA := statsA[container]
		cpuUsage := (statB.CPUUsage*1000 - statA.CPUUsage*1000) / float64(timeDelta.Nanoseconds()) * 100
		cpuSystem := (statB.CPUSystem*1000 - statA.CPUSystem*1000) / float64(timeDelta.Nanoseconds()) * 100
		cpuUser := (statB.CPUUser*1000 - statA.CPUUser*1000) / float64(timeDelta.Nanoseconds()) * 100

		stat := Stats{
			ContainerID:   container,
			CPUUsage:      cpuUsage,
			CPUSystem:     cpuSystem,
			CPUUser:       cpuUser,
			MemoryUsage:   statB.MemoryUsage,
			MemoryLimit:   statB.MemoryLimit,
			MemoryPercent: statB.MemoryPercent,
		}
		hostStatus.Containers = append(hostStatus.Containers, stat)
	}

	hostStatus.NumCPU = cpuStatB.CPUCount
	hostStatus.UserCPU = cpuStatB.User
	hostStatus.IdleCPU = cpuStatB.Idle
	hostStatus.SystemCPU = cpuStatB.System
	hostStatus.TotalCPU = cpuStatB.Total

	cpuAActive, cpuATotal := GetCPUStat(cpuStatA)
	cpuBActive, cpuBTotal := GetCPUStat(cpuStatB)
	cpuUsage := ((cpuBActive - cpuAActive) / (cpuBTotal - cpuATotal)) * 100
	hostStatus.UsageCPUPercent = cpuUsage

	memStat, err := memory.Get()
	if err != nil {
		return nil, err
	}
	hostStatus.TotalMem = memStat.Total
	hostStatus.UsedMem = memStat.Used
	hostStatus.InactiveMem = memStat.Inactive
	hostStatus.CachedMem = memStat.Cached
	hostStatus.FreeMem = memStat.Free
	hostStatus.AvailableMem = memStat.Available
	hostStatus.FreeMemPercent = float64(memStat.Available) / float64(memStat.Total) * 100

	var fsStat unix.Statfs_t
	unix.Statfs("/", &fsStat)
	hostStatus.FreeStorage = fsStat.Bfree * uint64(fsStat.Bsize)

	return &hostStatus, nil
}
