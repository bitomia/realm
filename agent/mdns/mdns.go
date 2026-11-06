package mdns

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"

	"github.com/grandcat/zeroconf"

	"github.com/bitomia/realm/agent/config"
)

const (
	serviceType = "_realm._tcp"
	serviceTTL  = 255
)

var serviceTXT = []string{"txtv=0"}

type MDNSService struct {
	avahi  *avahiPublisher
	server *zeroconf.Server
	mutex  sync.Mutex
}

var (
	instance *MDNSService
	once     sync.Once
)

func GetMDNSService() *MDNSService {
	once.Do(func() {
		instance = &MDNSService{}
	})
	return instance
}

func (m *MDNSService) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Without the TCP listener there is no endpoint to advertise.
	if config.Get().Agent.DisableTCP {
		slog.Info("Skipping mDNS service, TCP listener is disabled")
		return nil
	}

	slog.Info("Starting mDNS service")

	port := config.Get().Agent.ListenPort
	networkConfig := config.Get().NetworkConfig
	var networkInterfaces []net.Interface
	if networkConfig.IPAddress != nil {
		networkInterfaces = append(networkInterfaces, *networkConfig.Iface)

	}
	if config.Get().MeshConfig != nil {
		meshIface, err := net.InterfaceByName("netplane0")
		if err != nil {
			slog.Warn("Mesh interface not available, not advertising mDNS on it", "iface", "netplane0", "error", err)
		} else {
			networkInterfaces = append(networkInterfaces, *meshIface)
		}
	}
	slog.Info("mDNS network configuration", "ifaces", networkInterfaces)

	hostname, err := os.Hostname()
	if err != nil {
		slog.Error("Error getting hostname", "error", err.Error())
		return fmt.Errorf("failed to start mDNS server: cannot get hostname")
	}

	publisher, err := startAvahiPublisher(hostname, serviceType, port, serviceTXT, networkInterfaces)
	if err == nil {
		m.avahi = publisher
		slog.Info("mDNS service started with Avahi", "hostname", hostname, "port", port)
		return nil
	}
	slog.Info("Avahi not available, using the built-in mDNS responder", "reason", err.Error())

	server, err := zeroconf.Register(hostname, serviceType, "local.", port, serviceTXT, networkInterfaces)
	if err != nil {
		return fmt.Errorf("failed to start mDNS server: %w", err)
	}
	server.TTL(serviceTTL)

	m.server = server

	slog.Info("mDNS service started", "hostname", hostname, "port", port)

	return nil
}

func (m *MDNSService) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	slog.Info("Stopping mDNS service")

	if m.avahi != nil {
		m.avahi.Stop()
		m.avahi = nil
	}
	if m.server != nil {
		m.server.Shutdown()
		m.server = nil
	}
}
