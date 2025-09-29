package internal

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/bitomia/realm/internal"
	"github.com/bitomia/realm/internal/config"

	"github.com/bitomia/realm/cmd/log"
)

func GetNodes() map[string]config.Node {
	nodes := make(map[string]config.Node)
	seenUrls := make(map[url.URL]string)

	for _, node := range config.Get().Nodes {
		if existingName, exists := seenUrls[node.Url]; exists {
			log.Warn("Duplicate URL detected: %s (replacing node '%s' with '%s')\n", node.Url.String(), existingName, node.Name)
			delete(nodes, existingName)
		}
		nodes[node.Name] = node
		seenUrls[node.Url] = node.Name
	}

	services, _ := internal.QueryServices("_realm._tcp.local")
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
			urlStr := fmt.Sprintf("http://%s:%d", service.IPs[0], service.Port)
			url, err := url.Parse(urlStr)
			if err != nil {
				log.Warn("Invalid node URL %s", urlStr)
			}
			if existingName, exists := seenUrls[*url]; exists {
				log.Warn("Duplicate URL detected: %s (replacing node '%s' with '%s')\n", url, existingName, name)
				delete(nodes, existingName)
			}

			nodes[name] = config.Node{Name: name, Url: *url}
			seenUrls[*url] = name
		}
	}

	return nodes
}

func GetNode(nodeName string) config.Node {
	nodes := GetNodes()
	node, exists := nodes[nodeName]
	if !exists {
		log.Fatal("Node %s not found", nodeName)
	}
	return node
}
