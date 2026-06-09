package common

import "fmt"

func ResolveAgentURL(nodeConfig *NodeConfig) (string, error) {
	if nodeConfig == nil {
		return "", fmt.Errorf("nil nodeConfig")
	}

	if nodeConfig.Url == nil {
		return "", fmt.Errorf("'url' attribute not found for node '%s'", nodeConfig.Name)
	}

	// TODO add support for custom protocols
	return *nodeConfig.Url, nil
}
