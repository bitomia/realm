package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/cloudinit"
)

// modeNodeDriver runs every operation in the configured mode and counts the
// operations executed locally.
type modeNodeDriver struct {
	mode       common.RunMode
	localCalls int
}

func (d *modeNodeDriver) ID() common.NodeDriverID { return "client-mode-node" }

func (d *modeNodeDriver) Info() (common.NodeDriverInfo, error) {
	return common.NewNodeDriverInfo(d.ID(), nil,
		common.WithPowerOnMode(d.mode),
		common.WithPowerOffMode(d.mode),
		common.WithShutdownMode(d.mode),
		common.WithRestartMode(d.mode),
	)
}

func (d *modeNodeDriver) Config() common.NodeDriverConfig {
	return common.NodeDriverConfig{Driver: d.ID()}
}

func (d *modeNodeDriver) PowerOn(_ *cloudinit.CloudInit) error     { d.localCalls++; return nil }
func (d *modeNodeDriver) PowerOff() error                          { d.localCalls++; return nil }
func (d *modeNodeDriver) Shutdown(_ string, _ uint32) error        { d.localCalls++; return nil }
func (d *modeNodeDriver) Restart(_ string, _ uint32) error         { d.localCalls++; return nil }
func (d *modeNodeDriver) State() (common.NodeState, error)         { return common.NodeState{}, nil }
func (d *modeNodeDriver) UpdateStatus() (common.NodeStatus, error) { return common.NodeStatus{}, nil }

func TestClientNodeOperationsRunMode(t *testing.T) {
	operations := map[string]struct {
		path string
		run  func(Client, *common.Node) error
	}{
		"poweron":  {"/node/poweron", func(c Client, n *common.Node) error { return c.PowerOnNode(n) }},
		"poweroff": {"/node/poweroff", func(c Client, n *common.Node) error { return c.PowerOffNode(n) }},
		"shutdown": {"/node/shutdown", func(c Client, n *common.Node) error { return c.ShutdownNode(n, "", 0) }},
		"restart":  {"/node/restart", func(c Client, n *common.Node) error { return c.RestartNode(n, "", 0) }},
	}

	for name, op := range operations {
		for _, mode := range []common.RunMode{common.ClientMode, common.AgentMode} {
			var agentCalls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, op.path, r.URL.Path)
				agentCalls++
				w.WriteHeader(http.StatusOK)
			}))

			driver := &modeNodeDriver{mode: mode}
			node := &common.Node{Name: "lab1", Url: server.URL, Driver: driver}

			err := op.run(NewUnauthClient(), node)
			server.Close()

			assert.NoError(t, err, name)
			if mode == common.ClientMode {
				assert.Equal(t, 1, driver.localCalls, "%s in client mode must run locally", name)
				assert.Equal(t, 0, agentCalls, "%s in client mode must not call the agent", name)
			} else {
				assert.Equal(t, 0, driver.localCalls, "%s in agent mode must not run locally", name)
				assert.Equal(t, 1, agentCalls, "%s in agent mode must call the agent", name)
			}
		}
	}
}
