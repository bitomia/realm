package config

import (
	"crypto/sha256"
	"fmt"

	"gopkg.in/yaml.v3"
)

type ContainerConfig struct {
	Name      string
	Node      string   `mapstructure:"node"`
	DependsOn []string `mapstructure:"depends_on"`
	Image     string   `mapstructure:"image"`
}

type ProcessConfig struct {
	Name       string
	Node       string   `mapstructure:"node"`
	DependsOn  []string `mapstructure:"depends_on"`
	StartCmd   string   `mapstructure:"start_cmd"`
	StopSignal string   `mapstructure:"stop_signal"`
}

type LoadsConfig struct {
	Containers map[string]ContainerConfig `mapstructure:"containers"`
	Processes  map[string]ProcessConfig   `mapstructure:"processes"`

	loads map[string]*Load
}

func (l *LoadsConfig) newLoadNode(name string, node *Node, driver LoadDriver) (*Load, error) {
	if l.loads == nil {
		l.loads = make(map[string]*Load)
	}

	if _, exists := l.loads[name]; exists {
		return nil, fmt.Errorf("Node name not unique")
	}

	l.loads[name] = &Load{name: name, node: node, driver: driver}

	return l.loads[name], nil
}

func (l *LoadsConfig) Hash() [32]byte {
	var hashes [][32]byte
	for _, n := range l.loads {
		hashes = append(hashes, n.Hash())
	}

	var combined []byte
	for _, h := range hashes {
		combined = append(combined, h[:]...)
	}

	return sha256.Sum256(combined)
}

func (l *LoadsConfig) UnmarshalYAML(value *yaml.Node) error {
	fmt.Println("TEST!")
	return nil
}
