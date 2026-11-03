package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/cloudinit"
)

type fakeNodeDriver struct {
	powerOnMode  common.RunMode
	powerOnCalls int
}

func (d *fakeNodeDriver) ID() common.NodeDriverID { return "fake_node_driver" }

func (d *fakeNodeDriver) Info() (common.NodeDriverInfo, error) {
	return common.NewNodeDriverInfo(d.ID(), nil, common.WithPowerOnMode(d.powerOnMode))
}

func (d *fakeNodeDriver) Config() common.NodeDriverConfig {
	return common.NodeDriverConfig{Driver: d.ID()}
}

func (d *fakeNodeDriver) PowerOn(_ *cloudinit.CloudInit) error {
	d.powerOnCalls++
	return nil
}

func (d *fakeNodeDriver) PowerOff() error                          { return nil }
func (d *fakeNodeDriver) Shutdown(_ string, _ uint32) error        { return nil }
func (d *fakeNodeDriver) Restart(_ string, _ uint32) error         { return nil }
func (d *fakeNodeDriver) State() (common.NodeState, error)         { return common.NodeState{}, nil }
func (d *fakeNodeDriver) UpdateStatus() (common.NodeStatus, error) { return common.NodeStatus{}, nil }

func TestPowerOnNodeRejectsClientMode(t *testing.T) {
	driver := &fakeNodeDriver{powerOnMode: common.ClientMode}

	err := PowerOnNode(&common.Node{Name: "n1", Driver: driver})

	assert.ErrorContains(t, err, "poweron expects agent mode")
	assert.Equal(t, 0, driver.powerOnCalls)
}

func TestPowerOnNodeAgentMode(t *testing.T) {
	driver := &fakeNodeDriver{powerOnMode: common.AgentMode}

	err := PowerOnNode(&common.Node{Name: "n1", Driver: driver})

	assert.NoError(t, err)
	assert.Equal(t, 1, driver.powerOnCalls)
}

func TestSameLoadsConfigComparesLoads(t *testing.T) {
	newNode := func(loads map[string]any) *common.Node {
		node := &common.Node{Name: "n1", Driver: &fakeNodeDriver{}}
		for name, config := range loads {
			node.Loads = append(node.Loads, common.LoadConfig{Name: name, Node: node.Name, Driver: "fake_load_driver", DriverConfig: config})
		}
		return node
	}

	current := newNode(map[string]any{"web": "v1", "db": "v1"})

	assert.True(t, newNode(map[string]any{"db": "v1", "web": "v1"}).SameLoadsConfig(current))
	assert.False(t, newNode(map[string]any{"web": "v2", "db": "v1"}).SameLoadsConfig(current))
	assert.False(t, newNode(map[string]any{"web": "v1"}).SameLoadsConfig(current))
	assert.False(t, newNode(map[string]any{"web": "v1", "cache": "v1"}).SameLoadsConfig(current))
}

func TestDiffLoads(t *testing.T) {
	load := func(name string, config any) common.LoadConfig {
		return common.LoadConfig{Name: name, Node: "n1", Driver: "fake_load_driver", DriverConfig: config}
	}
	current := []common.LoadConfig{load("web", "v1"), load("db", "v1"), load("cache", "v1")}
	next := []common.LoadConfig{load("web", "v2"), load("db", "v1"), load("queue", "v1")}

	stale, fresh := diffLoads(current, next)

	assert.Equal(t, []string{"web", "cache"}, stale)
	assert.Equal(t, []common.LoadConfig{load("web", "v2"), load("queue", "v1")}, fresh)

	stale, fresh = diffLoads(current, current)
	assert.Empty(t, stale)
	assert.Empty(t, fresh)
}
