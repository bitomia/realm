package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	yamlConfig := `
nodes:
  lab1:
    url: http://192.168.1.54:9000
  lab2:
    url: http://192.168.1.55:9000

loads:
  include:
    - load1.yaml

  containers:
    web:
      node: lab1
      image: docker.io/nginx

    web2:
      node: lab2
      image: docker.io/nginx
      depends_on:
        - web

  processes:
    netcat:
      node: lab1
      start_cmd: nc -l 12345
      stop_signal: SIGHUP
      depends_on:
        - web
`

	err := readConfigFromReader(strings.NewReader(yamlConfig))

	assert.NoError(t, err)
	assert.NotNil(t, config.Loads)
	assert.Len(t, config.Loads.loads, 3)

	assert.NotNil(t, config.Loads.loads["web"])
	assert.Equal(t, config.Loads.loads["web"].name, "web")
	assert.Equal(t, config.Loads.loads["web"].driver.GetDriverType(), ContainerDriverType)
	assert.Equal(t, config.Loads.loads["web"].driver.(*ContainerDriver).Image, "docker.io/nginx")
}

func TestConfigCycleError(t *testing.T) {
	yamlConfig := `
nodes:
  lab1:
    url: http://192.168.1.54:9000
  lab2:
    url: http://192.168.1.55:9000

loads:
  include:
    - load1.yaml

  containers:
    web:
      node: lab1
      image: docker.io/nginx
      depends_on:
        - web2

    web2:
      node: lab2
      image: docker.io/nginx
      depends_on:
        - netcat

  processes:
    netcat:
      node: lab1
      start_cmd: nc -l 12345
      stop_signal: SIGHUP
      depends_on:
        - web
`
	err := readConfigFromReader(strings.NewReader(yamlConfig))
	assert.Error(t, err)
}
