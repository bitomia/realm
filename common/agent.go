package common

import "fmt"

func ResolveAgentURL(nodeConfig *NodeConfig) (string, error) {
	if nodeConfig == nil {
		return "", fmt.Errorf("nil nodeConfig")
	}

	// TODO add support for custom protocols
	return nodeConfig.Url, nil
}
