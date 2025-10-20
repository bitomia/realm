package config

import (
	"crypto/sha256"
	"fmt"

	"github.com/bitomia/realm/internal/loads"
	"github.com/bitomia/realm/internal/loads/drivers"
	"github.com/bitomia/realm/internal/node"
)

type LoadsConfig struct {
	Containers map[string]drivers.ContainerConfig `mapstructure:"containers"`
	Processes  map[string]drivers.ProcessConfig   `mapstructure:"processes"`

	loads map[string]*loads.Load
}

func (l *LoadsConfig) newLoad(name string, node *node.Node, driver drivers.LoadDriver) (*loads.Load, error) {
	if l.loads == nil {
		l.loads = make(map[string]*loads.Load)
	}

	if _, exists := l.loads[name]; exists {
		return nil, fmt.Errorf("Node name not unique")
	}
	l.loads[name] = &loads.Load{Name: name, Node: node, Driver: driver}
	return l.loads[name], nil
}

func (l *LoadsConfig) GetLoads() map[string]*loads.Load {
	loads := make(map[string]*loads.Load)
	for _, load := range l.loads {
		loads[load.Name] = load
	}
	return loads
}

func (l *LoadsConfig) Hash() [32]byte {
	var hashes [][32]byte
	for _, l := range l.loads {
		hashes = append(hashes, l.Hash())
	}

	var combined []byte
	for _, h := range hashes {
		combined = append(combined, h[:]...)
	}

	return sha256.Sum256(combined)
}
