package dto

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

type CreateContainerRequest struct {
	Image            string      `json:"image"`
	Owner            string      `json:"owner"`
	VolumeMountPoint string      `json:"volume_mount_point,omitempty"`
	MountVolume      MountVolume `json:"mount_volume,omitempty"`
	Env              []string    `json:"env,omitempty"`
	Quotas           Quotas      `json:"quotas,omitempty"`
}
