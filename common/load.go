package common

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"

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
	Node       *NodeConfig
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
		Driver:         l.Driver.GetLoadDriverID(),
		DriverConfig:   l.Driver.GetDriverConfig().DriverConfig,
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

	// TODO
	// DependsOn and Node references need to be resolved externally
	// as they require access to the full configuration context

	return nil
}

func (l *Load) Hash() [32]byte {
	data, err := json.Marshal(HashableLoadConfig{
		Name:         l.Name,
		Node:         l.Node.Name,
		Driver:       l.Driver.GetLoadDriverID(),
		DriverConfig: l.Driver.GetDriverConfig().DriverConfig,
		DependsOn:    l.GetDependencies(),
	})
	if err != nil {
		panic(err)
	}
	return sha256.Sum256(data)
}

func (l *Load) UpdateLoadChains(configGraph graph.Graph[string, string], loadsMap map[string]*Load) {
	l.StartChain = LoadChain{}
	graph.DFS(configGraph, l.Name, func(value string) bool {
		if load, exists := loadsMap[value]; exists {
			l.StartChain = append(l.StartChain, load)
		}
		return false
	})

	l.StopChain = LoadChain{}
	graph.DFS(configGraph, l.Name, func(value string) bool {
		if load, exists := loadsMap[value]; exists {
			l.StopChain = append([]*Load{load}, l.StopChain...)
		}
		return false
	})
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
