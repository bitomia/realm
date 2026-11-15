package golang

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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
			runNodeOperationCase(t, name, op.path, op.run, mode)
		}
	}
}

func runNodeOperationCase(t *testing.T, name, path string, run func(Client, *common.Node) error, mode common.RunMode) {
	t.Helper()

	var agentCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, path, r.URL.Path)
		agentCalls++
		w.WriteHeader(http.StatusOK)
	}))

	driver := &modeNodeDriver{mode: mode}
	node := &common.Node{Name: "lab1", Url: server.URL, Driver: driver}

	err := run(newTestClient(), node)
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

func TestWaitNodeReady(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/version", r.URL.Path)
		if calls.Add(1) < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"version":"test"}`))
	}))
	defer server.Close()

	node := &common.Node{Name: "lab1", Url: server.URL, Timeout: common.TimeoutConfig{Start: common.Duration(10 * time.Second)}}

	client := newTestClient()
	assert.NoError(t, client.WaitNodeReady(context.Background(), node))
	assert.Equal(t, int32(2), calls.Load())
}

func TestWaitNodeReadyTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	node := &common.Node{Name: "lab1", Url: server.URL, Timeout: common.TimeoutConfig{Start: common.Duration(100 * time.Millisecond)}}

	client := newTestClient()
	err := client.WaitNodeReady(context.Background(), node)
	assert.ErrorContains(t, err, "node not ready after 100ms: unexpected status 503 Service Unavailable")
}

func TestWaitNodeStopped(t *testing.T) {
	stopped := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/version", r.URL.Path)
		_, _ = w.Write([]byte(`{"version":"test"}`))
		close(stopped)
	}))
	go func() {
		<-stopped
		server.Close()
	}()

	node := &common.Node{Name: "lab1", Url: server.URL, Timeout: common.TimeoutConfig{Stop: common.Duration(10 * time.Second)}}

	client := newTestClient()
	assert.NoError(t, client.WaitNodeStopped(context.Background(), node))
}

func TestWaitNodeStoppedTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":"test"}`))
	}))
	defer server.Close()

	node := &common.Node{Name: "lab1", Url: server.URL, Timeout: common.TimeoutConfig{Stop: common.Duration(100 * time.Millisecond)}}

	client := newTestClient()
	err := client.WaitNodeStopped(context.Background(), node)
	assert.ErrorContains(t, err, "node still running after 100ms")
}
