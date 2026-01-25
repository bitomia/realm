package mdns

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"

	"github.com/grandcat/zeroconf"

	"github.com/bitomia/realm/common/config"
)

type MDNSService struct {
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

	slog.Info("Starting mDNS service")

	port := config.Get().Daemon.ListenPort
	networkConfig := config.Get().NetworkConfig
	var networkInterfaces []net.Interface
	if networkConfig.IPAddress != nil {
		networkInterfaces = append(networkInterfaces, *networkConfig.Iface)

	}
	slog.Info("mDNS network configuration", "ifaces", networkInterfaces)

	hostname, err := os.Hostname()
	if err != nil {
		slog.Error("Error getting hostname", "error", err.Error())
		return fmt.Errorf("failed to start mDNS server: cannot get hostname")
	}

	server, err := zeroconf.Register(hostname, "_realm._tcp", "local.", port, nil, networkInterfaces)
	if err != nil {
		return fmt.Errorf("failed to start mDNS server: %w", err)
	}
	server.TTL(255)

	m.server = server

	slog.Info("mDNS service started", "hostname", hostname, "port", port)

	return nil
}

func (m *MDNSService) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	slog.Info("Stopping mDNS service")

	if m.server != nil {
		m.server.Shutdown()
		m.server = nil
	}
}
