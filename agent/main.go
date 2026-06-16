package agent

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"github.com/bitomia/realm/agent/artifacts"
	"github.com/bitomia/realm/agent/auth"
	"github.com/bitomia/realm/agent/capabilities"
	"github.com/bitomia/realm/agent/cloudinit"
	agentConfig "github.com/bitomia/realm/agent/config"
	"github.com/bitomia/realm/agent/db"
	"github.com/bitomia/realm/agent/dns"
	"github.com/bitomia/realm/agent/health"
	"github.com/bitomia/realm/agent/id"
	"github.com/bitomia/realm/agent/mdns"
	"github.com/bitomia/realm/agent/proxy"
	"github.com/bitomia/realm/common/config"
)

var (
	globalSignalChannel = make(chan os.Signal, 1)
)

func Start(cfg *config.Config, purgeDB bool, onReady func()) {
	agentConfig.Set(cfg)

	// Configure slog handler based on log format
	var handler slog.Handler
	switch cfg.Agent.LogFormat {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, nil)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, nil)
	default:
		slog.Warn("Invalid log format, defaulting to text", "format", cfg.Agent.LogFormat)
		handler = slog.NewTextHandler(os.Stdout, nil)
	}
	slog.SetDefault(slog.New(handler))

	agentId, err := id.GetAgentId()
	if err != nil {
		slog.Error("Error getting agent ID", "error", err)
		os.Exit(1)
	}

	slog.Info("Initializing agent", "version", config.GetVersion(), "id", agentId)
	slog.Debug("Agent configuration", "config", *cfg)

	capabilities.Initialize(cfg)
	caps := capabilities.Get()
	caps.Print()

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
	if cfg.Agent.ProxyEnabled {
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

	auth.Initialize()

	router := mux.NewRouter()
	createBaseRoutes(router)

	if err := artifacts.Initialize(&cfg.Agent.Artifacts, router); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	if err := cloudinit.Initialize(router); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	serverAddr := fmt.Sprintf("%s:%d", cfg.Agent.ListenAddress, cfg.Agent.ListenPort)
	listener, err := net.Listen("tcp", serverAddr)
	if err != nil {
		slog.Error("Failed to listen", "addr", serverAddr, "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Handler: router,
	}

	go func() {
		slog.Info("Agent running", "addr", serverAddr)
		_ = server.Serve(listener)
		slog.Info("HTTP server stopped", "addr", serverAddr)
	}()

	if onReady != nil {
		onReady()
	}

	err = healthPublisher.PublishHealthy()
	if err != nil {
		slog.Error("Failed to publish healthy status", "error", err.Error())
	}

	signal.Notify(globalSignalChannel, syscall.SIGINT, syscall.SIGTERM)
	<-globalSignalChannel

	slog.Info("Received shutdown signal, gracefully stopping agent")

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
