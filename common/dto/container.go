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

type BindMount struct {
	Source      string `json:"source"`      // Host path
	Destination string `json:"destination"` // Container path
	ReadOnly    bool   `json:"readonly,omitempty"`
}

type CreateContainerRequest struct {
	Image       string         `json:"image"`
	MountVolume *[]MountVolume `json:"mount_volume,omitempty"`
	BindMounts  []BindMount    `json:"bind_mounts,omitempty"`
	Env         []string       `json:"env,omitempty"`
	Quotas      *Quotas        `json:"quotas,omitempty"`
	Entrypoint  *string        `json:"entrypoint,omitempty"`
	Args        []string       `json:"args,omitempty"`
	WorkingDir  *string        `json:"working_dir,omitempty"`
}

type UpdateContainerOpts struct {
	State      common.LoadState `json:"state"`
	StdoutPath string           `json:"stdout_path"`
	StderrPath string           `json:"stderr_path"`
}

type DeleteContainerOpts struct {
	RemoveVolume    bool `json:"remove_volume,omitempty"`
	RemoveSnapshots bool `json:"remove_snapshots,omitempty"`
}
