package common

import (
	"cmp"
	"fmt"
	"slices"
)

type JobDriverConfig struct {
	Driver       JobDriverID `json:"driver"`
	DriverConfig any         `json:"driver_config"`
}

type JobDriverErrorCode string

const (
	JobDriverErrAlreadyRegistered JobDriverErrorCode = "already_registered"
	JobDriverErrNotRegistered     JobDriverErrorCode = "not_registered"
	JobDriverErrBuildFailed       JobDriverErrorCode = "build_failed"
)

type JobDriverError struct {
	Code     JobDriverErrorCode
	DriverID JobDriverID
	Err      error
}

var jobDrivers = make(map[JobDriverID]JobDriverInfo)

func (e *JobDriverError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("jobDriverID '%s': %s: %v", e.DriverID, e.Code, e.Err)
	}
	return fmt.Sprintf("jobDriverID '%s': %s", e.DriverID, e.Code)
}

func RegisterJobDriver(d JobDriver) error {
	info := d.Info()
	if _, exists := jobDrivers[info.ID]; exists {
		return &JobDriverError{Code: JobDriverErrAlreadyRegistered, DriverID: info.ID}
	}

	jobDrivers[info.ID] = info
	return nil
}

func UnregisterJobDriver(id JobDriverID) error {
	if _, exists := jobDrivers[id]; !exists {
		return &JobDriverError{Code: JobDriverErrNotRegistered, DriverID: id}
	}

	delete(jobDrivers, id)
	return nil
}

func BuildJobDriver(d JobDriverConfig) (JobDriver, error) {
	if _, exists := jobDrivers[d.Driver]; !exists {
		return nil, &JobDriverError{Code: JobDriverErrNotRegistered, DriverID: d.Driver}
	}

	driver, err := jobDrivers[d.Driver].New(d.DriverConfig)
	if err != nil {
		return nil, &JobDriverError{Code: JobDriverErrBuildFailed, DriverID: d.Driver, Err: err}
	}

	return driver, nil
}

func RegisteredJobDrivers() []JobDriverInfo {
	infos := make([]JobDriverInfo, 0, len(jobDrivers))

	for _, info := range jobDrivers {
		infos = append(infos, info)
	}
	slices.SortFunc(infos, func(a, b JobDriverInfo) int {
		return cmp.Compare(a.ID, b.ID)
	})

	return infos
}
