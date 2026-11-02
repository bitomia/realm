package common

import (
	"encoding/json"
	"fmt"

	"github.com/bitomia/realm/common/cloudinit"
)

type NodeConfig struct {
	Name         string               `json:"name"`
	Url          string               `json:"url"`
	Timeout      TimeoutConfig        `json:"timeout,omitempty"`
	CloudInit    *cloudinit.CloudInit `json:"cloud_init,omitempty"`
	Driver       NodeDriverID         `json:"driver,omitempty"`
	DriverConfig *any                 `json:"driver_config,omitempty"`
	Registries   []RegistryConfig     `json:"registries,omitempty"`
	Loads        []LoadConfig         `json:"loads,omitempty"`
}

type Node struct {
	Name       string
	Url        string
	Timeout    TimeoutConfig
	CloudInit  *cloudinit.CloudInit
	Registries []RegistryConfig
	Driver     NodeDriver
	Loads      []LoadConfig
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
	node.Timeout = config.Timeout.WithDefaults()
	node.CloudInit = config.CloudInit
	node.Registries = config.Registries
	node.Driver = driver
	node.Loads = config.Loads

	return &node, nil
}

func (n *Node) MarshalJSON() ([]byte, error) {
	return json.Marshal(NodeConfig{
		Name:         n.Name,
		Url:          n.Url,
		Timeout:      n.Timeout,
		CloudInit:    n.CloudInit,
		Registries:   n.Registries,
		Driver:       n.Driver.ID(),
		DriverConfig: n.Driver.Config().DriverConfig,
		Loads:        n.Loads,
	})
}

func (n *Node) UnmarshalJSON(data []byte) error {
	nodeConfig := NodeConfig{}
	if err := json.Unmarshal(data, &nodeConfig); err != nil {
		return err
	}

	// Check loads belong to this node
	for _, load := range nodeConfig.Loads {
		if load.Node != nodeConfig.Name {
			return fmt.Errorf("load '%s' belongs to node '%s', not to '%s'", load.Name, load.Node, nodeConfig.Name)
		}
	}

	if naux, err := newNodeFromConfig(NewNodeContext(nodeConfig.Name), &nodeConfig); err != nil {
		return err
	} else {
		n.Name = naux.Name
		n.Url = naux.Url
		n.Timeout = naux.Timeout
		n.CloudInit = naux.CloudInit
		n.Registries = naux.Registries
		n.Driver = naux.Driver
		n.Loads = naux.Loads

		return nil
	}
}
