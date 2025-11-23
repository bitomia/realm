package loads

import (
	"fmt"
)

var loadDrivers = make(map[LoadDriverID]LoadDriverInfo)

func RegisterLoadDriver(d LoadDriver) error {
	info := d.DriverInfo()
	if _, exists := loadDrivers[info.ID]; exists {
		return fmt.Errorf("LoadDriverID '%s' already registered", info.ID)
	}
	loadDrivers[info.ID] = info
	return nil
}

func UnregisterLoadDriver(id LoadDriverID) error {
	if _, exists := loadDrivers[id]; !exists {
		return fmt.Errorf("LoadDriverID '%s' not registered", id)
	}
	delete(loadDrivers, id)
	return nil
}

func BuildLoadDriver(d LoadConfig) (LoadDriver, error) {
	if _, exists := loadDrivers[d.Driver]; !exists {
		return nil, fmt.Errorf("LoadDriverID '%s' not registered", d.Driver)
	}
	driver, err := loadDrivers[d.Driver].New(d.DriverConfig)
	if err != nil {
		return nil, err
	}
	return driver, nil
}
