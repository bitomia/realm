package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/bitomia/realm/internal"
)

type Load struct {
	Name      string
	Driver    internal.LoadDriver
	DependsOn []*Load
	Node      *Node
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
	return json.Marshal(&struct {
		Name       string              `json:"name"`
		DriverType string              `json:"driver_type"`
		Driver     internal.LoadDriver `json:"driver"`
		DependsOn  []string            `json:"depends_on"`
		Node       string              `json:"node"`
	}{
		Name:       l.Name,
		DriverType: l.Driver.GetDriverType().String(),
		Driver:     l.Driver,
		DependsOn:  dependsOn,
		Node:       nodeName,
	})
}

func (l *Load) UnmarshalJSON(data []byte) error {
	aux := &struct {
		Name       string          `json:"name"`
		DriverType string          `json:"driver_type"`
		Driver     json.RawMessage `json:"driver"`
		DependsOn  []string        `json:"depends_on"`
		Node       string          `json:"node"`
	}{}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	l.Name = aux.Name

	var driver internal.LoadDriver
	switch aux.DriverType {
	case internal.ProcessDriverTypeStr:
		d := &ProcessDriver{}
		if err := json.Unmarshal(aux.Driver, d); err != nil {
			return err
		}
		driver = d
	case internal.ContainerDriverTypeStr:
		d := &ContainerDriver{}
		if err := json.Unmarshal(aux.Driver, d); err != nil {
			return err
		}
		driver = d
	default:
		return fmt.Errorf("unknown driver type: %s", aux.DriverType)
	}
	l.Driver = driver

	// TODO
	// DependsOn and Node references need to be resolved externally
	// as they require access to the full configuration context

	return nil
}

func (l *Load) Hash() [32]byte {
	data, err := json.Marshal(l)
	fmt.Printf("%s\n", data)
	if err != nil {
		panic(err)
	}
	return sha256.Sum256(data)
}
