package common

import (
	"crypto/sha256"
	"encoding/json"
)

type NodeConfig struct {
	Name         string       `json:"name"`
	Url          string       `json:"url"`
	Driver       NodeDriverID `json:"driver,omitempty"`
	DriverConfig *any         `json:"driver_config,omitempty"`
}

type Node struct {
	Name   string
	Url    string
	Driver NodeDriver
}

func (n *Node) MarshalJSON() ([]byte, error) {
	return json.Marshal(NodeConfig{
		Name:         n.Name,
		Url:          n.Url,
		Driver:       n.Driver.GetNodeDriverID(),
		DriverConfig: n.Driver.GetDriverConfig().DriverConfig,
	})
}

func (n *Node) UnmarshalJSON(data []byte) error {
	aux := NodeConfig{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if driver, err := BuildNodeDriver(NodeDriverConfig{Driver: aux.Driver, DriverConfig: aux.DriverConfig}); err != nil {
		return err
	} else {
		n.Driver = driver
	}

	return nil
}

func (n *Node) Hash() [32]byte {
	data, err := json.Marshal(n)
	if err != nil {
		panic(err)
	}

	return sha256.Sum256(data)
}
