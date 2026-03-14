package common

type NodeState struct {
	NumCPU          int     `json:"ncpu"`
	UserCPU         uint64  `json:"cpu_user"`
	IdleCPU         uint64  `json:"cpu_idle"`
	SystemCPU       uint64  `json:"cpu_system"`
	TotalCPU        uint64  `json:"cpu_total"`
	UsageCPUPercent float64 `json:"cpu_usage_percentage"` // Important

	TotalMem       uint64  `json:"mem_total"`
	UsedMem        uint64  `json:"mem_used"`
	FreeMem        uint64  `json:"mem_free"`
	FreeMemPercent float64 `json:"mem_free_percentage"` // Important

	FreeStorage uint64 `json:"free_storage"`

	// Swap memory information
	SwapTotal uint64 `json:"swap_total"` // Total available swap memory in bytes
	SwapUsed  uint64 `json:"swap_used"`  // Total used swap memory in bytes
	SwapFree  uint64 `json:"swap_free"`  // Total free swap memory in bytes

	// CPU load average
	CpuLoadAvg float32 `json:"cpu_load_avg"` // CPU load average (from 0 to 1)

	// Process counts
	ProcTotalCount    uint32 `json:"proc_total_count"`    // Number of active processes
	ProcSleepingCount uint32 `json:"proc_sleeping_count"` // Number of sleeping processes
	ProcRunningCount  uint32 `json:"proc_running_count"`  // Number of running processes
	ProcZombieCount   uint32 `json:"proc_zombie_count"`   // Number of zombie processes
	ProcStoppedCount  uint32 `json:"proc_stopped_count"`  // Number of stopped processes
	ProcIdleCount     uint32 `json:"proc_idle_count"`     // Number of idle processes
	ProcThreadsCount  uint32 `json:"proc_threads_count"`  // Number of threads

	Containers []ContainerState `json:"containers,omitempty"`
}
