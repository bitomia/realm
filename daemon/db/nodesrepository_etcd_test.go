package db

import (
	"encoding/json"
	"testing"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/cloudinit"
	"github.com/stretchr/testify/assert"
)

const MockNodeDriverID common.NodeDriverID = "mock_node_driver"

type mockNodeDriver struct {
	Value      string
	ShouldFail bool
	config     common.NodeDriverConfig
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

func NewMockNodeDriverFromConfig(c *any) (common.NodeDriver, error) {
	config := (*c).(map[string]any)
	return &mockNodeDriver{
		Value:      config["value"].(string),
		ShouldFail: config["should_fail"].(bool),
	}, nil
}

func (m *mockNodeDriver) GetNodeDriverID() common.NodeDriverID {
	return MockNodeDriverID
}

func (m *mockNodeDriver) DriverInfo() (common.NodeDriverInfo, error) {
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

func (m *mockNodeDriver) Provision(nodeName string, cloudInit *cloudinit.CloudInit, repository common.NodesRepository) error {
	return repository.SetSelf(nodeName, m, cloudInit, nil)
}

func (m *mockNodeDriver) Deprovision(nodeName *string, repository common.NodesRepository) error {
	return repository.DeleteSelf()
}

func (m *mockNodeDriver) Start(nodeName *string, repository common.NodesRepository) error {
	return nil
}

func (m *mockNodeDriver) Stop(nodeName *string, message string, time uint32, repository common.NodesRepository, force bool) error {
	return nil
}

func (m *mockNodeDriver) Restart(nodeName *string, message string, time uint32, repository common.NodesRepository) error {
	return nil
}

func (m *mockNodeDriver) UpdateStatus(nodeName *string, repository common.NodesRepository) (common.NodeStatus, error) {
	return common.NodeStatus{StatusCode: common.NodeStatusReady}, nil
}

func (m *mockNodeDriver) GetDriverConfig() common.NodeDriverConfig {
	return m.config
}

func (m *mockNodeDriver) GetCapabilities() (common.Capabilities, error) {
	return nil, nil
}

func (m *mockNodeDriver) GetState(_ *string, _ common.NodesRepository) (common.NodeState, error) {
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
