package db

import (
	"encoding/json"
	"testing"

	"github.com/bitomia/realm/common"
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

func (m *mockNodeDriver) DriverInfo() common.NodeDriverInfo {
	return common.NodeDriverInfo{
		ID:  MockNodeDriverID,
		New: NewMockNodeDriverFromConfig,
	}
}

func (m *mockNodeDriver) Verify() error {
	return nil
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

func (m *mockNodeDriver) PlanAndRegister(nodeName string, repository common.NodesRepository) error {
	return repository.Create(nodeName, m, nil)
}

func (m *mockNodeDriver) Startup() error {
	return nil
}

func (m *mockNodeDriver) Shutdown(message string, time uint32) error {
	return nil
}

func (m *mockNodeDriver) Restart(message string, time uint32) error {
	return nil
}

func (m *mockNodeDriver) GetStatus() (common.NodeStatus, error) {
	return common.NodeAvailable, nil
}

func (m *mockNodeDriver) GetDriverConfig() common.NodeDriverConfig {
	return m.config
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

// Tests for Create method
func TestEtcdNodesRepository_Create(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("test-driver")
	err := repo.Create("test-node", driver, nil)

	assert.NoError(t, err)
}
