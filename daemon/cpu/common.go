package cpu

import (
	"context"
	"time"

	cgroupstats "github.com/containerd/cgroups/v2/stats"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/mackerelio/go-osstat/cpu"

	"github.com/bitomia/realm/internal/requests"
)

func GetCgroupMemLimit(memLimit float64) float64 {
	if memLimit == float64(^uint64(0)) {
		return GetMemLimit()
	}
	return memLimit
}

func GetMemUsage(memStat *cgroupstats.MemoryStat) float64 {
	if v := memStat.InactiveFile; v < memStat.Usage {
		return float64(memStat.Usage - v)
	}
	return float64(memStat.Usage)
}

func GetContainerState(ctx context.Context, client *containerd.Client) (map[string]requests.ContainerState, error) {
	containers, err := client.Containers(ctx)
	if err != nil {
		return nil, err
	}

	var stats = make(map[string]requests.ContainerState)
	for _, container := range containers {
		id := container.ID()
		task, err := container.Task(ctx, nil)
		if err != nil {
			if errdefs.IsNotFound(err) == false {
				return nil, err
			}
			continue
		}

		metrics, err := task.Metrics(ctx)
		if err != nil {
			return nil, err
		}

		var statsMetrics cgroupstats.Metrics
		if err := statsMetrics.Unmarshal(metrics.Data.Value); err != nil {
			return nil, err
		}
		memUsage := GetMemUsage(statsMetrics.Memory)
		memLimit := GetCgroupMemLimit(float64(statsMetrics.Memory.UsageLimit))
		memPercent := memUsage / memLimit * 100

		stats[id] = requests.ContainerState{
			ContainerID:   id,
			CPUUsage:      float64(statsMetrics.CPU.UsageUsec),
			CPUSystem:     float64(statsMetrics.CPU.SystemUsec),
			CPUUser:       float64(statsMetrics.CPU.UserUsec),
			MemoryUsage:   memUsage,
			MemoryLimit:   memLimit,
			MemoryPercent: memPercent,
		}
	}

	return stats, nil
}

func GetContainersState(ctx context.Context, client *containerd.Client) (*cpu.Stats, float64, []requests.ContainerState, error) {
	statsATime := time.Now()
	statsA, err := GetContainerState(ctx, client)
	if err != nil {
		return nil, 0, nil, err
	}
	cpuStatA, err := cpu.Get()
	if err != nil {
		return nil, 0, nil, err
	}

	time.Sleep(500 * time.Millisecond)
	timeDelta := time.Since(statsATime)

	statsB, err := GetContainerState(ctx, client)
	if err != nil {
		return nil, 0, nil, err
	}
	cpuStatB, err := cpu.Get()
	if err != nil {
		return nil, 0, nil, err
	}

	var containers []requests.ContainerState
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
		containers = append(containers, stat)
	}

	cpuAActive, cpuATotal := GetCPUStat(cpuStatA)
	cpuBActive, cpuBTotal := GetCPUStat(cpuStatB)
	cpuUsage := ((cpuBActive - cpuAActive) / (cpuBTotal - cpuATotal)) * 100

	return cpuStatB, cpuUsage, containers, nil
}
