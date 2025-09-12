package config

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

func getExeDir() string {
	execPath, err := os.Executable()
	if err != nil {
		slog.Error(err.Error())
	}

	return filepath.Dir(execPath)
}

func setDefaults() {
	viper.SetDefault("daemon.cni_path", "/opt/cni")
	viper.SetDefault("daemon.volumes_pool", "realm_volumes")
	viper.SetDefault("daemon.listen_address", "127.0.0.1")
	viper.SetDefault("daemon.listen_port", "9000")
	viper.SetDefault("daemon.local_caddy_url", "localhost:2019")
	viper.SetDefault("daemon.master_caddy_url", "localhost:2019")
	viper.SetDefault("daemon.containerd_sock", "/run/containerd/containerd.sock")
	viper.SetDefault("daemon.containerd_namespace", "realm")
	viper.SetDefault("daemon.containers_log_path", "/var/log/realm")
	viper.SetDefault("daemon.etcd_endpoints", []string{"127.0.0.1:2379"})
}
