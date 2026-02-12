package common

type Capabilities interface {
	ContainersEngine() bool
	ContainersNetworking() bool
	Volumes() bool
	VolumesZFS() bool
}
