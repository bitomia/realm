//go:build EE

package ee

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	netplane "github.com/bitomia/netplane/bindings/gonetplane"

	"github.com/bitomia/realm/common/config"
)

func StartMesh(cfg *config.Config) error {
	errChan := make(chan error)
	go startMesh(cfg, errChan)
	return <-errChan
}

func startMesh(cfg *config.Config, errChan chan error) {
	if cfg == nil || cfg.MeshConfig == nil {
		errChan <- fmt.Errorf("nil config")
		return
	}

	logFormat := netplane.LogFormatLogfmt
	if cfg.Agent.LogFormat == "json" {
		logFormat = netplane.LogFormatJSON
	}
	netplane.InitLogger(logFormat)

	meshDir := filepath.Join(cfg.DataPath, "mesh")
	if err := os.MkdirAll(meshDir, 0o700); err != nil {
		errChan <- fmt.Errorf("create mesh dir: %w", err)
		return
	}

	publicKeyPath := filepath.Join(meshDir, "public.key")
	privateKeyPath := filepath.Join(meshDir, "private.key")
	authKeyPath := filepath.Join(meshDir, "auth.key")

	if err := netplane.TryGenerateCryptoKeys(publicKeyPath, privateKeyPath); err != nil {
		//		return fmt.Errorf("generate crypto keys: %w", err)
	}

	server := cfg.MeshConfig.ServerUrl
	host := server
	if host == "" {
		errChan <- fmt.Errorf("mesh.server is not configured")
		return
	}

	transport := string(cfg.MeshConfig.Transport)
	if transport == "" {
		errChan <- fmt.Errorf("mesh.transport is not configured")
		return
	}

	netplane.ClientAuth(authKeyPath, publicKeyPath, privateKeyPath, host, cfg.MeshConfig.LinkCode, 8000)
	token, err := netplane.Run("netplane0", host, 5000, transport, authKeyPath, publicKeyPath, privateKeyPath)
	if err != nil {
		errChan <- fmt.Errorf("netplane run: %w", err)
		return
	}
	defer token.Free()
	errChan <- nil
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	token.Cancel()
	netplane.Stop()
}
