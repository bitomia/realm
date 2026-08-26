package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitomia/realm/common"
)

type stubJobDriver struct {
	id common.JobDriverID
}

func (d *stubJobDriver) ID() common.JobDriverID { return d.id }

func (d *stubJobDriver) Info() common.JobDriverInfo {
	return common.JobDriverInfo{
		ID:  d.id,
		New: func(config any) (common.JobDriver, error) { return &stubJobDriver{id: d.id}, nil },
	}
}

func (d *stubJobDriver) Run(w common.JobResultWriter, args ...string) error { return nil }

func (d *stubJobDriver) Config() common.JobDriverConfig {
	return common.JobDriverConfig{Driver: d.id, DriverConfig: nil}
}

func TestNewJobConfig(t *testing.T) {
	ResetJobsConfig()
	t.Cleanup(ResetJobsConfig)

	node := &common.Node{Name: "lab1"}
	driver := &stubJobDriver{id: "stub"}

	job, err := newJobConfig("greet", node, driver)
	require.NoError(t, err)
	require.NotNil(t, job)

	assert.Equal(t, "greet", job.Name)
	assert.Same(t, node, job.Node, "the job must reference the shared node instance")
	assert.Same(t, driver, job.Driver)
}

func TestNewJobConfigNotUnique(t *testing.T) {
	ResetJobsConfig()
	t.Cleanup(ResetJobsConfig)

	node := &common.Node{Name: "lab1"}

	_, err := newJobConfig("greet", node, &stubJobDriver{id: "stub"})
	require.NoError(t, err)

	job, err := newJobConfig("greet", node, &stubJobDriver{id: "stub"})
	assert.Nil(t, job)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job 'greet' not unique")
}

func TestGetJobsFilter(t *testing.T) {
	ResetJobsConfig()
	t.Cleanup(ResetJobsConfig)

	node := &common.Node{Name: "lab1"}
	for _, name := range []string{"greet", "backup", "cleanup"} {
		_, err := newJobConfig(name, node, &stubJobDriver{id: "stub"})
		require.NoError(t, err)
	}

	all := GetJobs()
	assert.Len(t, all, 3)
	assert.Contains(t, all, "greet")

	filtered := GetJobs("greet", "cleanup")
	assert.Len(t, filtered, 2)
	assert.Contains(t, filtered, "greet")
	assert.Contains(t, filtered, "cleanup")
	assert.NotContains(t, filtered, "backup")

	assert.Empty(t, GetJobs("unknown"))
}

func TestResetJobsConfig(t *testing.T) {
	ResetJobsConfig()
	t.Cleanup(ResetJobsConfig)

	_, err := newJobConfig("greet", &common.Node{Name: "lab1"}, &stubJobDriver{id: "stub"})
	require.NoError(t, err)
	require.Len(t, GetJobs(), 1)

	ResetJobsConfig()

	assert.Empty(t, GetJobs())

	// After a reset the same job name can be registered again.
	_, err = newJobConfig("greet", &common.Node{Name: "lab1"}, &stubJobDriver{id: "stub"})
	assert.NoError(t, err)
}

func TestConfigGetJob(t *testing.T) {
	ResetJobsConfig()
	t.Cleanup(ResetJobsConfig)

	node := &common.Node{Name: "lab1"}
	job, err := newJobConfig("greet", node, &stubJobDriver{id: "stub"})
	require.NoError(t, err)

	cfg := &Config{processedJobs: map[string]*common.Job{"greet": job}}

	assert.Same(t, job, cfg.GetJob("greet"))
	assert.Nil(t, cfg.GetJob("unknown"), "an unknown job name must return nil, not an empty job")
}
