package common

import (
	"fmt"
)

type LoadDriverConfig struct {
	Driver       LoadDriverID `json:"driver"`
	DriverConfig any          `json:"driver_config"`
}

var loadDrivers = make(map[LoadDriverID]LoadDriverInfo)

func RegisterLoadDriver(d LoadDriver) error {
	info := d.DriverInfo()
	if _, exists := loadDrivers[info.ID]; exists {
		return fmt.Errorf("loadDriverID '%s' already registered", info.ID)
	}
	loadDrivers[info.ID] = info
	return nil
}

func UnregisterLoadDriver(id LoadDriverID) error {
	if _, exists := loadDrivers[id]; !exists {
		return fmt.Errorf("loadDriverID '%s' not registered", id)
	}
	delete(loadDrivers, id)
	return nil
}

func BuildLoadDriver(d LoadDriverConfig) (LoadDriver, error) {
	if _, exists := loadDrivers[d.Driver]; !exists {
		return nil, fmt.Errorf("loadDriverID '%s' not registered", d.Driver)
	}
	driver, err := loadDrivers[d.Driver].New(d.DriverConfig)
	if err != nil {
		return nil, err
	}
	return driver, nil
}
