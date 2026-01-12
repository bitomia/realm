package drivers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bitomia/realm/common/config"
	loadsPkg "github.com/bitomia/realm/drivers/loads"
)

func init() {
	RegisterStdDrivers()
}

func TestConfig(t *testing.T) {
	yamlConfig := `
nodes:
  lab1:
    url: http://192.168.1.54:9000
  lab2:
    url: http://192.168.1.55:9000

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
	config.ResetConfig()
	err := config.InitFromBuffer(yamlConfig)
	assert.NoError(t, err)

	loads := config.GetLoadsFromConfig()
	assert.NotNil(t, loads)
	assert.Len(t, config.GetLoadsFromConfig(), 3)

	assert.NotNil(t, loads["web"])
	assert.Equal(t, loads["web"].Name, "web")
	assert.Equal(t, loads["web"].Driver.GetLoadDriverID(), loadsPkg.ContainerDriverID)
	assert.Equal(t, loads["web"].Driver.(*loadsPkg.ContainerDriver).Config.Image, "docker.io/nginx")
}

func TestConfigCycleError(t *testing.T) {
	yamlConfig := `
nodes:
  lab1:
    url: http://192.168.1.54:9000
  lab2:
    url: http://192.168.1.55:9000

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
	config.ResetConfig()
	err := config.InitFromBuffer(yamlConfig)
	assert.True(t, strings.Contains(err.Error(), "cycle detected"))
}

func TestProcessDriverInvalidCmd(t *testing.T) {
	yamlConfig := `
nodes:
  lab1:
    url: http://192.168.1.54:9000

loads:
  netcat:
    node: lab1
    driver: process
    driver_config:
      start_cmd: cmd invalid
      stop_signal: SIGHUP
      depends_on:
        - web
`
	config.ResetConfig()
	err := config.InitFromBuffer(yamlConfig)
	assert.Error(t, err)
}

// func TestProcessDriverValidCmd(t *testing.T) {
// 	yamlConfig := `
// nodes:
//   lab1:
//     url: http://192.168.1.54:9000

// loads:
//   processes:
//     netcat:
//       node: lab1
//       start_cmd: cmd
//       stop_signal: SIGHUP
// `
// 	resetConfig()
// 	err := readConfigFromReader(strings.NewReader(yamlConfig))

// 	loads := GetLoads()
// 	assert.NoError(t, err)
// 	assert.NotNil(t, loads)
// 	assert.Len(t, loads, 1)
// }

// func TestProcessDriverInvalidStopSignal(t *testing.T) {
// 	yamlConfig := `
// nodes:
//   lab1:
//     url: http://192.168.1.54:9000

// loads:
//   processes:
//     netcat:
//       node: lab1
//       start_cmd: cmd
//       stop_signal: INVALID
// `
// 	resetConfig()
// 	err := readConfigFromReader(strings.NewReader(yamlConfig))

// 	assert.Error(t, err)
// 	assert.Len(t, GetLoads(), 0)
// }
