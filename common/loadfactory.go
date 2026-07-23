package common

import (
	"fmt"
)

type LoadDriverConfig struct {
	Driver       LoadDriverID `json:"driver"`
	DriverConfig any          `json:"driver_config"`
}

type LoadDriverErrorCode string

const (
	LoadDriverErrAlreadyRegistered LoadDriverErrorCode = "already_registered"
	LoadDriverErrNotRegistered     LoadDriverErrorCode = "not_registered"
	LoadDriverErrBuildFailed       LoadDriverErrorCode = "build_failed"
)

type LoadDriverError struct {
	Code     LoadDriverErrorCode
	DriverID LoadDriverID
	Err      error
}

func (e *LoadDriverError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("loadDriverID '%s': %s: %v", e.DriverID, e.Code, e.Err)
	}
	return fmt.Sprintf("loadDriverID '%s': %s", e.DriverID, e.Code)
}

var loadDrivers = make(map[LoadDriverID]LoadDriverInfo)

func RegisterLoadDriver(d LoadDriver) error {
	info := d.Info()
	if _, exists := loadDrivers[info.ID]; exists {
		return &LoadDriverError{Code: LoadDriverErrAlreadyRegistered, DriverID: info.ID}
	}
	loadDrivers[info.ID] = info
	return nil
}

func UnregisterLoadDriver(id LoadDriverID) error {
	if _, exists := loadDrivers[id]; !exists {
		return &LoadDriverError{Code: LoadDriverErrNotRegistered, DriverID: id}
	}
	delete(loadDrivers, id)
	return nil
}

func BuildLoadDriver(d LoadDriverConfig) (LoadDriver, error) {
	if _, exists := loadDrivers[d.Driver]; !exists {
		return nil, &LoadDriverError{Code: LoadDriverErrNotRegistered, DriverID: d.Driver}
	}
	driver, err := loadDrivers[d.Driver].New(d.DriverConfig)
	if err != nil {
		return nil, &LoadDriverError{Code: LoadDriverErrBuildFailed, DriverID: d.Driver, Err: err}
	}
	return driver, nil
}
