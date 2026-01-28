package daemon

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/daemon/auth"
	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/daemon/dns"
	"github.com/bitomia/realm/daemon/health"
	"github.com/bitomia/realm/daemon/id"
	"github.com/bitomia/realm/daemon/mdns"
	"github.com/bitomia/realm/daemon/network"
	"github.com/bitomia/realm/daemon/proxy"
	"github.com/bitomia/realm/daemon/volumes"
)

var (
	globalSignalChannel = make(chan os.Signal, 1)
)

func Start(purgeDB bool) {
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

	slog.Info("Initializing volumes")
	if err := volumes.InitializeManager(cfg.Daemon.ZFS); err != nil {
		slog.Error("Volumes initialization failed", "error", err.Error())
		os.Exit(1)
	}
	volumesPath, err := volumes.GetVolumesPath()
	if err != nil {
		slog.Error("Cannot get volumes path", "error", err.Error())
		os.Exit(1)
	}
	if cfg.Daemon.ZFS {
		slog.Info("Volumes ready (ZFS)", "path", volumesPath)
	} else {
		slog.Info("Volumes ready (directory-based)", "path", volumesPath)
	}

	slog.Info("Validating CNI availability")
	if err := network.ValidateCNIAvailability(); err != nil {
		slog.Error("CNI validation failed", "error", err.Error())
		os.Exit(1)
	}
	slog.Info("CNI plugins validated successfully")

	db := db.GetDB()
	if db == nil {
		slog.Error("Failed to connect to database")
		os.Exit(1)
	}

	if purgeDB {
		slog.Warn("Purge database flag is set, purging database contents")
		if err := db.PurgeDB(); err != nil {
			slog.Error("Failed to purge database", "error", err.Error())
			os.Exit(1)
		}
	}

	if err := dns.Initialize(); err != nil {
		slog.Error("DNS initialization failed", "error", err.Error())
		os.Exit(1)
	}
	if cfg.Daemon.ProxyEnabled {
		proxy.Initialize()
	} else {
		slog.Info("Proxy is disabled, skipping initialization")
	}

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

	auth.Initialize()

	serverAddr := fmt.Sprintf("%s:%d", cfg.Daemon.ListenAddress, cfg.Daemon.ListenPort)
	server := &http.Server{
		Addr:    serverAddr,
		Handler: router,
	}

	go func() {
		slog.Info("Daemon running", "addr", serverAddr)
		server.ListenAndServe()
		slog.Info("HTTP server stopped", "addr", serverAddr)
	}()

	err = healthPublisher.PublishHealthy()
	if err != nil {
		slog.Error("Failed to publish healthy status", "error", err.Error())
	}

	signal.Notify(globalSignalChannel, syscall.SIGINT, syscall.SIGTERM)
	<-globalSignalChannel

	slog.Info("Received shutdown signal, gracefully stopping daemon")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	if err := dns.Shutdown(ctx); err != nil {
		slog.Error("DNS shutdown error", "error", err)
	}

	mdnsService.Stop()
	healthPublisher.Stop()
	db.Close()
}

func Stop() {
	globalSignalChannel <- syscall.SIGINT
}
