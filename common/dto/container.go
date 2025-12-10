package dto

import "github.com/bitomia/realm/common"

type CpuCFS struct {
	CpuQuota  int64  `json:"quota"`
	CpuPeriod uint64 `json:"period"`
}

type Quotas struct {
	MemLimit  *uint64 `json:"mem_limit,omitempty"` // Megabytes
	CpuCFS    *CpuCFS `json:"cpu_cfs,omitempty"`
	CpuShares *uint64 `json:"cpu_shares,omitempty"`
}

type MountVolume struct {
	VolumeMountPoint string  `json:"volume_mount_point"`
	VolumeSize       *string `json:"volume_size,omitempty"`
}

type CreateContainerRequest struct {
	Image       string         `json:"image"`
	Owner       string         `json:"owner,omitempty"`
	MountVolume *[]MountVolume `json:"mount_volume,omitempty"`
	Env         []string       `json:"env,omitempty"`
	Quotas      *Quotas        `json:"quotas,omitempty"`
}

type UpdateContainerOpts struct {
	State common.LoadState `json:"state"`
}

type DeleteContainerOpts struct {
	RemoveVolume    bool `json:"remove_volume,omitempty"`
	RemoveSnapshots bool `json:"remove_snapshots,omitempty"`
}
