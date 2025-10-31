package loads

import (
	"encoding/json"
	"fmt"
)

type LoadDriverBuilder func() (l LoadDriver, err error)

var loadDrivers = make(map[LoadDriverType]LoadDriverBuilder)

func RegisterLoadDriver(t LoadDriverType, b LoadDriverBuilder) error {
	if _, exists := loadDrivers[t]; exists {
		return fmt.Errorf("LoadDriverType '%s' already registered", t)
	}
	loadDrivers[t] = b
	return nil
}

func UnregisterLoadDriver(t LoadDriverType, b LoadDriverBuilder) error {
	if _, exists := loadDrivers[t]; !exists {
		return fmt.Errorf("LoadDriverType '%s' not registered", t)
	}
	delete(loadDrivers, t)
	return nil
}

func BuildLoadDriver(d LoadData) (LoadDriver, error) {
	if _, exists := loadDrivers[d.DriverType]; !exists {
		return nil, fmt.Errorf("LoadDriverType '%s' not registered", d.DriverType)
	}
	driver, err := loadDrivers[d.DriverType]()
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(d.Driver, driver); err != nil {
		return nil, err
	}
	return driver, nil
}
