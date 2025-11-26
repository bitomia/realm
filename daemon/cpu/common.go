package cpu

import (
	"context"
	"time"

	cgroupstats "github.com/containerd/cgroups/v2/stats"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/mackerelio/go-osstat/cpu"

	"github.com/bitomia/realm/internal/dto"
)

func getCgroupMemLimit(memLimit float64) float64 {
	if memLimit == float64(^uint64(0)) {
		return GetMemLimit()
	}
	return memLimit
}

func getMemUsage(memStat *cgroupstats.MemoryStat) float64 {
	if v := memStat.InactiveFile; v < memStat.Usage {
		return float64(memStat.Usage - v)
	}
	return float64(memStat.Usage)
}

func getContainersState(ctx context.Context, client *containerd.Client) (dto.ContainerStatesResponse, error) {
	containers, err := client.Containers(ctx)
	if err != nil {
		return nil, err
	}

	var stats = make(dto.ContainerStatesResponse)
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
		memUsage := getMemUsage(statsMetrics.Memory)
		memLimit := getCgroupMemLimit(float64(statsMetrics.Memory.UsageLimit))
		memPercent := memUsage / memLimit * 100

		stats[id] = dto.ContainerStateResponse{
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

// CollectNodeState collects and calculates resource usage statistics for all containers and the host node.
// It takes two snapshots of container and CPU metrics separated by an interval, then calculates
// the delta between them to determine accurate CPU usage percentages.
//
// Parameters:
//   - ctx: Go context
//   - client: Containerd client used to query container information
//
// Returns:
//   - *cpu.Stats: The final CPU statistics snapshot from the host system
//   - float64: The overall node CPU usage percentage during the measurement interval
//   - []dto.ContainerStateResponse: Slice containing resource usage stats for each container, including:
//   - CPU usage (total, system, and user percentages)
//   - Memory usage, limit, and percentage
//   - error: Any error encountered during metric collection, nil on success
//
// Note: Memory statistics are point-in-time values from the second snapshot, not deltas.
func CollectNodeState(ctx context.Context, client *containerd.Client) (*cpu.Stats, float64, []dto.ContainerStateResponse, error) {
	statsATime := time.Now()
	statsA, err := getContainersState(ctx, client)
	if err != nil {
		return nil, 0, nil, err
	}
	cpuStatA, err := cpu.Get()
	if err != nil {
		return nil, 0, nil, err
	}

	time.Sleep(500 * time.Millisecond)
	timeDelta := time.Since(statsATime)

	statsB, err := getContainersState(ctx, client)
	if err != nil {
		return nil, 0, nil, err
	}
	cpuStatB, err := cpu.Get()
	if err != nil {
		return nil, 0, nil, err
	}

	var containers []dto.ContainerStateResponse
	for container, statB := range statsB {
		statA := statsA[container]
		cpuUsage := (statB.CPUUsage*1000 - statA.CPUUsage*1000) / float64(timeDelta.Nanoseconds()) * 100
		cpuSystem := (statB.CPUSystem*1000 - statA.CPUSystem*1000) / float64(timeDelta.Nanoseconds()) * 100
		cpuUser := (statB.CPUUser*1000 - statA.CPUUser*1000) / float64(timeDelta.Nanoseconds()) * 100

		stat := dto.ContainerStateResponse{
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
