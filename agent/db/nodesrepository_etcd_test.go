package db

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/cloudinit"
)

const MockNodeDriverID common.NodeDriverID = "mock_node_driver"

type mockNodeDriver struct {
	Value      string
	ShouldFail bool
	config     common.NodeDriverConfig
	ctx        common.NodeContext
}

func newMockNodeDriver(value string) *mockNodeDriver {
	config := any(map[string]any{
		"value":       value,
		"should_fail": false,
	})
	return &mockNodeDriver{
		Value:      value,
		ShouldFail: false,
		config: common.NodeDriverConfig{
			Driver:       MockNodeDriverID,
			DriverConfig: &config,
		},
	}
}

func NewMockNodeDriverFromConfig(ctx common.NodeContext, c *any) (common.NodeDriver, error) {
	config := (*c).(map[string]any)
	return &mockNodeDriver{
		Value:      config["value"].(string),
		ShouldFail: config["should_fail"].(bool),
		ctx:        ctx,
	}, nil
}

func (m *mockNodeDriver) ID() common.NodeDriverID {
	return MockNodeDriverID
}

func (m *mockNodeDriver) Info() (common.NodeDriverInfo, error) {
	return common.NodeDriverInfo{
		ID:  MockNodeDriverID,
		New: NewMockNodeDriverFromConfig,
	}, nil
}

func (m *mockNodeDriver) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"value":       m.Value,
		"should_fail": m.ShouldFail,
	})
}

func (m *mockNodeDriver) UnmarshalJSON(data []byte) error {
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	m.Value = config["value"].(string)
	m.ShouldFail = config["should_fail"].(bool)
	return nil
}

func (m *mockNodeDriver) Register(cloudInit *cloudinit.CloudInit) error {
	if m.ctx.NodeName == nil {
		return fmt.Errorf("node name required")
	}
	return m.ctx.Repository.SetSelf(*m.ctx.NodeName, m, cloudInit, nil)
}

func (m *mockNodeDriver) Unregister() error {
	return m.ctx.Repository.DeleteSelf()
}

func (m *mockNodeDriver) PowerOn() error {
	return nil
}

func (m *mockNodeDriver) PowerOff() error {
	return nil
}

func (m *mockNodeDriver) Shutdown(message string, time uint32) error {
	return nil
}

func (m *mockNodeDriver) Restart(message string, time uint32) error {
	return nil
}

func (m *mockNodeDriver) RefreshStatus() (common.NodeStatus, error) {
	return common.NodeStatus{StatusCode: common.NodeStatusReady}, nil
}

func (m *mockNodeDriver) Config() common.NodeDriverConfig {
	return m.config
}

func (m *mockNodeDriver) State() (common.NodeState, error) {
	return common.NodeState{}, nil
}

func init() {
	common.RegisterNodeDriver(&mockNodeDriver{})
}

// Test helpers
func setupNodesRepository(t *testing.T) (*EtcdNodesRepository, func()) {
	db, cleanup := setupTestDB(t)
	repo := &EtcdNodesRepository{db: db}
	return repo, cleanup
}

// Tests for Set method
func TestEtcdNodesRepository_Set(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("test-driver")
	err := repo.SetSelf("test-node", driver, nil, nil)

	assert.NoError(t, err)
}
