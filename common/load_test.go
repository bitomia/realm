package common

import (
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/dominikbraun/graph"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type MockLoadDriver struct {
	driver       LoadDriverID
	driverConfig any
}

func (m *MockLoadDriver) ID() LoadDriverID {
	return m.driver
}

func (m *MockLoadDriver) Info() LoadDriverInfo {
	return LoadDriverInfo{
		ID:  m.driver,
		New: func(config any) (LoadDriver, error) { return m, nil },
	}
}

func (m *MockLoadDriver) MarshalJSON() ([]byte, error) {
	return []byte("{}"), nil
}

func (m *MockLoadDriver) UnmarshalJSON(data []byte) error {
	return nil
}

func (m *MockLoadDriver) Provision(node NodeDriver, repository DeploymentsRepository, loadName string) (DeploymentID, error) {
	return uuid.New(), nil
}

func (m *MockLoadDriver) Start(repository DeploymentsRepository, deployment Deployment) error {
	return nil
}

func (m *MockLoadDriver) Stop(repository DeploymentsRepository, deployment Deployment) error {
	return nil
}

func (m *MockLoadDriver) Deprovision(repository DeploymentsRepository, deployment Deployment) error {
	return nil
}

func (m *MockLoadDriver) Kill(repository DeploymentsRepository, deployment Deployment) error {
	return nil
}

func (m *MockLoadDriver) UpdateStatus(repository DeploymentsRepository, deployment Deployment) (DeploymentStatus, error) {
	return DeploymentStatus{}, nil
}

func (m *MockLoadDriver) Config() LoadDriverConfig {
	return LoadDriverConfig{
		Driver:       m.driver,
		DriverConfig: m.driverConfig,
	}
}

func (m *MockLoadDriver) StreamStdout(repository DeploymentsRepository, deployment Deployment, w io.Writer) error {
	return nil
}

func (m *MockLoadDriver) StreamStderr(repository DeploymentsRepository, deployment Deployment, w io.Writer) error {
	return nil
}

func (m *MockLoadDriver) ReadStdout(repository DeploymentsRepository, deployment Deployment, offset int64) ([]byte, int64, error) {
	return nil, 0, nil
}

func (m *MockLoadDriver) ReadStderr(repository DeploymentsRepository, deployment Deployment, offset int64) ([]byte, int64, error) {
	return nil, 0, nil
}

func createTestLoad(name, nodeName string, driverID LoadDriverID, driverConfig any) *Load {
	node := &Node{Name: nodeName}
	driver := &MockLoadDriver{driver: driverID, driverConfig: driverConfig}

	load := &Load{
		Name:      name,
		Driver:    driver,
		DependsOn: make([]*Load, 0),
		Node:      node,
	}

	return load
}

func buildTestGraph(loads []*Load) (graph.Graph[string, string], map[string]*Load) {
	g := graph.New(graph.StringHash, graph.Directed())
	loadsMap := make(map[string]*Load)

	for _, load := range loads {
		g.AddVertex(load.Name)
		loadsMap[load.Name] = load
	}

	for _, load := range loads {
		for _, dep := range load.DependsOn {
			g.AddEdge(load.Name, dep.Name)
		}
	}

	return g, loadsMap
}

func updateLoadChains(loads []*Load) {
	g, loadsMap := buildTestGraph(loads)
	for _, load := range loads {
		_ = load.UpdateLoadChains(g, loadsMap)
	}
}

func TestHashableLoadConfig_BasicHashing(t *testing.T) {
	tests := []struct {
		name     string
		load     *Load
		expected string
	}{
		{
			name: "simple_load_no_dependencies",
			load: createTestLoad("web", "node1", "container", map[string]any{"image": "nginx"}),
		},
		{
			name: "load_with_different_driver_config",
			load: createTestLoad("web", "node1", "container", map[string]any{"image": "apache"}),
		},
		{
			name: "load_on_different_node",
			load: createTestLoad("web", "node2", "container", map[string]any{"image": "nginx"}),
		},
		{
			name: "process_driver",
			load: createTestLoad("proc", "node1", "process", map[string]any{"start_cmd": "echo hello"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := tt.load.Hash()
			hash2 := tt.load.Hash()

			assert.Equal(t, hash1, hash2, "Hash should be consistent for same load")
			assert.NotEqual(t, [32]byte{}, hash1, "Hash should not be empty")
		})
	}
}

func TestHashableLoadConfig_HashUniqueness(t *testing.T) {
	load1 := createTestLoad("web", "node1", "container", map[string]any{"image": "nginx"})
	load2 := createTestLoad("web", "node1", "container", map[string]any{"image": "apache"})
	load3 := createTestLoad("web", "node2", "container", map[string]any{"image": "nginx"})
	load4 := createTestLoad("api", "node1", "container", map[string]any{"image": "nginx"})
	load5 := createTestLoad("web", "node1", "process", map[string]any{"image": "nginx"})

	hash1 := load1.Hash()
	hash2 := load2.Hash()
	hash3 := load3.Hash()
	hash4 := load4.Hash()
	hash5 := load5.Hash()

	assert.NotEqual(t, hash1, hash2, "Different driver configs should produce different hashes")
	assert.NotEqual(t, hash1, hash3, "Different nodes should produce different hashes")
	assert.NotEqual(t, hash1, hash4, "Different names should produce different hashes")
	assert.NotEqual(t, hash1, hash5, "Different drivers should produce different hashes")
}

func TestHashableLoadConfig_NilHandling(t *testing.T) {
	load := &Load{
		Name:      "test",
		Driver:    &MockLoadDriver{driver: "test", driverConfig: nil},
		DependsOn: nil,
		Node:      &Node{Name: "node1"},
	}

	hash := load.Hash()
	assert.NotEqual(t, [32]byte{}, hash, "Hash should handle nil dependencies gracefully")
}

func TestLoadChain_HashInvalidation(t *testing.T) {
	db := createTestLoad("db", "node1", "container", map[string]any{"image": "postgres"})
	api := createTestLoad("api", "node1", "container", map[string]any{"image": "golang"})
	web := createTestLoad("web", "node1", "container", map[string]any{"image": "nginx"})

	api.DependsOn = []*Load{db}
	web.DependsOn = []*Load{api}

	chain := LoadChain{db, api, web}
	originalChainHash := chain.Hash()

	t.Run("change_in_leaf_node_invalidates_chain", func(t *testing.T) {
		dbModified := createTestLoad("db", "node1", "container", map[string]any{"image": "mysql"})
		chainModified := LoadChain{dbModified, api, web}
		newChainHash := chainModified.Hash()

		assert.NotEqual(t, originalChainHash, newChainHash, "Changing leaf node should invalidate chain hash")
	})

	t.Run("change_in_middle_node_invalidates_chain", func(t *testing.T) {
		apiModified := createTestLoad("api", "node1", "container", map[string]any{"image": "python"})
		apiModified.DependsOn = []*Load{db}
		chainModified := LoadChain{db, apiModified, web}
		newChainHash := chainModified.Hash()

		assert.NotEqual(t, originalChainHash, newChainHash, "Changing middle node should invalidate chain hash")
	})

	t.Run("change_in_root_node_invalidates_chain", func(t *testing.T) {
		webModified := createTestLoad("web", "node1", "container", map[string]any{"image": "apache"})
		webModified.DependsOn = []*Load{api}
		chainModified := LoadChain{db, api, webModified}
		newChainHash := chainModified.Hash()

		assert.NotEqual(t, originalChainHash, newChainHash, "Changing root node should invalidate chain hash")
	})

	t.Run("reordering_chain_changes_hash", func(t *testing.T) {
		chainReordered := LoadChain{web, api, db}
		reorderedHash := chainReordered.Hash()

		assert.NotEqual(t, originalChainHash, reorderedHash, "Reordering chain should change hash")
	})
}

func TestLoadChain_DependencyChainHashing(t *testing.T) {
	load1 := createTestLoad("load1", "node1", "container", map[string]any{"image": "nginx"})
	load2 := createTestLoad("load2", "node1", "container", map[string]any{"image": "golang"})
	load3 := createTestLoad("load3", "node1", "container", map[string]any{"image": "redis"})
	load4 := createTestLoad("load4", "node1", "container", map[string]any{"image": "postgres"})

	load2.DependsOn = []*Load{load1}
	load3.DependsOn = []*Load{load2}
	load4.DependsOn = []*Load{load3}

	t.Run("chain_hashes_are_consistent", func(t *testing.T) {
		chain := LoadChain{load1, load2, load3, load4}
		hash1 := chain.Hash()
		hash2 := chain.Hash()

		assert.Equal(t, hash1, hash2, "Chain hash should be consistent")
	})

	t.Run("adding_load_to_chain_changes_hash", func(t *testing.T) {
		originalChain := LoadChain{load1, load2, load3}
		extendedChain := LoadChain{load1, load2, load3, load4}

		originalHash := originalChain.Hash()
		extendedHash := extendedChain.Hash()

		assert.NotEqual(t, originalHash, extendedHash, "Adding load to chain should change hash")
	})

	t.Run("removing_load_from_chain_changes_hash", func(t *testing.T) {
		fullChain := LoadChain{load1, load2, load3, load4}
		reducedChain := LoadChain{load1, load2, load3}

		fullHash := fullChain.Hash()
		reducedHash := reducedChain.Hash()

		assert.NotEqual(t, fullHash, reducedHash, "Removing load from chain should change hash")
	})

	t.Run("empty_chain_has_deterministic_hash", func(t *testing.T) {
		emptyChain1 := LoadChain{}
		emptyChain2 := LoadChain{}

		hash1 := emptyChain1.Hash()
		hash2 := emptyChain2.Hash()

		assert.Equal(t, hash1, hash2, "Empty chains should have same hash")
		assert.NotEqual(t, [32]byte{}, hash1, "Empty chain hash should not be zero")
	})
}

func TestLoadChain_ComplexDependencyScenarios(t *testing.T) {
	t.Run("deep_dependency_change_propagation", func(t *testing.T) {
		db := createTestLoad("db", "node1", "container", map[string]any{"image": "postgres:13", "port": 5432})
		cache := createTestLoad("cache", "node1", "container", map[string]any{"image": "redis:6", "port": 6379})
		api1 := createTestLoad("api1", "node1", "container", map[string]any{"image": "golang:1.19"})
		api2 := createTestLoad("api2", "node1", "container", map[string]any{"image": "nodejs:18"})
		web := createTestLoad("web", "node1", "container", map[string]any{"image": "nginx:latest"})

		api1.DependsOn = []*Load{db, cache}
		api2.DependsOn = []*Load{db}
		web.DependsOn = []*Load{api1, api2}

		originalChain := LoadChain{db, cache, api1, api2, web}
		originalHash := originalChain.Hash()

		db.Node.Name = "node2"
		newHash := originalChain.Hash()

		assert.NotEqual(t, originalHash, newHash, "Deep dependency change should propagate through chain")
	})

	t.Run("multiple_dependency_changes", func(t *testing.T) {
		serviceA := createTestLoad("serviceA", "node1", "process", map[string]any{"start_cmd": "serviceA", "port": 8080})
		serviceB := createTestLoad("serviceB", "node1", "process", map[string]any{"start_cmd": "serviceB", "port": 8081})
		serviceC := createTestLoad("serviceC", "node1", "process", map[string]any{"start_cmd": "serviceC", "port": 8082})

		serviceC.DependsOn = []*Load{serviceA, serviceB}

		originalChain := LoadChain{serviceA, serviceB, serviceC}
		originalHash := originalChain.Hash()

		serviceC.DependsOn = []*Load{serviceA}
		newHash := originalChain.Hash()

		assert.NotEqual(t, originalHash, newHash, "Multiple dependency changes should invalidate chain")
	})
}

func TestHashableLoadConfig_EdgeCases(t *testing.T) {
	t.Run("empty_driver_config", func(t *testing.T) {
		load1 := createTestLoad("test", "node1", "container", nil)
		load2 := createTestLoad("test", "node1", "container", map[string]any{})

		hash1 := load1.Hash()
		hash2 := load2.Hash()

		assert.NotEqual(t, hash1, hash2, "nil and empty map driver configs should produce different hashes")
	})

	t.Run("special_characters_in_names", func(t *testing.T) {
		load1 := createTestLoad("test-service", "node1", "container", map[string]any{"image": "nginx"})
		load2 := createTestLoad("test_service", "node1", "container", map[string]any{"image": "nginx"})
		load3 := createTestLoad("test service", "node1", "container", map[string]any{"image": "nginx"})

		hash1 := load1.Hash()
		hash2 := load2.Hash()
		hash3 := load3.Hash()

		assert.NotEqual(t, hash1, hash2, "Different name formats should produce different hashes")
		assert.NotEqual(t, hash1, hash3, "Names with spaces should produce different hashes")
		assert.NotEqual(t, hash2, hash3, "Underscore vs space should produce different hashes")
	})

	t.Run("unicode_characters", func(t *testing.T) {
		load1 := createTestLoad("test-α", "node1", "container", map[string]any{"image": "nginx"})
		load2 := createTestLoad("test-β", "node1", "container", map[string]any{"image": "nginx"})

		hash1 := load1.Hash()
		hash2 := load2.Hash()

		assert.NotEqual(t, hash1, hash2, "Unicode characters should be handled correctly in hashing")
	})

}

func TestHashableLoadConfig_FieldOrderIndependence(t *testing.T) {
	config1 := map[string]any{
		"image": "nginx",
		"env":   map[string]any{"A": "1", "B": "2"},
	}

	config2 := map[string]any{
		"env":   map[string]any{"B": "2", "A": "1"},
		"image": "nginx",
	}

	load1 := createTestLoad("test", "node1", "container", config1)
	load2 := createTestLoad("test", "node1", "container", config2)

	hash1 := load1.Hash()
	hash2 := load2.Hash()

	assert.Equal(t, hash1, hash2, "JSON field order should not affect hash")
}

func TestLoadChain_PerformanceAndScalability(t *testing.T) {
	t.Run("large_dependency_chain_performance", func(t *testing.T) {
		const chainSize = 100
		var loads []*Load
		var chain LoadChain

		for i := range chainSize {
			load := createTestLoad(
				fmt.Sprintf("load-%d", i),
				"node1",
				"container",
				map[string]any{"image": fmt.Sprintf("app-%d", i)},
			)
			if i > 0 {
				load.DependsOn = []*Load{loads[i-1]}
			}
			loads = append(loads, load)
			chain = append(chain, load)
		}

		start := time.Now()
		hash := chain.Hash()
		duration := time.Since(start)

		assert.NotEqual(t, [32]byte{}, hash, "Large chain should produce valid hash")
		assert.Less(t, duration, time.Second, "Large chain hashing should complete within reasonable time")
	})

	t.Run("wide_dependency_tree_performance", func(t *testing.T) {
		const fanOut = 50
		root := createTestLoad("root", "node1", "container", map[string]any{"image": "base"})
		var chain LoadChain = []*Load{root}

		for i := range fanOut {
			child := createTestLoad(
				fmt.Sprintf("child-%d", i),
				"node1",
				"container",
				map[string]any{"image": fmt.Sprintf("child-%d", i)},
			)
			child.DependsOn = []*Load{root}
			chain = append(chain, child)
		}

		start := time.Now()
		hash := chain.Hash()
		duration := time.Since(start)

		assert.NotEqual(t, [32]byte{}, hash, "Wide dependency tree should produce valid hash")
		assert.Less(t, duration, time.Second, "Wide dependency tree hashing should complete within reasonable time")
	})
}

func TestUpdateLoadChains_Integration(t *testing.T) {
	t.Run("simple_linear_dependency_chain", func(t *testing.T) {
		db := createTestLoad("db", "node1", "container", map[string]any{"image": "postgres"})
		api := createTestLoad("api", "node1", "container", map[string]any{"image": "golang"})
		web := createTestLoad("web", "node1", "container", map[string]any{"image": "nginx"})

		api.DependsOn = []*Load{db}
		web.DependsOn = []*Load{api}

		loads := []*Load{db, api, web}
		updateLoadChains(loads)

		assert.Equal(t, 1, len(db.StartChain), "DB should have 1 item in start chain (itself)")
		if len(db.StartChain) > 0 {
			assert.Equal(t, "db", db.StartChain[0].Name)
		}

		assert.Equal(t, 2, len(api.StartChain), "API should have 2 items in start chain")
		if len(api.StartChain) >= 2 {
			assert.Equal(t, "db", api.StartChain[0].Name)
			assert.Equal(t, "api", api.StartChain[1].Name)
		}

		assert.Equal(t, 3, len(web.StartChain), "Web should have 3 items in start chain")
		if len(web.StartChain) >= 3 {
			assert.Equal(t, "db", web.StartChain[0].Name)
			assert.Equal(t, "api", web.StartChain[1].Name)
			assert.Equal(t, "web", web.StartChain[2].Name)
		}
	})

	t.Run("complex_dependency_graph", func(t *testing.T) {
		db := createTestLoad("db", "node1", "container", map[string]any{"image": "postgres"})
		cache := createTestLoad("cache", "node1", "container", map[string]any{"image": "redis"})
		api1 := createTestLoad("api1", "node1", "container", map[string]any{"image": "golang"})
		api2 := createTestLoad("api2", "node1", "container", map[string]any{"image": "python"})
		web := createTestLoad("web", "node1", "container", map[string]any{"image": "nginx"})

		api1.DependsOn = []*Load{db, cache}
		api2.DependsOn = []*Load{db}
		web.DependsOn = []*Load{api1, api2}

		loads := []*Load{db, cache, api1, api2, web}
		updateLoadChains(loads)

		assert.Equal(t, 1, len(db.StartChain), "DB should have 1 item in start chain")
		assert.Equal(t, 1, len(cache.StartChain), "Cache should have 1 item in start chain")

		// Dependencies come first, visited in name order
		assert.Equal(t, []string{"cache", "db", "api1"}, chainNames(api1.StartChain))
		assert.Equal(t, []string{"db", "api2"}, chainNames(api2.StartChain))
		assert.Equal(t, []string{"cache", "db", "api1", "api2", "web"}, chainNames(web.StartChain))
		assert.Equal(t, []string{"web", "api2", "api1", "db", "cache"}, chainNames(web.StopChain))
	})

	t.Run("dependency_change_invalidates_chains", func(t *testing.T) {
		db := createTestLoad("db", "node1", "container", map[string]any{"image": "postgres"})
		api := createTestLoad("api", "node1", "container", map[string]any{"image": "golang"})
		web := createTestLoad("web", "node1", "container", map[string]any{"image": "nginx"})

		api.DependsOn = []*Load{db}
		web.DependsOn = []*Load{api}

		updateLoadChains([]*Load{db, api, web})
		originalHash := web.StartChain.Hash()

		api.Name = "api2"
		web.DependsOn = []*Load{api}

		updateLoadChains([]*Load{db, api, web})
		newHash := web.StartChain.Hash()

		assert.NotEqual(t, originalHash, newHash, "Changing dependency config should invalidate chain hash")
	})
}

func chainNames(chain LoadChain) []string {
	names := make([]string, len(chain))
	for i, l := range chain {
		names[i] = l.Name
	}
	return names
}

func TestLoad_UpdateLoadChainsOrder(t *testing.T) {
	db := createTestLoad("db", "node1", "container", map[string]any{"image": "postgres"})
	cache := createTestLoad("cache", "node1", "container", map[string]any{"image": "redis"})
	api := createTestLoad("api", "node1", "container", map[string]any{"image": "golang"})
	web := createTestLoad("web", "node1", "container", map[string]any{"image": "nginx"})

	// db is reached both directly from api and through cache
	cache.DependsOn = []*Load{db}
	api.DependsOn = []*Load{db, cache}
	web.DependsOn = []*Load{api}

	updateLoadChains([]*Load{db, cache, api, web})

	assert.Equal(t, []string{"db"}, chainNames(db.StartChain))
	assert.Equal(t, []string{"db", "cache"}, chainNames(cache.StartChain))
	assert.Equal(t, []string{"db", "cache", "api", "web"}, chainNames(web.StartChain))
	assert.Equal(t, []string{"web", "api", "cache", "db"}, chainNames(web.StopChain))
}

func TestLoad_UpdateLoadChainsDeterministic(t *testing.T) {
	loads := make([]*Load, 0, 6)
	root := createTestLoad("root", "node1", "container", map[string]any{"image": "nginx"})
	for i := range 5 {
		dep := createTestLoad(fmt.Sprintf("dep%d", i), "node1", "container", map[string]any{"image": "redis"})
		root.DependsOn = append(root.DependsOn, dep)
		loads = append(loads, dep)
	}
	loads = append(loads, root)

	updateLoadChains(loads)
	startHash, stopHash := root.StartChain.Hash(), root.StopChain.Hash()
	for range 20 {
		updateLoadChains(loads)
		assert.Equal(t, startHash, root.StartChain.Hash())
		assert.Equal(t, stopHash, root.StopChain.Hash())
	}
}

func TestLoad_UnmarshalJSONKeepsReferenceNames(t *testing.T) {
	driverID := LoadDriverID("mock-unmarshal")
	assert.NoError(t, RegisterLoadDriver(&MockLoadDriver{driver: driverID}))
	t.Cleanup(func() { _ = UnregisterLoadDriver(driverID) })

	db := createTestLoad("db", "node1", driverID, nil)
	api := createTestLoad("api", "node1", driverID, nil)
	api.DependsOn = []*Load{db}
	updateLoadChains([]*Load{db, api})

	data, err := json.Marshal(api)
	assert.NoError(t, err)

	var decoded Load
	assert.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "api", decoded.Name)
	assert.Equal(t, "node1", decoded.Node.Name)
	assert.Equal(t, []string{"db"}, decoded.GetDependencies())
	assert.Equal(t, api.Hash(), decoded.Hash())

	t.Run("resolve_references", func(t *testing.T) {
		node := &Node{Name: "node1", Url: "http://node1"}
		err := decoded.ResolveReferences(map[string]*Node{"node1": node}, map[string]*Load{"db": db, "api": api})
		assert.NoError(t, err)
		assert.Same(t, node, decoded.Node)
		assert.Same(t, db, decoded.DependsOn[0])
	})

	t.Run("unknown_dependency", func(t *testing.T) {
		var orphan Load
		assert.NoError(t, json.Unmarshal(data, &orphan))
		err := orphan.ResolveReferences(map[string]*Node{"node1": {Name: "node1"}}, map[string]*Load{})
		assert.ErrorContains(t, err, "load 'db' referenced as dependency by load 'api' does not exist")
	})

	t.Run("unknown_node", func(t *testing.T) {
		var orphan Load
		assert.NoError(t, json.Unmarshal(data, &orphan))
		err := orphan.ResolveReferences(map[string]*Node{}, map[string]*Load{"db": db})
		assert.ErrorContains(t, err, "node 'node1' referenced by load 'api' does not exist")
	})
}
