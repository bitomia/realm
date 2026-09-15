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
	"path/filepath"
	"strings"
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
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/common/logging"
)

var (
	globalSignalChannel = make(chan os.Signal, 1)
)

func Start(cfg *config.Config, purgeDB bool, onReady func()) {
	agentConfig.Set(cfg)

	// Configure slog handler based on log level and format. The LOG_LEVEL
	// environment variable, when set, takes precedence over the config file.
	levelName := cfg.Agent.LogLevel
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		levelName = envLevel
	}
	logLevel := slog.LevelInfo // default log level
	invalidLevel := false
	switch strings.ToLower(levelName) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info", "":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		invalidLevel = true
	}
	logOptions := slog.HandlerOptions{
		Level: logLevel,
	}
	var handler slog.Handler
	invalidFormat := false
	switch cfg.Agent.LogFormat {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, &logOptions)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, &logOptions)
	default:
		invalidFormat = true
		handler = slog.NewTextHandler(os.Stdout, &logOptions)
	}
	slog.SetDefault(slog.New(handler))

	// Warn through the configured handler, not the one it replaced.
	if invalidLevel {
		slog.Warn("Invalid log level, defaulting to info", "level", levelName)
	}
	if invalidFormat {
		slog.Warn("Invalid log format, defaulting to text", "format", cfg.Agent.LogFormat)
	}

	logging.BridgeContainerd(logLevel)

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

	common.SetNodeContextBuilder(func(nodeName string) common.NodeContext {
		return common.NodeContext{Repository: db.NodesRepository, Capabilities: capabilities.Get(), NodeName: nodeName, RunMode: common.AgentMode}
	})

	if err := dns.Initialize(); err != nil {
		slog.Error("DNS initialization failed", "error", err.Error())
		os.Exit(1)
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

	// Optionally expose the same API over a unix socket.
	socketPath := cfg.Agent.ListenSocket
	if socketPath != "" {
		socketListener, err := listenOnSocket(socketPath)
		if err != nil {
			slog.Error("Failed to listen on unix socket", "socket", socketPath, "error", err)
			os.Exit(1)
		}
		defer func() {
			if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
				slog.Error("Failed to remove unix socket", "socket", socketPath, "error", err)
			}
		}()

		go func() {
			slog.Info("Agent running on unix socket", "socket", socketPath)
			_ = server.Serve(socketListener)
			slog.Info("HTTP server stopped", "socket", socketPath)
		}()
	}

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

// listenOnSocket creates a unix socket listener at the given path, making sure
// the parent directory exists and that a stale socket left behind by a previous
// run does not prevent binding.
func listenOnSocket(socketPath string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create socket directory: %w", err)
	}

	// A socket file left over by an unclean shutdown is not connectable, so it
	// is safe to remove it before binding.
	if info, err := os.Stat(socketPath); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("%s exists and is not a socket", socketPath)
		}
		if conn, err := net.Dial("unix", socketPath); err == nil {
			conn.Close()
			return nil, fmt.Errorf("%s is already in use by another agent", socketPath)
		}
		if err := os.Remove(socketPath); err != nil {
			return nil, fmt.Errorf("failed to remove stale socket: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}

	if err := os.Chmod(socketPath, 0o660); err != nil {
		listener.Close()
		return nil, fmt.Errorf("failed to set socket permissions: %w", err)
	}

	return listener, nil
}

func Stop() {
	globalSignalChannel <- syscall.SIGINT
}
