package dto

import (
	"github.com/bitomia/realm/common"
)

type Capabilities struct {
	ContainersEngine     bool `json:"containers_engine"`
	ContainersNetworking bool `json:"containers_networking"`
	Volumes              bool `json:"volumes"`
	VolumesZFS           bool `json:"volumes_zfs"`
	VMM                  bool `json:"vmm"`
}

func NewCapabilities(c common.Capabilities) Capabilities {
	if c == nil {
		return Capabilities{}
	}
	return Capabilities{
		ContainersEngine:     c.ContainersEngine(),
		ContainersNetworking: c.ContainersNetworking(),
		Volumes:              c.Volumes(),
		VolumesZFS:           c.VolumesZFS(),
		VMM:                  c.VMM(),
	}
}
