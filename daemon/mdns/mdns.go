package mdns

import (
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/grandcat/zeroconf"

	"github.com/bitomia/realm/internal/config"
)

type MDNSService struct {
	server *zeroconf.Server
	mu     sync.Mutex
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
	m.mu.Lock()
	defer m.mu.Unlock()

	slog.Info("Starting mDNS service")

	port := config.Get().Daemon.ListenPort

	hostname, err := os.Hostname()
	if err != nil {
		slog.Error("Error getting hostname", "error", err.Error())
		return fmt.Errorf("failed to start mDNS server: cannot get hostname")
	}

	server, err := zeroconf.Register(hostname, "_realm._tcp", "local.", port, []string{"txtv=0"}, nil)
	if err != nil {
		return fmt.Errorf("failed to start mDNS server: %w", err)
	}

	m.server = server

	slog.Info("mDNS service started", "hostname", hostname, "port", port)

	return nil
}

func (m *MDNSService) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	slog.Info("Stopping mDNS service")

	if m.server != nil {
		m.server.Shutdown()
		m.server = nil
	}
}
