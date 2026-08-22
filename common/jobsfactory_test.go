package common

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterJobDriver(t *testing.T) {
	driver := &testJobDriver{id: "factory-register"}
	registerTestJobDriver(t, driver)

	built, err := BuildJobDriver(JobDriverConfig{Driver: "factory-register"})
	require.NoError(t, err)
	assert.Equal(t, JobDriverID("factory-register"), built.ID())
}

func TestRegisterJobDriverAlreadyRegistered(t *testing.T) {
	driver := &testJobDriver{id: "factory-duplicate"}
	registerTestJobDriver(t, driver)

	err := RegisterJobDriver(&testJobDriver{id: "factory-duplicate"})
	require.Error(t, err)

	var driverErr *JobDriverError
	require.True(t, errors.As(err, &driverErr))
	assert.Equal(t, JobDriverErrAlreadyRegistered, driverErr.Code)
	assert.Equal(t, JobDriverID("factory-duplicate"), driverErr.DriverID)
}

func TestUnregisterJobDriver(t *testing.T) {
	require.NoError(t, RegisterJobDriver(&testJobDriver{id: "factory-unregister"}))
	require.NoError(t, UnregisterJobDriver("factory-unregister"))

	// Once unregistered the driver can no longer be built...
	_, err := BuildJobDriver(JobDriverConfig{Driver: "factory-unregister"})
	require.Error(t, err)

	// ...and the same ID becomes available again.
	require.NoError(t, RegisterJobDriver(&testJobDriver{id: "factory-unregister"}))
	require.NoError(t, UnregisterJobDriver("factory-unregister"))
}

func TestUnregisterJobDriverNotRegistered(t *testing.T) {
	err := UnregisterJobDriver("factory-missing")
	require.Error(t, err)

	var driverErr *JobDriverError
	require.True(t, errors.As(err, &driverErr))
	assert.Equal(t, JobDriverErrNotRegistered, driverErr.Code)
	assert.Equal(t, JobDriverID("factory-missing"), driverErr.DriverID)
}

func TestBuildJobDriverNotRegistered(t *testing.T) {
	driver, err := BuildJobDriver(JobDriverConfig{Driver: "factory-unknown"})
	assert.Nil(t, driver)
	require.Error(t, err)

	var driverErr *JobDriverError
	require.True(t, errors.As(err, &driverErr))
	assert.Equal(t, JobDriverErrNotRegistered, driverErr.Code)
}

func TestBuildJobDriverPassesDriverConfig(t *testing.T) {
	registerTestJobDriver(t, &testJobDriver{id: "factory-config"})

	driverConfig := map[string]any{"greeting": "hola", "times": 2}
	built, err := BuildJobDriver(JobDriverConfig{Driver: "factory-config", DriverConfig: driverConfig})
	require.NoError(t, err)

	assert.Equal(t, JobDriverID("factory-config"), built.Config().Driver)
	assert.Equal(t, driverConfig, built.Config().DriverConfig)
}

func TestBuildJobDriverBuildsANewInstance(t *testing.T) {
	registerTestJobDriver(t, &testJobDriver{id: "factory-instances"})

	first, err := BuildJobDriver(JobDriverConfig{Driver: "factory-instances", DriverConfig: "a"})
	require.NoError(t, err)
	second, err := BuildJobDriver(JobDriverConfig{Driver: "factory-instances", DriverConfig: "b"})
	require.NoError(t, err)

	assert.NotSame(t, first, second)
	assert.Equal(t, "a", first.Config().DriverConfig)
	assert.Equal(t, "b", second.Config().DriverConfig)
}

func TestBuildJobDriverBuildFailed(t *testing.T) {
	buildErr := errors.New("bad driver config")
	registerTestJobDriver(t, &testJobDriver{id: "factory-buildfail", buildErr: buildErr})

	driver, err := BuildJobDriver(JobDriverConfig{Driver: "factory-buildfail"})
	assert.Nil(t, driver)
	require.Error(t, err)

	var driverErr *JobDriverError
	require.True(t, errors.As(err, &driverErr))
	assert.Equal(t, JobDriverErrBuildFailed, driverErr.Code)
	assert.Equal(t, buildErr, driverErr.Err)
	assert.Contains(t, err.Error(), "bad driver config")
}

func TestJobDriverErrorMessage(t *testing.T) {
	withoutCause := &JobDriverError{Code: JobDriverErrNotRegistered, DriverID: "hello"}
	assert.Equal(t, "jobDriverID 'hello': not_registered", withoutCause.Error())

	withCause := &JobDriverError{Code: JobDriverErrBuildFailed, DriverID: "hello", Err: errors.New("boom")}
	assert.Equal(t, "jobDriverID 'hello': build_failed: boom", withCause.Error())
}

func TestJobDriverRegistryIsIsolatedFromNodeAndLoadDrivers(t *testing.T) {
	// A job driver ID must not collide with a load/node driver namespace.
	registerTestJobDriver(t, &testJobDriver{id: "container"})

	built, err := BuildJobDriver(JobDriverConfig{Driver: "container"})
	require.NoError(t, err)
	assert.Equal(t, JobDriverID("container"), built.ID())
}
