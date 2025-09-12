package cpu

import (
	"bufio"
	"context"

	"os"
	"strconv"
	"strings"

	cgroupstats "github.com/containerd/cgroups/v2/stats"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
)

type Stats struct {
	ContainerID   string  `json:"container_id"`
	CPUUsage      float64 `json:"cpu_usage"` // Important (percentage)
	CPUSystem     float64 `json:"cpu_system"`
	CPUUser       float64 `json:"cpu_user"`
	MemoryUsage   float64 `json:"mem_usage"`
	MemoryLimit   float64 `json:"mem_limit"`
	MemoryPercent float64 `json:"mem_percentage"` // Important (percentage)
}

type HostStatus struct {
	NumCPU          int     `json:"ncpu"`
	UserCPU         uint64  `json:"cpu_user"`
	IdleCPU         uint64  `json:"cpu_idle"`
	SystemCPU       uint64  `json:"cpu_system"`
	TotalCPU        uint64  `json:"cpu_total"`
	UsageCPUPercent float64 `json:"cpu_usage_percentage"` // Important

	TotalMem       uint64  `json:"mem_total"`
	UsedMem        uint64  `json:"mem_used"`
	InactiveMem    uint64  `json:"mem_inactive"`
	CachedMem      uint64  `json:"mem_cached"`
	FreeMem        uint64  `json:"mem_free"`
	AvailableMem   uint64  `json:"mem_available"`
	FreeMemPercent float64 `json:"mem_free_percentage"` // Important

	FreeStorage uint64 `json:"free_storage"`

	Containers []Stats `json:"containers"`
}

func GetHostMemLimit() float64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return float64(^uint64(0))
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "MemTotal:") {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 2 {
				memKb, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					return float64(memKb * 1024) // kB to bytes
				}
			}
			break
		}
	}
	return float64(^uint64(0))
}

func GetCgroupMemLimit(memLimit float64) float64 {
	if memLimit == float64(^uint64(0)) {
		return GetHostMemLimit()
	}
	return memLimit
}

func GetMemUsage(memStat *cgroupstats.MemoryStat) float64 {
	if v := memStat.InactiveFile; v < memStat.Usage {
		return float64(memStat.Usage - v)
	}
	return float64(memStat.Usage)
}

func GetContainerStats(ctx context.Context, client *containerd.Client) (map[string]Stats, error) {
	containers, err := client.Containers(ctx)
	if err != nil {
		return nil, err
	}

	var stats = make(map[string]Stats)
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

		stats[id] = Stats{
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
