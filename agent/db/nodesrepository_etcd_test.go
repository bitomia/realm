package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		config: common.NodeDriverConfig{
			Driver:       MockNodeDriverID,
			DriverConfig: c,
		},
		ctx: ctx,
	}, nil
}

func (m *mockNodeDriver) ID() common.NodeDriverID {
	return MockNodeDriverID
}

func (m *mockNodeDriver) Info() (common.NodeDriverInfo, error) {
	return common.NewNodeDriverInfo(
		MockNodeDriverID,
		NewMockNodeDriverFromConfig,
	)
}

func (m *mockNodeDriver) Register() error {
	return m.ctx.Repository.SetSelf(m.ctx.NodeName, m, nil)
}

func (m *mockNodeDriver) Unregister() error {
	return m.ctx.Repository.DeleteSelf()
}

func (m *mockNodeDriver) PowerOn(_ *cloudinit.CloudInit) error {
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
	common.SetNodeContextBuilder(func(nodeName string) common.NodeContext {
		return common.NodeContext{Repository: repo, NodeName: nodeName, RunMode: common.AgentMode}
	})
	return repo, cleanup
}

// Tests for self node
func TestEtcdNodesRepository_SetSelf(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("test-driver")
	err := repo.SetSelf("test-node", driver, nil)

	assert.NoError(t, err)
}

func TestEtcdNodesRepository_GetSelf(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("test-driver")
	metadata := map[string]any{"key": "value"}
	require.NoError(t, repo.SetSelf("test-node", driver, metadata))

	entry, err := repo.GetSelf()

	require.NoError(t, err)
	assert.Equal(t, "test-node", entry.NodeName)
	require.IsType(t, &mockNodeDriver{}, entry.NodeDriver)
	assert.Equal(t, "test-driver", entry.NodeDriver.(*mockNodeDriver).Value)
	assert.Equal(t, map[string]any{"key": "value"}, entry.Metadata)
}

func TestEtcdNodesRepository_GetSelf_NotConfigured(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	_, err := repo.GetSelf()

	assert.ErrorIs(t, err, common.ErrNodeNotConfigured)
}

func TestEtcdNodesRepository_DeleteSelf(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("test-driver")
	require.NoError(t, repo.SetSelf("test-node", driver, nil))

	err := repo.DeleteSelf()

	require.NoError(t, err)
	_, err = repo.GetSelf()
	assert.ErrorIs(t, err, common.ErrNodeNotConfigured)
}

func TestEtcdNodesRepository_DeleteSelf_NotConfigured(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	err := repo.DeleteSelf()

	assert.ErrorIs(t, err, common.ErrNodeNotConfigured)
}

func TestEtcdNodesRepository_UpdateSelfMetadata(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("test-driver")
	require.NoError(t, repo.SetSelf("test-node", driver, map[string]any{"key": "value"}))

	err := repo.UpdateSelfMetadata(func(metadataPtr any) error {
		ptr := metadataPtr.(*any)
		*ptr = map[string]any{"key": "updated"}
		return nil
	})

	require.NoError(t, err)
	entry, err := repo.GetSelf()
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"key": "updated"}, entry.Metadata)
}

// Tests for guest nodes
func TestEtcdNodesRepository_SetGuestNode(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("guest-driver")
	err := repo.SetGuestNode("guest-node", driver, nil)

	assert.NoError(t, err)
}

func TestEtcdNodesRepository_GetGuestNode(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("guest-driver")
	metadata := map[string]any{"key": "value"}
	require.NoError(t, repo.SetGuestNode("guest-node", driver, metadata))

	entry, err := repo.GetGuestNode("guest-node")

	require.NoError(t, err)
	assert.Equal(t, "guest-node", entry.NodeName)
	require.IsType(t, &mockNodeDriver{}, entry.NodeDriver)
	assert.Equal(t, "guest-driver", entry.NodeDriver.(*mockNodeDriver).Value)
	assert.Equal(t, map[string]any{"key": "value"}, entry.Metadata)
}

func TestEtcdNodesRepository_GetGuestNode_NotFound(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	_, err := repo.GetGuestNode("missing-guest")

	assert.Error(t, err)
}

func TestEtcdNodesRepository_GetAllGuestNodes(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	require.NoError(t, repo.SetGuestNode("guest-1", newMockNodeDriver("driver-1"), nil))
	require.NoError(t, repo.SetGuestNode("guest-2", newMockNodeDriver("driver-2"), nil))

	nodes, err := repo.GetAllGuestNodes()

	require.NoError(t, err)
	assert.Len(t, nodes, 2)
	names := []string{nodes[0].NodeName, nodes[1].NodeName}
	assert.ElementsMatch(t, []string{"guest-1", "guest-2"}, names)
}

func TestEtcdNodesRepository_GetAllGuestNodes_Empty(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	nodes, err := repo.GetAllGuestNodes()

	require.NoError(t, err)
	assert.Empty(t, nodes)
}

func TestEtcdNodesRepository_DeleteGuestNode(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("guest-driver")
	require.NoError(t, repo.SetGuestNode("guest-node", driver, nil))

	err := repo.DeleteGuestNode("guest-node", driver, nil)

	require.NoError(t, err)
	_, err = repo.GetGuestNode("guest-node")
	assert.Error(t, err)
}

func TestEtcdNodesRepository_DeleteGuestNode_NotFound(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("guest-driver")
	err := repo.DeleteGuestNode("missing-guest", driver, nil)

	assert.ErrorIs(t, err, common.ErrNodeNotConfigured)
}

func TestEtcdNodesRepository_UpdateGuestMetadata(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("guest-driver")
	require.NoError(t, repo.SetGuestNode("guest-node", driver, map[string]any{"key": "value"}))

	err := repo.UpdateGuestMetadata("guest-node", func(metadataPtr any) error {
		ptr := metadataPtr.(*any)
		*ptr = map[string]any{"key": "updated"}
		return nil
	})

	require.NoError(t, err)
	entry, err := repo.GetGuestNode("guest-node")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"key": "updated"}, entry.Metadata)
}
