package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	d "github.com/bitomia/realm/drivers"
	"github.com/bitomia/realm/drivers/loads/drivers"
)

func init() {
	d.RegisterStdDrivers()
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

	resetConfig()
	err := readConfigFromReader(strings.NewReader(yamlConfig))
	assert.NoError(t, err)

	loads := GetLoads()

	assert.NotNil(t, loads)
	assert.Len(t, GetLoads(), 3)

	assert.NotNil(t, loads["web"])
	assert.Equal(t, loads["web"].Name, "web")
	assert.Equal(t, loads["web"].Driver.GetLoadDriverID(), drivers.ContainerDriverID)
	assert.Equal(t, loads["web"].Driver.(*drivers.ContainerDriver).Image, "docker.io/nginx")
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
	resetConfig()
	err := readConfigFromReader(strings.NewReader(yamlConfig))
	assert.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "cycle detected"))
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
	resetConfig()
	err := readConfigFromReader(strings.NewReader(yamlConfig))
	fmt.Printf("%s", err)
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
