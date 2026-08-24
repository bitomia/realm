package drivers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitomia/realm/common/config"
	jobsPkg "github.com/bitomia/realm/drivers/jobs/hello"
)

const jobsTestNodes = `
nodes:
  lab1:
    url: http://192.168.1.54:9000
    driver: linux
  lab2:
    url: http://192.168.1.55:9000
    driver: linux
`

func TestJobsConfig(t *testing.T) {
	yamlConfig := jobsTestNodes + `
jobs:
  greet:
    node: lab1
    driver: hello
`
	resetConfigs()

	cfg, err := config.InitFromBuffer(yamlConfig)
	require.NoError(t, err)

	job := cfg.GetJob("greet")
	require.NotNil(t, job)

	assert.Equal(t, "greet", job.Name)
	assert.Equal(t, jobsPkg.HelloDriverID, job.Driver.ID())

	require.NotNil(t, job.Node)
	assert.Equal(t, "lab1", job.Node.Name)
	assert.Same(t, cfg.GetNodes()["lab1"], job.Node, "the job must point at the configured node instance")
	assert.Equal(t, "http://192.168.1.54:9000", job.Node.Url)
}

func TestJobsConfigMultipleJobsAcrossNodes(t *testing.T) {
	yamlConfig := jobsTestNodes + `
jobs:
  greet:
    node: lab1
    driver: hello
  greet2:
    node: lab2
    driver: hello
`
	resetConfigs()

	cfg, err := config.InitFromBuffer(yamlConfig)
	require.NoError(t, err)

	jobs := config.GetJobs()
	assert.Len(t, jobs, 2)

	assert.Equal(t, "lab1", cfg.GetJob("greet").Node.Name)
	assert.Equal(t, "lab2", cfg.GetJob("greet2").Node.Name)

	filtered := config.GetJobs("greet2")
	assert.Len(t, filtered, 1)
	assert.Contains(t, filtered, "greet2")
}

func TestJobsConfigUnknownJob(t *testing.T) {
	yamlConfig := jobsTestNodes + `
jobs:
  greet:
    node: lab1
    driver: hello
`
	resetConfigs()

	cfg, err := config.InitFromBuffer(yamlConfig)
	require.NoError(t, err)

	assert.Nil(t, cfg.GetJob("nonexistent"))
}

func TestJobsConfigWithoutJobsSection(t *testing.T) {
	resetConfigs()

	cfg, err := config.InitFromBuffer(jobsTestNodes)
	require.NoError(t, err)

	assert.Empty(t, config.GetJobs())
	assert.Nil(t, cfg.GetJob("greet"))
}

func TestJobsConfigMissingNode(t *testing.T) {
	yamlConfig := jobsTestNodes + `
jobs:
  greet:
    driver: hello
`
	resetConfigs()

	_, err := config.InitFromBuffer(yamlConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job 'greet' with empty node field")
}

func TestJobsConfigMissingDriver(t *testing.T) {
	yamlConfig := jobsTestNodes + `
jobs:
  greet:
    node: lab1
`
	resetConfigs()

	_, err := config.InitFromBuffer(yamlConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "driver required for job 'greet'")
}

func TestJobsConfigUnknownNode(t *testing.T) {
	yamlConfig := jobsTestNodes + `
jobs:
  greet:
    node: lab404
    driver: hello
`
	resetConfigs()

	_, err := config.InitFromBuffer(yamlConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "node 'lab404' referenced by job 'greet' does not exist")
}

func TestJobsConfigUnknownDriver(t *testing.T) {
	yamlConfig := jobsTestNodes + `
jobs:
  greet:
    node: lab1
    driver: nosuchdriver
`
	resetConfigs()

	_, err := config.InitFromBuffer(yamlConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nosuchdriver")
	assert.Contains(t, err.Error(), "not_registered")
}

func TestJobsConfigCoexistsWithLoads(t *testing.T) {
	yamlConfig := jobsTestNodes + `
loads:
  web:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/nginx

jobs:
  greet:
    node: lab1
    driver: hello
`
	resetConfigs()

	cfg, err := config.InitFromBuffer(yamlConfig)
	require.NoError(t, err)

	assert.Len(t, cfg.GetLoads(), 1)
	assert.Len(t, config.GetJobs(), 1)
	assert.NotNil(t, cfg.GetJob("greet"))
	assert.NotNil(t, cfg.GetLoads()["web"])
}

// A load and a job may share a name: they live in separate namespaces.
func TestJobsConfigSameNameAsLoad(t *testing.T) {
	yamlConfig := jobsTestNodes + `
loads:
  web:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/nginx

jobs:
  web:
    node: lab1
    driver: hello
`
	resetConfigs()

	cfg, err := config.InitFromBuffer(yamlConfig)
	require.NoError(t, err)

	require.NotNil(t, cfg.GetJob("web"))
	assert.Equal(t, jobsPkg.HelloDriverID, cfg.GetJob("web").Driver.ID())
	require.NotNil(t, cfg.GetLoads()["web"])
}

func TestJobsConfigDriverIsRunnable(t *testing.T) {
	yamlConfig := jobsTestNodes + `
jobs:
  greet:
    node: lab1
    driver: hello
`
	resetConfigs()

	cfg, err := config.InitFromBuffer(yamlConfig)
	require.NoError(t, err)

	value, err := cfg.GetJob("greet").Driver.Run()
	require.NoError(t, err)
	require.NotNil(t, value)
	assert.Equal(t, "hello world", *value)
}

func TestJobsConfigHashIsStableAcrossReloads(t *testing.T) {
	yamlConfig := jobsTestNodes + `
jobs:
  greet:
    node: lab1
    driver: hello
`
	resetConfigs()
	first, err := config.InitFromBuffer(yamlConfig)
	require.NoError(t, err)
	firstHash := first.GetJob("greet").Hash()

	resetConfigs()
	second, err := config.InitFromBuffer(yamlConfig)
	require.NoError(t, err)
	secondHash := second.GetJob("greet").Hash()

	assert.Equal(t, firstHash, secondHash)

	// Moving the job to another node changes its hash.
	resetConfigs()
	moved, err := config.InitFromBuffer(jobsTestNodes + `
jobs:
  greet:
    node: lab2
    driver: hello
`)
	require.NoError(t, err)
	assert.NotEqual(t, firstHash, moved.GetJob("greet").Hash())
}
