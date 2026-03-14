package common

type ContainerState struct {
	ContainerID   string  `json:"container_id"`
	CPUUsage      float64 `json:"cpu_usage"` // Important (percentage)
	CPUSystem     float64 `json:"cpu_system"`
	CPUUser       float64 `json:"cpu_user"`
	MemoryUsage   float64 `json:"mem_usage"`
	MemoryLimit   float64 `json:"mem_limit"`
	MemoryPercent float64 `json:"mem_percentage"` // Important (percentage)
}

type ContainerStates map[string]ContainerState
