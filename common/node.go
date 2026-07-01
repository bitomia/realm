package common

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/bitomia/realm/common/cloudinit"
)

type NodeConfig struct {
	Name         string               `json:"name"`
	Url          string               `json:"url"`
	CloudInit    *cloudinit.CloudInit `json:"cloud_init,omitempty"`
	Driver       NodeDriverID         `json:"driver,omitempty"`
	DriverConfig *any                 `json:"driver_config,omitempty"`
}

type Node struct {
	Name      string
	Url       string
	CloudInit *cloudinit.CloudInit
	Driver    NodeDriver
}

func newNodeFromConfig(ctx NodeContext, config *NodeConfig) (*Node, error) {
	if config == nil {
		return nil, fmt.Errorf("nil config")
	}

	driver, err := BuildNodeDriver(ctx, NodeDriverConfig{Driver: config.Driver, DriverConfig: config.DriverConfig})
	if err != nil {
		return nil, err
	}

	agentURL, err := ResolveAgentURL(config)
	if err != nil {
		return nil, err
	}

	var node Node
	node.Name = config.Name
	node.Url = agentURL
	node.CloudInit = config.CloudInit
	node.Driver = driver

	return &node, nil
}

func (n *Node) MarshalJSON() ([]byte, error) {
	return json.Marshal(NodeConfig{
		Name:         n.Name,
		Url:          n.Url,
		CloudInit:    n.CloudInit,
		Driver:       n.Driver.ID(),
		DriverConfig: n.Driver.Config().DriverConfig,
	})
}

func (n *Node) UnmarshalJSON(data []byte) error {
	config := NodeConfig{}
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if naux, err := newNodeFromConfig(NodeContext{}, &config); err != nil {
		return err
	} else {
		n.Name = naux.Name
		n.Url = naux.Url
		n.CloudInit = naux.CloudInit
		n.Driver = naux.Driver
		return nil
	}
}

func (n *Node) Hash() [32]byte {
	data, err := json.Marshal(n)
	if err != nil {
		panic(err)
	}

	return sha256.Sum256(data)
}
