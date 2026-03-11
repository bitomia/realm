package dto

import "github.com/bitomia/realm/common"

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

	Containers []ContainerStateResponse `json:"containers,omitempty"`
}

type NodeResponse struct {
	State  NodeState         `json:"state"`
	Status common.NodeStatus `json:"status"`
}

// OsKind represents the type of operating system
type OsKind uint8

const (
	WindowsOs OsKind = iota
	LinuxOs
	AndroidOs
	AppleOs
	UnixOs
	PosixOs
	OtherOs
)

// CpuArch represents the type of CPU architecture
type CpuArch uint8

const (
	X86 CpuArch = iota
	X64
	Arm2
	Arm3
	Arm4T
	Arm5
	Arm6T2
	Arm6
	Arm7
	Arm7a
	Arm7r
	Arm7s
	Arm64
	Mips
	SuperH
	Ppc
	Ppc64
	Sparc
	M68k
	OtherArch
)

// SystemInfo contains static information about a computer
type SystemInfo struct {
	OsName            string  `json:"os_name"`              // Name of the OS, reported by the OS
	OsKind            OsKind  `json:"os_kind"`              // Kind of OS
	OsArch            CpuArch `json:"os_arch"`              // CPU architecture
	SysVendor         string  `json:"sys_vendor"`           // System vendor string
	NetDefaultGateway string  `json:"net_default_gateway"`  // Default network gateway
	NetHostName       string  `json:"net_host_name"`        // Network name of this host
	NetDomainName     string  `json:"net_domain_name"`      // Domain name of this host
	NetPrimaryDns     string  `json:"net_primary_dns"`      // Primary network DNS
	NetSecondaryDns   string  `json:"net_secondary_dns"`    // Secondary network DNS
	NetPrimaryIpAddr  string  `json:"net_primary_ip_addr"`  // Primary IP address
	NetMacAddress     string  `json:"net_mac_address"`      // Primary MAC address
	CpuModel          string  `json:"cpu_model"`            // CPU model string
	CpuStartTime      int64   `json:"cpu_start_time"`       // Time when the CPU got powered (Unix timestamp in nanoseconds)
	CpuTotalCores     uint32  `json:"cpu_total_cores"`      // Number of cores of the CPU
	CpuCoresPerSocket uint32  `json:"cpu_cores_per_socket"` // Number of cores per socket
	CpuTotalSockets   uint32  `json:"cpu_total_sockets"`    // Number of sockets
	CpuVendor         string  `json:"cpu_vendor"`           // CPU vendor string
	CpuMhz            uint32  `json:"cpu_mhz"`              // Maximum CPU frequency in Megahertz
	RamTotal          uint64  `json:"ram_total"`            // Total available RAM in bytes
}

type StopNodeRequest struct {
	WallMessage string  `json:"wall_message"`
	Time        uint32  `json:"time"`
	NodeName    *string `json:"node_name,omitempty"`
	Force       bool    `json:"force"`
}

type RestartNodeRequest struct {
	WallMessage string  `json:"wall_message"`
	Time        uint32  `json:"time"`
	NodeName    *string `json:"node_name,omitempty"`
}
