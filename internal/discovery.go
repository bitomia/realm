package internal

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/bitomia/realm/internal/config"
)

func GetDaemonAddresses() map[string]config.Daemon {
	daemons := make(map[string]config.Daemon)
	seenUrls := make(map[string]string)

	for _, daemon := range config.Get().Discovery.Daemons {
		if existingName, exists := seenUrls[daemon.Url]; exists {
			log.Printf("Duplicate URL detected: %s (replacing daemon '%s' with '%s')\n", daemon.Url, existingName, daemon.Name)
			delete(daemons, existingName)
		}
		daemons[daemon.Name] = daemon
		seenUrls[daemon.Url] = daemon.Name
	}

	services, _ := QueryServices("_realm._tcp.local")
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
				log.Printf("Duplicate URL detected: %s (replacing daemon '%s' with '%s')\n", url, existingName, name)
				delete(daemons, existingName)
			}

			daemons[name] = config.Daemon{Name: name, Url: url}
			seenUrls[url] = name
		}
	}

	return daemons
}
