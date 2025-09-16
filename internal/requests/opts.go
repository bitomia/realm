package requests

type CpuCFS struct {
	CpuQuota  int64  `json:"quota"`
	CpuPeriod uint64 `json:"period"`
}

type Quotas struct {
	VolumeSize *string `json:"volume_size,omitempty"`
	MemLimit   *uint64 `json:"mem_limit,omitempty"` // Megabytes
	CpuCFS     *CpuCFS `json:"cpu_cfs,omitempty"`
	CpuShares  *uint64 `json:"cpu_shares,omitempty"`
}

type MountVolume struct {
	Volume string `json:"volume"`
	Target string `json:"target"`
}

type CreateContainerOpts struct {
	Image            string      `json:"image"`
	Owner            string      `json:"owner"`
	VolumeMountPoint string      `json:"volume_mount_point,omitempty"`
	MountVolume      MountVolume `json:"mount_volume,omitempty"`
	Env              []string    `json:"env,omitempty"`
	Quotas           Quotas      `json:"quotas,omitempty"`
}

type PortmapOpts struct {
	HostPort      uint16 `json:"host_port"`
	ContainerPort uint16 `json:"container_port"`
	Protocol      string `json:"protocol"`
}

type StartNetworkOpts struct {
	Network string        `json:"network"`
	IPMasq  bool          `json:"ip_masq,omitempty"`
	DNS     bool          `json:"dns,omitempty"`
	PortMap []PortmapOpts `json:"portmap,omitempty"`
}
