package hello

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitomia/realm/common"
)

func TestHelloDriverID(t *testing.T) {
	driver := &HelloDriver{}

	assert.Equal(t, HelloDriverID, driver.ID())
	assert.Equal(t, common.JobDriverID("hello"), driver.ID())
}

func TestHelloDriverInfo(t *testing.T) {
	driver := &HelloDriver{}
	info := driver.Info()

	assert.Equal(t, HelloDriverID, info.ID)
	require.NotNil(t, info.New)

	built, err := info.New(nil)
	require.NoError(t, err)
	require.NotNil(t, built)
	assert.IsType(t, &HelloDriver{}, built)
	assert.Equal(t, HelloDriverID, built.ID())
}

func TestHelloDriverInfoIgnoresDriverConfig(t *testing.T) {
	built, err := (&HelloDriver{}).Info().New(map[string]any{"anything": true})

	require.NoError(t, err)
	require.NotNil(t, built)
	assert.Nil(t, built.Config().DriverConfig)
}

// runHello runs the driver against a recorder and returns what it streamed.
func runHello(t *testing.T, args ...string) ([]common.JobResult, error) {
	t.Helper()

	rec := httptest.NewRecorder()
	err := (&HelloDriver{}).Run(common.NewJobResultWriter(rec), args...)

	var results []common.JobResult
	decoder := json.NewDecoder(rec.Body)
	for decoder.More() {
		var result common.JobResult
		require.NoError(t, decoder.Decode(&result))
		results = append(results, result)
	}
	return results, err
}

func TestHelloDriverRun(t *testing.T) {
	results, err := runHello(t)

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].Value)
	assert.Equal(t, "hello world", *results[0].Value)
}

// The first argument is a repeat count, and every greeting is streamed.
func TestHelloDriverRunRepeats(t *testing.T) {
	results, err := runHello(t, "3")

	require.NoError(t, err)
	require.Len(t, results, 3)
	for _, result := range results {
		assert.Equal(t, "hello world", *result.Value)
	}
}

func TestHelloDriverRunIgnoresExtraArguments(t *testing.T) {
	results, err := runHello(t, "1", "two", "three")

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "hello world", *results[0].Value)
}

func TestHelloDriverRunInvalidRepeatCount(t *testing.T) {
	results, err := runHello(t, "many")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid repeat count "many"`)
	assert.Empty(t, results)
}

func TestHelloDriverRunNonPositiveRepeatCount(t *testing.T) {
	results, err := runHello(t, "0")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 1")
	assert.Empty(t, results)
}

func TestHelloDriverConfig(t *testing.T) {
	config := (&HelloDriver{}).Config()

	assert.Equal(t, HelloDriverID, config.Driver)
	assert.Nil(t, config.DriverConfig)
}

func TestHelloDriverSatisfiesJobDriver(t *testing.T) {
	var driver common.JobDriver = &HelloDriver{}
	assert.Equal(t, HelloDriverID, driver.ID())
}
