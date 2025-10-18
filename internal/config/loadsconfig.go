package config

import (
	"crypto/sha256"
	"fmt"

	"github.com/bitomia/realm/internal"
)

type LoadsConfig struct {
	Containers map[string]ContainerConfig `mapstructure:"containers"`
	Processes  map[string]ProcessConfig   `mapstructure:"processes"`

	loads map[string]*Load
}

func (l *LoadsConfig) newLoad(name string, node *Node, driver internal.LoadDriver) (*Load, error) {
	if l.loads == nil {
		l.loads = make(map[string]*Load)
	}

	if _, exists := l.loads[name]; exists {
		return nil, fmt.Errorf("Node name not unique")
	}
	l.loads[name] = &Load{Name: name, Node: node, Driver: driver}
	return l.loads[name], nil
}

func (l *LoadsConfig) GetLoads() []*Load {
	var loads []*Load
	for _, load := range l.loads {
		loads = append(loads, load)
	}
	return loads
}

func (l *LoadsConfig) Hash() [32]byte {
	var hashes [][32]byte
	for n, l := range l.loads {
		fmt.Printf("%s %v\n", n, l.Hash())
		hashes = append(hashes, l.Hash())
	}

	var combined []byte
	for _, h := range hashes {
		combined = append(combined, h[:]...)
	}

	return sha256.Sum256(combined)
}
