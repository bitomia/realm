package common

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/dominikbraun/graph"
)

type Hash [32]byte
type LoadChain []*Load

type LoadConfig struct {
	Name           string       `json:"name"`
	Node           string       `json:"node"`
	DependsOn      []string     `json:"depends_on"`
	Driver         LoadDriverID `json:"driver"`
	DriverConfig   any          `json:"driver_config"`
	StartChainHash string       `json:"start_chain_hash"`
	StopChainHash  string       `json:"stop_chain_hash"`
}

// Hash returns a hash of the config which does not depend on how its driver
// config is held: a typed struct and the map decoded from its JSON hash the same
func (c LoadConfig) Hash() Hash {
	data, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	// Round trip through a generic value so map keys are sorted on the final marshal
	var canonical any
	if err := json.Unmarshal(data, &canonical); err != nil {
		panic(err)
	}
	if data, err = json.Marshal(canonical); err != nil {
		panic(err)
	}
	return sha256.Sum256(data)
}

type Load struct {
	Name       string
	Node       *Node
	DependsOn  []*Load
	Driver     LoadDriver
	StartChain LoadChain
	StopChain  LoadChain
}

func (l *Load) GetDependencies() []string {
	dependsOn := make([]string, len(l.DependsOn))
	for i, dep := range l.DependsOn {
		dependsOn[i] = dep.Name
	}
	return dependsOn
}

func (l *Load) baseConfig() LoadConfig {
	var nodeName string
	if l.Node != nil {
		nodeName = l.Node.Name
	}

	return LoadConfig{
		Name:         l.Name,
		Node:         nodeName,
		Driver:       l.Driver.ID(),
		DriverConfig: l.Driver.Config().DriverConfig,
		DependsOn:    l.GetDependencies(),
	}
}

func (l *Load) GetConfig() LoadConfig {
	cfg := l.baseConfig()

	startChainHash := l.StartChain.Hash()
	stopChainHash := l.StopChain.Hash()
	cfg.StartChainHash = base64.StdEncoding.EncodeToString(startChainHash[:])
	cfg.StopChainHash = base64.StdEncoding.EncodeToString(stopChainHash[:])

	return cfg
}

func (l *Load) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.GetConfig())
}

func (l *Load) UnmarshalJSON(data []byte) error {
	aux := LoadConfig{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	driver, err := BuildLoadDriver(LoadDriverConfig{Driver: aux.Driver, DriverConfig: aux.DriverConfig})
	if err != nil {
		return err
	}
	l.Name = aux.Name
	l.Driver = driver

	// Node and DependsOn hold placeholders with only their names
	// because actual references require the full configuration
	l.Node = nil
	if aux.Node != "" {
		l.Node = &Node{Name: aux.Node}
	}
	l.DependsOn = make([]*Load, len(aux.DependsOn))
	for i, dep := range aux.DependsOn {
		l.DependsOn[i] = &Load{Name: dep}
	}

	return nil
}

// ResolveReferences replaces the Node and DependsOn entries of the load with
// the nodes and loads of the same name
func (l *Load) ResolveReferences(nodes map[string]*Node, loads map[string]*Load) error {
	if l.Node != nil {
		node, exists := nodes[l.Node.Name]
		if !exists {
			return fmt.Errorf("node '%s' referenced by load '%s' does not exist", l.Node.Name, l.Name)
		}
		l.Node = node
	}

	for i, dep := range l.DependsOn {
		load, exists := loads[dep.Name]
		if !exists {
			return fmt.Errorf("load '%s' referenced as dependency by load '%s' does not exist", dep.Name, l.Name)
		}
		l.DependsOn[i] = load
	}

	return nil
}

func (l *Load) UpdateLoadChains(configGraph graph.Graph[string, string], loadsMap map[string]*Load) error {
	adjacencyMap, err := configGraph.AdjacencyMap()
	if err != nil {
		return fmt.Errorf("failed to build chains for load %s: %w", l.Name, err)
	}
	if _, exists := adjacencyMap[l.Name]; !exists {
		return fmt.Errorf("failed to build chains for load %s: load not in graph", l.Name)
	}

	// Build start chain with a post-order DFS
	l.StartChain = LoadChain{}
	visited := make(map[string]bool)
	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		// Dependencies are visited sorted by name to keep chains and hashes deterministic
		for _, dep := range slices.Sorted(maps.Keys(adjacencyMap[name])) {
			visit(dep)
		}
		if load, exists := loadsMap[name]; exists {
			l.StartChain = append(l.StartChain, load)
		}
	}
	visit(l.Name)

	// Dependents shall be stopped before their dependencies
	l.StopChain = slices.Clone(l.StartChain)
	slices.Reverse(l.StopChain)

	return nil
}

// Hash loads in order
// Order is important that's why it receives an array of loads
func (chain LoadChain) Hash() Hash {
	var hashes [][32]byte
	for _, l := range chain {
		hashes = append(hashes, l.baseConfig().Hash())
	}

	var combined []byte
	for _, h := range hashes {
		combined = append(combined, h[:]...)
	}

	return sha256.Sum256(combined)
}
