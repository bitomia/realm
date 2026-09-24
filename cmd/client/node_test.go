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
	guest      bool
	localCalls int
}

func (d *modeNodeDriver) ID() common.NodeDriverID { return "client-mode-node" }

func (d *modeNodeDriver) Info() (common.NodeDriverInfo, error) {
	opts := []common.NewNodeDriverInfoOpts{
		common.WithPowerOnMode(d.mode),
		common.WithPowerOffMode(d.mode),
		common.WithShutdownMode(d.mode),
		common.WithRestartMode(d.mode),
	}
	if d.guest {
		opts = append(opts, common.WithGuestMode())
	}
	return common.NewNodeDriverInfo(d.ID(), nil, opts...)
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
		path      string
		guestPath string
		run       func(Client, *common.Node) error
	}{
		"poweron":  {"/node/poweron", "/node/guests/lab1/poweron", func(c Client, n *common.Node) error { return c.PowerOnNode(n) }},
		"poweroff": {"/node/poweroff", "/node/guests/lab1/poweroff", func(c Client, n *common.Node) error { return c.PowerOffNode(n) }},
		"shutdown": {"/node/shutdown", "/node/guests/lab1/shutdown", func(c Client, n *common.Node) error { return c.ShutdownNode(n, "", 0) }},
		"restart":  {"/node/restart", "/node/guests/lab1/restart", func(c Client, n *common.Node) error { return c.RestartNode(n, "", 0) }},
	}

	for name, op := range operations {
		for _, mode := range []common.RunMode{common.ClientMode, common.AgentMode} {
			for _, guest := range []bool{false, true} {
				runNodeOperationCase(t, name, op.path, op.guestPath, op.run, mode, guest)
			}
		}
	}
}

func runNodeOperationCase(t *testing.T, name, path, guestPath string, run func(Client, *common.Node) error, mode common.RunMode, guest bool) {
	t.Helper()

	expectedPath := path
	if guest {
		expectedPath = guestPath
	}

	var agentCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, expectedPath, r.URL.Path)
		agentCalls++
		w.WriteHeader(http.StatusOK)
	}))

	driver := &modeNodeDriver{mode: mode, guest: guest}
	node := &common.Node{Name: "lab1", Url: server.URL, Driver: driver}

	err := run(NewUnauthClient(), node)
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
