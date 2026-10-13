package golang

import "github.com/bitomia/realm/common"

type DriverEntry struct {
	ID        string `json:"id"`
	GuestMode *bool  `json:"guest_mode,omitempty"`
}

type RegisteredDrivers struct {
	Nodes []DriverEntry `json:"nodes"`
	Loads []DriverEntry `json:"loads"`
	Jobs  []DriverEntry `json:"jobs"`
}

// GetRegisteredDrivers returns what drivers are registered.
func GetRegisteredDrivers() RegisteredDrivers {
	drivers := RegisteredDrivers{
		Nodes: []DriverEntry{},
		Loads: []DriverEntry{},
		Jobs:  []DriverEntry{},
	}

	for _, info := range common.RegisteredNodeDrivers() {
		guestMode := info.GuestMode
		drivers.Nodes = append(drivers.Nodes, DriverEntry{ID: string(info.ID), GuestMode: &guestMode})
	}
	for _, info := range common.RegisteredLoadDrivers() {
		drivers.Loads = append(drivers.Loads, DriverEntry{ID: string(info.ID)})
	}
	for _, info := range common.RegisteredJobDrivers() {
		drivers.Jobs = append(drivers.Jobs, DriverEntry{ID: string(info.ID)})
	}

	return drivers
}
