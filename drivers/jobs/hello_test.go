package jobs

import (
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

func TestHelloDriverRun(t *testing.T) {
	value, err := (&HelloDriver{}).Run()

	require.NoError(t, err)
	require.NotNil(t, value)
	assert.Equal(t, "hello world", *value)
}

func TestHelloDriverRunIgnoresArguments(t *testing.T) {
	value, err := (&HelloDriver{}).Run("one", "two", "three")

	require.NoError(t, err)
	require.NotNil(t, value)
	assert.Equal(t, "hello world", *value)
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
