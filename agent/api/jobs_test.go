package api

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
)

// fakeJobDriver records the arguments Run is called with, streams the
// configured values and then returns the configured error.
type fakeJobDriver struct {
	id         common.JobDriverID
	values     []string
	err        error
	buildErr   error
	calls      int
	lastArgs   []string
	lastConfig any
}

func (d *fakeJobDriver) ID() common.JobDriverID { return d.id }

func (d *fakeJobDriver) Info() common.JobDriverInfo {
	return common.JobDriverInfo{
		ID: d.id,
		New: func(config any) (common.JobDriver, error) {
			if d.buildErr != nil {
				return nil, d.buildErr
			}
			d.lastConfig = config
			return d, nil
		},
	}
}

func (d *fakeJobDriver) Run(w common.JobResultWriter, args ...string) error {
	d.calls++
	d.lastArgs = args
	for _, value := range d.values {
		if err := w.WriteValue(value); err != nil {
			return err
		}
	}
	return d.err
}

func (d *fakeJobDriver) Config() common.JobDriverConfig {
	return common.JobDriverConfig{Driver: d.id, DriverConfig: nil}
}

// registerJob registers the driver for the duration of the test and returns
// the request payload for it.
func registerJob(t *testing.T, driver *fakeJobDriver) dto.JobRequest {
	t.Helper()

	driver.id = common.JobDriverID("fake-" + t.Name())
	require.NoError(t, common.RegisterJobDriver(driver))
	t.Cleanup(func() { require.NoError(t, common.UnregisterJobDriver(driver.id)) })

	return dto.JobRequest{Name: "greet", JobDriverConfig: driver.Config()}
}

// runJob runs the request against a recorder and returns the streamed results.
func runJob(t *testing.T, request dto.JobRequest) ([]common.JobResult, error) {
	t.Helper()

	rec := httptest.NewRecorder()
	err := RunJob(common.NewJobResultWriter(rec), request)

	var results []common.JobResult
	decoder := json.NewDecoder(rec.Body)
	for decoder.More() {
		var result common.JobResult
		require.NoError(t, decoder.Decode(&result))
		results = append(results, result)
	}

	return results, err
}

func TestRunJobSuccess(t *testing.T) {
	driver := &fakeJobDriver{values: []string{"hello world"}}

	results, err := runJob(t, registerJob(t, driver))

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].Value)
	assert.Equal(t, "hello world", *results[0].Value)
	assert.Nil(t, results[0].Err)
	assert.Equal(t, 1, driver.calls)
}

// Every value the driver writes reaches the caller, in order.
func TestRunJobStreamsEveryValue(t *testing.T) {
	driver := &fakeJobDriver{values: []string{"one", "two", "three"}}

	results, err := runJob(t, registerJob(t, driver))

	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "one", *results[0].Value)
	assert.Equal(t, "two", *results[1].Value)
	assert.Equal(t, "three", *results[2].Value)
}

// A driver failure is reported inside the stream, not as a transport error:
// the job ran, it just did not succeed.
func TestRunJobDriverError(t *testing.T) {
	driver := &fakeJobDriver{err: errors.New("boom")}

	results, err := runJob(t, registerJob(t, driver))

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Nil(t, results[0].Value)
	require.NotNil(t, results[0].Err)
	assert.Equal(t, "boom", *results[0].Err)
}

// A driver that fails halfway keeps the values it already streamed.
func TestRunJobDriverErrorAfterValues(t *testing.T) {
	driver := &fakeJobDriver{values: []string{"one"}, err: errors.New("boom")}

	results, err := runJob(t, registerJob(t, driver))

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "one", *results[0].Value)
	assert.Equal(t, "boom", *results[1].Err)
}

// A driver that cannot be built fails before anything is written, so the
// failure is returned and the caller can still answer with a status code.
func TestRunJobBuildError(t *testing.T) {
	driver := &fakeJobDriver{buildErr: errors.New("bad config")}

	results, err := runJob(t, registerJob(t, driver))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad config")
	assert.Empty(t, results)
	assert.Zero(t, driver.calls)
}

// An unregistered driver never runs, and never gets to write.
func TestRunJobUnknownDriver(t *testing.T) {
	results, err := runJob(t, dto.JobRequest{
		Name:            "greet",
		JobDriverConfig: common.JobDriverConfig{Driver: "nope"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")
	assert.Empty(t, results)
}

func TestRunJobWithoutResults(t *testing.T) {
	results, err := runJob(t, registerJob(t, &fakeJobDriver{}))

	require.NoError(t, err)
	assert.Empty(t, results, "a driver that writes nothing streams nothing")
}

func TestRunJobForwardsArguments(t *testing.T) {
	driver := &fakeJobDriver{}
	request := registerJob(t, driver)
	request.Arguments = []string{"one", "two", ""}

	_, err := runJob(t, request)

	require.NoError(t, err)
	assert.Equal(t, []string{"one", "two", ""}, driver.lastArgs)
}

func TestRunJobWithoutArguments(t *testing.T) {
	driver := &fakeJobDriver{}

	_, err := runJob(t, registerJob(t, driver))

	require.NoError(t, err)
	assert.Empty(t, driver.lastArgs)
}

// The driver config travels with the request and reaches the driver factory.
func TestRunJobForwardsDriverConfig(t *testing.T) {
	driver := &fakeJobDriver{}
	request := registerJob(t, driver)
	request.DriverConfig = map[string]any{"command": "echo"}

	_, err := runJob(t, request)

	require.NoError(t, err)
	assert.Equal(t, map[string]any{"command": "echo"}, driver.lastConfig)
}

func TestRunJobEmptyValue(t *testing.T) {
	results, err := runJob(t, registerJob(t, &fakeJobDriver{values: []string{""}}))

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].Value, "an empty string is a value, not a missing result")
	assert.Equal(t, "", *results[0].Value)
}
