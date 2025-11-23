package loads

import (
	"crypto/sha256"
	"encoding/json"

	"github.com/bitomia/realm/internal/node"
)

type LoadConfig struct {
	Name         string
	Node         string                 `mapstructure:"node"`
	DependsOn    []string               `mapstructure:"depends_on"`
	Driver       LoadDriverID           `mapstructure:"driver"`
	DriverConfig map[string]interface{} `mapstructure:"driver_config"`
}

type Load struct {
	Name      string
	Driver    LoadDriver
	DependsOn []*Load
	Node      *node.Node
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

	var driverConfig map[string]interface{}
	{
		driverConfigJson, err := json.Marshal(l.Driver)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(driverConfigJson, &driverConfig); err != nil {
			return nil, err
		}
	}

	return json.Marshal(&LoadConfig{
		Name:         l.Name,
		Driver:       l.Driver.GetLoadDriverID(),
		DriverConfig: driverConfig,
		DependsOn:    dependsOn,
		Node:         nodeName,
	})
}

func (l *Load) UnmarshalJSON(data []byte) error {
	aux := LoadConfig{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	driver, err := BuildLoadDriver(aux)
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
