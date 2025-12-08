package common

import (
	"crypto/sha256"
	"encoding/json"
)

type LoadState string

const (
	// LoadStart indicates the load should be running
	LoadStart LoadState = "start"
	// LoadStartFailed indicates the load failed to start
	LoadStartFailed LoadState = "start_failed"
	// LoadStop indicates the load should be stopped
	LoadStop LoadState = "stop"
	// LoadStopFailed indicates the load failed to stop
	LoadStopFailed LoadState = "stop_failed"
)

type LoadConfig struct {
	Name         string
	Node         string         `mapstructure:"node"`
	DependsOn    []string       `mapstructure:"depends_on"`
	Driver       LoadDriverID   `mapstructure:"driver"`
	DriverConfig map[string]any `mapstructure:"driver_config"`
}

type Load struct {
	Name      string
	Driver    LoadDriver
	DependsOn []*Load
	Node      *Node
	State     LoadState
}

func (l *Load) MarshalJSON() ([]byte, error) {
	var nodeName string
	if l.Node != nil {
		nodeName = l.Node.Name
	}

	dependsOn := make([]string, len(l.DependsOn))
	for i, dep := range l.DependsOn {
		dependsOn[i] = dep.Name
	}

	return json.Marshal(&LoadConfig{
		Name:         l.Name,
		Driver:       l.Driver.GetLoadDriverID(),
		DriverConfig: l.Driver.GetDriverConfig().DriverConfig,
		DependsOn:    dependsOn,
		Node:         nodeName,
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
	data, err := json.Marshal(l)
	if err != nil {
		panic(err)
	}
	return sha256.Sum256(data)
}
