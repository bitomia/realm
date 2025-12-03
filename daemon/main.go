package daemon

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"

	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/daemon/auth"
	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/daemon/dns"
	"github.com/bitomia/realm/daemon/health"
	"github.com/bitomia/realm/daemon/id"
	"github.com/bitomia/realm/daemon/mdns"
	"github.com/bitomia/realm/daemon/proxy"
	"github.com/bitomia/realm/daemon/volumes"
)

func Start() {
	cfg := config.Get()

	// Configure slog handler based on log format
	var handler slog.Handler
	switch cfg.Daemon.LogFormat {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, nil)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, nil)
	default:
		slog.Warn("Invalid log format, defaulting to text", "format", cfg.Daemon.LogFormat)
		handler = slog.NewTextHandler(os.Stdout, nil)
	}
	slog.SetDefault(slog.New(handler))

	daemonId, err := id.GetDaemonId()
	if err != nil {
		slog.Error("Error getting daemon ID", "error", err)
		os.Exit(1)
	}

	slog.Info("Initializing daemon", "version", config.GetVersion(), "id", daemonId)
	slog.Debug("Daemon configuration", "config", *cfg)

	slog.Info("Checking containerd version")
	containerdVersion, err := containers.GetContainerdVersion()
	if err != nil {
		slog.Error("Cannot get volumes path", "error", err.Error())
		os.Exit(1)
	}
	slog.Info("Containerd version", "version", containerdVersion)

	volumesPath, err := volumes.GetVolumesPath()
	if err != nil {
		slog.Error("Cannot get volumes path", "error", err.Error())
		os.Exit(1)
	}
	slog.Info("Volumes ready", "path", volumesPath)

	db := db.GetDB()
	if db == nil {
		slog.Error("Failed to connect to database")
		os.Exit(1)
	}

	dns.Initialize()
	if cfg.Daemon.ProxyEnabled {
		proxy.Initialize()
	} else {
		slog.Info("Proxy is disabled, skipping initialization")
	}
	containers.RestoreContainers(db)

	healthPublisher := health.GetHealthPublisher()
	err = healthPublisher.Start()
	if err != nil {
		slog.Error("Failed to start health publisher", "error", err.Error())
		os.Exit(1)
	}

	mdnsService := mdns.GetMDNSService()
	err = mdnsService.Start()
	if err != nil {
		slog.Error("Failed to start mDNS service", "error", err.Error())
		os.Exit(1)
	}

	router := mux.NewRouter()
	createRoutes(router)

	go func() {
		serverAddr := fmt.Sprintf("%s:%d", cfg.Daemon.ListenAddress, cfg.Daemon.ListenPort)
		auth.Initialize()
		slog.Info("Daemon running", "addr", serverAddr)
		http.ListenAndServe(serverAddr, router)
	}()

	err = healthPublisher.PublishHealthy()
	if err != nil {
		slog.Error("Failed to publish healthy status", "error", err.Error())
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	slog.Info("Received shutdown signal, gracefully stopping daemon")
	mdnsService.Stop()
	healthPublisher.Stop()
	db.Close()
}
