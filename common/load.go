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

type HashableLoadConfig struct {
	Name         string       `json:"name"`
	Node         string       `json:"node"`
	DependsOn    []string     `json:"depends_on"`
	Driver       LoadDriverID `json:"driver"`
	DriverConfig any          `json:"driver_config"`
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

func (l *Load) MarshalJSON() ([]byte, error) {
	var nodeName string
	if l.Node != nil {
		nodeName = l.Node.Name
	}

	startChainHash := l.StartChain.Hash()
	stopChainHash := l.StopChain.Hash()

	return json.Marshal(&LoadConfig{
		Name:           l.Name,
		Node:           nodeName,
		Driver:         l.Driver.ID(),
		DriverConfig:   l.Driver.Config().DriverConfig,
		DependsOn:      l.GetDependencies(),
		StartChainHash: base64.StdEncoding.EncodeToString(startChainHash[:]),
		StopChainHash:  base64.StdEncoding.EncodeToString(stopChainHash[:]),
	})
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

func (l *Load) Hash() [32]byte {
	data, err := json.Marshal(HashableLoadConfig{
		Name:         l.Name,
		Node:         l.Node.Name,
		Driver:       l.Driver.ID(),
		DriverConfig: l.Driver.Config().DriverConfig,
		DependsOn:    l.GetDependencies(),
	})
	if err != nil {
		panic(err)
	}
	return sha256.Sum256(data)
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
		hashes = append(hashes, l.Hash())
	}

	var combined []byte
	for _, h := range hashes {
		combined = append(combined, h[:]...)
	}

	return sha256.Sum256(combined)
}
