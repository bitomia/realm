package common

import "fmt"

func ResolveAgentURL(nodeConfig *NodeConfig) (string, error) {
	if nodeConfig == nil {
		return "", fmt.Errorf("nil nodeConfig")
	}

	if nodeConfig.Url == nil {
		// Use mDNS when Url is nil
		return fmt.Sprintf("http://%s.local:9000", nodeConfig.Name), nil
	}

	// TODO add support for custom protocols
	return *nodeConfig.Url, nil
}
