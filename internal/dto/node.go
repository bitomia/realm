package dto

type ContainerStatesResponse map[string]ContainerStateResponse

type ContainerStateResponse struct {
	ContainerID   string  `json:"container_id"`
	CPUUsage      float64 `json:"cpu_usage"` // Important (percentage)
	CPUSystem     float64 `json:"cpu_system"`
	CPUUser       float64 `json:"cpu_user"`
	MemoryUsage   float64 `json:"mem_usage"`
	MemoryLimit   float64 `json:"mem_limit"`
	MemoryPercent float64 `json:"mem_percentage"` // Important (percentage)
}

type NodeStateResponse struct {
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

	Containers []ContainerStateResponse `json:"containers"`
}
