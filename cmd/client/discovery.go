package client

import (
	"fmt"
	"net"
	"strings"

	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/internal"
)

func GetNodes() map[string]*common.NodeConfig {
	nodes := make(map[string]*common.NodeConfig)
	seenUrls := make(map[string]string)

	for name, node := range config.Get().Nodes {
		if existingName, exists := seenUrls[node.Url]; exists {
			log.Warn("Duplicate URL detected: %s (replacing node '%s' with '%s')\n", node.Url, existingName, node.Name)
			delete(nodes, existingName)
		}
		node.Name = name
		nodes[node.Name] = node
		seenUrls[node.Url] = node.Name
	}

	if config.Get().Discovery.MdnsEnabled {
		services, err := internal.QueryServices("_realm._tcp.local")
		if err != nil {
			log.Warn("mDNS discovery failed: %v", err)
		} else {
			addDiscoveredServices(services, nodes, seenUrls)
		}
	}

	return nodes
}

func addDiscoveredServices(services map[string]*internal.ServiceInfo, nodes map[string]*common.NodeConfig, seenUrls map[string]string) {
	for _, service := range services {
		if service.Hostname == "" || service.Port == 0 || len(service.IPs) == 0 {
			continue
		}

		for _, serviceIPStr := range service.IPs {
			serviceIP := net.ParseIP(serviceIPStr)
			if serviceIP == nil || serviceIP.To4() == nil {
				continue
			}

			serviceNameParts := strings.Split(service.Name, ".")
			if len(serviceNameParts) == 0 {
				continue
			}

			name := serviceNameParts[0]
			url := fmt.Sprintf("http://%s:%d", service.IPs[0], service.Port)
			if existingName, exists := seenUrls[url]; exists {
				log.Warn("Duplicate URL detected: %s (replacing node '%s' with '%s')\n", url, existingName, name)
				delete(nodes, existingName)
			}

			nodes[name] = &common.NodeConfig{Name: name, Url: url}
			seenUrls[url] = name
		}
	}
}

func GetNode(nodeName string) *common.NodeConfig {
	nodes := GetNodes()
	node, exists := nodes[nodeName]
	if !exists {
		log.Fatal("Node %s not found", nodeName)
	}
	return node
}
