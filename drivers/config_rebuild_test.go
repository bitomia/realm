package drivers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
)

func TestConfigWithLoad(t *testing.T) {
	resetConfigs()
	cfg, err := config.InitFromBuffer(`
nodes:
  lab1:
    url: http://192.168.1.54:9000
    driver: linux
loads:
  web:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/nginx
`)
	assert.NoError(t, err)

	newCfg, err := cfg.WithLoad("web2", common.LoadConfig{
		Node: "lab1", Driver: "container", DependsOn: []string{"web"},
		DriverConfig: map[string]any{"image": "docker.io/nginx"},
	})
	assert.NoError(t, err)
	assert.Len(t, newCfg.GetLoads(), 2)
	assert.Len(t, cfg.Loads, 1)

	_, err = newCfg.WithLoad("bad", common.LoadConfig{Node: "nope", Driver: "container"})
	assert.Error(t, err)
	_, err = newCfg.WithLoad("web", common.LoadConfig{Node: "lab1", Driver: "container", DependsOn: []string{"web2"}})
	assert.Error(t, err, "cycle must be rejected")
	assert.Len(t, config.GetLoadsFromConfig(), 2, "registries restored after failure")
}

func TestConfigWithNode(t *testing.T) {
	resetConfigs()
	cfg, err := config.InitFromBuffer(`
nodes:
  lab1:
    url: http://192.168.1.54:9000
    driver: linux
loads:
  web:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/nginx
`)
	assert.NoError(t, err)

	newCfg, err := cfg.WithNode("lab2", common.NodeConfig{Url: "http://192.168.1.55:9000", Driver: "linux"})
	assert.NoError(t, err)
	assert.Len(t, newCfg.GetNodes(), 2)
	assert.Len(t, cfg.Nodes, 1)

	// replacing a node keeps the loads that run on it
	newCfg, err = newCfg.WithNode("lab1", common.NodeConfig{Url: "http://192.168.1.60:9000", Driver: "linux"})
	assert.NoError(t, err)
	assert.Equal(t, "http://192.168.1.60:9000", newCfg.GetNodes("lab1")["lab1"].Url)
	assert.Equal(t, "lab1", newCfg.GetLoads("web")["web"].Node.Name)

	_, err = newCfg.WithNode("bad", common.NodeConfig{Url: "http://192.168.1.56:9000"})
	assert.Error(t, err, "driver is required")
	_, err = newCfg.WithNode("bad", common.NodeConfig{Url: "http://192.168.1.56:9000", Driver: "nope"})
	assert.Error(t, err, "unknown driver")
	assert.Len(t, newCfg.GetNodes(), 2)
}

func TestConfigWithout(t *testing.T) {
	resetConfigs()
	cfg, err := config.InitFromBuffer(`
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
`)
	assert.NoError(t, err)

	_, err = cfg.WithoutLoad("web")
	assert.Error(t, err, "web2 depends on web")
	_, err = cfg.WithoutNode("lab1")
	assert.Error(t, err, "loads run on lab1")
	assert.Len(t, config.GetLoadsFromConfig(), 2, "registries restored after failure")

	newCfg, err := cfg.WithoutNode("lab2")
	assert.NoError(t, err)
	assert.Len(t, newCfg.GetNodes(), 1)

	newCfg, err = newCfg.WithoutLoad("web2")
	assert.NoError(t, err)
	newCfg, err = newCfg.WithoutLoad("web")
	assert.NoError(t, err)
	assert.Empty(t, newCfg.GetLoads())
	assert.Len(t, cfg.Loads, 2)
}

func TestConfigWithJob(t *testing.T) {
	resetConfigs()
	cfg, err := config.InitFromBuffer(`
nodes:
  lab1:
    url: http://192.168.1.54:9000
    driver: linux
`)
	assert.NoError(t, err)

	newCfg, err := cfg.WithJob("greet", common.JobConfig{Node: "lab1", Driver: "hello"})
	assert.NoError(t, err)
	assert.NotNil(t, newCfg.GetJob("greet"))
	assert.Empty(t, cfg.Jobs)

	_, err = newCfg.WithJob("bad", common.JobConfig{Node: "nope", Driver: "hello"})
	assert.Error(t, err, "unknown node")
	_, err = newCfg.WithJob("bad", common.JobConfig{Node: "lab1", Driver: "nope"})
	assert.Error(t, err, "unknown driver")
	_, err = newCfg.WithoutNode("lab1")
	assert.Error(t, err, "greet runs on lab1")

	newCfg, err = newCfg.WithoutJob("greet")
	assert.NoError(t, err)
	assert.Nil(t, newCfg.GetJob("greet"))
}
