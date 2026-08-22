package api

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
)

// fakeJobDriver records the arguments Run is called with and returns a
// preconfigured value/error pair.
type fakeJobDriver struct {
	value    *string
	err      error
	calls    int
	lastArgs []string
}

func (d *fakeJobDriver) ID() common.JobDriverID { return "fake" }

func (d *fakeJobDriver) Info() common.JobDriverInfo {
	return common.JobDriverInfo{
		ID:  "fake",
		New: func(config any) (common.JobDriver, error) { return d, nil },
	}
}

func (d *fakeJobDriver) Run(args ...string) (*string, error) {
	d.calls++
	d.lastArgs = args
	return d.value, d.err
}

func (d *fakeJobDriver) Config() common.JobDriverConfig {
	return common.JobDriverConfig{Driver: "fake", DriverConfig: nil}
}

func jobWithDriver(driver common.JobDriver) *common.Job {
	return &common.Job{Name: "greet", Node: &common.Node{Name: "lab1"}, Driver: driver}
}

func TestRunJobNilJob(t *testing.T) {
	result, err := RunJob(dto.JobRequest{Job: nil})

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job nil")
}

func TestRunJobSuccess(t *testing.T) {
	value := "hello world"
	driver := &fakeJobDriver{value: &value}

	result, err := RunJob(dto.JobRequest{Job: jobWithDriver(driver)})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Value)
	assert.Equal(t, "hello world", *result.Value)
	assert.Nil(t, result.Err)
	assert.Equal(t, 1, driver.calls)
}

// A driver failure is reported inside the result, not as a transport error:
// the job ran, it just did not succeed.
func TestRunJobDriverError(t *testing.T) {
	value := "ignored"
	driver := &fakeJobDriver{value: &value, err: errors.New("boom")}

	result, err := RunJob(dto.JobRequest{Job: jobWithDriver(driver)})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.Value, "no value is reported when the driver fails")
	require.NotNil(t, result.Err)
	assert.Equal(t, "boom", *result.Err)
}

func TestRunJobNoValueNoError(t *testing.T) {
	result, err := RunJob(dto.JobRequest{Job: jobWithDriver(&fakeJobDriver{})})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.Value)
	assert.Nil(t, result.Err)
}

func TestRunJobForwardsArguments(t *testing.T) {
	driver := &fakeJobDriver{}

	_, err := RunJob(dto.JobRequest{Job: jobWithDriver(driver), Arguments: []string{"one", "two", ""}})

	require.NoError(t, err)
	assert.Equal(t, []string{"one", "two", ""}, driver.lastArgs)
}

func TestRunJobWithoutArguments(t *testing.T) {
	driver := &fakeJobDriver{}

	_, err := RunJob(dto.JobRequest{Job: jobWithDriver(driver)})

	require.NoError(t, err)
	assert.Empty(t, driver.lastArgs)
}

// The request path only needs the driver: the agent never resolves the node
// reference carried in the payload (see common.Job.UnmarshalJSON).
func TestRunJobWithoutNode(t *testing.T) {
	value := "hello world"
	job := &common.Job{Name: "greet", Driver: &fakeJobDriver{value: &value}}

	result, err := RunJob(dto.JobRequest{Job: job})

	require.NoError(t, err)
	require.NotNil(t, result.Value)
	assert.Equal(t, "hello world", *result.Value)
}

func TestRunJobEmptyValue(t *testing.T) {
	empty := ""
	result, err := RunJob(dto.JobRequest{Job: jobWithDriver(&fakeJobDriver{value: &empty})})

	require.NoError(t, err)
	require.NotNil(t, result.Value, "an empty string is a value, not a missing result")
	assert.Equal(t, "", *result.Value)
}
