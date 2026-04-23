package dto

import (
	"github.com/bitomia/realm/common"
)

type NodeCapabilities struct {
	ContainersEngine     bool `json:"containers_engine"`
	ContainersNetworking bool `json:"containers_networking"`
	Volumes              bool `json:"volumes"`
	VolumesZFS           bool `json:"volumes_zfs"`
	VMM                  bool `json:"vmm"`
}

func NewNodeCapabilities(c common.Capabilities) NodeCapabilities {
	if c == nil {
		return NodeCapabilities{}
	}
	return NodeCapabilities{
		ContainersEngine:     c.ContainersEngine(),
		ContainersNetworking: c.ContainersNetworking(),
		Volumes:              c.Volumes(),
		VolumesZFS:           c.VolumesZFS(),
		VMM:                  c.VMM(),
	}
}

type NodeResponse struct {
	State        common.NodeState  `json:"state"`
	Status       common.NodeStatus `json:"status"`
	Capabilities NodeCapabilities  `json:"capabilities"`
}

func NewNodeResponse() NodeResponse {
	return NodeResponse{
		State: common.NewNodeState(),
		Status: common.NodeStatus{
			StatusCode: common.NodeStatusOffline,
			Reason:     "",
		},
		Capabilities: NodeCapabilities{},
	}
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
