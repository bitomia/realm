package client

import (
	"fmt"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
)

func GetNodes(cfg *config.Config) (map[string]*common.NodeConfig, error) {
	nodes := make(map[string]*common.NodeConfig)
	seenUrls := make(map[string]string)

	for name, node := range cfg.Nodes {
		agentURL, err := common.ResolveAgentURL(node)
		if err != nil {
			return nil, err
		}
		if existingName, exists := seenUrls[agentURL]; exists {
			return nil, fmt.Errorf("Duplicated URL %s for nodes %s and %s ", node.Url, existingName, node.Name)
		}
		node.Name = name
		nodes[node.Name] = node
		seenUrls[agentURL] = node.Name
	}

	return nodes, nil
}

func GetNode(cfg *config.Config, nodeName string) (*common.NodeConfig, error) {
	nodes, err := GetNodes(cfg)
	if err != nil {
		return nil, err
	}

	node, exists := nodes[nodeName]
	if !exists {
		return nil, fmt.Errorf("Node %s not found", nodeName)
	}

	return node, nil
}
