package config

import (
	"fmt"
	"slices"

	"github.com/bitomia/realm/common"
)

var (
	nodesConfig map[string]*common.Node = make(map[string]*common.Node)
)

func ResetNodesConfig() {
	nodesConfig = make(map[string]*common.Node)
}

func newNodeConfig(nodeName string, node *common.NodeConfig, driver common.NodeDriver) (*common.Node, error) {
	if _, exists := nodesConfig[nodeName]; exists {
		return nil, fmt.Errorf("node '%s' not unique", nodeName)
	}
	nodesConfig[nodeName] = &common.Node{Name: nodeName, CloudInit: node.CloudInit, Url: node.Url, Driver: driver}
	return nodesConfig[nodeName], nil
}

func GetNodesFromConfig(nodesFilter ...string) map[string]*common.Node {
	nodes := make(map[string]*common.Node)
	for _, node := range nodesConfig {
		if len(nodesFilter) == 0 || slices.Contains(nodesFilter, node.Name) {
			nodes[node.Name] = node
		}
	}
	return nodes
}
