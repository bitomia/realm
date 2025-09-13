//go:build linux
// +build linux

package daemon

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"

	"github.com/bitomia/realm/daemon/auth"
	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/daemon/dns"
	"github.com/bitomia/realm/daemon/health"
	"github.com/bitomia/realm/daemon/mdns"
	"github.com/bitomia/realm/daemon/proxy"
	"github.com/bitomia/realm/daemon/volumes"
	"github.com/bitomia/realm/internal/config"
)

func Start() {
	slog.Info("Initializing daemon", "version", config.GetVersion())

	cfg := config.Get()
	if err := config.GetError(); err != nil {
		slog.Error("Error loading config", "error", err)
		os.Exit(1)
	}
	slog.Debug("Daemon configuration", "config", *cfg)

	volumesPath, err := volumes.GetVolumesPath()
	if err != nil {
		slog.Error("Cannot get volumes path", "error", err.Error())
		os.Exit(1)
	} else {
		slog.Info("Volumes ready", "path", volumesPath)
	}

	db := db.GetDB()
	if db == nil {
		slog.Error("Failed to connect to database")
		os.Exit(1)
	}

	dns.Initialize()
	proxy.Initialize()
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
