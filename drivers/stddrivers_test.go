package drivers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
	loadsPkg "github.com/bitomia/realm/drivers/loads"
)

func init() {
	if err := RegisterStdDrivers(); err != nil {
		panic(err)
	}

	common.SetNodeContextBuilder(func(nodeName string) common.NodeContext {
		return common.NodeContext{Repository: nil, NodeName: nodeName, RunMode: common.ClientMode}
	})
}

func resetConfigs() {
	config.ResetLoadsConfig()
	config.ResetNodesConfig()
}

func TestConfig(t *testing.T) {
	yamlConfig := `
nodes:
  lab1:
    url: http://192.168.1.54:9000
    driver: linux
  lab2:
    url: http://192.168.1.55:9000
    driver: linux

loads:
  web:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/nginx

  web2:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/nginx
    depends_on:
      - web

  netcat:
    node: lab1
    driver: process
    driver_config:
      start_cmd: nc
      stop_signal: SIGHUP
      depends_on:
        - web
`
	resetConfigs()

	cfg, err := config.InitFromBuffer(yamlConfig)
	assert.NoError(t, err)

	loads := cfg.GetLoads()
	assert.NotNil(t, loads)
	assert.Len(t, cfg.GetLoads(), 3)

	assert.NotNil(t, loads["web"])
	assert.Equal(t, loads["web"].Name, "web")
	assert.Equal(t, loads["web"].Driver.ID(), loadsPkg.ContainerDriverID)
	assert.Equal(t, loads["web"].Driver.(*loadsPkg.ContainerDriver).Config.Image, "docker.io/nginx")
}

func TestConfigCycleError(t *testing.T) {
	yamlConfig := `
nodes:
  lab1:
    url: http://192.168.1.54:9000
    driver: linux
  lab2:
    url: http://192.168.1.55:9000
    driver: linux

loads:
  web:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/nginx
    depends_on:
      - web2

  web2:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/nginx
    depends_on:
      - netcat

  netcat:
    node: lab1
    driver: process
    driver_config:
      start_cmd: nc
      stop_signal: SIGHUP
    depends_on:
      - web
`
	resetConfigs()

	_, err := config.InitFromBuffer(yamlConfig)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "cycle detected"))
}

func TestContainerDriverBindMounts(t *testing.T) {
	yamlConfig := `
nodes:
  bindmount_node:
    url: http://192.168.1.54:9000
    driver: linux

loads:
  bindmount_web:
    node: bindmount_node
    driver: container
    driver_config:
      image: docker.io/nginx
      bind_mounts:
        - source: ./si2
          destination: /home/runner/si2
        - source: ./entrypoint
          destination: /home/runner/entrypoint
        - source: /opt/si2_license
          destination: /opt/si2_license
          readonly: true
        - source: /opt/terrainData
          destination: /opt/terrainData
`
	resetConfigs()

	cfg, err := config.InitFromBuffer(yamlConfig)
	assert.NoError(t, err)

	loads := cfg.GetLoads()
	assert.NotNil(t, loads)
	assert.Len(t, loads, 1)

	webLoad := loads["bindmount_web"]
	assert.NotNil(t, webLoad)
	assert.Equal(t, webLoad.Driver.ID(), loadsPkg.ContainerDriverID)

	containerDriver := webLoad.Driver.(*loadsPkg.ContainerDriver)
	assert.Equal(t, "docker.io/nginx", containerDriver.Config.Image)
	assert.Len(t, containerDriver.Config.BindMounts, 4)

	// Verify first bind mount
	assert.Equal(t, "./si2", containerDriver.Config.BindMounts[0].Source)
	assert.Equal(t, "/home/runner/si2", containerDriver.Config.BindMounts[0].Destination)
	assert.False(t, containerDriver.Config.BindMounts[0].ReadOnly)

	// Verify second bind mount
	assert.Equal(t, "./entrypoint", containerDriver.Config.BindMounts[1].Source)
	assert.Equal(t, "/home/runner/entrypoint", containerDriver.Config.BindMounts[1].Destination)
	assert.False(t, containerDriver.Config.BindMounts[1].ReadOnly)

	// Verify third bind mount (read-only)
	assert.Equal(t, "/opt/si2_license", containerDriver.Config.BindMounts[2].Source)
	assert.Equal(t, "/opt/si2_license", containerDriver.Config.BindMounts[2].Destination)
	assert.True(t, containerDriver.Config.BindMounts[2].ReadOnly)

	// Verify fourth bind mount
	assert.Equal(t, "/opt/terrainData", containerDriver.Config.BindMounts[3].Source)
	assert.Equal(t, "/opt/terrainData", containerDriver.Config.BindMounts[3].Destination)
	assert.False(t, containerDriver.Config.BindMounts[3].ReadOnly)
}

func TestContainerDriverNoBindMounts(t *testing.T) {
	yamlConfig := `
nodes:
  nomount_node:
    url: http://192.168.1.54:9000
    driver: linux

loads:
  nomount_web:
    node: nomount_node
    driver: container
    driver_config:
      image: docker.io/nginx
`
	resetConfigs()

	cfg, err := config.InitFromBuffer(yamlConfig)
	assert.NoError(t, err)

	loads := cfg.GetLoads()
	assert.NotNil(t, loads)

	webLoad := loads["nomount_web"]
	assert.NotNil(t, webLoad)

	containerDriver := webLoad.Driver.(*loadsPkg.ContainerDriver)
	assert.Nil(t, containerDriver.Config.BindMounts)
}

func TestContainerDriverEmptyBindMounts(t *testing.T) {
	yamlConfig := `
nodes:
  emptymount_node:
    url: http://192.168.1.54:9000
    driver: linux

loads:
  emptymount_web:
    node: emptymount_node
    driver: container
    driver_config:
      image: docker.io/nginx
      bind_mounts: []
`
	resetConfigs()

	cfg, err := config.InitFromBuffer(yamlConfig)
	assert.NoError(t, err)

	loads := cfg.GetLoads()
	assert.NotNil(t, loads)

	webLoad := loads["emptymount_web"]
	assert.NotNil(t, webLoad)

	containerDriver := webLoad.Driver.(*loadsPkg.ContainerDriver)
	assert.NotNil(t, containerDriver.Config.BindMounts)
	assert.Len(t, containerDriver.Config.BindMounts, 0)
}
