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
	return m.ctx.Repository.SetSelf(common.Node{Name: m.ctx.NodeName, Url: "http://localhost:8080", Driver: m})
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

func (m *mockNodeDriver) UpdateStatus() (common.NodeStatus, error) {
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
func setupNodesRepository(t *testing.T) (*BoltNodesRepository, func()) {
	db, cleanup := setupTestDB(t)
	repo := &BoltNodesRepository{db: db}
	common.SetNodeContextBuilder(func(nodeName string) common.NodeContext {
		return common.NodeContext{Repository: repo, NodeName: nodeName, RunMode: common.AgentMode}
	})
	return repo, cleanup
}

// Tests for self node
func TestBoltNodesRepository_SetSelf(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("test-driver")
	err := repo.SetSelf(common.Node{Name: "test-node", Url: "http://localhost:8080", Driver: driver})

	assert.NoError(t, err)
}

func TestBoltNodesRepository_GetSelf(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("test-driver")
	registries := []common.RegistryConfig{{Host: "ghcr.io", Auth: common.RegistryAuth{Token: "token"}}}
	require.NoError(t, repo.SetSelf(common.Node{Name: "test-node", Url: "http://localhost:8080", Driver: driver, Registries: registries}))
	require.NoError(t, setSelfMetadata(repo, map[string]any{"key": "value"}))

	entry, err := repo.GetSelf()

	require.NoError(t, err)
	assert.Equal(t, "test-node", entry.Name)
	require.IsType(t, &mockNodeDriver{}, entry.Driver)
	assert.Equal(t, "test-driver", entry.Driver.(*mockNodeDriver).Value)
	assert.Equal(t, registries, entry.Registries)
	assert.Equal(t, map[string]any{"key": "value"}, entry.Metadata)
}

func TestBoltNodesRepository_GetSelf_NotConfigured(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	_, err := repo.GetSelf()

	assert.ErrorIs(t, err, common.ErrNodeNotConfigured)
}

func TestBoltNodesRepository_DeleteSelf(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("test-driver")
	require.NoError(t, repo.SetSelf(common.Node{Name: "test-node", Url: "http://localhost:8080", Driver: driver}))

	err := repo.DeleteSelf()

	require.NoError(t, err)
	_, err = repo.GetSelf()
	assert.ErrorIs(t, err, common.ErrNodeNotConfigured)
}

func TestBoltNodesRepository_DeleteSelf_NotConfigured(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	err := repo.DeleteSelf()

	assert.ErrorIs(t, err, common.ErrNodeNotConfigured)
}

func TestBoltNodesRepository_UpdateSelfMetadata(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("test-driver")
	require.NoError(t, repo.SetSelf(common.Node{Name: "test-node", Url: "http://localhost:8080", Driver: driver}))
	require.NoError(t, setSelfMetadata(repo, map[string]any{"key": "value"}))

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

func TestBoltNodesRepository_UpdateSelf(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	driver := newMockNodeDriver("test-driver")
	require.NoError(t, repo.SetSelf(common.Node{Name: "test-node", Url: "http://localhost:8080", Driver: driver}))
	require.NoError(t, setSelfMetadata(repo, map[string]any{"key": "value"}))

	err := repo.UpdateSelf(common.Node{Name: "test-node", Url: "http://localhost:9090", Driver: newMockNodeDriver("updated-driver")})

	require.NoError(t, err)
	entry, err := repo.GetSelf()
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9090", entry.Url)
	assert.Equal(t, "updated-driver", entry.Driver.(*mockNodeDriver).Value)
	assert.Equal(t, map[string]any{"key": "value"}, entry.Metadata)
}

func TestBoltNodesRepository_UpdateSelf_NotConfigured(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	err := repo.UpdateSelf(common.Node{Name: "test-node", Url: "http://localhost:8080", Driver: newMockNodeDriver("test-driver")})

	assert.ErrorIs(t, err, common.ErrNodeNotConfigured)
}

func setSelfMetadata(repo *BoltNodesRepository, metadata any) error {
	return repo.UpdateSelfMetadata(func(metadataPtr any) error {
		*(metadataPtr.(*any)) = metadata
		return nil
	})
}

func TestBoltNodesRepository_SetSelfWithLoads(t *testing.T) {
	repo, cleanup := setupNodesRepository(t)
	defer cleanup()

	node := common.Node{Name: "test-node", Url: "http://localhost:8080", Driver: newMockNodeDriver("test-driver")}
	node.Loads = []common.LoadConfig{
		{Name: "api", Node: node.Name, Driver: MockLoadDriverID, DriverConfig: map[string]any{"value": "api"}, DependsOn: []string{"db"}},
		{Name: "db", Node: node.Name, Driver: MockLoadDriverID, DriverConfig: map[string]any{"value": "db"}},
	}
	require.NoError(t, repo.SetSelf(node))

	entry, err := repo.GetSelf()

	require.NoError(t, err)
	require.Len(t, entry.Loads, 2)
	assert.Equal(t, "api", entry.Loads[0].Name)
	assert.Equal(t, "test-node", entry.Loads[0].Node)
	assert.Equal(t, []string{"db"}, entry.Loads[0].DependsOn)
	assert.Equal(t, "db", entry.Loads[1].Name)
	for i := range node.Loads {
		assert.Equal(t, node.Loads[i].Hash(), entry.Loads[i].Hash())
	}
}
